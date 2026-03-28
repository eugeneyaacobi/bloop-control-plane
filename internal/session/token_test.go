package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTokenManagerSignAndParse(t *testing.T) {
	mgr, err := NewTokenManager("test-secret")
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	expiresAt := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	token, err := mgr.Sign(TokenClaims{UserID: "user_gene", AccountID: "acct_default", Role: "owner", ExpiresAt: expiresAt.Unix()})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	sess, err := mgr.Parse(token, expiresAt.Add(-time.Minute))
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if sess.UserID != "user_gene" || sess.AccountID != "acct_default" || sess.Role != "owner" || sess.Prototype {
		t.Fatalf("unexpected session: %+v", sess)
	}
}

func TestResolverUsesBearerTokenBeforePrototypeFallback(t *testing.T) {
	mgr, err := NewTokenManager("test-secret")
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	token, err := mgr.Sign(TokenClaims{UserID: "user_real", AccountID: "acct_real", Role: "owner", ExpiresAt: now.Add(time.Hour).Unix()})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	resolver := Resolver{
		PrototypeAccountID: "acct_default",
		PrototypeUserID:    "user_gene",
		PrototypeRole:      "customer",
		AllowPrototype:     true,
		Tokens:             mgr,
		Now:                func() time.Time { return now },
	}

	var got Context
	handler := resolver.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/session/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got.UserID != "user_real" || got.AccountID != "acct_real" || got.Role != "owner" || got.Prototype {
		t.Fatalf("unexpected resolved session: %+v", got)
	}
}

func TestResolverFallsBackToPrototypeOnlyWhenEnabled(t *testing.T) {
	resolver := Resolver{
		PrototypeAccountID: "acct_default",
		PrototypeUserID:    "user_gene",
		PrototypeRole:      "customer",
		AllowPrototype:     true,
	}

	var got Context
	handler := resolver.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/customer/workspace", nil))

	if !got.Prototype || got.UserID != "user_gene" || got.AccountID != "acct_default" {
		t.Fatalf("unexpected prototype session: %+v", got)
	}
}
