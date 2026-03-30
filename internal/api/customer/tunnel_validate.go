package customer

import (
	"encoding/json"
	"net/http"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/models"
)

func (h *Handler) ValidateTunnel(w http.ResponseWriter, r *http.Request) {
	sess, ok := authz.RequireSession(w, r)
	if !ok {
		return
	}

	var req models.TunnelValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.Service.ValidateTunnel(r.Context(), sess.AccountID, req)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	authz.WriteJSON(w, http.StatusOK, resp)
}
