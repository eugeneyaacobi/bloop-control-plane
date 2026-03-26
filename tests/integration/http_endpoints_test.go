package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bloop-control-plane/internal/api"
	"bloop-control-plane/internal/audit"
	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/db"
	"bloop-control-plane/internal/db/migrations"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
)

type captureEmailSender struct {
	to    string
	token string
	err   error
}

func (c *captureEmailSender) SendVerificationEmail(ctx context.Context, toEmail, token string) error {
	c.to = toEmail
	c.token = token
	return c.err
}

func setupHTTPTest(t *testing.T) (*pgxpool.Pool, http.Handler, *captureEmailSender) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	migrationsDir := filepath.Join("..", "..", "internal", "db", "migrations")
	if err := migrations.Apply(context.Background(), pool, migrationsDir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	for _, table := range []string{"audit_events", "email_verifications", "signup_requests", "onboarding_steps", "review_flags", "tunnels", "memberships", "users", "accounts"} {
		if _, err := pool.Exec(context.Background(), "TRUNCATE TABLE "+table+" RESTART IDENTITY CASCADE"); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
	if err := db.Seed(context.Background(), pool); err != nil {
		t.Fatalf("seed db: %v", err)
	}
	customerRepo := repository.NewPostgresCustomerRepository(pool)
	adminRepo := repository.NewPostgresAdminRepository(pool)
	onboardingRepo := repository.NewPostgresOnboardingRepository(pool)
	signupRepo := repository.NewPostgresSignupRepository(pool)
	sessionRepo := repository.NewPostgresSessionRepository(pool)
	runtimeRepo := repository.NewPostgresRuntimeRepository(pool)
	email := &captureEmailSender{}
	cfg := &config.Config{VerificationTokenTTL: time.Hour, AllowDevAuthFallback: true, SessionSecret: "integration-test-secret", SessionCookieName: session.DefaultCookieName}
	issuerTokens, _ := session.NewTokenManager(cfg.SessionSecret)
	issuer := session.NewIssuer(issuerTokens, cfg.SessionCookieName, cfg.SessionTTL)
	provisioningRepo := repository.NewPostgresProvisioningRepository(pool)
	signupSvc := service.NewSignupService(signupRepo, email, audit.New(pool), cfg, issuer, provisioningRepo)
	router := api.NewRouter(api.RouterDeps{
		CustomerRepo:   customerRepo,
		AdminRepo:      adminRepo,
		OnboardingRepo: onboardingRepo,
		SessionRepo:    sessionRepo,
		RuntimeRepo:    runtimeRepo,
		SignupService:  signupSvc,
		Config:         cfg,
		IsReady:        func() bool { return true },
	})
	return pool, router, email
}

func TestHTTPReadEndpointsReturnJSON(t *testing.T) {
	pool, router, _ := setupHTTPTest(t)
	defer pool.Close()

	paths := []string{
		"/healthz",
		"/readyz",
		"/api/customer/workspace",
		"/api/customer/tunnels",
		"/api/customer/tunnels/api",
		"/api/admin/overview",
		"/api/admin/users",
		"/api/admin/tunnels",
		"/api/admin/review-queue",
		"/api/onboarding/steps",
		"/api/session/me",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 got %d for %s body=%s", w.Code, path, w.Body.String())
			}
			if got := w.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("expected application/json got %q for %s", got, path)
			}
		})
	}
}

func TestHTTPSignupFlowAndAuditEvents(t *testing.T) {
	pool, router, email := setupHTTPTest(t)
	defer pool.Close()

	requestBody := bytes.NewBufferString(`{"email":"http-test@example.com"}`)
	requestReq := httptest.NewRequest(http.MethodPost, "/api/onboarding/signup/request", requestBody)
	requestReq.Header.Set("Content-Type", "application/json")
	requestW := httptest.NewRecorder()
	router.ServeHTTP(requestW, requestReq)
	if requestW.Code != http.StatusAccepted {
		t.Fatalf("expected 202 got %d body=%s", requestW.Code, requestW.Body.String())
	}
	if email.token == "" {
		t.Fatalf("expected captured token from email sender")
	}
	if bytes.Contains(requestW.Body.Bytes(), []byte(email.token)) {
		t.Fatalf("raw token leaked in signup request response")
	}

	verifyPayload, _ := json.Marshal(map[string]string{"token": email.token})
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/onboarding/signup/verify", bytes.NewReader(verifyPayload))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyW := httptest.NewRecorder()
	router.ServeHTTP(verifyW, verifyReq)
	if verifyW.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", verifyW.Code, verifyW.Body.String())
	}
	var verifyResp struct {
		Verified bool   `json:"verified"`
		Status   string `json:"status"`
		Session  *struct {
			Token      string `json:"token"`
			CookieName string `json:"cookieName"`
		} `json:"session"`
	}
	if err := json.Unmarshal(verifyW.Body.Bytes(), &verifyResp); err != nil {
		t.Fatalf("decode verify response: %v", err)
	}
	if !verifyResp.Verified || verifyResp.Status != string(service.SignupVerifyStatusVerified) {
		t.Fatalf("expected verified status, got %+v", verifyResp)
	}
	if verifyResp.Session == nil || verifyResp.Session.Token == "" || verifyResp.Session.CookieName == "" {
		t.Fatalf("expected issued session in verify response, got %+v", verifyResp)
	}
	mgr, err := session.NewTokenManager("integration-test-secret")
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	issued, err := mgr.Parse(verifyResp.Session.Token, time.Now().UTC())
	if err != nil {
		t.Fatalf("parse issued session token: %v", err)
	}
	if issued.AccountID == "acct_default" || issued.UserID == "user_gene" {
		t.Fatalf("expected provisioned identity instead of seeded fallback, got %+v", issued)
	}
	cookies := verifyW.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name == "" || cookies[0].Value == "" {
		t.Fatalf("expected verify handler to set session cookie")
	}
	if bytes.Contains(verifyW.Body.Bytes(), []byte(email.token)) {
		t.Fatalf("raw token leaked in signup verify response")
	}

	var events []string
	rows, err := pool.Query(context.Background(), "select event_type from audit_events order by created_at asc")
	if err != nil {
		t.Fatalf("query audit events: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var event string
		if err := rows.Scan(&event); err != nil {
			t.Fatalf("scan audit event: %v", err)
		}
		events = append(events, event)
	}
	joined, _ := json.Marshal(events)
	for _, want := range []string{"signup.requested", "signup.email_sent", "signup.verified"} {
		if !bytes.Contains(joined, []byte(want)) {
			t.Fatalf("expected audit event %s in %s", want, string(joined))
		}
	}
	if bytes.Contains(joined, []byte(email.token)) {
		t.Fatalf("raw token leaked in audit event list")
	}

	repeatReq := httptest.NewRequest(http.MethodPost, "/api/onboarding/signup/verify", bytes.NewReader(verifyPayload))
	repeatReq.Header.Set("Content-Type", "application/json")
	repeatW := httptest.NewRecorder()
	router.ServeHTTP(repeatW, repeatReq)
	if repeatW.Code != http.StatusOK {
		t.Fatalf("expected repeat verify 200 got %d body=%s", repeatW.Code, repeatW.Body.String())
	}
	var repeatResp struct {
		Verified bool   `json:"verified"`
		Status   string `json:"status"`
		Session  *struct {
			Token string `json:"token"`
		} `json:"session"`
	}
	if err := json.Unmarshal(repeatW.Body.Bytes(), &repeatResp); err != nil {
		t.Fatalf("decode repeat verify response: %v", err)
	}
	if repeatResp.Verified || repeatResp.Status != string(service.SignupVerifyStatusUsed) {
		t.Fatalf("expected used status on repeat verify, got %+v", repeatResp)
	}
	if repeatResp.Session == nil || repeatResp.Session.Token == "" {
		t.Fatalf("expected repeat verify to refresh session for existing identity")
	}
}

func TestHTTPLogoutClearsCookie(t *testing.T) {
	_, router, _ := setupHTTPTest(t)
	req := httptest.NewRequest(http.MethodPost, "/api/session/logout", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 logout got %d body=%s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 || cookies[0].MaxAge != -1 {
		t.Fatalf("expected clearing cookie on logout")
	}
}

func TestHTTPSessionMeUsesSignedTokenWhenFallbackDisabled(t *testing.T) {
	pool, _, _ := setupHTTPTest(t)
	defer pool.Close()

	customerRepo := repository.NewPostgresCustomerRepository(pool)
	adminRepo := repository.NewPostgresAdminRepository(pool)
	onboardingRepo := repository.NewPostgresOnboardingRepository(pool)
	signupRepo := repository.NewPostgresSignupRepository(pool)
	sessionRepo := repository.NewPostgresSessionRepository(pool)
	runtimeRepo := repository.NewPostgresRuntimeRepository(pool)
	cfg := &config.Config{VerificationTokenTTL: time.Hour, AllowDevAuthFallback: false, SessionSecret: "integration-test-secret", SessionCookieName: session.DefaultCookieName}
	router := api.NewRouter(api.RouterDeps{
		CustomerRepo:   customerRepo,
		AdminRepo:      adminRepo,
		OnboardingRepo: onboardingRepo,
		SessionRepo:    sessionRepo,
		RuntimeRepo:    runtimeRepo,
		SignupService:  service.NewSignupService(signupRepo, &captureEmailSender{}, audit.New(pool), cfg, nil, nil),
		Config:         cfg,
		IsReady:        func() bool { return true },
	})

	unauthReq := httptest.NewRequest(http.MethodGet, "/api/session/me", nil)
	unauthW := httptest.NewRecorder()
	router.ServeHTTP(unauthW, unauthReq)
	if unauthW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without headers when fallback disabled, got %d", unauthW.Code)
	}

	mgr, err := session.NewTokenManager(cfg.SessionSecret)
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	token, err := mgr.Sign(session.TokenClaims{UserID: "user_gene", AccountID: "acct_default", Role: "owner", ExpiresAt: time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/session/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with signed token, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if resp["prototype"] != false {
		t.Fatalf("expected explicit header session to avoid prototype mode, got %+v", resp)
	}
	if resp["accountId"] != "acct_default" || resp["userId"] != "user_gene" {
		t.Fatalf("unexpected session response: %+v", resp)
	}
}
