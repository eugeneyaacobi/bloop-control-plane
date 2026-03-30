package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WebAuthnCredential represents a WebAuthn (security key) credential
type WebAuthnCredential struct {
	ID              string
	UserID          string
	CredentialID    []byte
	PublicKey       []byte
	AttestationType string
	AAGUID          []byte
	SignCount       int64
	Name            string
	Transports      []string
	LastUsedAt      *time.Time
	CreatedAt       time.Time
}

// WebAuthnChallenge represents an ephemeral WebAuthn challenge
type WebAuthnChallenge struct {
	ID        string
	UserID    string
	Challenge []byte
	Kind      string // "registration" or "authentication"
	ExpiresAt time.Time
	CreatedAt time.Time
}

// WebAuthnRepository handles WebAuthn credential and challenge storage
type WebAuthnRepository interface {
	// Credential management
	StoreCredential(ctx context.Context, cred WebAuthnCredential) error
	ListCredentialsByUser(ctx context.Context, userID string) ([]WebAuthnCredential, error)
	GetCredentialByID(ctx context.Context, credentialID string) (*WebAuthnCredential, error)
	GetCredentialByCredentialIDBytes(ctx context.Context, credentialID []byte) (*WebAuthnCredential, error)
	DeleteCredential(ctx context.Context, credentialID, userID string) error
	UpdateSignCount(ctx context.Context, credentialID string, signCount int64) error

	// Challenge management
	CreateChallenge(ctx context.Context, challenge WebAuthnChallenge) error
	GetChallenge(ctx context.Context, challengeID string) (*WebAuthnChallenge, error)
	DeleteChallenge(ctx context.Context, challengeID string) error
	CleanupExpiredChallenges(ctx context.Context) error
}

// PostgresWebAuthnRepository implements WebAuthnRepository for PostgreSQL
type PostgresWebAuthnRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresWebAuthnRepository creates a new PostgreSQL-based WebAuthn repository
func NewPostgresWebAuthnRepository(pool *pgxpool.Pool) *PostgresWebAuthnRepository {
	return &PostgresWebAuthnRepository{pool: pool}
}

// StoreCredential stores a new WebAuthn credential
func (r *PostgresWebAuthnRepository) StoreCredential(ctx context.Context, cred WebAuthnCredential) error {
	if cred.ID == "" {
		cred.ID = uuid.New().String()
	}
	if cred.CreatedAt.IsZero() {
		cred.CreatedAt = time.Now().UTC()
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO webauthn_credentials
		(id, user_id, credential_id, public_key, attestation_type, aaguid, sign_count, name, transports, last_used_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, cred.ID, cred.UserID, cred.CredentialID, cred.PublicKey, cred.AttestationType, cred.AAGUID, cred.SignCount, cred.Name, cred.Transports, cred.LastUsedAt, cred.CreatedAt)

	return err
}

// ListCredentialsByUser lists all credentials for a user
func (r *PostgresWebAuthnRepository) ListCredentialsByUser(ctx context.Context, userID string) ([]WebAuthnCredential, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, credential_id, public_key, attestation_type, aaguid, sign_count, name, transports, last_used_at, created_at
		FROM webauthn_credentials
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var credentials []WebAuthnCredential
	for rows.Next() {
		var cred WebAuthnCredential
		err := rows.Scan(&cred.ID, &cred.UserID, &cred.CredentialID, &cred.PublicKey, &cred.AttestationType, &cred.AAGUID, &cred.SignCount, &cred.Name, &cred.Transports, &cred.LastUsedAt, &cred.CreatedAt)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, cred)
	}

	return credentials, rows.Err()
}

// GetCredentialByID retrieves a credential by its database ID
func (r *PostgresWebAuthnRepository) GetCredentialByID(ctx context.Context, credentialID string) (*WebAuthnCredential, error) {
	var cred WebAuthnCredential
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, credential_id, public_key, attestation_type, aaguid, sign_count, name, transports, last_used_at, created_at
		FROM webauthn_credentials
		WHERE id = $1
	`, credentialID).Scan(&cred.ID, &cred.UserID, &cred.CredentialID, &cred.PublicKey, &cred.AttestationType, &cred.AAGUID, &cred.SignCount, &cred.Name, &cred.Transports, &cred.LastUsedAt, &cred.CreatedAt)

	if err != nil {
		return nil, nil
	}

	return &cred, nil
}

// GetCredentialByCredentialIDBytes retrieves a credential by its credential ID bytes
func (r *PostgresWebAuthnRepository) GetCredentialByCredentialIDBytes(ctx context.Context, credentialID []byte) (*WebAuthnCredential, error) {
	var cred WebAuthnCredential
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, credential_id, public_key, attestation_type, aaguid, sign_count, name, transports, last_used_at, created_at
		FROM webauthn_credentials
		WHERE credential_id = $1
	`, credentialID).Scan(&cred.ID, &cred.UserID, &cred.CredentialID, &cred.PublicKey, &cred.AttestationType, &cred.AAGUID, &cred.SignCount, &cred.Name, &cred.Transports, &cred.LastUsedAt, &cred.CreatedAt)

	if err != nil {
		return nil, nil
	}

	return &cred, nil
}

// DeleteCredential deletes a credential with ownership check
func (r *PostgresWebAuthnRepository) DeleteCredential(ctx context.Context, credentialID, userID string) error {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM webauthn_credentials
		WHERE id = $1 AND user_id = $2
	`, credentialID, userID)

	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return nil
	}

	return nil
}

// UpdateSignCount updates the sign count for a credential (for cloned key detection)
func (r *PostgresWebAuthnRepository) UpdateSignCount(ctx context.Context, credentialID string, signCount int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE webauthn_credentials
		SET sign_count = $2, last_used_at = NOW()
		WHERE id = $1
	`, credentialID, signCount)
	return err
}

// CreateChallenge stores a new WebAuthn challenge
func (r *PostgresWebAuthnRepository) CreateChallenge(ctx context.Context, challenge WebAuthnChallenge) error {
	if challenge.ID == "" {
		challenge.ID = uuid.New().String()
	}
	if challenge.CreatedAt.IsZero() {
		challenge.CreatedAt = time.Now().UTC()
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO webauthn_challenges
		(id, user_id, challenge, kind, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, challenge.ID, challenge.UserID, challenge.Challenge, challenge.Kind, challenge.ExpiresAt, challenge.CreatedAt)

	return err
}

// GetChallenge retrieves a challenge by ID
func (r *PostgresWebAuthnRepository) GetChallenge(ctx context.Context, challengeID string) (*WebAuthnChallenge, error) {
	var challenge WebAuthnChallenge
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, challenge, kind, expires_at, created_at
		FROM webauthn_challenges
		WHERE id = $1
	`, challengeID).Scan(&challenge.ID, &challenge.UserID, &challenge.Challenge, &challenge.Kind, &challenge.ExpiresAt, &challenge.CreatedAt)

	if err != nil {
		return nil, nil
	}

	return &challenge, nil
}

// DeleteChallenge deletes a challenge by ID
func (r *PostgresWebAuthnRepository) DeleteChallenge(ctx context.Context, challengeID string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM webauthn_challenges
		WHERE id = $1
	`, challengeID)
	return err
}

// CleanupExpiredChallenges removes expired challenges
func (r *PostgresWebAuthnRepository) CleanupExpiredChallenges(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM webauthn_challenges
		WHERE expires_at < NOW()
	`)
	return err
}

// InMemoryWebAuthnRepository is an in-memory implementation for testing
type InMemoryWebAuthnRepository struct {
	credentials map[string]WebAuthnCredential
	challenges  map[string]WebAuthnChallenge
}

// NewInMemoryWebAuthnRepository creates a new in-memory WebAuthn repository
func NewInMemoryWebAuthnRepository() *InMemoryWebAuthnRepository {
	return &InMemoryWebAuthnRepository{
		credentials: make(map[string]WebAuthnCredential),
		challenges:  make(map[string]WebAuthnChallenge),
	}
}

// StoreCredential stores a credential in memory
func (r *InMemoryWebAuthnRepository) StoreCredential(ctx context.Context, cred WebAuthnCredential) error {
	if cred.ID == "" {
		cred.ID = uuid.New().String()
	}
	if cred.CreatedAt.IsZero() {
		cred.CreatedAt = time.Now().UTC()
	}
	r.credentials[cred.ID] = cred
	return nil
}

// ListCredentialsByUser lists credentials for a user
func (r *InMemoryWebAuthnRepository) ListCredentialsByUser(ctx context.Context, userID string) ([]WebAuthnCredential, error) {
	var result []WebAuthnCredential
	for _, cred := range r.credentials {
		if cred.UserID == userID {
			result = append(result, cred)
		}
	}
	return result, nil
}

// GetCredentialByID retrieves a credential by ID
func (r *InMemoryWebAuthnRepository) GetCredentialByID(ctx context.Context, credentialID string) (*WebAuthnCredential, error) {
	cred, exists := r.credentials[credentialID]
	if !exists {
		return nil, nil
	}
	return &cred, nil
}

// GetCredentialByCredentialIDBytes retrieves a credential by credential ID bytes
func (r *InMemoryWebAuthnRepository) GetCredentialByCredentialIDBytes(ctx context.Context, credentialID []byte) (*WebAuthnCredential, error) {
	for _, cred := range r.credentials {
		if string(cred.CredentialID) == string(credentialID) {
			return &cred, nil
		}
	}
	return nil, nil
}

// DeleteCredential deletes a credential
func (r *InMemoryWebAuthnRepository) DeleteCredential(ctx context.Context, credentialID, userID string) error {
	cred, exists := r.credentials[credentialID]
	if !exists || cred.UserID != userID {
		return nil
	}
	delete(r.credentials, credentialID)
	return nil
}

// UpdateSignCount updates the sign count
func (r *InMemoryWebAuthnRepository) UpdateSignCount(ctx context.Context, credentialID string, signCount int64) error {
	cred, exists := r.credentials[credentialID]
	if !exists {
		return nil
	}
	now := time.Now().UTC()
	cred.SignCount = signCount
	cred.LastUsedAt = &now
	r.credentials[credentialID] = cred
	return nil
}

// CreateChallenge stores a challenge
func (r *InMemoryWebAuthnRepository) CreateChallenge(ctx context.Context, challenge WebAuthnChallenge) error {
	if challenge.ID == "" {
		challenge.ID = uuid.New().String()
	}
	if challenge.CreatedAt.IsZero() {
		challenge.CreatedAt = time.Now().UTC()
	}
	r.challenges[challenge.ID] = challenge
	return nil
}

// GetChallenge retrieves a challenge
func (r *InMemoryWebAuthnRepository) GetChallenge(ctx context.Context, challengeID string) (*WebAuthnChallenge, error) {
	challenge, exists := r.challenges[challengeID]
	if !exists {
		return nil, nil
	}
	return &challenge, nil
}

// DeleteChallenge deletes a challenge
func (r *InMemoryWebAuthnRepository) DeleteChallenge(ctx context.Context, challengeID string) error {
	delete(r.challenges, challengeID)
	return nil
}

// CleanupExpiredChallenges removes expired challenges
func (r *InMemoryWebAuthnRepository) CleanupExpiredChallenges(ctx context.Context) error {
	now := time.Now().UTC()
	for id, challenge := range r.challenges {
		if challenge.ExpiresAt.Before(now) {
			delete(r.challenges, id)
		}
	}
	return nil
}
