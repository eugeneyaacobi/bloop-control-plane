package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
)

func withAdminSession(req *http.Request, role string) *http.Request {
	return req.WithContext(session.NewContext(req.Context(), session.Context{UserID: "admin_test", Role: role}))
}

type fakeAdminRepo struct {
	overviewCounts [3]int
	users          []models.User
	tunnels        []models.Tunnel
	flags          []models.ReviewFlag
	err            error
}

func (f *fakeAdminRepo) OverviewStats(ctx context.Context) (int, int, int, error) {
	if f.err != nil {
		return 0, 0, 0, f.err
	}
	return f.overviewCounts[0], f.overviewCounts[1], f.overviewCounts[2], nil
}

func (f *fakeAdminRepo) ListUsers(ctx context.Context) ([]models.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.users, nil
}

func (f *fakeAdminRepo) ListTunnels(ctx context.Context) ([]models.Tunnel, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tunnels, nil
}

func (f *fakeAdminRepo) ListReviewFlags(ctx context.Context) ([]models.ReviewFlag, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.flags, nil
}

func TestOverviewReturnsJSON(t *testing.T) {
	h := &Handler{Service: service.NewAdminService(&fakeAdminRepo{overviewCounts: [3]int{1, 2, 3}, tunnels: []models.Tunnel{{ID: "api", Hostname: "api.bloop.to"}}}, nil)}
	req := withAdminSession(httptest.NewRequest(http.MethodGet, "/overview", nil), "admin")
	w := httptest.NewRecorder()

	h.Overview(w, req)

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
	stats := resp["overviewStats"].([]any)
	if len(stats) != 3 {
		t.Fatalf("unexpected stats response: %+v", resp)
	}
}

func TestAdminHandlersReturn500OnServiceError(t *testing.T) {
	h := &Handler{Service: service.NewAdminService(&fakeAdminRepo{err: errors.New("boom")}, nil)}
	cases := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{name: "overview", fn: h.Overview},
		{name: "users", fn: h.Users},
		{name: "tunnels", fn: h.Tunnels},
		{name: "review", fn: h.ReviewQueue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := withAdminSession(httptest.NewRequest(http.MethodGet, "/", nil), "admin")
			w := httptest.NewRecorder()
			tc.fn(w, req)
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("expected %d got %d", http.StatusInternalServerError, w.Code)
			}
		})
	}
}

func TestAdminHandlersRejectNonAdmin(t *testing.T) {
	h := &Handler{Service: service.NewAdminService(&fakeAdminRepo{}, nil)}
	for _, fn := range []func(http.ResponseWriter, *http.Request){h.Overview, h.Users, h.Tunnels, h.ReviewQueue} {
		w := httptest.NewRecorder()
		fn(w, withAdminSession(httptest.NewRequest(http.MethodGet, "/", nil), "customer"))
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected %d got %d", http.StatusForbidden, w.Code)
		}
	}
}

func TestAdminListHandlersReturnJSON(t *testing.T) {
	h := &Handler{Service: service.NewAdminService(&fakeAdminRepo{users: []models.User{{ID: "u1", Email: "gene@example.com"}}, tunnels: []models.Tunnel{{ID: "api", Hostname: "api.bloop.to"}}, flags: []models.ReviewFlag{{ID: "rf1", Item: "api.bloop.to", Reason: "public", Severity: "elevated"}}}, nil)}
	cases := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{name: "users", fn: h.Users},
		{name: "tunnels", fn: h.Tunnels},
		{name: "review", fn: h.ReviewQueue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := withAdminSession(httptest.NewRequest(http.MethodGet, "/", nil), "admin")
			w := httptest.NewRecorder()
			tc.fn(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
			}
			if got := w.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("expected application/json got %q", got)
			}
		})
	}
}
