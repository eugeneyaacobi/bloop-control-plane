package customer

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/security"
	"bloop-control-plane/internal/service"
	"github.com/go-chi/chi/v5"
)

type updateTunnelRequest struct {
	Target string `json:"target"`
	Access string `json:"access"`
	Region string `json:"region,omitempty"`
}

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

	var req updateTunnelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	req.Target = strings.TrimSpace(req.Target)
	req.Access = strings.TrimSpace(req.Access)
	req.Region = strings.TrimSpace(req.Region)

	if req.Target == "" || req.Access == "" {
		http.Error(w, "target and access are required", http.StatusBadRequest)
		return
	}
	if err := security.ValidateIdentifier(req.Target); err != nil {
		http.Error(w, "target contains invalid characters", http.StatusBadRequest)
		return
	}
	if !allowedAccessModes[req.Access] {
		http.Error(w, "unsupported access mode", http.StatusBadRequest)
		return
	}
	if req.Region != "" {
		if err := security.ValidateIdentifier(req.Region); err != nil {
			http.Error(w, "region contains invalid characters", http.StatusBadRequest)
			return
		}
	}

	updated, err := h.Service.UpdateTunnel(r.Context(), sess.UserID, sess.AccountID, id, service.UpdateTunnelInput{
		Target: req.Target,
		Access: req.Access,
		Region: req.Region,
	})
	if err != nil {
		if errors.Is(err, service.ErrTunnelNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	authz.WriteJSON(w, http.StatusOK, updated)
}
