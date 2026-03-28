package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OnboardingStep struct {
	ID      string `json:"id"`
	StepKey string `json:"stepKey"`
	Title   string `json:"title"`
	Detail  string `json:"detail"`
	State   string `json:"state"`
}

type OnboardingRepository interface {
	ListSteps(ctx context.Context, accountID string) ([]OnboardingStep, error)
}

type PostgresOnboardingRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOnboardingRepository(pool *pgxpool.Pool) *PostgresOnboardingRepository {
	return &PostgresOnboardingRepository{pool: pool}
}

func (r *PostgresOnboardingRepository) ListSteps(ctx context.Context, accountID string) ([]OnboardingStep, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, step_key, title, detail, state FROM onboarding_steps WHERE account_id = $1 ORDER BY created_at ASC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []OnboardingStep
	for rows.Next() {
		var step OnboardingStep
		if err := rows.Scan(&step.ID, &step.StepKey, &step.Title, &step.Detail, &step.State); err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}
