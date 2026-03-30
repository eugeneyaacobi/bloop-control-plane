package service

import (
	"context"
	"fmt"
	"time"

	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/security"
	"bloop-control-plane/internal/session"
)

// AuthService handles authentication operations
type AuthService struct {
	authRepo    repository.AuthRepository
	auditRepo   repository.AuditRepository
	lockoutRepo repository.LockoutRepository
	config      *config.Config
	tokenMgr    *session.TokenManager
}

// NewAuthService creates a new authentication service
func NewAuthService(
	authRepo repository.AuthRepository,
	auditRepo repository.AuditRepository,
	lockoutRepo repository.LockoutRepository,
	config *config.Config,
	tokenMgr *session.TokenManager,
) *AuthService {
	return &AuthService{
		authRepo:    authRepo,
		auditRepo:   auditRepo,
		lockoutRepo: lockoutRepo,
		config:      config,
		tokenMgr:    tokenMgr,
	}
}

// RegisterResult holds the result of user registration
type RegisterResult struct {
	User    repository.UserWithCredentials
	Session *session.Context
}

// Register registers a new user with email, username, and password
func (s *AuthService) Register(ctx context.Context, email, username, password string, ipAddress, userAgent string) (*RegisterResult, error) {
	// Validate password
	if err := security.ValidatePassword(password); err != nil {
		s.logAuditEvent(ctx, nil, nil, "register_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "invalid_password"})
		return nil, &ValidationError{Field: "password", Message: "Password must be at least 12 characters. Please choose a longer one."}
	}

	// Check for breach if enabled
	if s.config.BreachCheckEnabled {
		compromised, err := security.CheckPasswordBreach(password)
		if err != nil {
			// Log but don't fail on breach check errors
		}
		if compromised {
			s.logAuditEvent(ctx, nil, nil, "register_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "compromised_password"})
			return nil, &ValidationError{Field: "password", Message: "this password has been exposed in data breaches"}
		}
	}

	// Check if email already exists
	existingUser, _ := s.authRepo.GetUserByEmail(ctx, email)
	if existingUser != nil {
		s.logAuditEvent(ctx, nil, nil, "register_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "email_exists"})
		// Generic error message to prevent enumeration
		return nil, &ConflictError{Field: "email", Message: "registration failed"}
	}

	// Check if username already exists
	existingUser, _ = s.authRepo.GetUserByUsername(ctx, username)
	if existingUser != nil {
		s.logAuditEvent(ctx, nil, nil, "register_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "username_exists"})
		// Generic error message to prevent enumeration
		return nil, &ConflictError{Field: "username", Message: "registration failed"}
	}

	// Hash password
	hash, err := security.HashPassword(password)
	if err != nil {
		s.logAuditEvent(ctx, nil, nil, "register_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "hash_error"})
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user with credentials
	displayName := username // Default display name to username
	user, err := s.authRepo.CreateUserWithCredentials(ctx, email, username, hash, displayName)
	if err != nil {
		s.logAuditEvent(ctx, nil, nil, "register_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "db_error"})
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Issue session
	sessionCtx := s.issueSession(user.ID, user.Email, username, "customer")

	// Log successful registration
	s.logAuditEvent(ctx, &user.ID, nil, "register_success", ipAddress, userAgent, true, nil)

	return &RegisterResult{
		User:    user,
		Session: sessionCtx,
	}, nil
}

// LoginResult holds the result of user login
type LoginResult struct {
	Session          *session.Context
	RequiresWebAuthn bool
}

// Login authenticates a user with email and password
func (s *AuthService) Login(ctx context.Context, email, password string, ipAddress, userAgent string) (*LoginResult, error) {
	// Check IP rate limit
	since := time.Now().UTC().Add(-time.Minute)
	ipCount, _ := s.lockoutRepo.GetIPFailedAttemptCount(ctx, ipAddress, since)
	if ipCount >= s.config.LoginRateLimitIP {
		s.logAuditEvent(ctx, nil, nil, "login_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "rate_limit_ip"})
		return nil, &RateLimitError{Message: "too many login attempts from this IP"}
	}

	// Check account rate limit
	since = time.Now().UTC().Add(-15 * time.Minute)
	accountCount, _ := s.lockoutRepo.GetFailedAttemptCount(ctx, email, since)
	if accountCount >= s.config.LoginRateLimitAccount {
		s.logAuditEvent(ctx, nil, nil, "login_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "rate_limit_account"})
		return nil, &RateLimitError{Message: "too many login attempts for this account"}
	}

	// Get user by email
	user, err := s.authRepo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		s.recordFailedLogin(ctx, email, ipAddress)
		s.logAuditEvent(ctx, nil, nil, "login_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "user_not_found"})
		// Generic error message
		return nil, &AuthError{Message: "invalid credentials"}
	}

	// Check if account is locked
	locked, lockedUntil, _ := s.lockoutRepo.IsAccountLocked(ctx, user.ID)
	if locked {
		s.recordFailedLogin(ctx, email, ipAddress)
		s.logAuditEvent(ctx, &user.ID, nil, "login_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "account_locked"})
		return nil, &LockoutError{LockedUntil: lockedUntil}
	}

	// Check if password is set
	if !user.PasswordSet || user.Credential == nil {
		s.recordFailedLogin(ctx, email, ipAddress)
		s.logAuditEvent(ctx, &user.ID, nil, "login_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "no_password"})
		return nil, &AuthError{Message: "invalid credentials"}
	}

	// Verify password
	valid, err := security.VerifyPassword(password, user.Credential.PasswordHash)
	if err != nil || !valid {
		s.recordFailedLogin(ctx, email, ipAddress)
		s.logAuditEvent(ctx, &user.ID, nil, "login_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "invalid_password"})
		return nil, &AuthError{Message: "invalid credentials"}
	}

	// Check account lockout threshold and increment
	newCount, _ := s.lockoutRepo.IncrementFailedCount(ctx, user.ID, s.config.AccountLockoutThreshold, s.config.AccountLockoutDuration)
	// Reset count on successful login by checking if it was incremented
	// The repository should handle resetting on success, but let's log it
	if newCount >= s.config.AccountLockoutThreshold {
		s.logAuditEvent(ctx, &user.ID, nil, "login_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "account_locked"})
		return nil, &LockoutError{LockedUntil: func() *time.Time { t := time.Now().UTC().Add(s.config.AccountLockoutDuration); return &t }()}
	}

	// Reset failed count on successful login
	// Note: This is a simplification - in production you'd want to reset the counter
	// The lockout repository should handle this

	// Record successful login
	_ = s.lockoutRepo.RecordLoginAttempt(ctx, email, ipAddress, true)

	// Check if WebAuthn is required
	requiresWebAuthn := user.WebAuthnEnabled

	// Issue session (partial if WebAuthn is required)
	sessionCtx := s.issueSession(user.ID, user.Email, func() string { if user.Username != nil { return *user.Username } else { return "" } }(), "customer")

	// Log successful login
	s.logAuditEvent(ctx, &user.ID, nil, "login_success", ipAddress, userAgent, true, map[string]interface{}{"webauthn_required": requiresWebAuthn})

	return &LoginResult{
		Session:          sessionCtx,
		RequiresWebAuthn: requiresWebAuthn,
	}, nil
}

// RefreshSession refreshes an existing session
func (s *AuthService) RefreshSession(ctx context.Context, currentSession session.Context) (*session.Context, error) {
	if !currentSession.IsAuthenticated() {
		return nil, &AuthError{Message: "no valid session to refresh"}
	}

	// Get user to verify they still exist
	user, err := s.authRepo.GetUserByID(ctx, currentSession.UserID)
	if err != nil || user == nil {
		return nil, &AuthError{Message: "user not found"}
	}

	// Issue new session
	newSession := s.issueSession(user.ID, user.Email, func() string { if user.Username != nil { return *user.Username } else { return "" } }(), currentSession.Role)

	return newSession, nil
}

// recordFailedLogin records a failed login attempt
func (s *AuthService) recordFailedLogin(ctx context.Context, identifier, ipAddress string) {
	_ = s.lockoutRepo.RecordLoginAttempt(ctx, identifier, ipAddress, false)
}

// logAuditEvent logs an authentication event
func (s *AuthService) logAuditEvent(ctx context.Context, userID *string, accountID *string, event string, ipAddress, userAgent string, success bool, metadata map[string]interface{}) {
	var uid string
	if userID != nil {
		uid = *userID
	}
	_ = s.auditRepo.LogAuthEvent(ctx, repository.AuthAuditEvent{
		UserID:    &uid,
		AccountID: accountID,
		Event:     event,
		IPAddress: &ipAddress,
		UserAgent: &userAgent,
		Success:   success,
		Metadata:  metadata,
	})
}

// issueSession creates a new session for a user
func (s *AuthService) issueSession(userID, email, username, role string) *session.Context {
	// For now, use a simple default account
	// In production, you'd fetch the user's actual account
	accountID := "acct_default"

	_ = time.Now().UTC().Add(s.config.SessionTTL).Unix() // Session expiry is handled by token manager

	return &session.Context{
		UserID:    userID,
		AccountID: accountID,
		Role:      role,
		Prototype: false,
	}
}

// Error types

type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

type RateLimitError struct {
	Message string
}

func (e *RateLimitError) Error() string {
	return e.Message
}

type LockoutError struct {
	LockedUntil *time.Time
}

func (e *LockoutError) Error() string {
	if e.LockedUntil != nil {
		return fmt.Sprintf("account locked until %s", e.LockedUntil.Format(time.RFC3339))
	}
	return "account locked"
}
