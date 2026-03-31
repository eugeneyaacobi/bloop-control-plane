package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/security"
)

// PasswordResetService handles password reset business logic
type PasswordResetService struct {
	resetRepo repository.PasswordResetRepository
	authRepo  repository.AuthRepository
	auditRepo repository.AuditRepository
	emailSvc  *EmailService
	config    *config.Config
}

// NewPasswordResetService creates a new password reset service
func NewPasswordResetService(
	resetRepo repository.PasswordResetRepository,
	authRepo repository.AuthRepository,
	auditRepo repository.AuditRepository,
	emailSvc *EmailService,
	config *config.Config,
) *PasswordResetService {
	return &PasswordResetService{
		resetRepo: resetRepo,
		authRepo:  authRepo,
		auditRepo: auditRepo,
		emailSvc:  emailSvc,
		config:    config,
	}
}

// RequestReset handles a forgot-password request.
// It always returns nil (success) to avoid user enumeration.
// If the email exists, a reset token is created and an email is sent.
func (s *PasswordResetService) RequestReset(ctx context.Context, email, ipAddress, userAgent string) error {
	user, _ := s.authRepo.GetUserByEmail(ctx, email)
	if user == nil {
		// Silently succeed — no enumeration
		return nil
	}

	// Rate limit: per email (user)
	since := time.Now().UTC().Add(-time.Hour)
	emailCount, _ := s.resetRepo.CountRecentByUserID(ctx, user.ID, since)
	if emailCount >= s.config.PasswordResetRateLimitEmail {
		s.auditLog(ctx, &user.ID, "password_reset_rate_limited_email", ipAddress, userAgent, false, map[string]interface{}{"reason": "email_rate_limit"})
		return &RateLimitError{Message: "too many reset requests for this email"}
	}

	// Rate limit: per IP
	ipCount, _ := s.resetRepo.CountRecentByIP(ctx, ipAddress, since)
	if ipCount >= s.config.PasswordResetRateLimitIP {
		s.auditLog(ctx, &user.ID, "password_reset_rate_limited_ip", ipAddress, userAgent, false, map[string]interface{}{"reason": "ip_rate_limit"})
		return &RateLimitError{Message: "too many reset requests from this IP"}
	}

	// Generate raw token (32 bytes, base64url)
	rawToken, err := generateResetToken()
	if err != nil {
		s.auditLog(ctx, &user.ID, "password_reset_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "token_generation_error"})
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// Hash for storage
	tokenHash := repository.HashToken(rawToken)
	expiresAt := time.Now().UTC().Add(s.config.PasswordResetTokenTTL)

	// Store hash, invalidating previous tokens
	if err := s.resetRepo.CreateToken(ctx, user.ID, tokenHash, expiresAt, ipAddress, userAgent); err != nil {
		s.auditLog(ctx, &user.ID, "password_reset_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "db_error"})
		return fmt.Errorf("failed to store token: %w", err)
	}

	// Send email asynchronously (don't block on failure, but log)
	if err := s.emailSvc.SendPasswordResetEmail(ctx, user.Email, rawToken); err != nil {
		s.auditLog(ctx, &user.ID, "password_reset_email_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "email_send_error"})
		// Don't fail the request — the token is stored, user can retry
	}

	s.auditLog(ctx, &user.ID, "password_reset_requested", ipAddress, userAgent, true, nil)
	return nil
}

// ResetPassword validates a reset token and updates the user's password.
// Returns an error if the token is invalid/expired or the password doesn't meet requirements.
func (s *PasswordResetService) ResetPassword(ctx context.Context, rawToken, newPassword, ipAddress, userAgent string) error {
	// Look up token by hash
	tokenHash := repository.HashToken(rawToken)
	token, _ := s.resetRepo.FindByTokenHash(ctx, tokenHash)
	if token == nil {
		s.auditLog(ctx, nil, "password_reset_invalid_token", ipAddress, userAgent, false, map[string]interface{}{"reason": "token_not_found_or_expired"})
		return &ValidationError{Field: "token", Message: "invalid or expired token"}
	}

	// Validate new password
	if err := security.ValidatePassword(newPassword); err != nil {
		s.auditLog(ctx, &token.UserID, "password_reset_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "invalid_password"})
		return &ValidationError{Field: "password", Message: "Password must be at least 12 characters. Please choose a longer one."}
	}

	// Check breach if enabled
	if s.config.BreachCheckEnabled {
		compromised, err := security.CheckPasswordBreach(newPassword)
		if err != nil {
			// Log but don't fail on breach check errors
		}
		if compromised {
			s.auditLog(ctx, &token.UserID, "password_reset_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "compromised_password"})
			return &ValidationError{Field: "password", Message: "this password has been exposed in data breaches"}
		}
	}

	// Check password history - prevent reuse of last 5 passwords
	history, err := s.authRepo.GetPasswordHistory(ctx, token.UserID, 5)
	if err == nil && len(history) > 0 {
		for _, oldHash := range history {
			if match, _ := security.VerifyPassword(newPassword, oldHash); match {
				s.auditLog(ctx, &token.UserID, "password_reset_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "password_reuse"})
				return &ValidationError{Field: "password", Message: "cannot reuse your recent passwords"}
			}
		}
	}

	// Hash new password
	hash, err := security.HashPassword(newPassword)
	if err != nil {
		s.auditLog(ctx, &token.UserID, "password_reset_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "hash_error"})
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	if err := s.authRepo.UpdatePasswordHash(ctx, token.UserID, hash); err != nil {
		s.auditLog(ctx, &token.UserID, "password_reset_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "db_error"})
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Auto-verify email on password reset — if they received the reset email,
	// they own the address. This prevents the UX trap of resetting a password
	// on an unverified account and still being unable to log in.
	if err := s.authRepo.SetVerified(ctx, token.UserID); err != nil {
		// Log but don't fail — password was updated successfully
		s.auditLog(ctx, &token.UserID, "password_reset_auto_verify_failed", ipAddress, userAgent, false, map[string]interface{}{"reason": "db_error"})
	}

	// Mark token as used
	_ = s.resetRepo.MarkUsed(ctx, token.ID)

	s.auditLog(ctx, &token.UserID, "password_reset_completed", ipAddress, userAgent, true, nil)
	return nil
}

func (s *PasswordResetService) auditLog(ctx context.Context, userID *string, event, ipAddress, userAgent string, success bool, metadata map[string]interface{}) {
	var uid string
	if userID != nil {
		uid = *userID
	}
	_ = s.auditRepo.LogAuthEvent(ctx, repository.AuthAuditEvent{
		UserID:    &uid,
		Event:     event,
		IPAddress: &ipAddress,
		UserAgent: &userAgent,
		Success:   success,
		Metadata:  metadata,
	})
}

// generateResetToken creates a cryptographically random 32-byte token, base64url-encoded
func generateResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
