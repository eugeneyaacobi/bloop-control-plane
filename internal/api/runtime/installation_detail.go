package runtime

import (
	"net/http"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/session"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) InstallationDetail(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	if sess.AccountID == "" || h.RuntimeInstallations == nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	installationID := chi.URLParam(r, "id")
	result, err := h.RuntimeInstallations.GetInstallation(r.Context(), sess.AccountID, installationID)
	if err != nil || result == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	authz.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) RotateIngestToken(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	if sess.AccountID == "" || h.RuntimeInstallations == nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	installationID := chi.URLParam(r, "id")
	token, err := h.RuntimeInstallations.RotateIngestToken(r.Context(), sess.AccountID, installationID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	authz.WriteJSON(w, http.StatusOK, map[string]any{"installationId": installationID, "token": token})
}

func (h *Handler) RevokeInstallation(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	if sess.AccountID == "" || h.RuntimeInstallations == nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	installationID := chi.URLParam(r, "id")
	if err := h.RuntimeInstallations.RevokeInstallation(r.Context(), sess.AccountID, installationID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	authz.WriteJSON(w, http.StatusOK, map[string]any{"revoked": true, "installationId": installationID})
}
