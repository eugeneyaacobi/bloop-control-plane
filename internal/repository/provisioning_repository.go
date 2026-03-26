package repository

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProvisionedIdentity struct {
	UserID      string
	AccountID   string
	MembershipID string
	Role        string
	DisplayName string
	Email       string
	AccountName string
}

type ProvisioningRepository interface {
	ProvisionSignupIdentity(ctx context.Context, email string, now time.Time) (*ProvisionedIdentity, error)
	FindProvisionedIdentityByEmail(ctx context.Context, email string) (*ProvisionedIdentity, error)
}

type PostgresProvisioningRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresProvisioningRepository(pool *pgxpool.Pool) *PostgresProvisioningRepository {
	return &PostgresProvisioningRepository{pool: pool}
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	lower = nonSlug.ReplaceAllString(lower, "-")
	lower = strings.Trim(lower, "-")
	if lower == "" {
		return "user"
	}
	return lower
}

func displayNameFromEmail(email string) string {
	local := email
	if at := strings.Index(email, "@"); at > 0 {
		local = email[:at]
	}
	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	local = strings.ReplaceAll(local, "-", " ")
	local = strings.TrimSpace(local)
	if local == "" {
		return "Bloop user"
	}
	return strings.Title(local)
}

func (r *PostgresProvisioningRepository) FindProvisionedIdentityByEmail(ctx context.Context, email string) (*ProvisionedIdentity, error) {
	query := `
		SELECT u.id, a.id, m.id, m.role, u.display_name, u.email, a.display_name
		FROM users u
		JOIN memberships m ON m.user_id = u.id
		JOIN accounts a ON a.id = m.account_id
		WHERE u.email = $1
		LIMIT 1`
	var identity ProvisionedIdentity
	if err := r.pool.QueryRow(ctx, query, email).Scan(
		&identity.UserID,
		&identity.AccountID,
		&identity.MembershipID,
		&identity.Role,
		&identity.DisplayName,
		&identity.Email,
		&identity.AccountName,
	); err != nil {
		return nil, nil
	}
	return &identity, nil
}

func (r *PostgresProvisioningRepository) ProvisionSignupIdentity(ctx context.Context, email string, now time.Time) (*ProvisionedIdentity, error) {
	base := slugify(email)
	userID := fmt.Sprintf("user_%s", base)
	accountID := fmt.Sprintf("acct_%s", base)
	membershipID := fmt.Sprintf("mem_%s", base)
	displayName := displayNameFromEmail(email)
	accountName := fmt.Sprintf("%s / workspace", displayName)
	role := "owner"

	if _, err := r.pool.Exec(ctx, `INSERT INTO users (id, email, display_name) VALUES ($1, $2, $3) ON CONFLICT (email) DO NOTHING`, userID, email, displayName); err != nil {
		return nil, err
	}
	if _, err := r.pool.Exec(ctx, `INSERT INTO accounts (id, display_name) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`, accountID, accountName); err != nil {
		return nil, err
	}
	if _, err := r.pool.Exec(ctx, `INSERT INTO memberships (id, user_id, account_id, role) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING`, membershipID, userID, accountID, role); err != nil {
		return nil, err
	}
	if _, err := r.pool.Exec(ctx, `INSERT INTO onboarding_steps (id, account_id, step_key, title, detail, state) VALUES ($1, $2, 'connect-target', 'Connect first target', 'Link your first service to a named route.', 'active') ON CONFLICT (id) DO NOTHING`, fmt.Sprintf("ob_%d", now.UnixNano()), accountID); err != nil {
		return nil, err
	}
	return &ProvisionedIdentity{
		UserID:       userID,
		AccountID:    accountID,
		MembershipID: membershipID,
		Role:         role,
		DisplayName:  displayName,
		Email:        email,
		AccountName:  accountName,
	}, nil
}
