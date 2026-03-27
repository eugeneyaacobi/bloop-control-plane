package runtime

import "github.com/go-chi/chi/v5"

func Mount(r chi.Router, h *Handler) {
	r.Get("/installations", h.ListInstallations)
	r.Post("/installations", h.CreateInstallation)
	r.Get("/installations/{id}", h.InstallationDetail)
	r.Post("/installations/{id}/rotate-ingest-token", h.RotateIngestToken)
	r.Post("/installations/{id}/revoke", h.RevokeInstallation)
	r.Post("/enroll", h.Enroll)
	r.Post("/ingest/snapshot", h.IngestSnapshot)
}
