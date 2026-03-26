package authz

import (
	"encoding/json"
	"net/http"

	"bloop-control-plane/internal/session"
)

func RequireSession(w http.ResponseWriter, r *http.Request) (session.Context, bool) {
	sess, ok := session.FromContext(r.Context())
	if !ok || !sess.IsAuthenticated() || sess.AccountID == "" {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return session.Context{}, false
	}
	return sess, true
}

func RequireAnyAuthenticatedSession(w http.ResponseWriter, r *http.Request) (session.Context, bool) {
	sess, ok := session.FromContext(r.Context())
	if !ok || !sess.IsAuthenticated() {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return session.Context{}, false
	}
	return sess, true
}

func RequireAdmin(w http.ResponseWriter, r *http.Request) (session.Context, bool) {
	sess, ok := session.FromContext(r.Context())
	if !ok || !sess.IsAdmin() {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return session.Context{}, false
	}
	return sess, true
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
