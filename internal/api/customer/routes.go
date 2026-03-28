package customer

import (
	"github.com/go-chi/chi/v5"
)

func Mount(r chi.Router, h *Handler) {
	r.Get("/workspace", h.Workspace)
	r.Get("/tunnels", h.Tunnels)
	r.Post("/tunnels", h.CreateTunnel)
	r.Get("/tunnels/{id}", h.TunnelDetail)
	r.Patch("/tunnels/{id}", h.UpdateTunnel)
	r.Delete("/tunnels/{id}", h.DeleteTunnel)
}
