package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
)

// TokenService handles API token operations
type TokenService struct {
	tokenRepo repository.TokenRepository
	auditRepo repository.AuditRepository
	config    *config.Config
}

// NewTokenService creates a new token service
func NewTokenService(
	tokenRepo repository.TokenRepository,
	auditRepo repository.AuditRepository,
	config *config.Config,
) *TokenService {
	return &TokenService{
		tokenRepo: tokenRepo,
		auditRepo: auditRepo,
		config:    config,
	}
}

// CreateTokenResult holds the result of token creation
type CreateTokenResult struct {
	Token       models.TokenCreateResponse
	Plaintext   string
	TokenPrefix string
}

// CreateToken creates a new API token for a user
func (s *TokenService) CreateToken(ctx context.Context, userID, accountID, name string, expiresIn *string) (*CreateTokenResult, error) {
	// Parse expiry duration
	var expiresAt *time.Time
	if expiresIn != nil && *expiresIn != "" {
		duration, err := time.ParseDuration(*expiresIn)
		if err != nil {
			return nil, &ValidationError{Field: "expires_in", Message: "invalid duration format"}
		}
	 expiry := time.Now().UTC().Add(duration)
		expiresAt = &expiry
	} else {
		// Use default expiry
		expiry := time.Now().UTC().Add(s.config.APITokenDefaultExpiry)
		expiresAt = &expiry
	}

	// Generate 256-bit random token (64 hex chars = 32 bytes = 256 bits)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	plaintextToken := hex.EncodeToString(tokenBytes)

	// Compute hash and prefix
	tokenHash := repository.HashAPIToken(plaintextToken)
	tokenPrefix := repository.TokenPrefix(plaintextToken)

	// Store in database
	token, err := s.tokenRepo.CreateToken(ctx, userID, accountID, name, tokenHash, tokenPrefix, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// Convert to model type
	result := &CreateTokenResult{
		Token: models.TokenCreateResponse{
			ID:          token.ID,
			Name:        token.Name,
			Token:       plaintextToken,
			TokenPrefix: token.TokenPrefix,
			AccountID:   token.AccountID,
			ExpiresAt:   formatTimePtr(token.ExpiresAt),
			CreatedAt:   token.CreatedAt.Format(time.RFC3339),
		},
		Plaintext:   plaintextToken,
		TokenPrefix: tokenPrefix,
	}

	// Log audit event
	var userIDPtr = &userID
	_ = s.auditRepo.LogAuthEvent(ctx, repository.AuthAuditEvent{
		UserID:    userIDPtr,
		AccountID: &accountID,
		Event:     "token_created",
		Success:   true,
		Metadata:  map[string]interface{}{"token_id": token.ID, "token_name": name, "token_prefix": tokenPrefix},
	})

	return result, nil
}

// ListTokens lists all tokens for a user
func (s *TokenService) ListTokens(ctx context.Context, userID string) ([]models.TokenSummary, error) {
	tokens, err := s.tokenRepo.ListTokensByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tokens: %w", err)
	}

	result := make([]models.TokenSummary, len(tokens))
	for i, token := range tokens {
		result[i] = models.TokenSummary{
			ID:          token.ID,
			Name:        token.Name,
			TokenPrefix: token.TokenPrefix,
			AccountID:   token.AccountID,
			ExpiresAt:   formatTimePtr(token.ExpiresAt),
			RevokedAt:   formatTimePtr(token.RevokedAt),
			LastUsedAt:  formatTimePtr(token.LastUsedAt),
			CreatedAt:   token.CreatedAt.Format(time.RFC3339),
		}
	}

	return result, nil
}

// RevokeToken revokes a token by ID
func (s *TokenService) RevokeToken(ctx context.Context, userID, tokenID string) error {
	// Check ownership first
	token, err := s.tokenRepo.GetTokenByID(ctx, tokenID, userID)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}
	if token == nil {
		return &NotFoundError{Resource: "token", ID: tokenID}
	}

	// Revoke the token
	err = s.tokenRepo.RevokeToken(ctx, tokenID, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	// Log audit event
	var userIDPtr = &userID
	_ = s.auditRepo.LogAuthEvent(ctx, repository.AuthAuditEvent{
		UserID:    userIDPtr,
		AccountID: &token.AccountID,
		Event:     "token_revoked",
		Success:   true,
		Metadata:  map[string]interface{}{"token_id": tokenID, "token_name": token.Name, "token_prefix": token.TokenPrefix},
	})

	return nil
}

// RefreshTokenResult holds the result of token refresh
type RefreshTokenResult struct {
	Token       models.TokenRefreshResponse
	Plaintext   string
	TokenPrefix string
}

// RefreshToken creates a new token and revokes the old one
func (s *TokenService) RefreshToken(ctx context.Context, userID, tokenID string) (*RefreshTokenResult, error) {
	// Check ownership
	token, err := s.tokenRepo.GetTokenByID(ctx, tokenID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	if token == nil {
		return nil, &NotFoundError{Resource: "token", ID: tokenID}
	}

	// Revoke old token
	err = s.tokenRepo.RevokeToken(ctx, tokenID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke old token: %w", err)
	}

	// Create new token with same name and account
	// Use the same expiry calculation as the original token
	var expiresAt *time.Time
	if token.ExpiresAt != nil {
		// Keep the same expiry time
		expiresAt = token.ExpiresAt
	}

	// Generate new token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	plaintextToken := hex.EncodeToString(tokenBytes)

	// Compute hash and prefix
	tokenHash := repository.HashAPIToken(plaintextToken)
	tokenPrefix := repository.TokenPrefix(plaintextToken)

	// Store new token
	newToken, err := s.tokenRepo.CreateToken(ctx, userID, token.AccountID, token.Name, tokenHash, tokenPrefix, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create new token: %w", err)
	}

	// Convert to model type
	result := &RefreshTokenResult{
		Token: models.TokenRefreshResponse{
			ID:          newToken.ID,
			Name:        newToken.Name,
			TokenPrefix: newToken.TokenPrefix,
			ExpiresAt:   formatTimePtr(newToken.ExpiresAt),
			CreatedAt:   newToken.CreatedAt.Format(time.RFC3339),
		},
		Plaintext:   plaintextToken,
		TokenPrefix: tokenPrefix,
	}

	// Log audit event
	var userIDPtr = &userID
	_ = s.auditRepo.LogAuthEvent(ctx, repository.AuthAuditEvent{
		UserID:    userIDPtr,
		AccountID: &token.AccountID,
		Event:     "token_refreshed",
		Success:   true,
		Metadata:  map[string]interface{}{"old_token_id": tokenID, "new_token_id": newToken.ID, "token_name": token.Name},
	})

	return result, nil
}

// ValidateToken validates an API token for authentication
// Returns the token if valid, nil otherwise
func (s *TokenService) ValidateToken(ctx context.Context, tokenValue string) (*repository.APIToken, error) {
	if tokenValue == "" {
		return nil, errors.New("token value is required")
	}

	// Compute hash
	tokenHash := repository.HashAPIToken(tokenValue)

	// Lookup token by hash
	token, err := s.tokenRepo.LookupByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup token: %w", err)
	}
	if token == nil {
		return nil, nil
	}

	// Check if revoked
	if token.RevokedAt != nil {
		return nil, nil
	}

	// Check if expired
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now().UTC()) {
		return nil, nil
	}

	// Update last used timestamp
	_ = s.tokenRepo.UpdateLastUsed(ctx, token.ID)

	return token, nil
}

// formatTimePtr formats a time pointer to an RFC3339 string pointer
func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.Format(time.RFC3339)
	return &formatted
}
