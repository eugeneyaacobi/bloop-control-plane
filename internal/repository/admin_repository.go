package repository

import (
	"context"

	"bloop-control-plane/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminRepository interface {
	OverviewStats(ctx context.Context) (accountCount, publicRoutes, flaggedExposures int, err error)
	ListUsers(ctx context.Context) ([]models.User, error)
	ListTunnels(ctx context.Context) ([]models.Tunnel, error)
	ListReviewFlags(ctx context.Context) ([]models.ReviewFlag, error)
}

type PostgresAdminRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresAdminRepository(pool *pgxpool.Pool) *PostgresAdminRepository {
	return &PostgresAdminRepository{pool: pool}
}

func (r *PostgresAdminRepository) OverviewStats(ctx context.Context) (int, int, int, error) {
	var accountCount, publicRoutes, flagged int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&accountCount); err != nil {
		return 0, 0, 0, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tunnels WHERE access = 'public'`).Scan(&publicRoutes); err != nil {
		return 0, 0, 0, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM review_flags`).Scan(&flagged); err != nil {
		return 0, 0, 0, err
	}
	return accountCount, publicRoutes, flagged, nil
}

func (r *PostgresAdminRepository) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, email, display_name FROM users ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Email, &user.DisplayName); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *PostgresAdminRepository) ListTunnels(ctx context.Context) ([]models.Tunnel, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, hostname, target, access, status, COALESCE(region, ''), COALESCE(owner, ''), COALESCE(risk, '') FROM tunnels ORDER BY hostname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tunnels []models.Tunnel
	for rows.Next() {
		var tunnel models.Tunnel
		if err := rows.Scan(&tunnel.ID, &tunnel.Hostname, &tunnel.Target, &tunnel.Access, &tunnel.Status, &tunnel.Region, &tunnel.Owner, &tunnel.Risk); err != nil {
			return nil, err
		}
		tunnels = append(tunnels, tunnel)
	}
	return tunnels, rows.Err()
}

func (r *PostgresAdminRepository) ListReviewFlags(ctx context.Context) ([]models.ReviewFlag, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, item, reason, severity FROM review_flags ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flags []models.ReviewFlag
	for rows.Next() {
		var flag models.ReviewFlag
		if err := rows.Scan(&flag.ID, &flag.Item, &flag.Reason, &flag.Severity); err != nil {
			return nil, err
		}
		flags = append(flags, flag)
	}
	return flags, rows.Err()
}
