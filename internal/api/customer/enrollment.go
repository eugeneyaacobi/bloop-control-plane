package customer

import (
	"encoding/json"
	"net/http"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/models"
)

func (h *Handler) VerifyEnrollment(w http.ResponseWriter, r *http.Request) {
	sess, ok := authz.RequireSession(w, r)
	if !ok {
		return
	}

	_ = sess // session is validated but enrollment verification is token-based

	var req models.EnrollmentVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.Service.VerifyEnrollment(r.Context(), req.Token)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if !resp.Valid {
		authz.WriteJSON(w, http.StatusUnauthorized, resp)
		return
	}

	authz.WriteJSON(w, http.StatusOK, resp)
}
