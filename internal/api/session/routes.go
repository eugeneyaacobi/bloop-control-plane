package session

import "github.com/go-chi/chi/v5"

func Mount(r chi.Router, h *Handler) {
	r.Get("/me", h.Me)
	r.Post("/logout", h.Logout)
}
