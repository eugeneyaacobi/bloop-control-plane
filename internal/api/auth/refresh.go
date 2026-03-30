package authapi

import (
	"errors"
	"net/http"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
)

// Refresh handles session token refresh
// POST /api/auth/refresh
// Uses existing session cookie
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current session from context
	currentSession, ok := session.FromContext(ctx)
	if !ok || !currentSession.IsAuthenticated() {
		writeJSON(w, http.StatusUnauthorized, models.AuthError{Error: "no valid session"})
		return
	}

	// Refresh session
	newSession, err := h.AuthService.RefreshSession(ctx, currentSession)
	if err != nil {
		var authErr *service.AuthError
		if errors.As(err, &authErr) {
			writeJSON(w, http.StatusUnauthorized, models.AuthError{Error: authErr.Message})
			return
		}
		writeJSON(w, http.StatusInternalServerError, models.AuthError{Error: "refresh failed"})
		return
	}

	// Issue new session token
	token, err := h.issueToken(newSession.UserID, newSession.AccountID, newSession.Role)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.AuthError{Error: "failed to create session"})
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     h.SessionName,
		Value:    token,
		Path:     "/",
		MaxAge:   0,
		Secure:   h.SecureCookie,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Domain:   h.CookieDomain,
	})

	// Return new session context
	// The refresh response won't have email/displayName since we only have the session context
	writeJSON(w, http.StatusOK, models.RefreshResponse{
		User: models.UserContext{
			ID:        newSession.UserID,
			AccountID: newSession.AccountID,
			Role:      newSession.Role,
		},
	})
}
