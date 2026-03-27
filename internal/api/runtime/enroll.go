package runtime

import (
	"encoding/json"
	"net/http"

	"bloop-control-plane/internal/api/authz"
)

type enrollRequest struct {
	Token         string `json:"token"`
	ClientName    string `json:"clientName"`
	ClientVersion string `json:"clientVersion"`
}

func (h *Handler) Enroll(w http.ResponseWriter, r *http.Request) {
	if h.RuntimeInstallations == nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	var req enrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	result, err := h.RuntimeInstallations.Enroll(r.Context(), req.Token, req.ClientName, req.ClientVersion)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	authz.WriteJSON(w, http.StatusOK, map[string]any{
		"installation": result.Installation,
		"ingest": map[string]any{"token": result.IngestToken},
	})
}
