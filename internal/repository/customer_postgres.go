package repository

import (
	"context"
	"time"

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

func (r *PostgresCustomerRepository) ListInstallations(ctx context.Context, accountID string) ([]models.RuntimeInstallation, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, account_id, name, COALESCE(environment,''), status, created_at, updated_at, last_seen_at FROM runtime_installations WHERE account_id = $1 ORDER BY created_at DESC`, accountID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []models.RuntimeInstallation
	for rows.Next() {
		var inst models.RuntimeInstallation
		if err := rows.Scan(&inst.ID, &inst.AccountID, &inst.Name, &inst.Environment, &inst.Status, &inst.CreatedAt, &inst.UpdatedAt, &inst.LastSeenAt); err != nil { return nil, err }
		out = append(out, inst)
	}
	return out, rows.Err()
}

func (r *PostgresCustomerRepository) GetRuntimeOverlayByTunnel(ctx context.Context, accountID, tunnelID string) (*RuntimeOverlay, error) {
	var overlay RuntimeOverlay
	var observedAt *time.Time
	if err := r.pool.QueryRow(ctx, `SELECT COALESCE(ri.id,''), COALESCE(ri.name,''), COALESCE(rts.status,''), COALESCE(rts.degraded, false), rts.observed_at, COALESCE(rts.hostname,''), CASE WHEN rts.id IS NULL THEN 'declared_not_observed' WHEN ri.status = 'revoked' THEN 'revoked_installation_observed' WHEN ri.last_seen_at IS NULL OR ri.last_seen_at < NOW() - INTERVAL '15 minutes' THEN 'stale_installation' WHEN t.hostname <> rts.hostname THEN 'hostname_mismatch' ELSE '' END FROM tunnels t LEFT JOIN runtime_tunnel_snapshots rts ON rts.account_id = t.account_id AND rts.tunnel_id = t.id LEFT JOIN runtime_installations ri ON ri.id = rts.installation_id WHERE t.account_id = $1 AND t.id = $2`, accountID, tunnelID).Scan(&overlay.InstallationID, &overlay.InstallationName, &overlay.Status, &overlay.Degraded, &observedAt, &overlay.ObservedHostname, &overlay.Drift); err != nil {
		return nil, nil
	}
	if overlay.Status == "" {
		overlay.Status = "awaiting-runtime"
	}
	if observedAt != nil {
		formatted := observedAt.Format(time.RFC3339)
		overlay.LastSeenAt = &formatted
	}
	return &overlay, nil
}
