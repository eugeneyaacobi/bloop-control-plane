package customer

import (
	"net/http"

	"bloop-control-plane/internal/api/authz"
)

func (h *Handler) Tunnels(w http.ResponseWriter, r *http.Request) {
	sess, ok := authz.RequireSession(w, r)
	if !ok {
		return
	}

	resp, err := h.Service.ListTunnels(r.Context(), sess.AccountID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	authz.WriteJSON(w, http.StatusOK, resp)
}
