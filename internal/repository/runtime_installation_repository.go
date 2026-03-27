package repository

import (
	"context"
	"time"

	"bloop-control-plane/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RuntimeInstallationRepository interface {
	CreateInstallation(ctx context.Context, accountID, name, environment string, now time.Time) (models.RuntimeInstallation, error)
	ListInstallations(ctx context.Context, accountID string) ([]models.RuntimeInstallation, error)
	GetInstallation(ctx context.Context, accountID, installationID string) (*models.RuntimeInstallation, error)
	CreateToken(ctx context.Context, installationID, kind, tokenHash string, expiresAt *time.Time, now time.Time) (models.RuntimeInstallationToken, error)
	GetActiveTokenByHash(ctx context.Context, tokenHash, kind string, now time.Time) (*models.RuntimeInstallationToken, *models.RuntimeInstallation, error)
	MarkTokenUsed(ctx context.Context, tokenID string, usedAt time.Time) error
	RevokeToken(ctx context.Context, tokenID string, revokedAt time.Time) error
	RevokeActiveTokensByKind(ctx context.Context, installationID, kind string, revokedAt time.Time) error
	UpdateInstallationStatus(ctx context.Context, installationID, status string, now time.Time) error
	UpdateInstallationLastSeen(ctx context.Context, installationID string, seenAt time.Time) error
	ListInstallationEvents(ctx context.Context, accountID, installationID string, limit int) ([]map[string]any, error)
}

type PostgresRuntimeInstallationRepository struct { pool *pgxpool.Pool }

func NewPostgresRuntimeInstallationRepository(pool *pgxpool.Pool) *PostgresRuntimeInstallationRepository {
	return &PostgresRuntimeInstallationRepository{pool: pool}
}

func (r *PostgresRuntimeInstallationRepository) CreateInstallation(ctx context.Context, accountID, name, environment string, now time.Time) (models.RuntimeInstallation, error) {
	m := models.RuntimeInstallation{}
	id := "inst_" + now.Format("20060102150405.000000000")
	if err := r.pool.QueryRow(ctx, `INSERT INTO runtime_installations (id, account_id, name, environment, status, created_at, updated_at) VALUES ($1,$2,$3,$4,'pending',$5,$5) RETURNING id, account_id, name, COALESCE(environment,''), status, created_at, updated_at, last_seen_at`, id, accountID, name, environment, now).Scan(&m.ID, &m.AccountID, &m.Name, &m.Environment, &m.Status, &m.CreatedAt, &m.UpdatedAt, &m.LastSeenAt); err != nil {
		return models.RuntimeInstallation{}, err
	}
	return m, nil
}

func (r *PostgresRuntimeInstallationRepository) ListInstallations(ctx context.Context, accountID string) ([]models.RuntimeInstallation, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, account_id, name, COALESCE(environment,''), status, created_at, updated_at, last_seen_at FROM runtime_installations WHERE account_id = $1 ORDER BY created_at DESC`, accountID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []models.RuntimeInstallation
	for rows.Next() {
		var m models.RuntimeInstallation
		if err := rows.Scan(&m.ID, &m.AccountID, &m.Name, &m.Environment, &m.Status, &m.CreatedAt, &m.UpdatedAt, &m.LastSeenAt); err != nil { return nil, err }
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *PostgresRuntimeInstallationRepository) GetInstallation(ctx context.Context, accountID, installationID string) (*models.RuntimeInstallation, error) {
	var m models.RuntimeInstallation
	if err := r.pool.QueryRow(ctx, `SELECT id, account_id, name, COALESCE(environment,''), status, created_at, updated_at, last_seen_at FROM runtime_installations WHERE account_id = $1 AND id = $2`, accountID, installationID).Scan(&m.ID, &m.AccountID, &m.Name, &m.Environment, &m.Status, &m.CreatedAt, &m.UpdatedAt, &m.LastSeenAt); err != nil {
		return nil, nil
	}
	return &m, nil
}

func (r *PostgresRuntimeInstallationRepository) CreateToken(ctx context.Context, installationID, kind, tokenHash string, expiresAt *time.Time, now time.Time) (models.RuntimeInstallationToken, error) {
	m := models.RuntimeInstallationToken{}
	id := "rit_" + now.Format("20060102150405.000000000")
	if err := r.pool.QueryRow(ctx, `INSERT INTO runtime_installation_tokens (id, installation_id, kind, token_hash, expires_at, created_at) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, installation_id, kind, token_hash, expires_at, revoked_at, created_at, last_used_at`, id, installationID, kind, tokenHash, expiresAt, now).Scan(&m.ID, &m.InstallationID, &m.Kind, &m.TokenHash, &m.ExpiresAt, &m.RevokedAt, &m.CreatedAt, &m.LastUsedAt); err != nil {
		return models.RuntimeInstallationToken{}, err
	}
	return m, nil
}

func (r *PostgresRuntimeInstallationRepository) GetActiveTokenByHash(ctx context.Context, tokenHash, kind string, now time.Time) (*models.RuntimeInstallationToken, *models.RuntimeInstallation, error) {
	var tok models.RuntimeInstallationToken
	var inst models.RuntimeInstallation
	if err := r.pool.QueryRow(ctx, `SELECT t.id, t.installation_id, t.kind, t.token_hash, t.expires_at, t.revoked_at, t.created_at, t.last_used_at, i.id, i.account_id, i.name, COALESCE(i.environment,''), i.status, i.created_at, i.updated_at, i.last_seen_at FROM runtime_installation_tokens t JOIN runtime_installations i ON i.id = t.installation_id WHERE t.token_hash = $1 AND t.kind = $2 AND t.revoked_at IS NULL AND (t.expires_at IS NULL OR t.expires_at > $3)`, tokenHash, kind, now).Scan(&tok.ID, &tok.InstallationID, &tok.Kind, &tok.TokenHash, &tok.ExpiresAt, &tok.RevokedAt, &tok.CreatedAt, &tok.LastUsedAt, &inst.ID, &inst.AccountID, &inst.Name, &inst.Environment, &inst.Status, &inst.CreatedAt, &inst.UpdatedAt, &inst.LastSeenAt); err != nil {
		return nil, nil, nil
	}
	return &tok, &inst, nil
}

func (r *PostgresRuntimeInstallationRepository) MarkTokenUsed(ctx context.Context, tokenID string, usedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE runtime_installation_tokens SET last_used_at = $2 WHERE id = $1`, tokenID, usedAt)
	return err
}

func (r *PostgresRuntimeInstallationRepository) RevokeToken(ctx context.Context, tokenID string, revokedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE runtime_installation_tokens SET revoked_at = $2 WHERE id = $1`, tokenID, revokedAt)
	return err
}

func (r *PostgresRuntimeInstallationRepository) RevokeActiveTokensByKind(ctx context.Context, installationID, kind string, revokedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE runtime_installation_tokens SET revoked_at = $3 WHERE installation_id = $1 AND kind = $2 AND revoked_at IS NULL`, installationID, kind, revokedAt)
	return err
}

func (r *PostgresRuntimeInstallationRepository) UpdateInstallationStatus(ctx context.Context, installationID, status string, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE runtime_installations SET status = $2, updated_at = $3 WHERE id = $1`, installationID, status, now)
	return err
}

func (r *PostgresRuntimeInstallationRepository) UpdateInstallationLastSeen(ctx context.Context, installationID string, seenAt time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE runtime_installations SET last_seen_at = $2, updated_at = $2 WHERE id = $1`, installationID, seenAt)
	return err
}

func (r *PostgresRuntimeInstallationRepository) ListInstallationEvents(ctx context.Context, accountID, installationID string, limit int) ([]map[string]any, error) {
	if limit <= 0 { limit = 10 }
	rows, err := r.pool.Query(ctx, `SELECT kind, level, message, occurred_at FROM runtime_events WHERE account_id = $1 AND installation_id = $2 ORDER BY occurred_at DESC LIMIT $3`, accountID, installationID, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var kind, level, message string
		var occurredAt time.Time
		if err := rows.Scan(&kind, &level, &message, &occurredAt); err != nil { return nil, err }
		out = append(out, map[string]any{"kind": kind, "level": level, "message": message, "occurredAt": occurredAt})
	}
	return out, rows.Err()
}
