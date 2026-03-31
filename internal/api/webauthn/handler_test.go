package webauthn

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"

	"github.com/go-chi/chi/v5"
)

// Mock repositories
type mockWebAuthnRepo struct {
	credentials map[string]repository.WebAuthnCredential
	challenges  map[string]repository.WebAuthnChallenge
}

func newMockWebAuthnRepo() *mockWebAuthnRepo {
	return &mockWebAuthnRepo{
		credentials: make(map[string]repository.WebAuthnCredential),
		challenges:  make(map[string]repository.WebAuthnChallenge),
	}
}

func (r *mockWebAuthnRepo) StoreCredential(ctx context.Context, cred repository.WebAuthnCredential) error {
	if cred.ID == "" {
		cred.ID = "cred_" + time.Now().Format("20060102150405")
	}
	if cred.CreatedAt.IsZero() {
		cred.CreatedAt = time.Now().UTC()
	}
	r.credentials[cred.ID] = cred
	return nil
}

func (r *mockWebAuthnRepo) ListCredentialsByUser(ctx context.Context, userID string) ([]repository.WebAuthnCredential, error) {
	var result []repository.WebAuthnCredential
	for _, cred := range r.credentials {
		if cred.UserID == userID {
			result = append(result, cred)
		}
	}
	return result, nil
}

func (r *mockWebAuthnRepo) GetCredentialByID(ctx context.Context, credentialID string) (*repository.WebAuthnCredential, error) {
	cred, exists := r.credentials[credentialID]
	if !exists {
		return nil, nil
	}
	return &cred, nil
}

func (r *mockWebAuthnRepo) GetCredentialByCredentialIDBytes(ctx context.Context, credentialID []byte) (*repository.WebAuthnCredential, error) {
	for _, cred := range r.credentials {
		if string(cred.CredentialID) == string(credentialID) {
			return &cred, nil
		}
	}
	return nil, nil
}

func (r *mockWebAuthnRepo) DeleteCredential(ctx context.Context, credentialID, userID string) error {
	cred, exists := r.credentials[credentialID]
	if !exists || cred.UserID != userID {
		return nil
	}
	delete(r.credentials, credentialID)
	return nil
}

func (r *mockWebAuthnRepo) UpdateSignCount(ctx context.Context, credentialID string, signCount int64) error {
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

func (r *mockWebAuthnRepo) CreateChallenge(ctx context.Context, challenge repository.WebAuthnChallenge) error {
	if challenge.ID == "" {
		challenge.ID = "chal_" + time.Now().Format("20060102150405")
	}
	if challenge.CreatedAt.IsZero() {
		challenge.CreatedAt = time.Now().UTC()
	}
	r.challenges[challenge.ID] = challenge
	return nil
}

func (r *mockWebAuthnRepo) GetChallenge(ctx context.Context, challengeID string) (*repository.WebAuthnChallenge, error) {
	challenge, exists := r.challenges[challengeID]
	if !exists {
		return nil, nil
	}
	return &challenge, nil
}

func (r *mockWebAuthnRepo) DeleteChallenge(ctx context.Context, challengeID string) error {
	delete(r.challenges, challengeID)
	return nil
}

func (r *mockWebAuthnRepo) CleanupExpiredChallenges(ctx context.Context) error {
	now := time.Now().UTC()
	for id, challenge := range r.challenges {
		if challenge.ExpiresAt.Before(now) {
			delete(r.challenges, id)
		}
	}
	return nil
}

// Mock auth repository (minimal for WebAuthn tests)
type mockAuthRepo struct {
	users map[string]repository.UserWithCredentials
	passwordHistory map[string][]string
}

func newMockAuthRepo() *mockAuthRepo {
	return &mockAuthRepo{users:          make(map[string]repository.UserWithCredentials),
		passwordHistory: make(map[string][]string),}
}

func (r *mockAuthRepo) CreateUserWithCredentials(ctx context.Context, email, username, passwordHash, displayName string) (repository.UserWithCredentials, error) {
	return repository.UserWithCredentials{}, nil
}

func (r *mockAuthRepo) GetUserByEmail(ctx context.Context, email string) (*repository.UserWithCredentials, error) {
	for _, user := range r.users {
		if user.Email == email {
			return &user, nil
		}
	}
	return nil, nil
}

func (r *mockAuthRepo) GetUserByUsername(ctx context.Context, username string) (*repository.UserWithCredentials, error) {
	return nil, nil
}

func (r *mockAuthRepo) GetUserByID(ctx context.Context, userID string) (*repository.UserWithCredentials, error) {
	user, exists := r.users[userID]
	if !exists {
		return nil, nil
	}
	return &user, nil
}

func (r *mockAuthRepo) GetCredentialsByUserID(ctx context.Context, userID string) (*repository.UserCredential, error) {
	return nil, nil
}

func (r *mockAuthRepo) UpdatePasswordHash(ctx context.Context, userID, passwordHash string) error {
	return nil
}

func (r *mockAuthRepo) SetPasswordSet(ctx context.Context, userID string, passwordSet bool) error {
	return nil
}

func (r *mockAuthRepo) UpdateUsername(ctx context.Context, userID, username string) error {
	return nil
}

func (r *mockAuthRepo) SetWebAuthnEnabled(ctx context.Context, userID string, enabled bool) error {
	user, exists := r.users[userID]
	if exists {
		user.WebAuthnEnabled = enabled
		r.users[userID] = user
	}
	return nil
}

func (r *mockAuthRepo) SetVerified(ctx context.Context, userID string) error {
	return nil
}


func (r *mockAuthRepo) GetPasswordHistory(ctx context.Context, userID string, limit int) ([]string, error) {
	return []string{}, nil
}

func (r *mockAuthRepo) AddPasswordHistory(ctx context.Context, userID, passwordHash string) error {
	return nil
}
func (r *mockAuthRepo) GetRoleByUserID(ctx context.Context, userID string) (string, error) {
	return "customer", nil
}

// Mock audit repository
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
	return r.events, nil
}

// Helper functions
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
		WebAuthnRPID:    "localhost",
		WebAuthnRPName:  "Test App",
		WebAuthnOrigins: []string{"http://localhost:3000"},
	}
}

func newTestTokenManager() *session.TokenManager {
	tm, _ := session.NewTokenManager("test-secret-key-for-webauthn")
	return tm
}

// Test: Begin registration requires auth
func TestBeginRegistrationRequiresAuth(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/register-begin", nil)
	w := httptest.NewRecorder()

	h.BeginRegistration(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test: Finish registration requires auth
func TestFinishRegistrationRequiresAuth(t *testing.T) {
	h := &Handler{}

	body := `{"challenge_id":"chal_123","credential":{}}`
	req := httptest.NewRequest(http.MethodPost, "/register-finish", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.FinishRegistration(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test: List credentials requires auth
func TestListCredentialsRequiresAuth(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/credentials", nil)
	w := httptest.NewRecorder()

	h.ListCredentials(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test: Delete credential requires auth
func TestDeleteCredentialRequiresAuth(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodDelete, "/credentials/cred_123", nil)
	req = withChiRouteContext(req, "cred_123")
	w := httptest.NewRecorder()

	h.DeleteCredential(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test: Begin login does NOT require auth (public endpoint)
func TestBeginLoginNoAuthRequired(t *testing.T) {
	webauthnRepo := newMockWebAuthnRepo()
	authRepo := newMockAuthRepo()
	auditRepo := newMockAuditRepo()

	// Create user with WebAuthn enabled
	username := "testuser"
	userID := "user_123"
	authRepo.users[userID] = repository.UserWithCredentials{
		ID:              userID,
		Email:           "test@example.com",
		Username:        &username,
		DisplayName:     "Test User",
		WebAuthnEnabled: true,
	}

	// Add a credential
	webauthnRepo.credentials["cred_1"] = repository.WebAuthnCredential{
		ID:           "cred_1",
		UserID:       userID,
		CredentialID: []byte("test-credential-id"),
		PublicKey:    []byte("test-public-key"),
		Name:         "Security Key",
		CreatedAt:    time.Now().UTC(),
	}

	webauthnSvc, err := service.NewWebAuthnService(authRepo, webauthnRepo, auditRepo, newTestConfig())
	if err != nil {
		t.Fatalf("failed to create WebAuthn service: %v", err)
	}

	h := &Handler{
		WebAuthnService: webauthnSvc,
		TokenManager:    newTestTokenManager(),
		SessionName:     "session",
	}

	body := `{"email":"test@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/login-begin", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.BeginLogin(w, req)

	// Should NOT return 401 - it's a public endpoint
	if w.Code == http.StatusUnauthorized {
		t.Fatal("begin login should not require authentication")
	}
}

// Test: Begin login requires email
func TestBeginLoginRequiresEmail(t *testing.T) {
	h := &Handler{}

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/login-begin", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.BeginLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Test: Finish login requires challenge_id and email
func TestFinishLoginRequiresFields(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name string
		body string
	}{
		{"missing challenge_id", `{"email":"test@example.com"}`},
		{"missing email", `{"challenge_id":"chal_123"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/login-finish", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.FinishLogin(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}

// Test: Finish registration requires challenge_id
func TestFinishRegistrationRequiresChallengeID(t *testing.T) {
	h := &Handler{}

	body := `{"credential":{}}`
	req := withAuthSession(httptest.NewRequest(http.MethodPost, "/register-finish", bytes.NewBufferString(body)), "user_123")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.FinishRegistration(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Test: List credentials returns empty list for user with no credentials
func TestListCredentialsEmpty(t *testing.T) {
	webauthnRepo := newMockWebAuthnRepo()
	authRepo := newMockAuthRepo()
	auditRepo := newMockAuditRepo()

	webauthnSvc, err := service.NewWebAuthnService(authRepo, webauthnRepo, auditRepo, newTestConfig())
	if err != nil {
		t.Fatalf("failed to create WebAuthn service: %v", err)
	}

	h := &Handler{
		WebAuthnService: webauthnSvc,
	}

	req := withAuthSession(httptest.NewRequest(http.MethodGet, "/credentials", nil), "user_123")
	w := httptest.NewRecorder()

	h.ListCredentials(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp models.WebAuthnCredentialListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Credentials) != 0 {
		t.Fatalf("expected empty credentials list, got %d", len(resp.Credentials))
	}
}

// Test: Delete credential not found
func TestDeleteCredentialNotFound(t *testing.T) {
	webauthnRepo := newMockWebAuthnRepo()
	authRepo := newMockAuthRepo()
	auditRepo := newMockAuditRepo()

	webauthnSvc, err := service.NewWebAuthnService(authRepo, webauthnRepo, auditRepo, newTestConfig())
	if err != nil {
		t.Fatalf("failed to create WebAuthn service: %v", err)
	}

	h := &Handler{
		WebAuthnService: webauthnSvc,
	}

	req := withAuthSession(httptest.NewRequest(http.MethodDelete, "/credentials/nonexistent", nil), "user_123")
	req = withChiRouteContext(req, "nonexistent")
	w := httptest.NewRecorder()

	h.DeleteCredential(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// Test: Delete credential enforces ownership
func TestDeleteCredentialOwnership(t *testing.T) {
	webauthnRepo := newMockWebAuthnRepo()
	authRepo := newMockAuthRepo()
	auditRepo := newMockAuditRepo()

	// Add credential for user_123
	webauthnRepo.credentials["cred_1"] = repository.WebAuthnCredential{
		ID:           "cred_1",
		UserID:       "user_123",
		CredentialID: []byte("test-credential-id"),
		PublicKey:    []byte("test-public-key"),
		Name:         "Security Key",
		CreatedAt:    time.Now().UTC(),
	}

	webauthnSvc, err := service.NewWebAuthnService(authRepo, webauthnRepo, auditRepo, newTestConfig())
	if err != nil {
		t.Fatalf("failed to create WebAuthn service: %v", err)
	}

	h := &Handler{
		WebAuthnService: webauthnSvc,
	}

	// Try to delete as different user
	req := withAuthSession(httptest.NewRequest(http.MethodDelete, "/credentials/cred_1", nil), "user_456")
	req = withChiRouteContext(req, "cred_1")
	w := httptest.NewRecorder()

	h.DeleteCredential(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// Test: Deleting last credential auto-disables WebAuthn
func TestDeleteLastCredentialAutoDisables(t *testing.T) {
	webauthnRepo := newMockWebAuthnRepo()
	authRepo := newMockAuthRepo()
	auditRepo := newMockAuditRepo()

	userID := "user_123"
	username := "testuser"

	// Create user with WebAuthn enabled
	authRepo.users[userID] = repository.UserWithCredentials{
		ID:              userID,
		Email:           "test@example.com",
		Username:        &username,
		DisplayName:     "Test User",
		WebAuthnEnabled: true,
	}

	// Add single credential
	webauthnRepo.credentials["cred_1"] = repository.WebAuthnCredential{
		ID:           "cred_1",
		UserID:       userID,
		CredentialID: []byte("test-credential-id"),
		PublicKey:    []byte("test-public-key"),
		Name:         "Security Key",
		CreatedAt:    time.Now().UTC(),
	}

	webauthnSvc, err := service.NewWebAuthnService(authRepo, webauthnRepo, auditRepo, newTestConfig())
	if err != nil {
		t.Fatalf("failed to create WebAuthn service: %v", err)
	}

	h := &Handler{
		WebAuthnService: webauthnSvc,
	}

	// Delete the only credential
	req := withAuthSession(httptest.NewRequest(http.MethodDelete, "/credentials/cred_1", nil), userID)
	req = withChiRouteContext(req, "cred_1")
	w := httptest.NewRecorder()

	h.DeleteCredential(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Verify WebAuthn is now disabled for user
	user, _ := authRepo.GetUserByID(context.Background(), userID)
	if user == nil {
		t.Fatal("user not found")
	}
	if user.WebAuthnEnabled {
		t.Fatal("expected WebAuthn to be disabled after deleting last credential")
	}
}

// Test: List credentials returns credentials for user
func TestListCredentialsWithData(t *testing.T) {
	webauthnRepo := newMockWebAuthnRepo()
	authRepo := newMockAuthRepo()
	auditRepo := newMockAuditRepo()

	userID := "user_123"

	// Add credentials for user
	webauthnRepo.credentials["cred_1"] = repository.WebAuthnCredential{
		ID:           "cred_1",
		UserID:       userID,
		CredentialID: []byte("cred-id-1"),
		PublicKey:    []byte("public-key-1"),
		Name:         "Security Key 1",
		CreatedAt:    time.Now().UTC(),
	}
	webauthnRepo.credentials["cred_2"] = repository.WebAuthnCredential{
		ID:           "cred_2",
		UserID:       userID,
		CredentialID: []byte("cred-id-2"),
		PublicKey:    []byte("public-key-2"),
		Name:         "Security Key 2",
		CreatedAt:    time.Now().UTC(),
	}

	webauthnSvc, err := service.NewWebAuthnService(authRepo, webauthnRepo, auditRepo, newTestConfig())
	if err != nil {
		t.Fatalf("failed to create WebAuthn service: %v", err)
	}

	h := &Handler{
		WebAuthnService: webauthnSvc,
	}

	req := withAuthSession(httptest.NewRequest(http.MethodGet, "/credentials", nil), userID)
	w := httptest.NewRecorder()

	h.ListCredentials(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp models.WebAuthnCredentialListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Credentials) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(resp.Credentials))
	}
}
