package customer

import (
	"net/http"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/security"
	service_errors "bloop-control-plane/internal/service"
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

	err := h.Service.DeleteTunnel(r.Context(), sess.AccountID, id)
	if err != nil {
		if service_errors.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
