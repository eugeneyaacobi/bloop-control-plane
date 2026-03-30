package repository

import (
	"context"
	"fmt"
	"time"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRuntimeRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRuntimeRepository(pool *pgxpool.Pool) *PostgresRuntimeRepository {
	return &PostgresRuntimeRepository{pool: pool}
}

func (r *PostgresRuntimeRepository) ProjectAccount(ctx context.Context, account models.Account, tunnels []models.Tunnel) (runtime.AccountProjection, error) {
	projection := runtime.AccountProjection{}
	rows, err := r.pool.Query(ctx, `
		SELECT tunnel_id, access_mode, status, degraded
		FROM runtime_tunnel_snapshots
		WHERE account_id = $1
		ORDER BY observed_at DESC`, account.ID)
	if err == nil {
		count := 0
		for rows.Next() {
			var tunnelID, accessMode, status string
			var degraded bool
			if err := rows.Scan(&tunnelID, &accessMode, &status, &degraded); err != nil {
				return runtime.AccountProjection{}, err
			}
			count++
			projection.ActiveRoutes++
			if accessMode != "public" {
				projection.ProtectedRoutes++
			}
			if degraded || status != "healthy" {
				projection.DegradedRoutes++
			}
		}
		rows.Close()
		if count == 0 {
			for _, tunnel := range tunnels {
				projection.ActiveRoutes++
				if tunnel.Access != "public" {
					projection.ProtectedRoutes++
				}
				if tunnel.Status != "healthy" {
					projection.DegradedRoutes++
				}
			}
		}
	} else {
		for _, tunnel := range tunnels {
			projection.ActiveRoutes++
			if tunnel.Access != "public" {
				projection.ProtectedRoutes++
			}
			if tunnel.Status != "healthy" {
				projection.DegradedRoutes++
			}
		}
	}

	rows, err = r.pool.Query(ctx, `
		SELECT id, kind, message, level
		FROM runtime_events
		WHERE account_id = $1
		ORDER BY occurred_at DESC
		LIMIT 5`, account.ID)
	if err == nil {
		for rows.Next() {
			var id, kind, message, level string
			if err := rows.Scan(&id, &kind, &message, &level); err != nil {
				return runtime.AccountProjection{}, err
			}
			projection.RecentActivity = append(projection.RecentActivity, runtime.Activity{ID: id, Level: level, Message: fmt.Sprintf("%s: %s", kind, message)})
		}
		rows.Close()
	}
	if len(projection.RecentActivity) > 0 {
		return projection, nil
	}

	rows, err = r.pool.Query(ctx, `
		SELECT id, step_key, title, state
		FROM onboarding_steps
		WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT 5`, account.ID)
	if err != nil {
		return runtime.AccountProjection{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, stepKey, title, state string
		if err := rows.Scan(&id, &stepKey, &title, &state); err != nil {
			return runtime.AccountProjection{}, err
		}
		projection.RecentActivity = append(projection.RecentActivity, runtime.Activity{
			ID:      fmt.Sprintf("onboarding-%s", id),
			Level:   levelForOnboardingState(state),
			Message: fmt.Sprintf("Onboarding %s: %s (%s)", stepKey, title, state),
		})
	}
	if err := rows.Err(); err != nil {
		return runtime.AccountProjection{}, err
	}
	if len(projection.RecentActivity) == 0 {
		projection.RecentActivity = []runtime.Activity{{
			ID:      fmt.Sprintf("acct-%s-routes", account.ID),
			Level:   "info",
			Message: fmt.Sprintf("%s currently has %d routes in view", account.DisplayName, projection.ActiveRoutes),
		}}
	}
	return projection, nil
}

func (r *PostgresRuntimeRepository) ProjectGlobal(ctx context.Context, tunnels []models.Tunnel, flags []models.ReviewFlag) (runtime.GlobalProjection, error) {
	projection := runtime.GlobalProjection{FlaggedExposures: len(flags)}

	rows, err := r.pool.Query(ctx, `
		SELECT id, kind, message, level
		FROM runtime_events
		ORDER BY occurred_at DESC
		LIMIT 5`)
	if err == nil {
		for rows.Next() {
			var id, kind, message, level string
			if err := rows.Scan(&id, &kind, &message, &level); err != nil {
				return runtime.GlobalProjection{}, err
			}
			projection.RecentActivity = append(projection.RecentActivity, runtime.Activity{ID: id, Level: level, Message: fmt.Sprintf("%s: %s", kind, message)})
		}
		rows.Close()
	}
	var runtimeFlagged int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM runtime_tunnel_snapshots WHERE access_mode = 'public' OR degraded = true OR status <> 'healthy'`).Scan(&runtimeFlagged); err == nil && runtimeFlagged > projection.FlaggedExposures {
		projection.FlaggedExposures = runtimeFlagged
	}
	if len(projection.RecentActivity) > 0 {
		return projection, nil
	}

	rows, err = r.pool.Query(ctx, `
		SELECT id, item, reason, severity
		FROM review_flags
		ORDER BY created_at DESC
		LIMIT 5`)
	if err != nil {
		return runtime.GlobalProjection{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, item, reason, severity string
		if err := rows.Scan(&id, &item, &reason, &severity); err != nil {
			return runtime.GlobalProjection{}, err
		}
		projection.RecentActivity = append(projection.RecentActivity, runtime.Activity{
			ID:      id,
			Level:   severityLevel(severity),
			Message: fmt.Sprintf("Review %s: %s", item, reason),
		})
	}
	if err := rows.Err(); err != nil {
		return runtime.GlobalProjection{}, err
	}
	if len(projection.RecentActivity) == 0 && len(tunnels) > 0 {
		for _, tunnel := range tunnels {
			if tunnel.Access == "public" {
				projection.RecentActivity = []runtime.Activity{{
					ID:      fmt.Sprintf("public-%s", tunnel.ID),
					Level:   "warn",
					Message: fmt.Sprintf("Public route live: %s -> %s", tunnel.Hostname, tunnel.Target),
				}}
				break
			}
		}
	}
	return projection, nil
}

func levelForOnboardingState(state string) string {
	switch state {
	case "blocked", "error":
		return "critical"
	case "active", "pending":
		return "warn"
	default:
		return "info"
	}
}

func severityLevel(severity string) string {
	switch severity {
	case "critical", "high", "elevated":
		return "critical"
	case "medium", "warn":
		return "warn"
	default:
		return "info"
	}
}

func (r *PostgresRuntimeRepository) CreateIngestToken(ctx context.Context, installationID string) (string, error) {
	token := fmt.Sprintf("ingest_%s_%d", installationID, time.Now().UnixNano())
	_, err := r.pool.Exec(ctx, `INSERT INTO runtime_ingest_tokens (installation_id, token, created_at) VALUES ($1, $2, NOW())`, installationID, token)
	if err != nil {
		return "", fmt.Errorf("create ingest token: %w", err)
	}
	return token, nil
}

func (r *PostgresRuntimeRepository) VerifyInstallationToken(ctx context.Context, token string) (*models.RuntimeInstallationToken, error) {
	var rit models.RuntimeInstallationToken
	err := r.pool.QueryRow(ctx, `SELECT id, installation_id, kind FROM runtime_installation_tokens WHERE token = $1 AND revoked_at IS NULL`, token).Scan(&rit.ID, &rit.InstallationID, &rit.Kind)
	if err != nil {
		return nil, fmt.Errorf("verify installation token: %w", err)
	}
	return &rit, nil
}
