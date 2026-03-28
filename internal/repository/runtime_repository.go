package repository

import (
	"context"
	"fmt"

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
	for _, tunnel := range tunnels {
		projection.ActiveRoutes++
		if tunnel.Access != "public" {
			projection.ProtectedRoutes++
		}
		if tunnel.Status != "healthy" {
			projection.DegradedRoutes++
		}
	}

	rows, err := r.pool.Query(ctx, `
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
