package session

import (
	"net/http"

	"bloop-control-plane/internal/api/authz"
)

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookieName := h.CookieName
	if cookieName == "" {
		cookieName = "bloop_session"
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.CookieSecure,
		Domain:   h.CookieDomain,
	})
	authz.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "loggedOut": true})
}
