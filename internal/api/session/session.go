package session

import (
	"net/http"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/service"
)

type Handler struct {
	Service       *service.SessionService
	CookieName    string
	CookieSecure  bool
	CookieDomain  string
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	sess, ok := authz.RequireAnyAuthenticatedSession(w, r)
	if !ok {
		return
	}
	resp, err := h.Service.GetMe(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if resp.Role == "" {
		resp.Role = sess.Role
	}
	authz.WriteJSON(w, http.StatusOK, resp)
}
