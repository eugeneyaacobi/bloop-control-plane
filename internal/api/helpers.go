package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"bloop-control-plane/internal/session"
)

// requestLogger logs each request with method, path, status, and duration.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", r.RemoteAddr,
		)
	})
}

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
