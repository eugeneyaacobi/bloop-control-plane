package admin

import "github.com/go-chi/chi/v5"

func Mount(r chi.Router, h *Handler) {
	r.Get("/overview", h.Overview)
	r.Get("/users", h.Users)
	r.Get("/tunnels", h.Tunnels)
	r.Get("/review-queue", h.ReviewQueue)
}
