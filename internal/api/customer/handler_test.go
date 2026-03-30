package customer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/runtime"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
	"github.com/go-chi/chi/v5"
)

type fakeCustomerRepo struct {
	workspaceAccount models.Account
	workspaceTunnels []models.Tunnel
	listTunnels      []models.Tunnel
	tunnel           *models.Tunnel
	installations    []models.RuntimeInstallation
	overlay          *repository.RuntimeOverlay
	lastAccountID    string
	err              error
	createdTunnel    *models.Tunnel
	updatedTunnel    *models.Tunnel
	deletedTunnelID string
	hostnameTunnel   *models.Tunnel
	runtimeStatus    struct {
		status     string
		degraded   bool
		observedAt *time.Time
	}
}

func (f *fakeCustomerRepo) GetWorkspace(ctx context.Context, accountID string) (models.Account, []models.Tunnel, error) {
	f.lastAccountID = accountID
	if f.err != nil {
		return models.Account{}, nil, f.err
	}
	return f.workspaceAccount, f.workspaceTunnels, nil
}

func (f *fakeCustomerRepo) ListTunnels(ctx context.Context, accountID string) ([]models.Tunnel, error) {
	f.lastAccountID = accountID
	if f.err != nil {
		return nil, f.err
	}
	return f.listTunnels, nil
}

func (f *fakeCustomerRepo) GetTunnelByID(ctx context.Context, accountID, tunnelID string) (*models.Tunnel, error) {
	f.lastAccountID = accountID
	if f.err != nil {
		return nil, f.err
	}
	return f.tunnel, nil
}

func (f *fakeCustomerRepo) ListInstallations(ctx context.Context, accountID string) ([]models.RuntimeInstallation, error) {
	f.lastAccountID = accountID
	if f.err != nil {
		return nil, f.err
	}
	return f.installations, nil
}

func (f *fakeCustomerRepo) GetRuntimeOverlayByTunnel(ctx context.Context, accountID, tunnelID string) (*repository.RuntimeOverlay, error) {
	f.lastAccountID = accountID
	if f.err != nil {
		return nil, f.err
	}
	return f.overlay, nil
}

func (f *fakeCustomerRepo) CreateTunnel(ctx context.Context, accountID string, tunnel models.Tunnel) (*models.Tunnel, error) {
	f.lastAccountID = accountID
	if f.err != nil {
		return nil, f.err
	}
	now := time.Now()
	tunnel.CreatedAt = now
	tunnel.UpdatedAt = now
	tunnel.AccountID = accountID
	f.createdTunnel = &tunnel
	return &tunnel, nil
}

func (f *fakeCustomerRepo) UpdateTunnel(ctx context.Context, accountID, tunnelID string, tunnel models.Tunnel) (*models.Tunnel, error) {
	f.lastAccountID = accountID
	if f.err != nil {
		return nil, f.err
	}
	if f.tunnel == nil {
		return nil, nil // simulate not found
	}
	tunnel.ID = tunnelID
	tunnel.AccountID = accountID
	tunnel.CreatedAt = f.tunnel.CreatedAt
	tunnel.UpdatedAt = time.Now()
	f.updatedTunnel = &tunnel
	return &tunnel, nil
}

func (f *fakeCustomerRepo) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	f.lastAccountID = accountID
	if f.err != nil {
		return f.err
	}
	if f.tunnel == nil {
		return nil // simulate not found (returns no error)
	}
	f.deletedTunnelID = tunnelID
	return nil
}

func (f *fakeCustomerRepo) GetTunnelByHostname(ctx context.Context, hostname string) (*models.Tunnel, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.hostnameTunnel, nil
}

func (f *fakeCustomerRepo) GetRuntimeStatusByTunnelID(ctx context.Context, accountID, tunnelID string) (status string, degraded bool, observedAt *time.Time, err error) {
	if f.err != nil {
		return "", false, nil, f.err
	}
	return f.runtimeStatus.status, f.runtimeStatus.degraded, f.runtimeStatus.observedAt, nil
}

func withCustomerSession(req *http.Request, accountID string) *http.Request {
	return req.WithContext(session.NewContext(req.Context(), session.Context{UserID: "user_test", AccountID: accountID, Role: "customer"}))
}

func TestWorkspaceReturnsJSON(t *testing.T) {
	repo := &fakeCustomerRepo{workspaceAccount: models.Account{ID: "acct_default", DisplayName: "Gene / default-org"}, workspaceTunnels: []models.Tunnel{{ID: "api", Hostname: "api.bloop.to", Access: "public"}}}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}
	req := withCustomerSession(httptest.NewRequest(http.MethodGet, "/workspace", nil), "acct_from_header")
	w := httptest.NewRecorder()

	h.Workspace(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json got %q", got)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["accountName"] != "Gene / default-org" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if repo.lastAccountID != "acct_from_header" {
		t.Fatalf("expected session account id to be used, got %q", repo.lastAccountID)
	}
}

func TestWorkspaceReturns500OnServiceError(t *testing.T) {
	h := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{err: errors.New("boom")}, nil)}
	req := withCustomerSession(httptest.NewRequest(http.MethodGet, "/workspace", nil), "acct_default")
	w := httptest.NewRecorder()

	h.Workspace(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestTunnelsReturnsJSON(t *testing.T) {
	h := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{listTunnels: []models.Tunnel{{ID: "api", Hostname: "api.bloop.to"}}}, nil)}
	req := withCustomerSession(httptest.NewRequest(http.MethodGet, "/tunnels", nil), "acct_default")
	w := httptest.NewRecorder()

	h.Tunnels(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json got %q", got)
	}
}

func TestCustomerHandlersRequireSession(t *testing.T) {
	h := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{}, nil)}
	for _, fn := range []func(http.ResponseWriter, *http.Request){h.Workspace, h.Tunnels} {
		w := httptest.NewRecorder()
		fn(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected %d got %d", http.StatusUnauthorized, w.Code)
		}
	}
}

func TestTunnelDetailPaths(t *testing.T) {
	good := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{tunnel: &models.Tunnel{ID: "api", Hostname: "api.bloop.to"}}, nil)}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "api")
	req := withCustomerSession(httptest.NewRequest(http.MethodGet, "/tunnels/api", nil), "acct_default")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	good.TunnelDetail(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}

	badReq := withCustomerSession(httptest.NewRequest(http.MethodGet, "/tunnels/../../bad", nil), "acct_default")
	badCtx := chi.NewRouteContext()
	badCtx.URLParams.Add("id", "../../bad")
	badReq = badReq.WithContext(context.WithValue(badReq.Context(), chi.RouteCtxKey, badCtx))
	badW := httptest.NewRecorder()
	good.TunnelDetail(badW, badReq)
	if badW.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, badW.Code)
	}

	notFound := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{tunnel: nil}, nil)}
	nfReq := withCustomerSession(httptest.NewRequest(http.MethodGet, "/tunnels/missing", nil), "acct_default")
	nfCtx := chi.NewRouteContext()
	nfCtx.URLParams.Add("id", "missing")
	nfReq = nfReq.WithContext(context.WithValue(nfReq.Context(), chi.RouteCtxKey, nfCtx))
	nfW := httptest.NewRecorder()
	notFound.TunnelDetail(nfW, nfReq)
	if nfW.Code != http.StatusNotFound {
		t.Fatalf("expected %d got %d", http.StatusNotFound, nfW.Code)
	}

	broken := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{err: errors.New("boom")}, nil)}
	errReq := withCustomerSession(httptest.NewRequest(http.MethodGet, "/tunnels/api", nil), "acct_default")
	errCtx := chi.NewRouteContext()
	errCtx.URLParams.Add("id", "api")
	errReq = errReq.WithContext(context.WithValue(errReq.Context(), chi.RouteCtxKey, errCtx))
	errW := httptest.NewRecorder()
	broken.TunnelDetail(errW, errReq)
	if errW.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d got %d", http.StatusInternalServerError, errW.Code)
	}
}

// T024: Tests for tunnel CRUD endpoints

func TestCreateTunnel(t *testing.T) {
	repo := &fakeCustomerRepo{}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}

	body := `{"id":"webapp","hostname":"webapp.bloop.to","target":"localhost:8080","access":"public"}`
	req := withCustomerSession(httptest.NewRequest(http.MethodPost, "/tunnels", strings.NewReader(body)), "acct_default")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateTunnel(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d", http.StatusCreated, w.Code)
	}

	var resp models.Tunnel
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != "webapp" || resp.Hostname != "webapp.bloop.to" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if repo.createdTunnel == nil {
		t.Fatal("expected tunnel to be created")
	}
}

func TestCreateTunnelConflict(t *testing.T) {
	existingTunnel := &models.Tunnel{ID: "existing", Hostname: "taken.bloop.to"}
	repo := &fakeCustomerRepo{hostnameTunnel: existingTunnel}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}

	body := `{"id":"new","hostname":"taken.bloop.to","target":"localhost:8080","access":"public"}`
	req := withCustomerSession(httptest.NewRequest(http.MethodPost, "/tunnels", strings.NewReader(body)), "acct_default")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateTunnel(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected %d got %d", http.StatusConflict, w.Code)
	}
}

func TestCreateTunnelValidationErrors(t *testing.T) {
	h := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{}, nil)}

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"missing id", `{"hostname":"webapp.bloop.to","target":"localhost:8080","access":"public"}`, http.StatusUnprocessableEntity},
		{"missing hostname", `{"id":"webapp","target":"localhost:8080","access":"public"}`, http.StatusUnprocessableEntity},
		{"missing target", `{"id":"webapp","hostname":"webapp.bloop.to","access":"public"}`, http.StatusUnprocessableEntity},
		{"missing access", `{"id":"webapp","hostname":"webapp.bloop.to","target":"localhost:8080"}`, http.StatusUnprocessableEntity},
		{"invalid id", `{"id":"../../bad","hostname":"webapp.bloop.to","target":"localhost:8080","access":"public"}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := withCustomerSession(httptest.NewRequest(http.MethodPost, "/tunnels", strings.NewReader(tt.body)), "acct_default")
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.CreateTunnel(w, req)
			if w.Code != tt.status {
				t.Fatalf("expected %d got %d", tt.status, w.Code)
			}
		})
	}
}

func TestUpdateTunnel(t *testing.T) {
	existing := &models.Tunnel{ID: "api", Hostname: "api.bloop.to", Access: "public", CreatedAt: time.Now()}
	repo := &fakeCustomerRepo{tunnel: existing}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}

	body := `{"access":"basic_auth","basic_auth":{"username":"admin","password":"secret"}}`
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "api")
	req := withCustomerSession(httptest.NewRequest(http.MethodPut, "/tunnels/api", strings.NewReader(body)), "acct_default")
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateTunnel(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d, body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	if repo.updatedTunnel == nil {
		t.Fatal("expected tunnel to be updated")
	}
}

func TestUpdateTunnelNotFound(t *testing.T) {
	repo := &fakeCustomerRepo{tunnel: nil}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}

	body := `{"access":"basic_auth"}`
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "missing")
	req := withCustomerSession(httptest.NewRequest(http.MethodPut, "/tunnels/missing", strings.NewReader(body)), "acct_default")
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateTunnel(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected %d got %d", http.StatusNotFound, w.Code)
	}
}

func TestDeleteTunnel(t *testing.T) {
	existing := &models.Tunnel{ID: "api", Hostname: "api.bloop.to"}
	repo := &fakeCustomerRepo{tunnel: existing}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "api")
	req := withCustomerSession(httptest.NewRequest(http.MethodDelete, "/tunnels/api", nil), "acct_default")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DeleteTunnel(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected %d got %d", http.StatusNoContent, w.Code)
	}

	if repo.deletedTunnelID != "api" {
		t.Fatalf("expected tunnel id to be 'api', got %q", repo.deletedTunnelID)
	}
}

func TestDeleteTunnelNotFound(t *testing.T) {
	repo := &fakeCustomerRepo{tunnel: nil}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "missing")
	req := withCustomerSession(httptest.NewRequest(http.MethodDelete, "/tunnels/missing", nil), "acct_default")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DeleteTunnel(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected %d got %d", http.StatusNotFound, w.Code)
	}
}

// T025: Tests for validation endpoint

func TestValidateTunnelValid(t *testing.T) {
	repo := &fakeCustomerRepo{}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}

	body := `{"hostname":"api.bloop.to","target":"localhost:8080","access":"public","local_port":8080}`
	req := withCustomerSession(httptest.NewRequest(http.MethodPost, "/tunnels/validate", strings.NewReader(body)), "acct_default")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ValidateTunnel(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}

	var resp models.TunnelValidationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Valid {
		t.Fatalf("expected valid=true, got %+v", resp)
	}
}

func TestValidateTunnelDuplicateHostname(t *testing.T) {
	existing := &models.Tunnel{ID: "existing", Hostname: "taken.bloop.to", AccountID: "other_account"}
	repo := &fakeCustomerRepo{hostnameTunnel: existing}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}

	body := `{"hostname":"taken.bloop.to","target":"localhost:8080","access":"public"}`
	req := withCustomerSession(httptest.NewRequest(http.MethodPost, "/tunnels/validate", strings.NewReader(body)), "acct_default")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ValidateTunnel(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}

	var resp models.TunnelValidationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Valid {
		t.Fatal("expected valid=false for duplicate hostname")
	}
	if len(resp.Errors) == 0 {
		t.Fatal("expected errors for duplicate hostname")
	}
}

func TestValidateTunnelInvalidPort(t *testing.T) {
	repo := &fakeCustomerRepo{}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}

	body := `{"hostname":"api.bloop.to","target":"localhost:8080","access":"public","local_port":99999}`
	req := withCustomerSession(httptest.NewRequest(http.MethodPost, "/tunnels/validate", strings.NewReader(body)), "acct_default")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ValidateTunnel(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}

	var resp models.TunnelValidationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Valid {
		t.Fatal("expected valid=false for invalid port")
	}
}

func TestValidateTunnelMissingFields(t *testing.T) {
	repo := &fakeCustomerRepo{}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}

	body := `{}`
	req := withCustomerSession(httptest.NewRequest(http.MethodPost, "/tunnels/validate", strings.NewReader(body)), "acct_default")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ValidateTunnel(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}

	var resp models.TunnelValidationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Valid {
		t.Fatal("expected valid=false for missing fields")
	}
}

// T026: Tests for status endpoint

func TestTunnelStatusWithRuntimeData(t *testing.T) {
	now := time.Now()
	repo := &fakeCustomerRepo{
		runtimeStatus: struct {
			status     string
			degraded   bool
			observedAt *time.Time
		}{status: "healthy", degraded: false, observedAt: &now},
	}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "api")
	req := withCustomerSession(httptest.NewRequest(http.MethodGet, "/tunnels/api/status", nil), "acct_default")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.TunnelStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}

	var resp models.TunnelStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "healthy" {
		t.Fatalf("expected status=healthy, got %q", resp.Status)
	}
	if resp.Stale {
		t.Fatal("expected stale=false with runtime data")
	}
}

func TestTunnelStatusStale(t *testing.T) {
	repo := &fakeCustomerRepo{runtimeStatus: struct {
		status     string
		degraded   bool
		observedAt *time.Time
	}{status: "", degraded: false, observedAt: nil}}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "api")
	req := withCustomerSession(httptest.NewRequest(http.MethodGet, "/tunnels/api/status", nil), "acct_default")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.TunnelStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}

	var resp models.TunnelStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Stale {
		t.Fatal("expected stale=true without runtime data")
	}
	if resp.Status != "unknown" {
		t.Fatalf("expected status=unknown, got %q", resp.Status)
	}
}

// T027: Tests for config schema and enrollment verify

func TestConfigSchema(t *testing.T) {
	h := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{}, nil)}

	req := withCustomerSession(httptest.NewRequest(http.MethodGet, "/config/schema", nil), "acct_default")
	w := httptest.NewRecorder()

	h.ConfigSchema(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}

	var resp models.ConfigSchemaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.AccessModes) == 0 {
		t.Fatal("expected access modes in schema")
	}
	if resp.DefaultRelayURL == "" {
		t.Fatal("expected default relay URL in schema")
	}
}

func TestVerifyEnrollmentValid(t *testing.T) {
	h := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{}, nil)}

	body := `{"token":"valid_token_123"}`
	req := withCustomerSession(httptest.NewRequest(http.MethodPost, "/enrollment/verify", strings.NewReader(body)), "acct_default")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyEnrollment(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}

	var resp models.EnrollmentVerifyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Valid {
		t.Fatalf("expected valid=true, got %+v", resp)
	}
	if resp.InstallationID == "" {
		t.Fatal("expected installation ID in response")
	}
	if resp.IngestToken == "" {
		t.Fatal("expected ingest token in response")
	}
}

func TestVerifyEnrollmentInvalid(t *testing.T) {
	// Create a runtime repo that returns nil (invalid token)
	runtimeRepo := &errorRuntimeRepo{}
	h := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{}, runtimeRepo)}

	body := `{"token":"invalid_token"}`
	req := withCustomerSession(httptest.NewRequest(http.MethodPost, "/enrollment/verify", strings.NewReader(body)), "acct_default")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyEnrollment(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d got %d", http.StatusUnauthorized, w.Code)
	}

	var resp models.EnrollmentVerifyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Valid {
		t.Fatal("expected valid=false for invalid token")
	}
}

// Helper types for tests

type errorRuntimeRepo struct{}

func (r *errorRuntimeRepo) ProjectAccount(ctx context.Context, account models.Account, tunnels []models.Tunnel) (runtime.AccountProjection, error) {
	return runtime.AccountProjection{}, nil
}

func (r *errorRuntimeRepo) ProjectGlobal(ctx context.Context, tunnels []models.Tunnel, flags []models.ReviewFlag) (runtime.GlobalProjection, error) {
	return runtime.GlobalProjection{}, nil
}

func (r *errorRuntimeRepo) VerifyInstallationToken(ctx context.Context, token string) (*models.RuntimeInstallationToken, error) {
	return nil, nil // simulate invalid token
}

func (r *errorRuntimeRepo) CreateIngestToken(ctx context.Context, installationID string) (string, error) {
	return "", nil
}
