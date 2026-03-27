package runtime

import (
	"encoding/json"
	"net/http"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
)

type installationCreateRequest struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

type installationCreateResponse struct {
	Installation service.CreateRuntimeInstallationResult `json:"-"`
}

func (h *Handler) CreateInstallation(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	if sess.AccountID == "" {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if h.RuntimeInstallations == nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	var req installationCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	result, err := h.RuntimeInstallations.CreateInstallation(r.Context(), sess.AccountID, req.Name, req.Environment)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	authz.WriteJSON(w, http.StatusCreated, map[string]any{
		"installation": result.Installation,
		"enrollment": map[string]any{"token": result.EnrollmentToken, "expiresAt": result.EnrollmentExpiresAt},
	})
}

func (h *Handler) ListInstallations(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	if sess.AccountID == "" {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if h.RuntimeInstallations == nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	installations, err := h.RuntimeInstallations.ListInstallations(r.Context(), sess.AccountID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	authz.WriteJSON(w, http.StatusOK, map[string]any{"installations": installations})
}
