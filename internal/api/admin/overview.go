package admin

import (
	"net/http"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/service"
)

type Handler struct {
	Service *service.AdminService
}

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	if _, ok := authz.RequireAdmin(w, r); !ok {
		return
	}

	resp, err := h.Service.GetOverview(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	authz.WriteJSON(w, http.StatusOK, resp)
}
