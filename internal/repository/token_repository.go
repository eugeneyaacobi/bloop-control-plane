package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// APIToken represents an API token for relay authentication
type APIToken struct {
	ID          string
	UserID      string
	AccountID   string
	Name        string
	TokenHash   string // SHA-256 hash of the actual token
	TokenPrefix string // First 8 chars of token for identification
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	LastUsedAt  *time.Time
	CreatedAt   time.Time
}

// TokenRepository handles API token storage and retrieval
type TokenRepository interface {
	// CreateToken creates a new API token
	CreateToken(ctx context.Context, userID, accountID, name, tokenHash, tokenPrefix string, expiresAt *time.Time) (APIToken, error)

	// ListTokensByUser lists all tokens for a user (active and revoked)
	ListTokensByUser(ctx context.Context, userID string) ([]APIToken, error)

	// ListActiveTokensByUser lists only active (non-expired, non-revoked) tokens for a user
	ListActiveTokensByUser(ctx context.Context, userID string) ([]APIToken, error)

	// GetTokenByID retrieves a token by ID with ownership check
	GetTokenByID(ctx context.Context, tokenID, userID string) (*APIToken, error)

	// RevokeToken revokes a token by ID with ownership check
	RevokeToken(ctx context.Context, tokenID, userID string) error

	// LookupByHash looks up a token by its hash for authentication validation
	LookupByHash(ctx context.Context, tokenHash string) (*APIToken, error)

	// UpdateLastUsed updates the last_used_at timestamp
	UpdateLastUsed(ctx context.Context, tokenID string) error

	// DeleteToken permanently deletes a token by ID with ownership check
	DeleteToken(ctx context.Context, tokenID, userID string) error
}

// PostgresTokenRepository implements TokenRepository for PostgreSQL
type PostgresTokenRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresTokenRepository creates a new PostgreSQL-based token repository
func NewPostgresTokenRepository(pool *pgxpool.Pool) *PostgresTokenRepository {
	return &PostgresTokenRepository{pool: pool}
}

// CreateToken creates a new API token
func (r *PostgresTokenRepository) CreateToken(ctx context.Context, userID, accountID, name, tokenHash, tokenPrefix string, expiresAt *time.Time) (APIToken, error) {
	tokenID := uuid.New().String()
	now := time.Now().UTC()

	var token APIToken
	err := r.pool.QueryRow(ctx, `
		INSERT INTO api_tokens (id, user_id, account_id, name, token_hash, token_prefix, expires_at, revoked_at, last_used_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, NULL, $8)
		RETURNING id, user_id, account_id, name, token_hash, token_prefix, expires_at, revoked_at, last_used_at, created_at
	`, tokenID, userID, accountID, name, tokenHash, tokenPrefix, expiresAt, now).Scan(
		&token.ID, &token.UserID, &token.AccountID, &token.Name, &token.TokenHash, &token.TokenPrefix, &token.ExpiresAt, &token.RevokedAt, &token.LastUsedAt, &token.CreatedAt)

	return token, err
}

// ListTokensByUser lists all tokens for a user
func (r *PostgresTokenRepository) ListTokensByUser(ctx context.Context, userID string) ([]APIToken, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, account_id, name, token_hash, token_prefix, expires_at, revoked_at, last_used_at, created_at
		FROM api_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []APIToken
	for rows.Next() {
		var token APIToken
		err := rows.Scan(&token.ID, &token.UserID, &token.AccountID, &token.Name, &token.TokenHash, &token.TokenPrefix, &token.ExpiresAt, &token.RevokedAt, &token.LastUsedAt, &token.CreatedAt)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, rows.Err()
}

// ListActiveTokensByUser lists only active tokens for a user
func (r *PostgresTokenRepository) ListActiveTokensByUser(ctx context.Context, userID string) ([]APIToken, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, account_id, name, token_hash, token_prefix, expires_at, revoked_at, last_used_at, created_at
		FROM api_tokens
		WHERE user_id = $1
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []APIToken
	for rows.Next() {
		var token APIToken
		err := rows.Scan(&token.ID, &token.UserID, &token.AccountID, &token.Name, &token.TokenHash, &token.TokenPrefix, &token.ExpiresAt, &token.RevokedAt, &token.LastUsedAt, &token.CreatedAt)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, rows.Err()
}

// GetTokenByID retrieves a token by ID with ownership check
func (r *PostgresTokenRepository) GetTokenByID(ctx context.Context, tokenID, userID string) (*APIToken, error) {
	var token APIToken
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, account_id, name, token_hash, token_prefix, expires_at, revoked_at, last_used_at, created_at
		FROM api_tokens
		WHERE id = $1 AND user_id = $2
	`, tokenID, userID).Scan(
		&token.ID, &token.UserID, &token.AccountID, &token.Name, &token.TokenHash, &token.TokenPrefix, &token.ExpiresAt, &token.RevokedAt, &token.LastUsedAt, &token.CreatedAt)

	if err != nil {
		return nil, nil
	}

	return &token, nil
}

// RevokeToken revokes a token by ID with ownership check
func (r *PostgresTokenRepository) RevokeToken(ctx context.Context, tokenID, userID string) error {
	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, `
		UPDATE api_tokens
		SET revoked_at = $3
		WHERE id = $1 AND user_id = $2
	`, tokenID, userID, now)

	if err != nil {
		return err
	}

	// Check if a row was actually updated
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		// Token not found or not owned by user
		return nil
	}

	return nil
}

// LookupByHash looks up a token by its hash for authentication validation
func (r *PostgresTokenRepository) LookupByHash(ctx context.Context, tokenHash string) (*APIToken, error) {
	var token APIToken
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, account_id, name, token_hash, token_prefix, expires_at, revoked_at, last_used_at, created_at
		FROM api_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(
		&token.ID, &token.UserID, &token.AccountID, &token.Name, &token.TokenHash, &token.TokenPrefix, &token.ExpiresAt, &token.RevokedAt, &token.LastUsedAt, &token.CreatedAt)

	if err != nil {
		return nil, nil
	}

	return &token, nil
}

// UpdateLastUsed updates the last_used_at timestamp
func (r *PostgresTokenRepository) UpdateLastUsed(ctx context.Context, tokenID string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE api_tokens
		SET last_used_at = $2
		WHERE id = $1
	`, tokenID, now)
	return err
}

// DeleteToken permanently deletes a token by ID with ownership check
func (r *PostgresTokenRepository) DeleteToken(ctx context.Context, tokenID, userID string) error {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM api_tokens
		WHERE id = $1 AND user_id = $2
	`, tokenID, userID)

	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return nil
	}

	return nil
}

// InMemoryTokenRepository is an in-memory implementation for testing
type InMemoryTokenRepository struct {
	tokens map[string]APIToken
}

// NewInMemoryTokenRepository creates a new in-memory token repository
func NewInMemoryTokenRepository() *InMemoryTokenRepository {
	return &InMemoryTokenRepository{
		tokens: make(map[string]APIToken),
	}
}

// CreateToken creates a token in memory
func (r *InMemoryTokenRepository) CreateToken(ctx context.Context, userID, accountID, name, tokenHash, tokenPrefix string, expiresAt *time.Time) (APIToken, error) {
	tokenID := uuid.New().String()
	now := time.Now().UTC()

	token := APIToken{
		ID:          tokenID,
		UserID:      userID,
		AccountID:   accountID,
		Name:        name,
		TokenHash:   tokenHash,
		TokenPrefix: tokenPrefix,
		ExpiresAt:   expiresAt,
		RevokedAt:   nil,
		LastUsedAt:  nil,
		CreatedAt:   now,
	}

	r.tokens[tokenID] = token
	return token, nil
}

// ListTokensByUser lists all tokens for a user
func (r *InMemoryTokenRepository) ListTokensByUser(ctx context.Context, userID string) ([]APIToken, error) {
	var result []APIToken
	for _, token := range r.tokens {
		if token.UserID == userID {
			result = append(result, token)
		}
	}
	return result, nil
}

// ListActiveTokensByUser lists only active tokens for a user
func (r *InMemoryTokenRepository) ListActiveTokensByUser(ctx context.Context, userID string) ([]APIToken, error) {
	var result []APIToken
	now := time.Now().UTC()
	for _, token := range r.tokens {
		if token.UserID == userID && token.RevokedAt == nil && (token.ExpiresAt == nil || token.ExpiresAt.After(now)) {
			result = append(result, token)
		}
	}
	return result, nil
}

// GetTokenByID retrieves a token by ID with ownership check
func (r *InMemoryTokenRepository) GetTokenByID(ctx context.Context, tokenID, userID string) (*APIToken, error) {
	token, exists := r.tokens[tokenID]
	if !exists || token.UserID != userID {
		return nil, nil
	}
	return &token, nil
}

// RevokeToken revokes a token by ID with ownership check
func (r *InMemoryTokenRepository) RevokeToken(ctx context.Context, tokenID, userID string) error {
	token, exists := r.tokens[tokenID]
	if !exists || token.UserID != userID {
		return nil
	}
	now := time.Now().UTC()
	token.RevokedAt = &now
	r.tokens[tokenID] = token
	return nil
}

// LookupByHash looks up a token by its hash
func (r *InMemoryTokenRepository) LookupByHash(ctx context.Context, tokenHash string) (*APIToken, error) {
	for _, token := range r.tokens {
		if token.TokenHash == tokenHash {
			return &token, nil
		}
	}
	return nil, nil
}

// UpdateLastUsed updates the last_used_at timestamp
func (r *InMemoryTokenRepository) UpdateLastUsed(ctx context.Context, tokenID string) error {
	token, exists := r.tokens[tokenID]
	if !exists {
		return nil
	}
	now := time.Now().UTC()
	token.LastUsedAt = &now
	r.tokens[tokenID] = token
	return nil
}

// DeleteToken permanently deletes a token by ID with ownership check
func (r *InMemoryTokenRepository) DeleteToken(ctx context.Context, tokenID, userID string) error {
	token, exists := r.tokens[tokenID]
	if !exists || token.UserID != userID {
		return nil
	}
	delete(r.tokens, tokenID)
	return nil
}

// HashAPIToken computes the SHA-256 hash of an API token value
func HashAPIToken(tokenValue string) string {
	hash := sha256.Sum256([]byte(tokenValue))
	return hex.EncodeToString(hash[:])
}

// TokenPrefix extracts the prefix (first 8 chars) from a token for display
func TokenPrefix(tokenValue string) string {
	if len(tokenValue) < 8 {
		return tokenValue
	}
	return tokenValue[:8]
}
