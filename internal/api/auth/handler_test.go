package authapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/security"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"

	"github.com/go-chi/chi/v5"
)

// Mock user for testing
type mockUser struct {
	id              string
	email           string
	username        string
	displayName     string
	passwordHash    string
	webAuthnEnabled bool
}

// Mock repositories
type mockAuthRepo struct {
	users      map[string]*mockUser
	byEmail    map[string]*mockUser
	byUsername map[string]*mockUser
}

func newMockAuthRepo() *mockAuthRepo {
	return &mockAuthRepo{
		users:      make(map[string]*mockUser),
		byEmail:    make(map[string]*mockUser),
		byUsername: make(map[string]*mockUser),
	}
}

var userCounter int

func (r *mockAuthRepo) CreateUserWithCredentials(ctx context.Context, email, username, passwordHash, displayName string) (repository.UserWithCredentials, error) {
	userCounter++
	userID := "user_" + strconv.Itoa(userCounter)
	credID := "cred_" + strconv.Itoa(userCounter)
	now := time.Now().UTC()

	user := &mockUser{
		id:              userID,
		email:           email,
		username:        username,
		displayName:     displayName,
		passwordHash:    passwordHash,
		webAuthnEnabled: false,
	}

	r.users[userID] = user
	r.byEmail[email] = user
	r.byUsername[username] = user

	return repository.UserWithCredentials{
		ID:              userID,
		Email:           email,
		Username:        &username,
		DisplayName:     displayName,
		PasswordSet:     true,
		WebAuthnEnabled: false,
		Credential: &repository.UserCredential{
			ID:                credID,
			UserID:            userID,
			PasswordHash:      passwordHash,
			PasswordAlgorithm: "argon2id",
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}, nil
}

func (r *mockAuthRepo) GetUserByEmail(ctx context.Context, email string) (*repository.UserWithCredentials, error) {
	user := r.byEmail[email]
	if user == nil {
		return nil, nil
	}
	return &repository.UserWithCredentials{
		ID:              user.id,
		Email:           user.email,
		Username:        &user.username,
		DisplayName:     user.displayName,
		PasswordSet:     user.passwordHash != "",
		WebAuthnEnabled: user.webAuthnEnabled,
		Credential: &repository.UserCredential{
			UserID:       user.id,
			PasswordHash: user.passwordHash,
		},
	}, nil
}

func (r *mockAuthRepo) GetUserByUsername(ctx context.Context, username string) (*repository.UserWithCredentials, error) {
	user := r.byUsername[username]
	if user == nil {
		return nil, nil
	}
	return &repository.UserWithCredentials{
		ID:              user.id,
		Email:           user.email,
		Username:        &user.username,
		DisplayName:     user.displayName,
		PasswordSet:     user.passwordHash != "",
		WebAuthnEnabled: user.webAuthnEnabled,
	}, nil
}

func (r *mockAuthRepo) GetUserByID(ctx context.Context, userID string) (*repository.UserWithCredentials, error) {
	user, exists := r.users[userID]
	if !exists {
		return nil, nil
	}
	return &repository.UserWithCredentials{
		ID:              user.id,
		Email:           user.email,
		Username:        &user.username,
		DisplayName:     user.displayName,
		PasswordSet:     user.passwordHash != "",
		WebAuthnEnabled: user.webAuthnEnabled,
		Credential: &repository.UserCredential{
			UserID:       user.id,
			PasswordHash: user.passwordHash,
		},
	}, nil
}

func (r *mockAuthRepo) GetCredentialsByUserID(ctx context.Context, userID string) (*repository.UserCredential, error) {
	user, exists := r.users[userID]
	if !exists || user.passwordHash == "" {
		return nil, nil
	}
	return &repository.UserCredential{
		UserID:       user.id,
		PasswordHash: user.passwordHash,
	}, nil
}

func (r *mockAuthRepo) UpdatePasswordHash(ctx context.Context, userID, passwordHash string) error {
	user, exists := r.users[userID]
	if exists {
		user.passwordHash = passwordHash
	}
	return nil
}

func (r *mockAuthRepo) SetPasswordSet(ctx context.Context, userID string, passwordSet bool) error {
	return nil
}

func (r *mockAuthRepo) UpdateUsername(ctx context.Context, userID, username string) error {
	user, exists := r.users[userID]
	if !exists {
		return nil
	}
	oldUsername := user.username
	delete(r.byUsername, oldUsername)
	user.username = username
	r.byUsername[username] = user
	return nil
}

func (r *mockAuthRepo) SetWebAuthnEnabled(ctx context.Context, userID string, enabled bool) error {
	user, exists := r.users[userID]
	if exists {
		user.webAuthnEnabled = enabled
	}
	return nil
}

type mockLockoutRepo struct {
	attempts []repository.LoginAttempt
	lockouts map[string]repository.AccountLockout
}

func newMockLockoutRepo() *mockLockoutRepo {
	return &mockLockoutRepo{
		attempts: make([]repository.LoginAttempt, 0),
		lockouts: make(map[string]repository.AccountLockout),
	}
}

func (r *mockLockoutRepo) RecordLoginAttempt(ctx context.Context, identifier, ipAddress string, success bool) error {
	r.attempts = append(r.attempts, repository.LoginAttempt{
		ID:          "attempt_" + strconv.Itoa(len(r.attempts)),
		Identifier:  identifier,
		IPAddress:   ipAddress,
		Success:     success,
		AttemptedAt: time.Now().UTC(),
	})
	return nil
}

func (r *mockLockoutRepo) GetFailedAttemptCount(ctx context.Context, identifier string, since time.Time) (int, error) {
	count := 0
	for _, a := range r.attempts {
		if a.Identifier == identifier && !a.Success && a.AttemptedAt.After(since) {
			count++
		}
	}
	return count, nil
}

func (r *mockLockoutRepo) GetIPFailedAttemptCount(ctx context.Context, ipAddress string, since time.Time) (int, error) {
	count := 0
	for _, a := range r.attempts {
		if a.IPAddress == ipAddress && !a.Success && a.AttemptedAt.After(since) {
			count++
		}
	}
	return count, nil
}

func (r *mockLockoutRepo) LockAccount(ctx context.Context, userID string, lockedUntil time.Time, lockedBy string) error {
	r.lockouts[userID] = repository.AccountLockout{
		ID:          "lockout_" + strconv.Itoa(len(r.lockouts)),
		UserID:      userID,
		LockedUntil: &lockedUntil,
		FailedCount: 0,
		LockedBy:    lockedBy,
	}
	return nil
}

func (r *mockLockoutRepo) IsAccountLocked(ctx context.Context, userID string) (bool, *time.Time, error) {
	lockout, exists := r.lockouts[userID]
	if !exists {
		return false, nil, nil
	}
	if lockout.LockedUntil == nil {
		return false, nil, nil
	}
	if lockout.LockedUntil.Before(time.Now().UTC()) {
		delete(r.lockouts, userID)
		return false, nil, nil
	}
	return true, lockout.LockedUntil, nil
}

func (r *mockLockoutRepo) UnlockAccount(ctx context.Context, userID string) error {
	delete(r.lockouts, userID)
	return nil
}

func (r *mockLockoutRepo) IncrementFailedCount(ctx context.Context, userID string, threshold int, lockoutDuration time.Duration) (int, error) {
	lockout, exists := r.lockouts[userID]
	now := time.Now().UTC()
	if !exists {
		lockout = repository.AccountLockout{
			ID:          "lockout_" + strconv.Itoa(len(r.lockouts)+1),
			UserID:      userID,
			FailedCount: 0,
		}
	}
	lockout.FailedCount++
	lockout.LastFailedAt = &now
	if lockout.FailedCount >= threshold {
		lockedUntil := now.Add(lockoutDuration)
		lockout.LockedUntil = &lockedUntil
	}
	r.lockouts[userID] = lockout
	return lockout.FailedCount, nil
}

type mockAuditRepo struct {
	events []repository.AuthAuditEvent
}

func newMockAuditRepo() *mockAuditRepo {
	return &mockAuditRepo{events: make([]repository.AuthAuditEvent, 0)}
}

func (r *mockAuditRepo) LogAuthEvent(ctx context.Context, event repository.AuthAuditEvent) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	r.events = append(r.events, event)
	return nil
}

func (r *mockAuditRepo) GetRecentEvents(ctx context.Context, userID *string, limit int) ([]repository.AuthAuditEvent, error) {
	var result []repository.AuthAuditEvent
	for _, e := range r.events {
		if userID == nil || (e.UserID != nil && *e.UserID == *userID) {
			result = append(result, e)
		}
	}
	if len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result, nil
}

// Helper functions
func newTestTokenManager() *session.TokenManager {
	tm, _ := session.NewTokenManager("test-secret-key-for-testing")
	return tm
}

func withSession(req *http.Request, sess session.Context) *http.Request {
	return req.WithContext(session.NewContext(req.Context(), sess))
}

func withAuthSession(req *http.Request, userID string) *http.Request {
	sess := session.Context{
		UserID:    userID,
		AccountID: "acct_test",
		Role:      "customer",
	}
	return withSession(req, sess)
}

func withChiRouteContext(req *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func newTestConfig() *config.Config {
	return &config.Config{
		PasswordMinLength:       12,
		LoginRateLimitIP:        10,
		LoginRateLimitAccount:   5,
		AccountLockoutThreshold: 20,
		AccountLockoutDuration:  time.Hour,
	}
}

// =============================================================================
// Register Handler Tests
// =============================================================================

func TestRegisterSuccess(t *testing.T) {
	authRepo := newMockAuthRepo()
	lockoutRepo := newMockLockoutRepo()
	auditRepo := newMockAuditRepo()
	tokenMgr := newTestTokenManager()
	cfg := newTestConfig()
	authService := service.NewAuthService(authRepo, auditRepo, lockoutRepo, cfg, tokenMgr)

	h := NewHandler(authService, tokenMgr, "session", false, "")

	body := `{"email":"test@example.com","username":"testuser","password":"SecurePassword123!"}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d (body: %s)", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp models.LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.User.Email != "test@example.com" {
		t.Fatalf("expected email test@example.com, got %s", resp.User.Email)
	}

	// Check session cookie
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie to be set")
	}
	if cookies[0].Name != "session" {
		t.Fatalf("expected cookie name 'session', got %s", cookies[0].Name)
	}
	if !cookies[0].HttpOnly {
		t.Fatal("expected cookie to be HttpOnly")
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	authRepo := newMockAuthRepo()
	lockoutRepo := newMockLockoutRepo()
	auditRepo := newMockAuditRepo()
	tokenMgr := newTestTokenManager()
	cfg := newTestConfig()
	authService := service.NewAuthService(authRepo, auditRepo, lockoutRepo, cfg, tokenMgr)

	// Pre-create user
	hashedPassword, _ := security.HashPassword("Password123!")
	authRepo.CreateUserWithCredentials(context.Background(), "test@example.com", "existinguser", hashedPassword, "Existing User")

	h := NewHandler(authService, tokenMgr, "session", false, "")

	body := `{"email":"test@example.com","username":"newuser","password":"SecurePassword123!"}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

func TestRegisterWeakPassword(t *testing.T) {
	authRepo := newMockAuthRepo()
	lockoutRepo := newMockLockoutRepo()
	auditRepo := newMockAuditRepo()
	tokenMgr := newTestTokenManager()
	cfg := newTestConfig()
	authService := service.NewAuthService(authRepo, auditRepo, lockoutRepo, cfg, tokenMgr)

	h := NewHandler(authService, tokenMgr, "session", false, "")

	body := `{"email":"test@example.com","username":"testuser","password":"short"}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, w.Code)
	}
}

func TestRegisterMissingFields(t *testing.T) {
	authRepo := newMockAuthRepo()
	lockoutRepo := newMockLockoutRepo()
	auditRepo := newMockAuditRepo()
	tokenMgr := newTestTokenManager()
	cfg := newTestConfig()
	authService := service.NewAuthService(authRepo, auditRepo, lockoutRepo, cfg, tokenMgr)

	h := NewHandler(authService, tokenMgr, "session", false, "")

	tests := []struct {
		name string
		body string
	}{
		{"missing email", `{"username":"testuser","password":"SecurePassword123!"}`},
		{"missing username", `{"email":"test@example.com","password":"SecurePassword123!"}`},
		{"missing password", `{"email":"test@example.com","username":"testuser"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.Register(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestRegisterInvalidJSON(t *testing.T) {
	h := NewHandler(nil, nil, "session", false, "")

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// =============================================================================
// Login Handler Tests
// =============================================================================

func TestLoginSuccess(t *testing.T) {
	authRepo := newMockAuthRepo()
	lockoutRepo := newMockLockoutRepo()
	auditRepo := newMockAuditRepo()
	tokenMgr := newTestTokenManager()
	cfg := newTestConfig()
	authService := service.NewAuthService(authRepo, auditRepo, lockoutRepo, cfg, tokenMgr)

	// Create user with known password
	hashedPassword, _ := security.HashPassword("SecurePassword123!")
	user, _ := authRepo.CreateUserWithCredentials(context.Background(), "test@example.com", "testuser", hashedPassword, "Test User")

	h := NewHandler(authService, tokenMgr, "session", false, "")

	body := `{"email":"test@example.com","password":"SecurePassword123!"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body: %s)", http.StatusOK, w.Code, w.Body.String())
	}

	var resp models.LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.User.Email != "test@example.com" {
		t.Fatalf("expected email test@example.com, got %s", resp.User.Email)
	}

	// Check session cookie
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie to be set")
	}

	_ = user // avoid unused variable error
}

func TestLoginWrongPassword(t *testing.T) {
	authRepo := newMockAuthRepo()
	lockoutRepo := newMockLockoutRepo()
	auditRepo := newMockAuditRepo()
	tokenMgr := newTestTokenManager()
	cfg := newTestConfig()
	authService := service.NewAuthService(authRepo, auditRepo, lockoutRepo, cfg, tokenMgr)

	// Create user with known password
	hashedPassword, _ := security.HashPassword("SecurePassword123!")
	authRepo.CreateUserWithCredentials(context.Background(), "test@example.com", "testuser", hashedPassword, "Test User")

	h := NewHandler(authService, tokenMgr, "session", false, "")

	body := `{"email":"test@example.com","password":"WrongPassword123!"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp models.AuthError
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Generic error message (no user enumeration)
	if resp.Error != "invalid credentials" {
		t.Fatalf("expected 'invalid credentials' error, got %s", resp.Error)
	}
}

func TestLoginUserNotFound(t *testing.T) {
	authRepo := newMockAuthRepo()
	lockoutRepo := newMockLockoutRepo()
	auditRepo := newMockAuditRepo()
	tokenMgr := newTestTokenManager()
	cfg := newTestConfig()
	authService := service.NewAuthService(authRepo, auditRepo, lockoutRepo, cfg, tokenMgr)

	h := NewHandler(authService, tokenMgr, "session", false, "")

	body := `{"email":"nonexistent@example.com","password":"SecurePassword123!"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp models.AuthError
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Generic error message (no user enumeration)
	if resp.Error != "invalid credentials" {
		t.Fatalf("expected 'invalid credentials' error, got %s", resp.Error)
	}
}

func TestLoginLockedAccount(t *testing.T) {
	authRepo := newMockAuthRepo()
	lockoutRepo := newMockLockoutRepo()
	auditRepo := newMockAuditRepo()
	tokenMgr := newTestTokenManager()
	cfg := newTestConfig()
	authService := service.NewAuthService(authRepo, auditRepo, lockoutRepo, cfg, tokenMgr)

	// Create user with known password
	hashedPassword, _ := security.HashPassword("SecurePassword123!")
	user, _ := authRepo.CreateUserWithCredentials(context.Background(), "test@example.com", "testuser", hashedPassword, "Test User")

	// Lock the account
	lockoutRepo.LockAccount(context.Background(), user.ID, time.Now().Add(time.Hour), "test")

	h := NewHandler(authService, tokenMgr, "session", false, "")

	body := `{"email":"test@example.com","password":"SecurePassword123!"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusLocked {
		t.Fatalf("expected status %d, got %d", http.StatusLocked, w.Code)
	}
}

func TestLoginRateLimited(t *testing.T) {
	authRepo := newMockAuthRepo()
	lockoutRepo := newMockLockoutRepo()
	auditRepo := newMockAuditRepo()
	tokenMgr := newTestTokenManager()
	cfg := &config.Config{
		LoginRateLimitIP:        2, // Low limit for testing
		LoginRateLimitAccount:   2,
		AccountLockoutThreshold: 20,
		AccountLockoutDuration:  time.Hour,
	}
	authService := service.NewAuthService(authRepo, auditRepo, lockoutRepo, cfg, tokenMgr)

	// Create user
	hashedPassword, _ := security.HashPassword("SecurePassword123!")
	authRepo.CreateUserWithCredentials(context.Background(), "test@example.com", "testuser", hashedPassword, "Test User")

	// Simulate failed attempts to trigger rate limit
	for i := 0; i < 5; i++ {
		lockoutRepo.RecordLoginAttempt(context.Background(), "test@example.com", "192.168.1.1", false)
	}

	h := NewHandler(authService, tokenMgr, "session", false, "")

	body := `{"email":"test@example.com","password":"SecurePassword123!"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, w.Code)
	}
}

func TestLoginMissingFields(t *testing.T) {
	h := NewHandler(nil, nil, "session", false, "")

	tests := []struct {
		name string
		body string
	}{
		{"missing password", `{"email":"test@example.com"}`},
		{"missing email", `{"password":"SecurePassword123!"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.Login(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}

// =============================================================================
// Refresh Handler Tests
// =============================================================================

func TestRefreshSuccess(t *testing.T) {
	authRepo := newMockAuthRepo()
	lockoutRepo := newMockLockoutRepo()
	auditRepo := newMockAuditRepo()
	tokenMgr := newTestTokenManager()
	cfg := newTestConfig()
	authService := service.NewAuthService(authRepo, auditRepo, lockoutRepo, cfg, tokenMgr)

	// Create user
	hashedPassword, _ := security.HashPassword("SecurePassword123!")
	user, _ := authRepo.CreateUserWithCredentials(context.Background(), "test@example.com", "testuser", hashedPassword, "Test User")

	h := NewHandler(authService, tokenMgr, "session", false, "")

	// Create session context
	sess := session.Context{
		UserID:    user.ID,
		AccountID: "acct_default",
		Role:      "customer",
	}

	req := withSession(httptest.NewRequest(http.MethodPost, "/refresh", nil), sess)
	w := httptest.NewRecorder()

	h.Refresh(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body: %s)", http.StatusOK, w.Code, w.Body.String())
	}

	var resp models.RefreshResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.User.ID != user.ID {
		t.Fatalf("expected user ID %s, got %s", user.ID, resp.User.ID)
	}
}

func TestRefreshNoSession(t *testing.T) {
	h := NewHandler(nil, newTestTokenManager(), "session", false, "")

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	w := httptest.NewRecorder()

	h.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestRefreshUnauthenticatedSession(t *testing.T) {
	h := NewHandler(nil, newTestTokenManager(), "session", false, "")

	// Empty session context (not authenticated)
	sess := session.Context{}

	req := withSession(httptest.NewRequest(http.MethodPost, "/refresh", nil), sess)
	w := httptest.NewRecorder()

	h.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestRefreshUserNotFound(t *testing.T) {
	authRepo := newMockAuthRepo()
	lockoutRepo := newMockLockoutRepo()
	auditRepo := newMockAuditRepo()
	tokenMgr := newTestTokenManager()
	cfg := newTestConfig()
	authService := service.NewAuthService(authRepo, auditRepo, lockoutRepo, cfg, tokenMgr)

	h := NewHandler(authService, tokenMgr, "session", false, "")

	// Session for non-existent user
	sess := session.Context{
		UserID:    "nonexistent-user-id",
		AccountID: "acct_default",
		Role:      "customer",
	}

	req := withSession(httptest.NewRequest(http.MethodPost, "/refresh", nil), sess)
	w := httptest.NewRecorder()

	h.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}
