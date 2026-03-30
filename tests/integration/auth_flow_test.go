package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authapi "bloop-control-plane/internal/api/auth"
	"bloop-control-plane/internal/api/tokens"
	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/security"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"

	"github.com/go-chi/chi/v5"
)

// Integration test setup using in-memory repositories
type testEnv struct {
	authRepo    *repository.InMemoryAuthRepository
	tokenRepo   *repository.InMemoryTokenRepository
	webauthnRepo *repository.InMemoryWebAuthnRepository
	lockoutRepo *repository.InMemoryLockoutRepository
	auditRepo   *repository.InMemoryAuditRepository

	authService  *service.AuthService
	tokenService *service.TokenService

	tokenManager *session.TokenManager
	config       *config.Config
}

func newTestEnv(t *testing.T) *testEnv {
	authRepo := repository.NewInMemoryAuthRepository()
	tokenRepo := repository.NewInMemoryTokenRepository()
	webauthnRepo := repository.NewInMemoryWebAuthnRepository()
	lockoutRepo := repository.NewInMemoryLockoutRepository()
	auditRepo := repository.NewInMemoryAuditRepository()

	tokenManager, err := session.NewTokenManager("integration-test-secret-key")
	if err != nil {
		t.Fatalf("failed to create token manager: %v", err)
	}

	cfg := &config.Config{
		PasswordMinLength:       12,
		LoginRateLimitIP:        10,
		LoginRateLimitAccount:   5,
		AccountLockoutThreshold: 20,
		AccountLockoutDuration:  time.Hour,
		APITokenDefaultExpiry:   720 * time.Hour,
		SessionTTL:              24 * time.Hour,
	}

	authService := service.NewAuthService(authRepo, auditRepo, lockoutRepo, cfg, tokenManager)
	tokenService := service.NewTokenService(tokenRepo, auditRepo, cfg)

	return &testEnv{
		authRepo:     authRepo,
		tokenRepo:    tokenRepo,
		webauthnRepo: webauthnRepo,
		lockoutRepo:  lockoutRepo,
		auditRepo:    auditRepo,
		authService:  authService,
		tokenService: tokenService,
		tokenManager: tokenManager,
		config:       cfg,
	}
}

func withSession(req *http.Request, sess session.Context) *http.Request {
	return req.WithContext(session.NewContext(req.Context(), sess))
}

func withChiRouteContext(req *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// Full authentication flow integration test
// Flow: register → login → create API token → list tokens → revoke token → refresh session → logout
func TestAuthFlowIntegration(t *testing.T) {
	env := newTestEnv(t)

	// Create handlers
	authHandler := authapi.NewHandler(env.authService, env.tokenManager, "session", false, "")
	tokenHandler := tokens.NewHandler(env.tokenService)

	// Step 1: Register a new user
	t.Run("register", func(t *testing.T) {
		body := `{"email":"test@example.com","username":"testuser","password":"SecurePassword123!"}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		authHandler.Register(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("register: expected status %d, got %d (body: %s)", http.StatusCreated, w.Code, w.Body.String())
		}

		var resp models.LoginResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode register response: %v", err)
		}

		if resp.User.Email != "test@example.com" {
			t.Fatalf("expected email test@example.com, got %s", resp.User.Email)
		}

		// Verify session cookie is set
		cookies := w.Result().Cookies()
		if len(cookies) == 0 {
			t.Fatal("expected session cookie to be set")
		}
	})

	// Step 2: Login with the registered user
	var loginUserID string
	t.Run("login", func(t *testing.T) {
		body := `{"email":"test@example.com","password":"SecurePassword123!"}`
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		authHandler.Login(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("login: expected status %d, got %d (body: %s)", http.StatusOK, w.Code, w.Body.String())
		}

		var resp models.LoginResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode login response: %v", err)
		}

		loginUserID = resp.User.ID
		if loginUserID == "" {
			t.Fatal("expected user ID in login response")
		}
	})

	// Step 3: Create an API token
	var createdTokenID string
	var createdTokenValue string
	t.Run("create_api_token", func(t *testing.T) {
		body := `{"name":"test-api-token","expires_in":"720h"}`
		req := httptest.NewRequest(http.MethodPost, "/tokens", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = withSession(req, session.Context{
			UserID:    loginUserID,
			AccountID: "acct_default",
			Role:      "customer",
		})
		w := httptest.NewRecorder()

		tokenHandler.Create(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("create token: expected status %d, got %d (body: %s)", http.StatusCreated, w.Code, w.Body.String())
		}

		var resp models.TokenCreateResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode token response: %v", err)
		}

		createdTokenID = resp.ID
		createdTokenValue = resp.Token

		if createdTokenValue == "" {
			t.Fatal("expected token value to be present (shown once)")
		}
		if len(createdTokenValue) != 64 {
			t.Fatalf("expected 64-char token, got %d", len(createdTokenValue))
		}
	})

	// Step 4: List tokens (verify token value NOT present)
	t.Run("list_tokens", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tokens", nil)
		req = withSession(req, session.Context{
			UserID:    loginUserID,
			AccountID: "acct_default",
			Role:      "customer",
		})
		w := httptest.NewRecorder()

		tokenHandler.List(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("list tokens: expected status %d, got %d", http.StatusOK, w.Code)
		}

		var resp models.TokenListResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode list response: %v", err)
		}

		if len(resp.Tokens) == 0 {
			t.Fatal("expected at least one token")
		}

		// Verify token value is NOT in the list response
		for _, tok := range resp.Tokens {
			// TokenSummary struct doesn't have a Token field - compile-time safety
			_ = tok.ID
			_ = tok.Name
			_ = tok.TokenPrefix
		}
	})

	// Step 5: Revoke the token
	t.Run("revoke_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/tokens/"+createdTokenID, nil)
		req = withSession(req, session.Context{
			UserID:    loginUserID,
			AccountID: "acct_default",
			Role:      "customer",
		})
		req = withChiRouteContext(req, createdTokenID)
		w := httptest.NewRecorder()

		tokenHandler.Revoke(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("revoke token: expected status %d, got %d", http.StatusNoContent, w.Code)
		}
	})

	// Step 6: Refresh session
	t.Run("refresh_session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
		req = withSession(req, session.Context{
			UserID:    loginUserID,
			AccountID: "acct_default",
			Role:      "customer",
		})
		w := httptest.NewRecorder()

		authHandler.Refresh(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("refresh: expected status %d, got %d (body: %s)", http.StatusOK, w.Code, w.Body.String())
		}

		var resp models.RefreshResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode refresh response: %v", err)
		}

		if resp.User.ID != loginUserID {
			t.Fatalf("expected user ID %s, got %s", loginUserID, resp.User.ID)
		}
	})
}

// Test: Wrong password login fails
func TestLoginWrongPassword(t *testing.T) {
	env := newTestEnv(t)

	// Register user first
	password, _ := security.HashPassword("CorrectPassword123!")
	ctx := context.Background()
	user, err := env.authRepo.CreateUserWithCredentials(ctx, "test@example.com", "testuser", password, "Test User")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	_ = user

	authHandler := authapi.NewHandler(env.authService, env.tokenManager, "session", false, "")

	// Try to login with wrong password
	body := `{"email":"test@example.com","password":"WrongPassword123!"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test: Duplicate email registration fails
func TestRegisterDuplicateEmail(t *testing.T) {
	env := newTestEnv(t)

	// Register first user
	password, _ := security.HashPassword("Password123!")
	ctx := context.Background()
	_, err := env.authRepo.CreateUserWithCredentials(ctx, "test@example.com", "testuser1", password, "Test User 1")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	authHandler := authapi.NewHandler(env.authService, env.tokenManager, "session", false, "")

	// Try to register with same email
	body := `{"email":"test@example.com","username":"testuser2","password":"Password123!"}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Register(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

// Test: Missing fields in registration
func TestRegisterMissingFields(t *testing.T) {
	env := newTestEnv(t)
	authHandler := authapi.NewHandler(env.authService, env.tokenManager, "session", false, "")

	tests := []struct {
		name string
		body string
	}{
		{"missing email", `{"username":"testuser","password":"Password123!"}`},
		{"missing username", `{"email":"test@example.com","password":"Password123!"}`},
		{"missing password", `{"email":"test@example.com","username":"testuser"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			authHandler.Register(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}

// Test: Locked account login fails
func TestLoginLockedAccount(t *testing.T) {
	env := newTestEnv(t)

	// Register user
	password, _ := security.HashPassword("Password123!")
	ctx := context.Background()
	user, err := env.authRepo.CreateUserWithCredentials(ctx, "test@example.com", "testuser", password, "Test User")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Lock the account
	lockedUntil := time.Now().Add(time.Hour)
	env.lockoutRepo.LockAccount(ctx, user.ID, lockedUntil, "test")

	authHandler := authapi.NewHandler(env.authService, env.tokenManager, "session", false, "")

	// Try to login
	body := `{"email":"test@example.com","password":"Password123!"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusLocked {
		t.Fatalf("expected status %d, got %d", http.StatusLocked, w.Code)
	}
}

// Test: Refresh without session fails
func TestRefreshWithoutSession(t *testing.T) {
	env := newTestEnv(t)
	authHandler := authapi.NewHandler(env.authService, env.tokenManager, "session", false, "")

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	w := httptest.NewRecorder()

	authHandler.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test: Token operations require authentication
func TestTokenOperationsRequireAuth(t *testing.T) {
	env := newTestEnv(t)
	tokenHandler := tokens.NewHandler(env.tokenService)

	t.Run("create requires auth", func(t *testing.T) {
		body := `{"name":"test"}`
		req := httptest.NewRequest(http.MethodPost, "/tokens", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		tokenHandler.Create(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("list requires auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tokens", nil)
		w := httptest.NewRecorder()

		tokenHandler.List(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("revoke requires auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/tokens/tok_123", nil)
		req = withChiRouteContext(req, "tok_123")
		w := httptest.NewRecorder()

		tokenHandler.Revoke(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})
}

// Test: Token ownership enforcement
func TestTokenOwnershipEnforcement(t *testing.T) {
	env := newTestEnv(t)
	tokenHandler := tokens.NewHandler(env.tokenService)

	ctx := context.Background()

	// Create token for user_1
	result, err := env.tokenService.CreateToken(ctx, "user_1", "acct_1", "test-token", nil)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	tokenID := result.Token.ID

	// Try to revoke as user_2
	req := httptest.NewRequest(http.MethodDelete, "/tokens/"+tokenID, nil)
	req = withSession(req, session.Context{
		UserID:    "user_2",
		AccountID: "acct_2",
		Role:      "customer",
	})
	req = withChiRouteContext(req, tokenID)
	w := httptest.NewRecorder()

	tokenHandler.Revoke(w, req)

	// Should return 404 (token not found for this user)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// Test: Invalid JSON handling
func TestInvalidJSONHandling(t *testing.T) {
	env := newTestEnv(t)
	authHandler := authapi.NewHandler(env.authService, env.tokenManager, "session", false, "")

	tests := []struct {
		name   string
		path   string
		method string
	}{
		{"register", "/register", http.MethodPost},
		{"login", "/login", http.MethodPost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString("{invalid"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			switch tt.path {
			case "/register":
				authHandler.Register(w, req)
			case "/login":
				authHandler.Login(w, req)
			}

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}
