package repository

import (
	"context"
	"time"

	"bloop-control-plane/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EmailVerification struct {
	ID              string
	SignupRequestID string
	TokenHash       string
	State           string
	ExpiresAt       time.Time
	VerifiedAt      *time.Time
}

type SignupRepository interface {
	CreateSignupRequest(ctx context.Context, id, email, state string) error
	CreateEmailVerification(ctx context.Context, id, signupRequestID, tokenHash, state string, expiresAt time.Time) error
	FindVerificationByTokenHash(ctx context.Context, tokenHash string) (*EmailVerification, *models.SignupRequest, error)
	MarkVerificationCompleted(ctx context.Context, verificationID, signupRequestID string, verifiedAt time.Time) error
}

type PostgresSignupRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresSignupRepository(pool *pgxpool.Pool) *PostgresSignupRepository {
	return &PostgresSignupRepository{pool: pool}
}

func (r *PostgresSignupRepository) CreateSignupRequest(ctx context.Context, id, email, state string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO signup_requests (id, email, state) VALUES ($1, $2, $3)`, id, email, state)
	return err
}

func (r *PostgresSignupRepository) CreateEmailVerification(ctx context.Context, id, signupRequestID, tokenHash, state string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO email_verifications (id, signup_request_id, token_hash, state, expires_at) VALUES ($1, $2, $3, $4, $5)`, id, signupRequestID, tokenHash, state, expiresAt)
	return err
}

func (r *PostgresSignupRepository) FindVerificationByTokenHash(ctx context.Context, tokenHash string) (*EmailVerification, *models.SignupRequest, error) {
	query := `
		SELECT ev.id, ev.signup_request_id, ev.token_hash, ev.state, ev.expires_at, ev.verified_at,
		       sr.id, sr.email, sr.state
		FROM email_verifications ev
		JOIN signup_requests sr ON sr.id = ev.signup_request_id
		WHERE ev.token_hash = $1
		LIMIT 1`

	var ev EmailVerification
	var signup models.SignupRequest
	if err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&ev.ID,
		&ev.SignupRequestID,
		&ev.TokenHash,
		&ev.State,
		&ev.ExpiresAt,
		&ev.VerifiedAt,
		&signup.ID,
		&signup.Email,
		&signup.State,
	); err != nil {
		return nil, nil, nil
	}
	return &ev, &signup, nil
}

func (r *PostgresSignupRepository) MarkVerificationCompleted(ctx context.Context, verificationID, signupRequestID string, verifiedAt time.Time) error {
	if _, err := r.pool.Exec(ctx, `UPDATE email_verifications SET state = 'verified', verified_at = $2 WHERE id = $1`, verificationID, verifiedAt); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `UPDATE signup_requests SET state = 'verified' WHERE id = $1`, signupRequestID)
	return err
}
