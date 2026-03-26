package onboarding

import "github.com/go-chi/chi/v5"

func Mount(r chi.Router, h *Handler) {
	r.Get("/steps", h.Steps)
	r.Post("/signup/request", h.SignupRequest)
	r.Post("/signup/verify", h.SignupVerify)
}
