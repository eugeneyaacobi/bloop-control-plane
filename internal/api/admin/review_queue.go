package admin

import (
	"net/http"

	"bloop-control-plane/internal/api/authz"
)

func (h *Handler) ReviewQueue(w http.ResponseWriter, r *http.Request) {
	if _, ok := authz.RequireAdmin(w, r); !ok {
		return
	}

	resp, err := h.Service.ListReviewQueue(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	authz.WriteJSON(w, http.StatusOK, resp)
}
