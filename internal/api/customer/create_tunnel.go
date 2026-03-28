package customer

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/security"
	"bloop-control-plane/internal/service"
)

var allowedAccessModes = map[string]bool{
	"public":          true,
	"basic-auth":      true,
	"token-protected": true,
}

type createTunnelRequest struct {
	Hostname string `json:"hostname"`
	Target   string `json:"target"`
	Access   string `json:"access"`
	Region   string `json:"region,omitempty"`
}

func (h *Handler) CreateTunnel(w http.ResponseWriter, r *http.Request) {
	sess, ok := authz.RequireSession(w, r)
	if !ok {
		return
	}

	var req createTunnelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	req.Hostname = strings.TrimSpace(req.Hostname)
	req.Target = strings.TrimSpace(req.Target)
	req.Access = strings.TrimSpace(req.Access)
	req.Region = strings.TrimSpace(req.Region)

	if req.Hostname == "" || req.Target == "" {
		http.Error(w, "hostname and target are required", http.StatusBadRequest)
		return
	}
	if err := security.ValidateIdentifier(req.Hostname); err != nil {
		http.Error(w, "hostname contains invalid characters", http.StatusBadRequest)
		return
	}
	if err := security.ValidateIdentifier(req.Target); err != nil {
		http.Error(w, "target contains invalid characters", http.StatusBadRequest)
		return
	}
	if req.Region != "" {
		if err := security.ValidateIdentifier(req.Region); err != nil {
			http.Error(w, "region contains invalid characters", http.StatusBadRequest)
			return
		}
	}
	if req.Access != "" && !allowedAccessModes[req.Access] {
		http.Error(w, "unsupported access mode", http.StatusBadRequest)
		return
	}

	created, err := h.Service.CreateTunnel(r.Context(), sess.AccountID, service.CreateTunnelInput{
		Hostname: req.Hostname,
		Target:   req.Target,
		Access:   req.Access,
		Region:   req.Region,
	})
	if err != nil {
		if errors.Is(err, service.ErrTunnelAlreadyExists) {
			http.Error(w, "tunnel already exists", http.StatusConflict)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	authz.WriteJSON(w, http.StatusCreated, created)
}
