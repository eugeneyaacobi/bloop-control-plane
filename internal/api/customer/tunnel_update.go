package customer

import (
	"encoding/json"
	"net/http"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/security"
	service_errors "bloop-control-plane/internal/service"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) UpdateTunnel(w http.ResponseWriter, r *http.Request) {
	sess, ok := authz.RequireSession(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	if err := security.ValidateIdentifier(id); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var req models.TunnelUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	tunnel, err := h.Service.UpdateTunnel(r.Context(), sess.AccountID, id, req)
	if err != nil {
		if service_errors.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		if service_errors.IsConflict(err) {
			authz.WriteJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if service_errors.IsValidation(err) {
			authz.WriteJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	authz.WriteJSON(w, http.StatusOK, tunnel)
}
