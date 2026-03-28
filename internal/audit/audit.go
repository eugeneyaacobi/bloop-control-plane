package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Recorder struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Recorder {
	return &Recorder{pool: pool}
}

func (r *Recorder) Record(ctx context.Context, eventType, actorID, targetType, targetID string, metadata string) error {
	if r == nil || r.pool == nil {
		return nil
	}
	id := fmt.Sprintf("ae_%d", time.Now().UTC().UnixNano())
	_, err := r.pool.Exec(ctx, `INSERT INTO audit_events (id, event_type, actor_id, target_type, target_id, metadata) VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6::jsonb)`, id, eventType, actorID, targetType, targetID, metadata)
	return err
}
