package tokens

import (
	"bloop-control-plane/internal/service"

	"github.com/go-chi/chi/v5"
)

// Mount sets up the token routes
func Mount(r chi.Router, handler *Handler) {
	r.Post("/", handler.Create)
	r.Get("/", handler.List)
	r.Delete("/{id}", handler.Revoke)
	r.Post("/{id}/refresh", handler.Refresh)
}

// Handler dependencies
type Handler struct {
	TokenService *service.TokenService
}

// NewHandler creates a new token handler
func NewHandler(tokenService *service.TokenService) *Handler {
	return &Handler{
		TokenService: tokenService,
	}
}
