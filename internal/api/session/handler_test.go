package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/service"
	sessionctx "bloop-control-plane/internal/session"
)

type fakeSessionRepo struct {
	identity *repository.SessionIdentity
}

func (f *fakeSessionRepo) ResolveIdentity(ctx context.Context, sess sessionctx.Context) (*repository.SessionIdentity, error) {
	return f.identity, nil
}

func TestMeReturnsSessionContext(t *testing.T) {
	h := &Handler{Service: service.NewSessionService(&fakeSessionRepo{identity: &repository.SessionIdentity{
		User:       &models.User{ID: "user_gene", Email: "gene@example.com", DisplayName: "Gene"},
		Account:    &models.Account{ID: "acct_default", DisplayName: "Gene / default-org"},
		Membership: &models.Membership{ID: "mem_1", UserID: "user_gene", AccountID: "acct_default", Role: "owner"},
	}})}
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req = req.WithContext(sessionctx.NewContext(req.Context(), sessionctx.Context{UserID: "user_gene", AccountID: "acct_default", Role: "owner", Prototype: false}))
	w := httptest.NewRecorder()

	h.Me(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["userId"] != "user_gene" || resp["accountId"] != "acct_default" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestMeRequiresAuthentication(t *testing.T) {
	h := &Handler{Service: service.NewSessionService(nil)}
	w := httptest.NewRecorder()
	h.Me(w, httptest.NewRequest(http.MethodGet, "/me", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}
