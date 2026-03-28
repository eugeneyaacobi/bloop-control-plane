package customer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
	"github.com/go-chi/chi/v5"
)

type fakeCustomerRepo struct {
	workspaceAccount models.Account
	workspaceTunnels []models.Tunnel
	listTunnels      []models.Tunnel
	tunnel           *models.Tunnel
	createdTunnel    *models.Tunnel
	updatedTunnel    *models.Tunnel
	lastAccountID    string
	lastCreate       repository.CreateTunnelParams
	lastUpdate       repository.UpdateTunnelParams
	deletedTunnelID  string
	deleteErr        error
	err              error
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

func (f *fakeCustomerRepo) CreateTunnel(ctx context.Context, accountID string, params repository.CreateTunnelParams) (*models.Tunnel, error) {
	f.lastAccountID = accountID
	f.lastCreate = params
	if f.err != nil {
		return nil, f.err
	}
	if f.createdTunnel != nil {
		return f.createdTunnel, nil
	}
	return &models.Tunnel{ID: "new-tunnel", Hostname: params.Hostname, Target: params.Target, Access: params.Access, Status: params.Status, Region: params.Region}, nil
}

func (f *fakeCustomerRepo) UpdateTunnel(ctx context.Context, accountID, tunnelID string, params repository.UpdateTunnelParams) (*models.Tunnel, error) {
	f.lastAccountID = accountID
	f.lastUpdate = params
	if f.err != nil {
		return nil, f.err
	}
	if f.updatedTunnel != nil {
		return f.updatedTunnel, nil
	}
	return &models.Tunnel{ID: tunnelID, Hostname: "api.bloop.to", Target: params.Target, Access: params.Access, Status: "healthy", Region: params.Region}, nil
}

func (f *fakeCustomerRepo) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	f.lastAccountID = accountID
	f.deletedTunnelID = tunnelID
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
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

func TestCreateTunnelReturnsCreatedJSON(t *testing.T) {
	repo := &fakeCustomerRepo{}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}
	body := bytes.NewBufferString(`{"hostname":"new.bloop.to","target":"svc:8080","access":"basic-auth","region":"iad-1"}`)
	req := withCustomerSession(httptest.NewRequest(http.MethodPost, "/tunnels", body), "acct_default")
	w := httptest.NewRecorder()

	h.CreateTunnel(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d body=%s", http.StatusCreated, w.Code, w.Body.String())
	}
	if repo.lastAccountID != "acct_default" {
		t.Fatalf("expected account id acct_default got %q", repo.lastAccountID)
	}
	if repo.lastCreate.Hostname != "new.bloop.to" || repo.lastCreate.Target != "svc:8080" || repo.lastCreate.Access != "basic-auth" {
		t.Fatalf("unexpected create params: %+v", repo.lastCreate)
	}
	if repo.lastCreate.Status != "healthy" {
		t.Fatalf("expected default status healthy got %q", repo.lastCreate.Status)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json got %q", got)
	}
}

func TestUpdateTunnelReturnsJSON(t *testing.T) {
	repo := &fakeCustomerRepo{}
	h := &Handler{Service: service.NewCustomerService(repo, nil)}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "api")
	req := withCustomerSession(httptest.NewRequest(http.MethodPatch, "/tunnels/api", bytes.NewBufferString(`{"target":"svc:9090","access":"public","region":"ord-1"}`)), "acct_default")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateTunnel(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, w.Code, w.Body.String())
	}
	if repo.lastUpdate.Target != "svc:9090" || repo.lastUpdate.Access != "public" || repo.lastUpdate.Region != "ord-1" {
		t.Fatalf("unexpected update params: %+v", repo.lastUpdate)
	}
}

func TestDeleteTunnelReturnsNoContent(t *testing.T) {
	repo := &fakeCustomerRepo{}
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
		t.Fatalf("expected deleted tunnel api got %q", repo.deletedTunnelID)
	}
}

func TestCreateTunnelRejectsBadInput(t *testing.T) {
	h := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{}, nil)}
	for _, tc := range []string{
		`{"hostname":"","target":"svc:8080"}`,
		`{"hostname":"bad host","target":"svc:8080"}`,
		`{"hostname":"ok.bloop.to","target":"bad target space"}`,
		`{"hostname":"ok.bloop.to","target":"svc:8080","access":"unknown"}`,
	} {
		w := httptest.NewRecorder()
		req := withCustomerSession(httptest.NewRequest(http.MethodPost, "/tunnels", bytes.NewBufferString(tc)), "acct_default")
		h.CreateTunnel(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected %d got %d for payload %s", http.StatusBadRequest, w.Code, tc)
		}
	}
}

func TestUpdateTunnelRejectsBadInput(t *testing.T) {
	h := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{}, nil)}
	for _, tc := range []string{
		`{"target":"","access":"public"}`,
		`{"target":"bad target","access":"public"}`,
		`{"target":"svc:8080","access":"unknown"}`,
	} {
		w := httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "api")
		req := withCustomerSession(httptest.NewRequest(http.MethodPatch, "/tunnels/api", bytes.NewBufferString(tc)), "acct_default")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		h.UpdateTunnel(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected %d got %d for payload %s", http.StatusBadRequest, w.Code, tc)
		}
	}
}

func TestCreateTunnelRejectsConflict(t *testing.T) {
	h := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{err: service.ErrTunnelAlreadyExists}, nil)}
	req := withCustomerSession(httptest.NewRequest(http.MethodPost, "/tunnels", bytes.NewBufferString(`{"hostname":"dup.bloop.to","target":"svc:8080"}`)), "acct_default")
	w := httptest.NewRecorder()

	h.CreateTunnel(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected %d got %d", http.StatusConflict, w.Code)
	}
}

func TestCustomerHandlersRequireSession(t *testing.T) {
	h := &Handler{Service: service.NewCustomerService(&fakeCustomerRepo{}, nil)}
	for _, fn := range []func(http.ResponseWriter, *http.Request){h.Workspace, h.Tunnels, h.CreateTunnel, h.DeleteTunnel} {
		w := httptest.NewRecorder()
		body := bytes.NewBufferString(`{"hostname":"api.bloop.to","target":"svc:8080"}`)
		req := httptest.NewRequest(http.MethodGet, "/", body)
		fn(w, req)
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
