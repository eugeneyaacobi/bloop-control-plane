package models

import "time"

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type Account struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type Membership struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	AccountID string `json:"accountId"`
	Role      string `json:"role"`
}

type Tunnel struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Hostname  string    `json:"hostname"`
	Target    string    `json:"target"`
	Access    string    `json:"access"`
	Status    string    `json:"status"`
	Region    string    `json:"region,omitempty"`
	Owner     string    `json:"owner,omitempty"`
	Risk      string    `json:"risk,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ReviewFlag struct {
	ID       string `json:"id"`
	Item     string `json:"item"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
}

type SignupRequest struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	State string `json:"state"`
}

type RuntimeInstallation struct {
	ID          string     `json:"id"`
	AccountID   string     `json:"accountId"`
	Name        string     `json:"name"`
	Environment string     `json:"environment,omitempty"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	LastSeenAt  *time.Time `json:"lastSeenAt,omitempty"`
}

type RuntimeInstallationToken struct {
	ID             string     `json:"id"`
	InstallationID string     `json:"installationId"`
	Kind           string     `json:"kind"`
	TokenHash      string     `json:"-"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	RevokedAt      *time.Time `json:"revokedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	LastUsedAt     *time.Time `json:"lastUsedAt,omitempty"`
}

type RuntimeTunnelBinding struct {
	ID                string    `json:"id"`
	AccountID         string    `json:"accountId"`
	InstallationID    string    `json:"installationId"`
	TunnelID          string    `json:"tunnelId"`
	RuntimeTunnelName string    `json:"runtimeTunnelName"`
	Hostname          string    `json:"hostname"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// Tunnel management request/response types

type TunnelCreateRequest struct {
	ID        string `json:"id"`
	Hostname  string `json:"hostname"`
	Target    string `json:"target"`
	Access    string `json:"access"`
	BasicAuth *struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"basic_auth,omitempty"`
	TokenEnv string `json:"token_env,omitempty"`
}

type TunnelUpdateRequest struct {
	Hostname  *string `json:"hostname,omitempty"`
	Target    *string `json:"target,omitempty"`
	Access    *string `json:"access,omitempty"`
	BasicAuth *struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"basic_auth,omitempty"`
	TokenEnv *string `json:"token_env,omitempty"`
}

type TunnelValidationRequest struct {
	Hostname string `json:"hostname"`
	Target   string `json:"target"`
	Access   string `json:"access"`
	LocalPort *int  `json:"local_port,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type TunnelValidationResponse struct {
	Valid  bool         `json:"valid"`
	Errors []FieldError `json:"errors,omitempty"`
}

type TunnelStatusResponse struct {
	Status     string  `json:"status"`
	ObservedAt *string `json:"observed_at,omitempty"`
	Degraded   bool    `json:"degraded"`
	Stale      bool    `json:"stale"`
}

type AccessModeInfo struct {
	Mode        string `json:"mode"`
	Description string `json:"description"`
	Requires    []string `json:"requires,omitempty"`
}

type HostnamePattern struct {
	Pattern        string `json:"pattern"`
	Description    string `json:"description"`
	Example        string `json:"example"`
}

type ConfigSchemaResponse struct {
	AccessModes      []AccessModeInfo   `json:"access_modes"`
	HostnamePatterns []HostnamePattern `json:"hostname_patterns"`
	DefaultPorts     []int              `json:"default_ports"`
	DefaultRelayURL  string             `json:"default_relay_url"`
}

type EnrollmentVerifyRequest struct {
	Token string `json:"token"`
}

type EnrollmentVerifyResponse struct {
	Valid        bool   `json:"valid"`
	InstallationID string `json:"installation_id,omitempty"`
	IngestToken    string `json:"ingest_token,omitempty"`
	Error          string `json:"error,omitempty"`
}

// Auth, Token Management & WebAuthn 2FA Types

// RegistrationRequest represents a user registration request
type RegistrationRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	User              UserContext `json:"user"`
	RequiresWebAuthn  bool        `json:"requires_webauthn"`
	WebAuthnChallenge *string     `json:"webauthn_challenge,omitempty"`
}

// UserContext represents user information in responses
type UserContext struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Username    *string `json:"username,omitempty"`
	DisplayName string `json:"display_name"`
	AccountID   string `json:"account_id"`
	Role        string `json:"role"`
}

// RefreshRequest represents a session refresh request
type RefreshRequest struct {
	// No body needed - uses existing session cookie
}

// RefreshResponse represents a session refresh response
type RefreshResponse struct {
	User UserContext `json:"user"`
}

// AuthError represents an authentication error response
type AuthError struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// API Token Types

// TokenCreateRequest represents a token creation request
type TokenCreateRequest struct {
	Name      string  `json:"name"`
	AccountID string  `json:"account_id"`
	ExpiresIn *string `json:"expires_in,omitempty"` // e.g., "720h"
}

// TokenCreateResponse represents the response after creating a token
// The token value is only shown once
type TokenCreateResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Token       string     `json:"token"` // Only shown once at creation
	TokenPrefix string     `json:"token_prefix"`
	AccountID   string     `json:"account_id"`
	ExpiresAt   *string    `json:"expires_at,omitempty"`
	CreatedAt   string     `json:"created_at"`
}

// TokenListResponse represents a list of tokens
// Token values are NEVER included in listings
type TokenListResponse struct {
	Tokens []TokenSummary `json:"tokens"`
}

// TokenSummary represents a token in listings (without the actual token value)
type TokenSummary struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	AccountID   string     `json:"account_id"`
	ExpiresAt   *string    `json:"expires_at,omitempty"`
	RevokedAt   *string    `json:"revoked_at,omitempty"`
	LastUsedAt  *string    `json:"last_used_at,omitempty"`
	CreatedAt   string     `json:"created_at"`
}

// TokenRefreshResponse represents the response after refreshing a token
type TokenRefreshResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Token       string  `json:"token"` // New token value, shown once
	TokenPrefix string  `json:"token_prefix"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

// WebAuthn Types

// WebAuthnBeginRegistrationResponse represents the beginning of WebAuthn registration
type WebAuthnBeginRegistrationResponse struct {
	PublicKeyCredentialCreationOptions any `json:"publicKeyCredentialCreationOptions"`
}

// WebAuthnFinishRegistrationRequest represents the completion of WebAuthn registration
type WebAuthnFinishRegistrationRequest struct {
	Credential any `json:"credential"`
}

// WebAuthnFinishRegistrationResponse represents a completed WebAuthn registration
type WebAuthnFinishRegistrationResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	AddedAt     string    `json:"added_at"`
}

// WebAuthnBeginLoginRequest represents the beginning of WebAuthn login
type WebAuthnBeginLoginRequest struct {
	Email string `json:"email"`
}

// WebAuthnBeginLoginResponse represents the response for WebAuthn login begin
type WebAuthnBeginLoginResponse struct {
	PublicKeyCredentialRequestOptions any `json:"publicKeyCredentialRequestOptions"`
}

// WebAuthnFinishLoginRequest represents the completion of WebAuthn login
type WebAuthnFinishLoginRequest struct {
	Email      string `json:"email"`
	Credential any    `json:"credential"`
}

// WebAuthnCredentialListResponse represents a list of WebAuthn credentials
type WebAuthnCredentialListResponse struct {
	Credentials []WebAuthnCredentialSummary `json:"credentials"`
}

// WebAuthnCredentialSummary represents a WebAuthn credential in listings
type WebAuthnCredentialSummary struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	AddedAt     string     `json:"added_at"`
	LastUsedAt  *string    `json:"last_used_at,omitempty"`
}

// WebAuthnEnabledResponse represents the WebAuthn enabled state
type WebAuthnEnabledResponse struct {
	Enabled bool `json:"enabled"`
}
