package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuthAuditEvent represents an authentication event in the audit log
type AuthAuditEvent struct {
	ID        string
	UserID    *string
	AccountID *string
	Event     string
	IPAddress *string
	UserAgent *string
	Success   bool
	Metadata  map[string]interface{}
	CreatedAt time.Time
}

// AuditRepository handles authentication audit logging
type AuditRepository interface {
	LogAuthEvent(ctx context.Context, event AuthAuditEvent) error
	GetRecentEvents(ctx context.Context, userID *string, limit int) ([]AuthAuditEvent, error)
}

// PostgresAuditRepository implements AuditRepository for PostgreSQL
type PostgresAuditRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAuditRepository creates a new PostgreSQL-based audit repository
func NewPostgresAuditRepository(pool *pgxpool.Pool) *PostgresAuditRepository {
	return &PostgresAuditRepository{pool: pool}
}

// LogAuthEvent logs an authentication event to the audit log
// All auth events (login, logout, token creation, token revocation, 2FA enable/disable, failed attempts) are logged
func (r *PostgresAuditRepository) LogAuthEvent(ctx context.Context, event AuthAuditEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	var metadataJSON []byte
	var err error
	if event.Metadata != nil {
		metadataJSON, err = json.Marshal(event.Metadata)
		if err != nil {
			return err
		}
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO auth_audit_log (id, user_id, account_id, event, ip_address, user_agent, success, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, event.ID, event.UserID, event.AccountID, event.Event, event.IPAddress, event.UserAgent, event.Success, metadataJSON, event.CreatedAt)
	return err
}

// GetRecentEvents retrieves recent audit events, optionally filtered by user
func (r *PostgresAuditRepository) GetRecentEvents(ctx context.Context, userID *string, limit int) ([]AuthAuditEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000 // Prevent unbounded queries
	}

	var rows pgx.Rows
	var err error

	if userID != nil {
		rows, err = r.pool.Query(ctx, `
			SELECT id, user_id, account_id, event, ip_address, user_agent, success, metadata, created_at
			FROM auth_audit_log
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		`, *userID, limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, user_id, account_id, event, ip_address, user_agent, success, metadata, created_at
			FROM auth_audit_log
			ORDER BY created_at DESC
			LIMIT $1
		`, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AuthAuditEvent
	for rows.Next() {
		var event AuthAuditEvent
		var metadataJSON []byte
		err := rows.Scan(&event.ID, &event.UserID, &event.AccountID, &event.Event, &event.IPAddress, &event.UserAgent, &event.Success, &metadataJSON, &event.CreatedAt)
		if err != nil {
			return nil, err
		}

		if metadataJSON != nil {
			err = json.Unmarshal(metadataJSON, &event.Metadata)
			if err != nil {
				return nil, err
			}
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

// InMemoryAuditRepository is an in-memory implementation for testing
type InMemoryAuditRepository struct {
	events []AuthAuditEvent
}

// NewInMemoryAuditRepository creates a new in-memory audit repository
func NewInMemoryAuditRepository() *InMemoryAuditRepository {
	return &InMemoryAuditRepository{
		events: make([]AuthAuditEvent, 0),
	}
}

// LogAuthEvent logs an event to memory
func (r *InMemoryAuditRepository) LogAuthEvent(ctx context.Context, event AuthAuditEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	r.events = append(r.events, event)
	return nil
}

// GetRecentEvents retrieves events from memory
func (r *InMemoryAuditRepository) GetRecentEvents(ctx context.Context, userID *string, limit int) ([]AuthAuditEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	var filtered []AuthAuditEvent
	for _, e := range r.events {
		if userID == nil || (e.UserID != nil && *e.UserID == *userID) {
			filtered = append(filtered, e)
		}
	}

	// Return most recent first
	start := 0
	if len(filtered) > limit {
		start = len(filtered) - limit
	}

	result := make([]AuthAuditEvent, 0, limit)
	for i := len(filtered) - 1; i >= start; i-- {
		result = append(result, filtered[i])
	}

	return result, nil
}
