package security

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnConfig holds configuration for WebAuthn
type WebAuthnConfig struct {
	RPID          string   // Relying Party ID (e.g., "bloop.to")
	RPName        string   // Relying Party Name (e.g., "Bloop")
	RPOrigins     []string // Allowed origins (e.g., ["https://bloop.to"])
	Timeout       int      // Timeout in milliseconds (default 60000)
}

// WebAuthnUser wraps our user model to implement webauthn.User interface
type WebAuthnUser struct {
	ID          string
	DisplayName string
	Credentials []webauthn.Credential
}

// WebAuthnCredential wraps our credential model for the library interface
type WebAuthnCredential struct {
	ID              []byte
	PublicKey       []byte
	AttestationType string
	Transport       []string
	Flags           webauthn.CredentialFlags
	Authenticator   webauthn.Authenticator
}

// webauthn.User interface implementation

// WebAuthnID returns the user's WebAuthn ID (must be stable)
func (u WebAuthnUser) WebAuthnID() []byte {
	return []byte(u.ID)
}

// WebAuthnName returns the user's username
func (u WebAuthnUser) WebAuthnName() string {
	return u.DisplayName
}

// WebAuthnDisplayName returns the user's display name
func (u WebAuthnUser) WebAuthnDisplayName() string {
	return u.DisplayName
}

// WebAuthnCredentials returns the user's credentials
func (u WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

// WebAuthnIcon returns the user's icon URL (optional)
func (u WebAuthnUser) WebAuthnIcon() string {
	return ""
}

// NewWebAuthn creates a new WebAuthn instance with the given config
func NewWebAuthn(config WebAuthnConfig) (*webauthn.WebAuthn, error) {
	if config.RPID == "" {
		return nil, fmt.Errorf("RPID is required")
	}
	if config.RPName == "" {
		return nil, fmt.Errorf("RPName is required")
	}
	if len(config.RPOrigins) == 0 {
		return nil, fmt.Errorf("at least one RPOrigin is required")
	}
	if config.Timeout == 0 {
		config.Timeout = 60000 // 60 seconds default
	}

	wconf := webauthn.Config{
		RPDisplayName: config.RPName,
		RPID:          config.RPID,
		RPOrigins:     config.RPOrigins,
	}

	return webauthn.New(&wconf)
}

// GenerateChallenge generates a random WebAuthn challenge
func GenerateChallenge() ([]byte, error) {
	challenge := make([]byte, 32)
	_, err := rand.Read(challenge)
	if err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}
	return challenge, nil
}

// EncodeChallenge encodes a challenge to base64url for transmission
func EncodeChallenge(challenge []byte) string {
	return base64.RawURLEncoding.EncodeToString(challenge)
}

// DecodeChallenge decodes a base64url challenge
func DecodeChallenge(encoded string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(encoded)
}

// CredentialFromModel converts our repository credential model to a webauthn.Credential
func CredentialFromModel(credID []byte, publicKey []byte, signCount int64, attestationType string, transports []string) webauthn.Credential {
	// Convert []string to []protocol.AuthenticatorTransport
	t := make([]protocol.AuthenticatorTransport, len(transports))
	for i, tr := range transports {
		t[i] = protocol.AuthenticatorTransport(tr)
	}

	return webauthn.Credential{
		ID:              credID,
		PublicKey:       publicKey,
		AttestationType: attestationType,
		Transport:       t,
		Flags: webauthn.CredentialFlags{
			UserPresent:    true,
			UserVerified:   true,
			BackupEligible: false,
			BackupState:    false,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:    []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			SignCount: uint32(signCount),
		},
	}
}

// FormatCredentialID formats a credential ID for display/logging
func FormatCredentialID(credID []byte) string {
	return base64.RawURLEncoding.EncodeToString(credID)
}
