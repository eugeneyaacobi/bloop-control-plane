package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionVersionRepository interface {
	GetSessionVersion(ctx context.Context, scopeKey string) (int64, error)
	BumpSessionVersion(ctx context.Context, scopeKey string, now time.Time) (int64, error)
}

type PostgresSessionVersionRepository struct { pool *pgxpool.Pool }

func NewPostgresSessionVersionRepository(pool *pgxpool.Pool) *PostgresSessionVersionRepository {
	return &PostgresSessionVersionRepository{pool: pool}
}

func SessionScopeKey(userID, accountID, role string) string {
	return fmt.Sprintf("%s|%s|%s", userID, accountID, role)
}

func (r *PostgresSessionVersionRepository) GetSessionVersion(ctx context.Context, scopeKey string) (int64, error) {
	var version int64
	err := r.pool.QueryRow(ctx, `SELECT version FROM session_versions WHERE scope_key = $1`, scopeKey).Scan(&version)
	if err != nil {
		return 1, nil
	}
	return version, nil
}

func (r *PostgresSessionVersionRepository) BumpSessionVersion(ctx context.Context, scopeKey string, now time.Time) (int64, error) {
	var version int64
	err := r.pool.QueryRow(ctx, `INSERT INTO session_versions (scope_key, version, updated_at) VALUES ($1, 2, $2) ON CONFLICT (scope_key) DO UPDATE SET version = session_versions.version + 1, updated_at = EXCLUDED.updated_at RETURNING version`, scopeKey, now).Scan(&version)
	return version, err
}
