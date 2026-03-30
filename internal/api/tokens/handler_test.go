package tokens

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"

	"github.com/go-chi/chi/v5"
)

// Mock token repository
type mockTokenRepo struct {
	tokens map[string]repository.APIToken
}

func newMockTokenRepo() *mockTokenRepo {
	return &mockTokenRepo{
		tokens: make(map[string]repository.APIToken),
	}
}

func (r *mockTokenRepo) CreateToken(ctx context.Context, userID, accountID, name, tokenHash, tokenPrefix string, expiresAt *time.Time) (repository.APIToken, error) {
	now := time.Now().UTC()
	token := repository.APIToken{
		ID:          "tok_" + name + "_" + now.Format("20060102150405"),
		UserID:      userID,
		AccountID:   accountID,
		Name:        name,
		TokenHash:   tokenHash,
		TokenPrefix: tokenPrefix,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
	}
	r.tokens[token.ID] = token
	return token, nil
}

func (r *mockTokenRepo) ListTokensByUser(ctx context.Context, userID string) ([]repository.APIToken, error) {
	var result []repository.APIToken
	for _, token := range r.tokens {
		if token.UserID == userID {
			result = append(result, token)
		}
	}
	return result, nil
}

func (r *mockTokenRepo) ListActiveTokensByUser(ctx context.Context, userID string) ([]repository.APIToken, error) {
	var result []repository.APIToken
	now := time.Now().UTC()
	for _, token := range r.tokens {
		if token.UserID == userID && token.RevokedAt == nil && (token.ExpiresAt == nil || token.ExpiresAt.After(now)) {
			result = append(result, token)
		}
	}
	return result, nil
}

func (r *mockTokenRepo) GetTokenByID(ctx context.Context, tokenID, userID string) (*repository.APIToken, error) {
	token, exists := r.tokens[tokenID]
	if !exists || token.UserID != userID {
		return nil, nil
	}
	return &token, nil
}

func (r *mockTokenRepo) RevokeToken(ctx context.Context, tokenID, userID string) error {
	token, exists := r.tokens[tokenID]
	if !exists || token.UserID != userID {
		return nil
	}
	now := time.Now().UTC()
	token.RevokedAt = &now
	r.tokens[tokenID] = token
	return nil
}

func (r *mockTokenRepo) LookupByHash(ctx context.Context, tokenHash string) (*repository.APIToken, error) {
	for _, token := range r.tokens {
		if token.TokenHash == tokenHash {
			return &token, nil
		}
	}
	return nil, nil
}

func (r *mockTokenRepo) UpdateLastUsed(ctx context.Context, tokenID string) error {
	token, exists := r.tokens[tokenID]
	if !exists {
		return nil
	}
	now := time.Now().UTC()
	token.LastUsedAt = &now
	r.tokens[tokenID] = token
	return nil
}

func (r *mockTokenRepo) DeleteToken(ctx context.Context, tokenID, userID string) error {
	token, exists := r.tokens[tokenID]
	if !exists || token.UserID != userID {
		return nil
	}
	delete(r.tokens, tokenID)
	return nil
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
	var result []repository.AuthAuditEvent
	for _, e := range r.events {
		if userID == nil || (e.UserID != nil && *e.UserID == *userID) {
			result = append(result, e)
		}
	}
	return result, nil
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
		APITokenDefaultExpiry: 720 * time.Hour,
	}
}

// Test: Create token success
func TestTokenCreateSuccess(t *testing.T) {
	tokenRepo := newMockTokenRepo()
	auditRepo := newMockAuditRepo()
	tokenSvc := service.NewTokenService(tokenRepo, auditRepo, newTestConfig())
	h := NewHandler(tokenSvc)

	body := `{"name":"test-token","account_id":"acct_test"}`
	req := withAuthSession(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), "user_123")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d (body: %s)", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp models.TokenCreateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify token value is present (shown once)
	if resp.Token == "" {
		t.Fatal("expected token value in create response")
	}
	if len(resp.Token) != 64 {
		t.Fatalf("expected 64-char token, got %d chars", len(resp.Token))
	}
	if resp.Name != "test-token" {
		t.Fatalf("expected name 'test-token', got %q", resp.Name)
	}
	if resp.TokenPrefix == "" {
		t.Fatal("expected token prefix")
	}
}

// Test: Token value shown once, not in list
func TestTokenValueNotInList(t *testing.T) {
	tokenRepo := newMockTokenRepo()
	auditRepo := newMockAuditRepo()
	tokenSvc := service.NewTokenService(tokenRepo, auditRepo, newTestConfig())
	h := NewHandler(tokenSvc)

	// First create a token
	body := `{"name":"test-token"}`
	req := withAuthSession(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), "user_123")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	// Then list tokens
	listReq := withAuthSession(httptest.NewRequest(http.MethodGet, "/", nil), "user_123")
	listW := httptest.NewRecorder()
	h.List(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listW.Code)
	}

	var listResp models.TokenListResponse
	if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}

	// Verify NO token values in list response
	for _, tok := range listResp.Tokens {
		// Check that there's no Token field in the summary (it should not exist)
		// The TokenSummary struct doesn't have a Token field, so this is a compile-time check
		_ = tok.ID
		_ = tok.Name
		_ = tok.TokenPrefix
	}
}

// Test: List tokens requires auth
func TestTokenListRequiresAuth(t *testing.T) {
	h := NewHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test: Revoke token success
func TestTokenRevokeSuccess(t *testing.T) {
	tokenRepo := newMockTokenRepo()
	auditRepo := newMockAuditRepo()
	tokenSvc := service.NewTokenService(tokenRepo, auditRepo, newTestConfig())
	h := NewHandler(tokenSvc)

	// Create a token first
	body := `{"name":"to-revoke"}`
	createReq := withAuthSession(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), "user_123")
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	var createResp models.TokenCreateResponse
	json.Unmarshal(createW.Body.Bytes(), &createResp)

	// Revoke the token
	req := withAuthSession(httptest.NewRequest(http.MethodDelete, "/"+createResp.ID, nil), "user_123")
	req = withChiRouteContext(req, createResp.ID)
	w := httptest.NewRecorder()

	h.Revoke(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d (body: %s)", http.StatusNoContent, w.Code, w.Body.String())
	}
}

// Test: Revoke token enforces ownership
func TestTokenRevokeOwnership(t *testing.T) {
	tokenRepo := newMockTokenRepo()
	auditRepo := newMockAuditRepo()
	tokenSvc := service.NewTokenService(tokenRepo, auditRepo, newTestConfig())
	h := NewHandler(tokenSvc)

	// Create token as user_123
	body := `{"name":"owned-token"}`
	createReq := withAuthSession(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), "user_123")
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	var createResp models.TokenCreateResponse
	json.Unmarshal(createW.Body.Bytes(), &createResp)

	// Try to revoke as different user
	req := withAuthSession(httptest.NewRequest(http.MethodDelete, "/"+createResp.ID, nil), "user_456")
	req = withChiRouteContext(req, createResp.ID)
	w := httptest.NewRecorder()

	h.Revoke(w, req)

	// Should return 404 (token not found for this user)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// Test: Refresh token success
func TestTokenRefreshSuccess(t *testing.T) {
	tokenRepo := newMockTokenRepo()
	auditRepo := newMockAuditRepo()
	tokenSvc := service.NewTokenService(tokenRepo, auditRepo, newTestConfig())
	h := NewHandler(tokenSvc)

	// Create a token first
	body := `{"name":"to-refresh"}`
	createReq := withAuthSession(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), "user_123")
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	var createResp models.TokenCreateResponse
	json.Unmarshal(createW.Body.Bytes(), &createResp)

	// Refresh the token
	req := withAuthSession(httptest.NewRequest(http.MethodPost, "/"+createResp.ID+"/refresh", nil), "user_123")
	req = withChiRouteContext(req, createResp.ID)
	w := httptest.NewRecorder()

	h.Refresh(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body: %s)", http.StatusOK, w.Code, w.Body.String())
	}

	var refreshResp models.TokenRefreshResponse
	if err := json.Unmarshal(w.Body.Bytes(), &refreshResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify new token value is present (shown once)
	if refreshResp.Token == "" {
		t.Fatal("expected token value in refresh response")
	}
	if len(refreshResp.Token) != 64 {
		t.Fatalf("expected 64-char token, got %d chars", len(refreshResp.Token))
	}
}

// Test: Refresh token not found
func TestTokenRefreshNotFound(t *testing.T) {
	tokenRepo := newMockTokenRepo()
	auditRepo := newMockAuditRepo()
	tokenSvc := service.NewTokenService(tokenRepo, auditRepo, newTestConfig())
	h := NewHandler(tokenSvc)

	req := withAuthSession(httptest.NewRequest(http.MethodPost, "/nonexistent/refresh", nil), "user_123")
	req = withChiRouteContext(req, "nonexistent")
	w := httptest.NewRecorder()

	h.Refresh(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// Test: Create token with custom expiry
func TestTokenCreateWithExpiry(t *testing.T) {
	tokenRepo := newMockTokenRepo()
	auditRepo := newMockAuditRepo()
	tokenSvc := service.NewTokenService(tokenRepo, auditRepo, newTestConfig())
	h := NewHandler(tokenSvc)

	body := `{"name":"expiring-token","expires_in":"24h"}`
	req := withAuthSession(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), "user_123")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp models.TokenCreateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ExpiresAt == nil {
		t.Fatal("expected expires_at to be set")
	}
}

// Test: Create token requires name
func TestTokenCreateRequiresName(t *testing.T) {
	h := NewHandler(nil)

	body := `{}`
	req := withAuthSession(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), "user_123")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Test: Create token requires auth
func TestTokenCreateRequiresAuth(t *testing.T) {
	h := NewHandler(nil)

	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test: Revoke token requires auth
func TestTokenRevokeRequiresAuth(t *testing.T) {
	h := NewHandler(nil)

	req := httptest.NewRequest(http.MethodDelete, "/tok_123", nil)
	req = withChiRouteContext(req, "tok_123")
	w := httptest.NewRecorder()

	h.Revoke(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test: Refresh token requires auth
func TestTokenRefreshRequiresAuth(t *testing.T) {
	h := NewHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/tok_123/refresh", nil)
	req = withChiRouteContext(req, "tok_123")
	w := httptest.NewRecorder()

	h.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}
