package customer

import (
	"github.com/go-chi/chi/v5"
)

func Mount(r chi.Router, h *Handler) {
	r.Get("/workspace", h.Workspace)
	r.Get("/tunnels", h.Tunnels)
	r.Get("/tunnels/{id}", h.TunnelDetail)

	// CRUD routes
	r.Post("/tunnels", h.CreateTunnel)
	r.Put("/tunnels/{id}", h.UpdateTunnel)
	r.Delete("/tunnels/{id}", h.DeleteTunnel)

	// Validation, status, config, and enrollment routes
	r.Post("/tunnels/validate", h.ValidateTunnel)
	r.Get("/tunnels/{id}/status", h.TunnelStatus)
	r.Get("/config/schema", h.ConfigSchema)
	r.Post("/enrollment/verify", h.VerifyEnrollment)
}
