package customer

import (
	"encoding/json"
	"net/http"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/security"
	service_errors "bloop-control-plane/internal/service"
)

func (h *Handler) CreateTunnel(w http.ResponseWriter, r *http.Request) {
	sess, ok := authz.RequireSession(w, r)
	if !ok {
		return
	}

	var req models.TunnelCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ID == "" {
		http.Error(w, "id is required", http.StatusUnprocessableEntity)
		return
	}
	if err := security.ValidateIdentifier(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Hostname == "" {
		http.Error(w, "hostname is required", http.StatusUnprocessableEntity)
		return
	}
	if req.Target == "" {
		http.Error(w, "target is required", http.StatusUnprocessableEntity)
		return
	}
	if req.Access == "" {
		http.Error(w, "access is required", http.StatusUnprocessableEntity)
		return
	}

	tunnel, err := h.Service.CreateTunnel(r.Context(), sess.AccountID, req)
	if err != nil {
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

	authz.WriteJSON(w, http.StatusCreated, tunnel)
}
