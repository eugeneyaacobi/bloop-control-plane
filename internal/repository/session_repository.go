package repository

import (
	"context"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionIdentity struct {
	User       *models.User       `json:"user,omitempty"`
	Account    *models.Account    `json:"account,omitempty"`
	Membership *models.Membership `json:"membership,omitempty"`
}

type SessionRepository interface {
	ResolveIdentity(ctx context.Context, sess session.Context) (*SessionIdentity, error)
}

type PostgresSessionRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresSessionRepository(pool *pgxpool.Pool) *PostgresSessionRepository {
	return &PostgresSessionRepository{pool: pool}
}

func (r *PostgresSessionRepository) ResolveIdentity(ctx context.Context, sess session.Context) (*SessionIdentity, error) {
	identity := &SessionIdentity{}
	if sess.UserID != "" {
		var user models.User
		err := r.pool.QueryRow(ctx, `SELECT id, email, display_name FROM users WHERE id = $1`, sess.UserID).Scan(&user.ID, &user.Email, &user.DisplayName)
		if err == nil {
			identity.User = &user
		}
	}
	if sess.AccountID != "" {
		var account models.Account
		err := r.pool.QueryRow(ctx, `SELECT id, display_name FROM accounts WHERE id = $1`, sess.AccountID).Scan(&account.ID, &account.DisplayName)
		if err == nil {
			identity.Account = &account
		}
	}
	if sess.UserID != "" && sess.AccountID != "" {
		var membership models.Membership
		err := r.pool.QueryRow(ctx, `SELECT id, user_id, account_id, role FROM memberships WHERE user_id = $1 AND account_id = $2 ORDER BY created_at DESC LIMIT 1`, sess.UserID, sess.AccountID).Scan(&membership.ID, &membership.UserID, &membership.AccountID, &membership.Role)
		if err == nil {
			identity.Membership = &membership
		}
	}
	return identity, nil
}
