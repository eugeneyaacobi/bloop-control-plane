package api

import (
	"encoding/json"
	"net/http"

	"bloop-control-plane/internal/session"
)

func requireSession(w http.ResponseWriter, r *http.Request) (session.Context, bool) {
	sess, ok := session.FromContext(r.Context())
	if !ok || !sess.IsAuthenticated() || sess.AccountID == "" {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return session.Context{}, false
	}
	return sess, true
}

func requireAdmin(w http.ResponseWriter, r *http.Request) (session.Context, bool) {
	sess, ok := session.FromContext(r.Context())
	if !ok || !sess.IsAdmin() {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return session.Context{}, false
	}
	return sess, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
