package customer

import (
	"errors"
	"net/http"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/security"
	"bloop-control-plane/internal/service"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) DeleteTunnel(w http.ResponseWriter, r *http.Request) {
	sess, ok := authz.RequireSession(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	if err := security.ValidateIdentifier(id); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if err := h.Service.DeleteTunnel(r.Context(), sess.UserID, sess.AccountID, id); err != nil {
		if errors.Is(err, service.ErrTunnelNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
