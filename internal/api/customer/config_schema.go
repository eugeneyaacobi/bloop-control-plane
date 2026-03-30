package customer

import (
	"net/http"

	"bloop-control-plane/internal/api/authz"
)

func (h *Handler) ConfigSchema(w http.ResponseWriter, r *http.Request) {
	sess, ok := authz.RequireSession(w, r)
	if !ok {
		return
	}

	_ = sess // session is validated but not used for config schema

	resp, err := h.Service.GetConfigSchema(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	authz.WriteJSON(w, http.StatusOK, resp)
}
