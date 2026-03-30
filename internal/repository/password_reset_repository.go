package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PasswordResetToken represents a password reset token record
type PasswordResetToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
	IPAddress string
	UserAgent string
}

// PasswordResetRepository handles password reset token storage
type PasswordResetRepository interface {
	// CreateToken stores a new reset token (hash) and invalidates previous unused tokens for the user
	CreateToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time, ipAddress, userAgent string) error

	// FindByTokenHash finds an unused, non-expired token by its hash
	FindByTokenHash(ctx context.Context, tokenHash string) (*PasswordResetToken, error)

	// MarkUsed marks a token as used
	MarkUsed(ctx context.Context, tokenID string) error

	// CountRecentByUserID counts tokens created for a user since a given time
	CountRecentByUserID(ctx context.Context, userID string, since time.Time) (int, error)

	// CountRecentByIP counts tokens created from an IP since a given time
	CountRecentByIP(ctx context.Context, ipAddress string, since time.Time) (int, error)
}

// HashToken computes the SHA-256 hash of a raw token for storage
func HashToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return fmt.Sprintf("%x", h[:])
}

// PostgresPasswordResetRepository implements PasswordResetRepository for PostgreSQL
type PostgresPasswordResetRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresPasswordResetRepository creates a new PostgreSQL-based password reset repository
func NewPostgresPasswordResetRepository(pool *pgxpool.Pool) *PostgresPasswordResetRepository {
	return &PostgresPasswordResetRepository{pool: pool}
}

// CreateToken stores a new reset token and invalidates previous unused tokens for the user
func (r *PostgresPasswordResetRepository) CreateToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time, ipAddress, userAgent string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Invalidate previous unused tokens for this user
	_, err = tx.Exec(ctx, `
		UPDATE password_reset_tokens SET used_at = now()
		WHERE user_id = $1 AND used_at IS NULL
	`, userID)
	if err != nil {
		return err
	}

	// Insert new token
	id := uuid.New().String()
	_, err = tx.Exec(ctx, `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, userID, tokenHash, expiresAt, ipAddress, userAgent)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// FindByTokenHash finds an unused, non-expired token by its hash
func (r *PostgresPasswordResetRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*PasswordResetToken, error) {
	var t PasswordResetToken
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, used_at, created_at, COALESCE(ip_address, ''), COALESCE(user_agent, '')
		FROM password_reset_tokens
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
	`, tokenHash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt, &t.IPAddress, &t.UserAgent)
	if err != nil {
		return nil, nil
	}
	return &t, nil
}

// MarkUsed marks a token as used
func (r *PostgresPasswordResetRepository) MarkUsed(ctx context.Context, tokenID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE password_reset_tokens SET used_at = now() WHERE id = $1
	`, tokenID)
	return err
}

// CountRecentByUserID counts tokens created for a user since a given time
func (r *PostgresPasswordResetRepository) CountRecentByUserID(ctx context.Context, userID string, since time.Time) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = $1 AND created_at > $2
	`, userID, since).Scan(&count)
	return count, err
}

// CountRecentByIP counts tokens created from an IP since a given time
func (r *PostgresPasswordResetRepository) CountRecentByIP(ctx context.Context, ipAddress string, since time.Time) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM password_reset_tokens WHERE ip_address = $1 AND created_at > $2
	`, ipAddress, since).Scan(&count)
	return count, err
}

// InMemoryPasswordResetRepository is an in-memory implementation for testing
type InMemoryPasswordResetRepository struct {
	tokens []*PasswordResetToken
}

// NewInMemoryPasswordResetRepository creates a new in-memory password reset repository
func NewInMemoryPasswordResetRepository() *InMemoryPasswordResetRepository {
	return &InMemoryPasswordResetRepository{}
}

// CreateToken stores a new reset token and invalidates previous unused tokens
func (r *InMemoryPasswordResetRepository) CreateToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time, ipAddress, userAgent string) error {
	// Invalidate previous unused tokens
	for _, t := range r.tokens {
		if t.UserID == userID && t.UsedAt == nil {
			now := time.Now().UTC()
			t.UsedAt = &now
		}
	}

	r.tokens = append(r.tokens, &PasswordResetToken{
		ID:        uuid.New().String(),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		CreatedAt: time.Now().UTC(),
	})
	return nil
}

// FindByTokenHash finds an unused, non-expired token by its hash
func (r *InMemoryPasswordResetRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*PasswordResetToken, error) {
	for _, t := range r.tokens {
		if t.TokenHash == tokenHash && t.UsedAt == nil && t.ExpiresAt.After(time.Now().UTC()) {
			return t, nil
		}
	}
	return nil, nil
}

// MarkUsed marks a token as used
func (r *InMemoryPasswordResetRepository) MarkUsed(ctx context.Context, tokenID string) error {
	for _, t := range r.tokens {
		if t.ID == tokenID {
			now := time.Now().UTC()
			t.UsedAt = &now
			return nil
		}
	}
	return nil
}

// CountRecentByUserID counts tokens created for a user since a given time
func (r *InMemoryPasswordResetRepository) CountRecentByUserID(ctx context.Context, userID string, since time.Time) (int, error) {
	count := 0
	for _, t := range r.tokens {
		if t.UserID == userID && t.CreatedAt.After(since) {
			count++
		}
	}
	return count, nil
}

// CountRecentByIP counts tokens created from an IP since a given time
func (r *InMemoryPasswordResetRepository) CountRecentByIP(ctx context.Context, ipAddress string, since time.Time) (int, error) {
	count := 0
	for _, t := range r.tokens {
		if t.IPAddress == ipAddress && t.CreatedAt.After(since) {
			count++
		}
	}
	return count, nil
}
