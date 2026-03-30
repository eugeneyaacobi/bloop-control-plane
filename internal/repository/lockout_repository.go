package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LockoutRepository handles account lockout state and login attempt tracking
type LockoutRepository interface {
	// RecordLoginAttempt records a login attempt (success or failure)
	RecordLoginAttempt(ctx context.Context, identifier, ipAddress string, success bool) error

	// GetFailedAttemptCount counts failed login attempts since a given time
	GetFailedAttemptCount(ctx context.Context, identifier string, since time.Time) (int, error)

	// GetIPFailedAttemptCount counts failed login attempts from an IP since a given time
	GetIPFailedAttemptCount(ctx context.Context, ipAddress string, since time.Time) (int, error)

	// LockAccount locks an account until a given time
	LockAccount(ctx context.Context, userID string, lockedUntil time.Time, lockedBy string) error

	// IsAccountLocked checks if an account is currently locked
	IsAccountLocked(ctx context.Context, userID string) (bool, *time.Time, error)

	// UnlockAccount unlocks an account
	UnlockAccount(ctx context.Context, userID string) error

	// IncrementFailedCount increments the failed count for a user and locks if threshold reached
	IncrementFailedCount(ctx context.Context, userID string, threshold int, lockoutDuration time.Duration) (int, error)
}

// PostgresLockoutRepository implements LockoutRepository for PostgreSQL
type PostgresLockoutRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresLockoutRepository creates a new PostgreSQL-based lockout repository
func NewPostgresLockoutRepository(pool *pgxpool.Pool) *PostgresLockoutRepository {
	return &PostgresLockoutRepository{pool: pool}
}

// RecordLoginAttempt records a login attempt in the login_attempts table
func (r *PostgresLockoutRepository) RecordLoginAttempt(ctx context.Context, identifier, ipAddress string, success bool) error {
	id := uuid.New().String()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO login_attempts (id, identifier, ip_address, success, attempted_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, id, identifier, ipAddress, success)
	return err
}

// GetFailedAttemptCount counts failed login attempts for an identifier since a given time
func (r *PostgresLockoutRepository) GetFailedAttemptCount(ctx context.Context, identifier string, since time.Time) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM login_attempts
		WHERE identifier = $1 AND success = false AND attempted_at >= $2
	`, identifier, since).Scan(&count)
	return count, err
}

// GetIPFailedAttemptCount counts failed login attempts from an IP since a given time
func (r *PostgresLockoutRepository) GetIPFailedAttemptCount(ctx context.Context, ipAddress string, since time.Time) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM login_attempts
		WHERE ip_address = $1 AND success = false AND attempted_at >= $2
	`, ipAddress, since).Scan(&count)
	return count, err
}

// LockAccount locks an account until a given time
func (r *PostgresLockoutRepository) LockAccount(ctx context.Context, userID string, lockedUntil time.Time, lockedBy string) error {
	// Use INSERT ... ON CONFLICT to handle both new and existing lockouts
	id := uuid.New().String()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO account_lockouts (id, user_id, locked_until, failed_count, last_failed_at, locked_by)
		VALUES ($1, $2, $3, 0, NOW(), $4)
		ON CONFLICT (user_id) DO UPDATE
		SET locked_until = EXCLUDED.locked_until,
		    failed_count = EXCLUDED.failed_count,
		    last_failed_at = EXCLUDED.last_failed_at,
		    locked_by = EXCLUDED.locked_by
	`, id, userID, lockedUntil, lockedBy)
	return err
}

// IsAccountLocked checks if an account is currently locked
func (r *PostgresLockoutRepository) IsAccountLocked(ctx context.Context, userID string) (bool, *time.Time, error) {
	var lockedUntil time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT locked_until FROM account_lockouts
		WHERE user_id = $1 AND locked_until > NOW()
	`, userID).Scan(&lockedUntil)

	if err != nil {
		// No rows means not locked
		return false, nil, nil
	}

	return true, &lockedUntil, nil
}

// UnlockAccount unlocks an account
func (r *PostgresLockoutRepository) UnlockAccount(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM account_lockouts WHERE user_id = $1
	`, userID)
	return err
}

// IncrementFailedCount increments the failed count and locks if threshold reached
func (r *PostgresLockoutRepository) IncrementFailedCount(ctx context.Context, userID string, threshold int, lockoutDuration time.Duration) (int, error) {
	id := uuid.New().String()
	lockedUntil := time.Now().UTC().Add(lockoutDuration)

	var newCount int
	err := r.pool.QueryRow(ctx, `
		INSERT INTO account_lockouts (id, user_id, locked_until, failed_count, last_failed_at, locked_by)
		VALUES ($1, $2, NULL, 1, NOW(), 'failed_login')
		ON CONFLICT (user_id) DO UPDATE
		SET failed_count = account_lockouts.failed_count + 1,
		    last_failed_at = EXCLUDED.last_failed_at,
		    locked_until = CASE
		        WHEN (account_lockouts.failed_count + 1) >= $3 THEN $4
		        ELSE account_lockouts.locked_until
		    END
		RETURNING failed_count
	`, id, userID, threshold, lockedUntil).Scan(&newCount)

	return newCount, err
}

// InMemoryLockoutRepository is an in-memory implementation for testing
type InMemoryLockoutRepository struct {
	attempts   []LoginAttempt
	lockouts   map[string]AccountLockout
}

// LoginAttempt represents a login attempt
type LoginAttempt struct {
	ID         string
	Identifier string
	IPAddress  string
	Success    bool
	AttemptedAt time.Time
}

// AccountLockout represents an account lockout state
type AccountLockout struct {
	ID          string
	UserID      string
	LockedUntil *time.Time
	FailedCount int
	LastFailedAt *time.Time
	LockedBy    string
}

// NewInMemoryLockoutRepository creates a new in-memory lockout repository
func NewInMemoryLockoutRepository() *InMemoryLockoutRepository {
	return &InMemoryLockoutRepository{
		attempts: make([]LoginAttempt, 0),
		lockouts: make(map[string]AccountLockout),
	}
}

// RecordLoginAttempt records a login attempt in memory
func (r *InMemoryLockoutRepository) RecordLoginAttempt(ctx context.Context, identifier, ipAddress string, success bool) error {
	r.attempts = append(r.attempts, LoginAttempt{
		ID:         uuid.New().String(),
		Identifier: identifier,
		IPAddress:  ipAddress,
		Success:    success,
		AttemptedAt: time.Now().UTC(),
	})
	return nil
}

// GetFailedAttemptCount counts failed attempts since a given time
func (r *InMemoryLockoutRepository) GetFailedAttemptCount(ctx context.Context, identifier string, since time.Time) (int, error) {
	count := 0
	for _, a := range r.attempts {
		if a.Identifier == identifier && !a.Success && a.AttemptedAt.After(since) {
			count++
		}
	}
	return count, nil
}

// GetIPFailedAttemptCount counts failed attempts from an IP since a given time
func (r *InMemoryLockoutRepository) GetIPFailedAttemptCount(ctx context.Context, ipAddress string, since time.Time) (int, error) {
	count := 0
	for _, a := range r.attempts {
		if a.IPAddress == ipAddress && !a.Success && a.AttemptedAt.After(since) {
			count++
		}
	}
	return count, nil
}

// LockAccount locks an account in memory
func (r *InMemoryLockoutRepository) LockAccount(ctx context.Context, userID string, lockedUntil time.Time, lockedBy string) error {
	r.lockouts[userID] = AccountLockout{
		ID:          uuid.New().String(),
		UserID:      userID,
		LockedUntil: &lockedUntil,
		FailedCount: 0,
		LastFailedAt: nil,
		LockedBy:    lockedBy,
	}
	return nil
}

// IsAccountLocked checks if an account is locked
func (r *InMemoryLockoutRepository) IsAccountLocked(ctx context.Context, userID string) (bool, *time.Time, error) {
	lockout, exists := r.lockouts[userID]
	if !exists || lockout.LockedUntil == nil {
		return false, nil, nil
	}

	if lockout.LockedUntil.Before(time.Now().UTC()) {
		// Lockout has expired
		delete(r.lockouts, userID)
		return false, nil, nil
	}

	return true, lockout.LockedUntil, nil
}

// UnlockAccount unlocks an account in memory
func (r *InMemoryLockoutRepository) UnlockAccount(ctx context.Context, userID string) error {
	delete(r.lockouts, userID)
	return nil
}

// IncrementFailedCount increments failed count and locks if threshold reached
func (r *InMemoryLockoutRepository) IncrementFailedCount(ctx context.Context, userID string, threshold int, lockoutDuration time.Duration) (int, error) {
	lockout, exists := r.lockouts[userID]
	if !exists {
		now := time.Now().UTC()
		lockout = AccountLockout{
			ID:          uuid.New().String(),
			UserID:      userID,
			LockedUntil: nil,
			FailedCount: 0,
			LastFailedAt: &now,
			LockedBy:    "failed_login",
		}
	}

	lockout.FailedCount++
	now := time.Now().UTC()
	lockout.LastFailedAt = &now

	if lockout.FailedCount >= threshold {
		lockedUntil := now.Add(lockoutDuration)
		lockout.LockedUntil = &lockedUntil
	}

	r.lockouts[userID] = lockout
	return lockout.FailedCount, nil
}
