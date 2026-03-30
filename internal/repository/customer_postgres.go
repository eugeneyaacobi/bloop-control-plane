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
	rows, err := r.pool.Query(ctx, `SELECT id, account_id, hostname, target, access, status, COALESCE(region, ''), COALESCE(owner, ''), COALESCE(risk, ''), created_at, updated_at FROM tunnels WHERE account_id = $1 ORDER BY hostname`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tunnels []models.Tunnel
	for rows.Next() {
		var tunnel models.Tunnel
		if err := rows.Scan(&tunnel.ID, &tunnel.AccountID, &tunnel.Hostname, &tunnel.Target, &tunnel.Access, &tunnel.Status, &tunnel.Region, &tunnel.Owner, &tunnel.Risk, &tunnel.CreatedAt, &tunnel.UpdatedAt); err != nil {
			return nil, err
		}
		tunnels = append(tunnels, tunnel)
	}
	return tunnels, rows.Err()
}

func (r *PostgresCustomerRepository) GetTunnelByID(ctx context.Context, accountID, tunnelID string) (*models.Tunnel, error) {
	var tunnel models.Tunnel
	if err := r.pool.QueryRow(ctx, `SELECT id, account_id, hostname, target, access, status, COALESCE(region, ''), COALESCE(owner, ''), COALESCE(risk, ''), created_at, updated_at FROM tunnels WHERE account_id = $1 AND id = $2`, accountID, tunnelID).Scan(&tunnel.ID, &tunnel.AccountID, &tunnel.Hostname, &tunnel.Target, &tunnel.Access, &tunnel.Status, &tunnel.Region, &tunnel.Owner, &tunnel.Risk, &tunnel.CreatedAt, &tunnel.UpdatedAt); err != nil {
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

func (r *PostgresCustomerRepository) CreateTunnel(ctx context.Context, accountID string, tunnel models.Tunnel) (*models.Tunnel, error) {
	var created models.Tunnel
	err := r.pool.QueryRow(ctx, `
		INSERT INTO tunnels (id, account_id, hostname, target, access, status, region, owner, risk, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, account_id, hostname, target, access, status, COALESCE(region, ''), COALESCE(owner, ''), COALESCE(risk, ''), created_at, updated_at
	`, tunnel.ID, accountID, tunnel.Hostname, tunnel.Target, tunnel.Access, tunnel.Status, tunnel.Region, tunnel.Owner, tunnel.Risk).Scan(
		&created.ID, &created.AccountID, &created.Hostname, &created.Target, &created.Access, &created.Status, &created.Region, &created.Owner, &created.Risk, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *PostgresCustomerRepository) UpdateTunnel(ctx context.Context, accountID, tunnelID string, tunnel models.Tunnel) (*models.Tunnel, error) {
	var updated models.Tunnel
	err := r.pool.QueryRow(ctx, `
		UPDATE tunnels
		SET hostname = COALESCE($3, hostname),
		    target = COALESCE($4, target),
		    access = COALESCE($5, access),
		    status = COALESCE($6, status),
		    region = COALESCE($7, region),
		    owner = COALESCE($8, owner),
		    risk = COALESCE($9, risk),
		    updated_at = NOW()
		WHERE account_id = $1 AND id = $2
		RETURNING id, account_id, hostname, target, access, status, COALESCE(region, ''), COALESCE(owner, ''), COALESCE(risk, ''), created_at, updated_at
	`, accountID, tunnelID, &tunnel.Hostname, &tunnel.Target, &tunnel.Access, &tunnel.Status, &tunnel.Region, &tunnel.Owner, &tunnel.Risk).Scan(
		&updated.ID, &updated.AccountID, &updated.Hostname, &updated.Target, &updated.Access, &updated.Status, &updated.Region, &updated.Owner, &updated.Risk, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *PostgresCustomerRepository) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM tunnels WHERE account_id = $1 AND id = $2`, accountID, tunnelID)
	return err
}

func (r *PostgresCustomerRepository) GetTunnelByHostname(ctx context.Context, hostname string) (*models.Tunnel, error) {
	var tunnel models.Tunnel
	err := r.pool.QueryRow(ctx, `
		SELECT id, account_id, hostname, target, access, status, COALESCE(region, ''), COALESCE(owner, ''), COALESCE(risk, ''), created_at, updated_at
		FROM tunnels WHERE hostname = $1
	`, hostname).Scan(
		&tunnel.ID, &tunnel.AccountID, &tunnel.Hostname, &tunnel.Target, &tunnel.Access, &tunnel.Status, &tunnel.Region, &tunnel.Owner, &tunnel.Risk, &tunnel.CreatedAt, &tunnel.UpdatedAt,
	)
	if err != nil {
		return nil, nil
	}
	return &tunnel, nil
}

func (r *PostgresCustomerRepository) GetRuntimeStatusByTunnelID(ctx context.Context, accountID, tunnelID string) (status string, degraded bool, observedAt *time.Time, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(status, ''), COALESCE(degraded, false), observed_at
		FROM runtime_tunnel_snapshots
		WHERE account_id = $1 AND tunnel_id = $2
		ORDER BY observed_at DESC
		LIMIT 1
	`, accountID, tunnelID).Scan(&status, &degraded, &observedAt)
	if err != nil {
		return "", false, nil, nil
	}
	return status, degraded, observedAt, nil
}

func (r *InMemoryCustomerRepository) ListInstallations(ctx context.Context, accountID string) ([]models.RuntimeInstallation, error) {
	return []models.RuntimeInstallation{}, nil
}

func (r *InMemoryCustomerRepository) GetRuntimeOverlayByTunnel(ctx context.Context, accountID, tunnelID string) (*RuntimeOverlay, error) {
	return &RuntimeOverlay{Status: "awaiting-runtime"}, nil
}

func (r *InMemoryCustomerRepository) CreateTunnel(ctx context.Context, accountID string, tunnel models.Tunnel) (*models.Tunnel, error) {
	return &tunnel, nil
}

func (r *InMemoryCustomerRepository) UpdateTunnel(ctx context.Context, accountID, tunnelID string, tunnel models.Tunnel) (*models.Tunnel, error) {
	return &tunnel, nil
}

func (r *InMemoryCustomerRepository) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	return nil
}

func (r *InMemoryCustomerRepository) GetTunnelByHostname(ctx context.Context, hostname string) (*models.Tunnel, error) {
	for _, t := range r.Tunnels {
		if t.Hostname == hostname {
			return &t, nil
		}
	}
	return nil, nil
}

func (r *InMemoryCustomerRepository) GetRuntimeStatusByTunnelID(ctx context.Context, accountID, tunnelID string) (status string, degraded bool, observedAt *time.Time, err error) {
	return "unknown", false, nil, nil
}
