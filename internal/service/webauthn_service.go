package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/security"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

// WebAuthnService handles WebAuthn 2FA operations
type WebAuthnService struct {
	authRepo    repository.AuthRepository
	webauthnRepo repository.WebAuthnRepository
	auditRepo   repository.AuditRepository
	config      *config.Config
	webAuthn    *webauthn.WebAuthn
}

// NewWebAuthnService creates a new WebAuthn service
func NewWebAuthnService(
	authRepo repository.AuthRepository,
	webauthnRepo repository.WebAuthnRepository,
	auditRepo repository.AuditRepository,
	config *config.Config,
) (*WebAuthnService, error) {
	// Initialize WebAuthn
	wconf := security.WebAuthnConfig{
		RPID:      config.WebAuthnRPID,
		RPName:    config.WebAuthnRPName,
		RPOrigins: config.WebAuthnOrigins,
		Timeout:   60000, // 60 seconds
	}

	wa, err := security.NewWebAuthn(wconf)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize WebAuthn: %w", err)
	}

	return &WebAuthnService{
		authRepo:    authRepo,
		webauthnRepo: webauthnRepo,
		auditRepo:   auditRepo,
		config:      config,
		webAuthn:    wa,
	}, nil
}

// BeginRegistrationResult holds the result of beginning WebAuthn registration
type BeginRegistrationResult struct {
	CreationOptions any    // webauthn.ProtocolOptions
	ChallengeID     string // ID to retrieve the challenge later
}

// BeginRegistration begins the WebAuthn registration ceremony
func (s *WebAuthnService) BeginRegistration(ctx context.Context, userID string, ipAddress, userAgent string) (*BeginRegistrationResult, error) {
	// Get user
	user, err := s.authRepo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Load existing credentials
	creds, err := s.webauthnRepo.ListCredentialsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}

	// Convert to WebAuthn library format
	webauthnCreds := make([]webauthn.Credential, len(creds))
	for i, cred := range creds {
		webauthnCreds[i] = security.CredentialFromModel(
			cred.CredentialID,
			cred.PublicKey,
			cred.SignCount,
			cred.AttestationType,
			cred.Transports,
		)
	}

	// Create WebAuthn user
	wuser := security.WebAuthnUser{
		ID:          user.ID,
		DisplayName: func() string { if user.Username != nil { return *user.Username } else { return user.Email } }(),
		Credentials: webauthnCreds,
	}

	// Generate registration options
	options, sessionData, err := s.webAuthn.BeginRegistration(wuser)
	if err != nil {
		return nil, fmt.Errorf("failed to begin registration: %w", err)
	}

	// Store challenge
	challenge := repository.WebAuthnChallenge{
		ID:        uuid.New().String(),
		UserID:    userID,
		Challenge: []byte(sessionData.Challenge), // Convert string to []byte
		Kind:      "registration",
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
		CreatedAt: time.Now().UTC(),
	}

	err = s.webauthnRepo.CreateChallenge(ctx, challenge)
	if err != nil {
		return nil, fmt.Errorf("failed to store challenge: %w", err)
	}

	// Log audit event
	var userIDPtr = &userID
	_ = s.auditRepo.LogAuthEvent(ctx, repository.AuthAuditEvent{
		UserID:    userIDPtr,
		Event:     "webauthn_registration_begin",
		IPAddress: &ipAddress,
		UserAgent: &userAgent,
		Success:   true,
	})

	return &BeginRegistrationResult{
		CreationOptions: options,
		ChallengeID:     challenge.ID,
	}, nil
}

// FinishRegistrationResult holds the result of finishing WebAuthn registration
type FinishRegistrationResult struct {
	Credential models.WebAuthnFinishRegistrationResponse
}

// FinishRegistration completes the WebAuthn registration ceremony
func (s *WebAuthnService) FinishRegistration(ctx context.Context, userID, challengeID string, response string, credentialName string, ipAddress, userAgent string) (*FinishRegistrationResult, error) {
	// Get challenge
	challenge, err := s.webauthnRepo.GetChallenge(ctx, challengeID)
	if err != nil || challenge == nil {
		return nil, fmt.Errorf("challenge not found or expired")
	}

	// Verify challenge ownership
	if challenge.UserID != userID {
		return nil, fmt.Errorf("challenge does not belong to user")
	}

	// Verify not expired
	if challenge.ExpiresAt.Before(time.Now().UTC()) {
		// Clean up expired challenge
		_ = s.webauthnRepo.DeleteChallenge(ctx, challengeID)
		return nil, fmt.Errorf("challenge expired")
	}

	// Get user for WebAuthn library
	user, err := s.authRepo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Load existing credentials
	creds, _ := s.webauthnRepo.ListCredentialsByUser(ctx, userID)
	webauthnCreds := make([]webauthn.Credential, len(creds))
	for i, cred := range creds {
		webauthnCreds[i] = security.CredentialFromModel(
			cred.CredentialID,
			cred.PublicKey,
			cred.SignCount,
			cred.AttestationType,
			cred.Transports,
		)
	}

	// Create WebAuthn user
	wuser := security.WebAuthnUser{
		ID:          user.ID,
		DisplayName: func() string { if user.Username != nil { return *user.Username } else { return user.Email } }(),
		Credentials: webauthnCreds,
	}

	// Reconstruct session data from stored challenge
	sessionData := webauthn.SessionData{
		Challenge: string(challenge.Challenge),
	}

	// Parse the credential response from JSON
	var credentialResponse protocol.ParsedCredentialCreationData
	if err := json.Unmarshal([]byte(response), &credentialResponse); err != nil {
		return nil, fmt.Errorf("invalid credential response: %w", err)
	}

	// Finish registration using the WebAuthn library
	credential, err := s.webAuthn.CreateCredential(wuser, sessionData, &credentialResponse)
	if err != nil {
		return nil, fmt.Errorf("credential verification failed: %w", err)
	}

	// Store the verified credential
	credID := uuid.New().String()

	// Extract transports from the credential
	transports := make([]string, len(credential.Transport))
	for i, t := range credential.Transport {
		transports[i] = string(t)
	}

	newCred := repository.WebAuthnCredential{
		ID:              credID,
		UserID:          userID,
		CredentialID:    credential.ID,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		AAGUID:          credential.Authenticator.AAGUID,
		SignCount:       int64(credential.Authenticator.SignCount),
		Name:            credentialName,
		Transports:      transports,
		CreatedAt:       time.Now().UTC(),
	}

	err = s.webauthnRepo.StoreCredential(ctx, newCred)
	if err != nil {
		return nil, fmt.Errorf("failed to store credential: %w", err)
	}

	// Enable WebAuthn for user
	err = s.authRepo.SetWebAuthnEnabled(ctx, userID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to enable WebAuthn: %w", err)
	}

	// Clean up challenge
	_ = s.webauthnRepo.DeleteChallenge(ctx, challengeID)

	// Log audit event
	var userIDPtr = &userID
	_ = s.auditRepo.LogAuthEvent(ctx, repository.AuthAuditEvent{
		UserID:    userIDPtr,
		Event:     "webauthn_registration_complete",
		IPAddress: &ipAddress,
		UserAgent: &userAgent,
		Success:   true,
		Metadata:  map[string]interface{}{"credential_id": credID, "credential_name": credentialName},
	})

	return &FinishRegistrationResult{
		Credential: models.WebAuthnFinishRegistrationResponse{
			ID:      credID,
			Name:    credentialName,
			AddedAt: newCred.CreatedAt.Format(time.RFC3339),
		},
	}, nil
}

// BeginLoginResult holds the result of beginning WebAuthn login
type BeginLoginResult struct {
	RequestOptions any    // webauthn.ProtocolOptions
	ChallengeID    string // ID to retrieve the challenge later
}

// BeginLogin begins the WebAuthn login ceremony
func (s *WebAuthnService) BeginLogin(ctx context.Context, email string, ipAddress, userAgent string) (*BeginLoginResult, error) {
	// Get user by email
	user, err := s.authRepo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		// Don't reveal whether user exists
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if WebAuthn is enabled
	if !user.WebAuthnEnabled {
		return nil, fmt.Errorf("WebAuthn not enabled for user")
	}

	// Load credentials
	creds, err := s.webauthnRepo.ListCredentialsByUser(ctx, user.ID)
	if err != nil || len(creds) == 0 {
		return nil, fmt.Errorf("no credentials found")
	}

	// Convert to WebAuthn library format
	webauthnCreds := make([]webauthn.Credential, len(creds))
	for i, cred := range creds {
		webauthnCreds[i] = security.CredentialFromModel(
			cred.CredentialID,
			cred.PublicKey,
			cred.SignCount,
			cred.AttestationType,
			cred.Transports,
		)
	}

	// Create WebAuthn user
	wuser := security.WebAuthnUser{
		ID:          user.ID,
		DisplayName: func() string { if user.Username != nil { return *user.Username } else { return user.Email } }(),
		Credentials: webauthnCreds,
	}

	// Generate authentication options
	options, sessionData, err := s.webAuthn.BeginLogin(wuser)
	if err != nil {
		return nil, fmt.Errorf("failed to begin login: %w", err)
	}

	// Store challenge
	challenge := repository.WebAuthnChallenge{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Challenge: []byte(sessionData.Challenge), // Convert string to []byte
		Kind:      "authentication",
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
		CreatedAt: time.Now().UTC(),
	}

	err = s.webauthnRepo.CreateChallenge(ctx, challenge)
	if err != nil {
		return nil, fmt.Errorf("failed to store challenge: %w", err)
	}

	// Log audit event
	var userIDPtr = &user.ID
	_ = s.auditRepo.LogAuthEvent(ctx, repository.AuthAuditEvent{
		UserID:    userIDPtr,
		Event:     "webauthn_login_begin",
		IPAddress: &ipAddress,
		UserAgent: &userAgent,
		Success:   true,
	})

	return &BeginLoginResult{
		RequestOptions: options,
		ChallengeID:    challenge.ID,
	}, nil
}

// FinishLogin completes the WebAuthn login ceremony
func (s *WebAuthnService) FinishLogin(ctx context.Context, email, challengeID string, response string, ipAddress, userAgent string) (string, error) {
	// Get challenge
	challenge, err := s.webauthnRepo.GetChallenge(ctx, challengeID)
	if err != nil || challenge == nil {
		return "", fmt.Errorf("challenge not found or expired")
	}

	// Verify not expired
	if challenge.ExpiresAt.Before(time.Now().UTC()) {
		_ = s.webauthnRepo.DeleteChallenge(ctx, challengeID)
		return "", fmt.Errorf("challenge expired")
	}

	// Get user
	user, err := s.authRepo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return "", fmt.Errorf("user not found")
	}

	// Load credentials
	creds, err := s.webauthnRepo.ListCredentialsByUser(ctx, user.ID)
	if err != nil || len(creds) == 0 {
		return "", fmt.Errorf("no credentials found")
	}

	webauthnCreds := make([]webauthn.Credential, len(creds))
	for i, cred := range creds {
		webauthnCreds[i] = security.CredentialFromModel(
			cred.CredentialID,
			cred.PublicKey,
			cred.SignCount,
			cred.AttestationType,
			cred.Transports,
		)
	}

	wuser := security.WebAuthnUser{
		ID:          user.ID,
		DisplayName: func() string { if user.Username != nil { return *user.Username } else { return user.Email } }(),
		Credentials: webauthnCreds,
	}

	// Reconstruct session data from stored challenge
	sessionData := webauthn.SessionData{
		Challenge: string(challenge.Challenge),
	}

	// Parse the credential assertion response from JSON
	var assertionResponse protocol.ParsedCredentialAssertionData
	if err := json.Unmarshal([]byte(response), &assertionResponse); err != nil {
		return "", fmt.Errorf("invalid assertion response: %w", err)
	}

	// Validate the assertion using the WebAuthn library
	credential, err := s.webAuthn.ValidateLogin(wuser, sessionData, &assertionResponse)
	if err != nil {
		return "", fmt.Errorf("authentication verification failed: %w", err)
	}

	// Clean up challenge
	_ = s.webauthnRepo.DeleteChallenge(ctx, challengeID)

	// Update sign count from the verified credential
	credIDStr := base64.RawURLEncoding.EncodeToString(credential.ID)
	_ = s.webauthnRepo.UpdateSignCount(ctx, credIDStr, int64(credential.Authenticator.SignCount))

	// Log audit event
	var userIDPtr = &user.ID
	_ = s.auditRepo.LogAuthEvent(ctx, repository.AuthAuditEvent{
		UserID:    userIDPtr,
		Event:     "webauthn_login_complete",
		IPAddress: &ipAddress,
		UserAgent: &userAgent,
		Success:   true,
	})

	return user.ID, nil
}

// ListCredentials lists a user's WebAuthn credentials
func (s *WebAuthnService) ListCredentials(ctx context.Context, userID string) ([]models.WebAuthnCredentialSummary, error) {
	creds, err := s.webauthnRepo.ListCredentialsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	result := make([]models.WebAuthnCredentialSummary, len(creds))
	for i, cred := range creds {
		result[i] = models.WebAuthnCredentialSummary{
			ID:         cred.ID,
			Name:       cred.Name,
			AddedAt:    cred.CreatedAt.Format(time.RFC3339),
			LastUsedAt: formatTimePtr(cred.LastUsedAt),
		}
	}

	return result, nil
}

// DeleteCredential deletes a WebAuthn credential
func (s *WebAuthnService) DeleteCredential(ctx context.Context, userID, credentialID string, ipAddress, userAgent string) error {
	// Check ownership
	cred, err := s.webauthnRepo.GetCredentialByID(ctx, credentialID)
	if err != nil || cred == nil {
		return &NotFoundError{Resource: "credential", ID: credentialID}
	}
	if cred.UserID != userID {
		return fmt.Errorf("credential does not belong to user")
	}

	// Delete credential
	err = s.webauthnRepo.DeleteCredential(ctx, credentialID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	// Check if this was the last credential
	remainingCreds, _ := s.webauthnRepo.ListCredentialsByUser(ctx, userID)
	if len(remainingCreds) == 0 {
		// Disable WebAuthn for user
		_ = s.authRepo.SetWebAuthnEnabled(ctx, userID, false)
	}

	// Log audit event
	// Log audit event
	var userIDPtr = &userID
	_ = s.auditRepo.LogAuthEvent(ctx, repository.AuthAuditEvent{
		UserID:    userIDPtr,
		Event:     "webauthn_credential_deleted",
		IPAddress: &ipAddress,
		UserAgent: &userAgent,
		Success:   true,
		Metadata:  map[string]interface{}{"credential_id": credentialID, "credential_name": cred.Name},
	})

	return nil
}
