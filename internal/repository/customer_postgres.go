package repository

import (
	"context"
	"errors"
	"strings"

	"bloop-control-plane/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &tunnel, nil
}

func (r *PostgresCustomerRepository) CreateTunnel(ctx context.Context, accountID string, params CreateTunnelParams) (*models.Tunnel, error) {
	id := strings.NewReplacer(".", "-", ":", "-", "/", "-").Replace(params.Hostname)
	var tunnel models.Tunnel
	err := r.pool.QueryRow(ctx, `
		INSERT INTO tunnels (id, account_id, hostname, target, access, status, region, owner, risk)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''))
		RETURNING id, hostname, target, access, status, COALESCE(region, ''), COALESCE(owner, ''), COALESCE(risk, '')
	`, id, accountID, params.Hostname, params.Target, params.Access, params.Status, params.Region, params.Owner, params.Risk).Scan(
		&tunnel.ID,
		&tunnel.Hostname,
		&tunnel.Target,
		&tunnel.Access,
		&tunnel.Status,
		&tunnel.Region,
		&tunnel.Owner,
		&tunnel.Risk,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, errors.New("tunnel already exists")
		}
		return nil, err
	}
	return &tunnel, nil
}

func (r *PostgresCustomerRepository) UpdateTunnel(ctx context.Context, accountID, tunnelID string, params UpdateTunnelParams) (*models.Tunnel, error) {
	var tunnel models.Tunnel
	err := r.pool.QueryRow(ctx, `
		UPDATE tunnels
		SET target = $3, access = $4, region = NULLIF($5, '')
		WHERE account_id = $1 AND id = $2
		RETURNING id, hostname, target, access, status, COALESCE(region, ''), COALESCE(owner, ''), COALESCE(risk, '')
	`, accountID, tunnelID, params.Target, params.Access, params.Region).Scan(
		&tunnel.ID,
		&tunnel.Hostname,
		&tunnel.Target,
		&tunnel.Access,
		&tunnel.Status,
		&tunnel.Region,
		&tunnel.Owner,
		&tunnel.Risk,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &tunnel, nil
}

func (r *PostgresCustomerRepository) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM tunnels WHERE account_id = $1 AND id = $2`, accountID, tunnelID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
