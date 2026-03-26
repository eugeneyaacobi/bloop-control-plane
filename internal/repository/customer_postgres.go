package repository

import (
	"context"

	"bloop-control-plane/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresCustomerRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCustomerRepository(pool *pgxpool.Pool) *PostgresCustomerRepository {
	return &PostgresCustomerRepository{pool: pool}
}

func (r *PostgresCustomerRepository) GetWorkspace(ctx context.Context, accountID string) (models.Account, []models.Tunnel, error) {
	var account models.Account
	if err := r.pool.QueryRow(ctx, `SELECT id, display_name FROM accounts WHERE id = $1`, accountID).Scan(&account.ID, &account.DisplayName); err != nil {
		return models.Account{}, nil, err
	}
	tunnels, err := r.ListTunnels(ctx, accountID)
	if err != nil {
		return models.Account{}, nil, err
	}
	return account, tunnels, nil
}

func (r *PostgresCustomerRepository) ListTunnels(ctx context.Context, accountID string) ([]models.Tunnel, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, hostname, target, access, status, COALESCE(region, ''), COALESCE(owner, ''), COALESCE(risk, '') FROM tunnels WHERE account_id = $1 ORDER BY hostname`, accountID)
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

func (r *PostgresCustomerRepository) GetTunnelByID(ctx context.Context, accountID, tunnelID string) (*models.Tunnel, error) {
	var tunnel models.Tunnel
	if err := r.pool.QueryRow(ctx, `SELECT id, hostname, target, access, status, COALESCE(region, ''), COALESCE(owner, ''), COALESCE(risk, '') FROM tunnels WHERE account_id = $1 AND id = $2`, accountID, tunnelID).Scan(&tunnel.ID, &tunnel.Hostname, &tunnel.Target, &tunnel.Access, &tunnel.Status, &tunnel.Region, &tunnel.Owner, &tunnel.Risk); err != nil {
		return nil, nil
	}
	return &tunnel, nil
}
