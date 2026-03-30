package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserCredential represents a user's password credential
type UserCredential struct {
	ID                string
	UserID            string
	PasswordHash      string
	PasswordAlgorithm string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// UserWithCredentials represents a user with their credential data
type UserWithCredentials struct {
	ID               string
	Email            string
	Username         *string
	DisplayName      string
	PasswordSet      bool
	WebAuthnEnabled  bool
	Credential       *UserCredential
}

// AuthRepository handles user authentication data
type AuthRepository interface {
	// CreateUserWithCredentials creates a user with password credentials in a transaction
	CreateUserWithCredentials(ctx context.Context, email, username, passwordHash, displayName string) (UserWithCredentials, error)

	// GetUserByEmail retrieves a user by email
	GetUserByEmail(ctx context.Context, email string) (*UserWithCredentials, error)

	// GetUserByUsername retrieves a user by username
	GetUserByUsername(ctx context.Context, username string) (*UserWithCredentials, error)

	// GetUserByID retrieves a user by ID
	GetUserByID(ctx context.Context, userID string) (*UserWithCredentials, error)

	// GetCredentialsByUserID retrieves credentials for a user
	GetCredentialsByUserID(ctx context.Context, userID string) (*UserCredential, error)

	// UpdatePasswordHash updates a user's password hash
	UpdatePasswordHash(ctx context.Context, userID, passwordHash string) error

	// SetPasswordSet sets the password_set flag on a user
	SetPasswordSet(ctx context.Context, userID string, passwordSet bool) error

	// UpdateUsername updates a user's username
	UpdateUsername(ctx context.Context, userID, username string) error

	// SetWebAuthnEnabled sets the webauthn_enabled flag on a user
	SetWebAuthnEnabled(ctx context.Context, userID string, enabled bool) error
}

// PostgresAuthRepository implements AuthRepository for PostgreSQL
type PostgresAuthRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAuthRepository creates a new PostgreSQL-based auth repository
func NewPostgresAuthRepository(pool *pgxpool.Pool) *PostgresAuthRepository {
	return &PostgresAuthRepository{pool: pool}
}

// CreateUserWithCredentials creates a user with password credentials in a transaction
func (r *PostgresAuthRepository) CreateUserWithCredentials(ctx context.Context, email, username, passwordHash, displayName string) (UserWithCredentials, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return UserWithCredentials{}, err
	}
	defer tx.Rollback(ctx)

	userID := uuid.New().String()
	credID := uuid.New().String()
	now := time.Now().UTC()

	// Insert user
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, email, username, display_name, password_set, webauthn_enabled)
		VALUES ($1, $2, $3, $4, true, false)
	`, userID, email, username, displayName)
	if err != nil {
		return UserWithCredentials{}, err
	}

	// Insert credential
	_, err = tx.Exec(ctx, `
		INSERT INTO user_credentials (id, user_id, password_hash, password_algorithm, created_at, updated_at)
		VALUES ($1, $2, $3, 'argon2id', $4, $4)
	`, credID, userID, passwordHash, now)
	if err != nil {
		return UserWithCredentials{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return UserWithCredentials{}, err
	}

	return UserWithCredentials{
		ID:              userID,
		Email:           email,
		Username:        &username,
		DisplayName:     displayName,
		PasswordSet:     true,
		WebAuthnEnabled: false,
		Credential: &UserCredential{
			ID:                credID,
			UserID:            userID,
			PasswordHash:      passwordHash,
			PasswordAlgorithm: "argon2id",
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}, nil
}

// GetUserByEmail retrieves a user by email with optional credentials
func (r *PostgresAuthRepository) GetUserByEmail(ctx context.Context, email string) (*UserWithCredentials, error) {
	var user UserWithCredentials
	var username *string

	err := r.pool.QueryRow(ctx, `
		SELECT id, email, username, display_name, password_set, webauthn_enabled
		FROM users WHERE email = $1
	`, email).Scan(&user.ID, &user.Email, &username, &user.DisplayName, &user.PasswordSet, &user.WebAuthnEnabled)

	if err != nil {
		return nil, nil
	}

	user.Username = username

	// Load credentials if password is set
	if user.PasswordSet {
		cred, err := r.GetCredentialsByUserID(ctx, user.ID)
		if err == nil {
			user.Credential = cred
		}
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by username with optional credentials
func (r *PostgresAuthRepository) GetUserByUsername(ctx context.Context, username string) (*UserWithCredentials, error) {
	var user UserWithCredentials
	var uname *string

	err := r.pool.QueryRow(ctx, `
		SELECT id, email, username, display_name, password_set, webauthn_enabled
		FROM users WHERE username = $1
	`, username).Scan(&user.ID, &user.Email, &uname, &user.DisplayName, &user.PasswordSet, &user.WebAuthnEnabled)

	if err != nil {
		return nil, nil
	}

	user.Username = uname

	// Load credentials if password is set
	if user.PasswordSet {
		cred, err := r.GetCredentialsByUserID(ctx, user.ID)
		if err == nil {
			user.Credential = cred
		}
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID with optional credentials
func (r *PostgresAuthRepository) GetUserByID(ctx context.Context, userID string) (*UserWithCredentials, error) {
	var user UserWithCredentials
	var username *string

	err := r.pool.QueryRow(ctx, `
		SELECT id, email, username, display_name, password_set, webauthn_enabled
		FROM users WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &username, &user.DisplayName, &user.PasswordSet, &user.WebAuthnEnabled)

	if err != nil {
		return nil, nil
	}

	user.Username = username

	// Load credentials if password is set
	if user.PasswordSet {
		cred, err := r.GetCredentialsByUserID(ctx, user.ID)
		if err == nil {
			user.Credential = cred
		}
	}

	return &user, nil
}

// GetCredentialsByUserID retrieves credentials for a user
func (r *PostgresAuthRepository) GetCredentialsByUserID(ctx context.Context, userID string) (*UserCredential, error) {
	var cred UserCredential
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, password_hash, password_algorithm, created_at, updated_at
		FROM user_credentials WHERE user_id = $1
	`, userID).Scan(&cred.ID, &cred.UserID, &cred.PasswordHash, &cred.PasswordAlgorithm, &cred.CreatedAt, &cred.UpdatedAt)

	if err != nil {
		return nil, nil
	}

	return &cred, nil
}

// UpdatePasswordHash updates a user's password hash
func (r *PostgresAuthRepository) UpdatePasswordHash(ctx context.Context, userID, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_credentials
		SET password_hash = $2, updated_at = NOW()
		WHERE user_id = $1
	`, userID, passwordHash)
	return err
}

// SetPasswordSet sets the password_set flag on a user
func (r *PostgresAuthRepository) SetPasswordSet(ctx context.Context, userID string, passwordSet bool) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET password_set = $2 WHERE id = $1
	`, userID, passwordSet)
	return err
}

// UpdateUsername updates a user's username
func (r *PostgresAuthRepository) UpdateUsername(ctx context.Context, userID, username string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET username = $2 WHERE id = $1
	`, userID, username)
	return err
}

// SetWebAuthnEnabled sets the webauthn_enabled flag on a user
func (r *PostgresAuthRepository) SetWebAuthnEnabled(ctx context.Context, userID string, enabled bool) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET webauthn_enabled = $2 WHERE id = $1
	`, userID, enabled)
	return err
}

// InMemoryAuthRepository is an in-memory implementation for testing
type InMemoryAuthRepository struct {
	users       map[string]UserWithCredentials
	byEmail     map[string]*UserWithCredentials
	byUsername  map[string]*UserWithCredentials
}

// NewInMemoryAuthRepository creates a new in-memory auth repository
func NewInMemoryAuthRepository() *InMemoryAuthRepository {
	return &InMemoryAuthRepository{
		users:      make(map[string]UserWithCredentials),
		byEmail:    make(map[string]*UserWithCredentials),
		byUsername: make(map[string]*UserWithCredentials),
	}
}

// CreateUserWithCredentials creates a user in memory
func (r *InMemoryAuthRepository) CreateUserWithCredentials(ctx context.Context, email, username, passwordHash, displayName string) (UserWithCredentials, error) {
	userID := uuid.New().String()
	credID := uuid.New().String()
	now := time.Now().UTC()

	user := UserWithCredentials{
		ID:              userID,
		Email:           email,
		Username:        &username,
		DisplayName:     displayName,
		PasswordSet:     true,
		WebAuthnEnabled: false,
		Credential: &UserCredential{
			ID:                credID,
			UserID:            userID,
			PasswordHash:      passwordHash,
			PasswordAlgorithm: "argon2id",
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}

	r.users[userID] = user
	r.byEmail[email] = &user
	r.byUsername[username] = &user

	return user, nil
}

// GetUserByEmail retrieves a user by email
func (r *InMemoryAuthRepository) GetUserByEmail(ctx context.Context, email string) (*UserWithCredentials, error) {
	return r.byEmail[email], nil
}

// GetUserByUsername retrieves a user by username
func (r *InMemoryAuthRepository) GetUserByUsername(ctx context.Context, username string) (*UserWithCredentials, error) {
	return r.byUsername[username], nil
}

// GetUserByID retrieves a user by ID
func (r *InMemoryAuthRepository) GetUserByID(ctx context.Context, userID string) (*UserWithCredentials, error) {
	user, exists := r.users[userID]
	if !exists {
		return nil, nil
	}
	return &user, nil
}

// GetCredentialsByUserID retrieves credentials for a user
func (r *InMemoryAuthRepository) GetCredentialsByUserID(ctx context.Context, userID string) (*UserCredential, error) {
	user, exists := r.users[userID]
	if !exists {
		return nil, nil
	}
	return user.Credential, nil
}

// UpdatePasswordHash updates a user's password hash
func (r *InMemoryAuthRepository) UpdatePasswordHash(ctx context.Context, userID, passwordHash string) error {
	user, exists := r.users[userID]
	if !exists {
		return nil
	}
	if user.Credential != nil {
		user.Credential.PasswordHash = passwordHash
		user.Credential.UpdatedAt = time.Now().UTC()
	}
	return nil
}

// SetPasswordSet sets the password_set flag
func (r *InMemoryAuthRepository) SetPasswordSet(ctx context.Context, userID string, passwordSet bool) error {
	user, exists := r.users[userID]
	if !exists {
		return nil
	}
	user.PasswordSet = passwordSet
	return nil
}

// UpdateUsername updates a user's username
func (r *InMemoryAuthRepository) UpdateUsername(ctx context.Context, userID, username string) error {
	user, exists := r.users[userID]
	if !exists {
		return nil
	}
	oldUsername := ""
	if user.Username != nil {
		oldUsername = *user.Username
		delete(r.byUsername, oldUsername)
	}
	user.Username = &username
	r.users[userID] = user // Update the value in the users map
	r.byUsername[username] = &user // Update the pointer in the username map
	return nil
}

// SetWebAuthnEnabled sets the webauthn_enabled flag
func (r *InMemoryAuthRepository) SetWebAuthnEnabled(ctx context.Context, userID string, enabled bool) error {
	user, exists := r.users[userID]
	if !exists {
		return nil
	}
	user.WebAuthnEnabled = enabled
	r.users[userID] = user // Update the map
	return nil
}
