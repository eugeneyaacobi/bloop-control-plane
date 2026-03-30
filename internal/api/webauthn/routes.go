package webauthn

import (
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"

	"github.com/go-chi/chi/v5"
)

// Mount sets up the WebAuthn routes
func Mount(r chi.Router, handler *Handler) {
	// Registration ceremony
	r.Post("/register-begin", handler.BeginRegistration)
	r.Post("/register-finish", handler.FinishRegistration)

	// Login ceremony
	r.Post("/login-begin", handler.BeginLogin)
	r.Post("/login-finish", handler.FinishLogin)

	// Credential management
	r.Get("/credentials", handler.ListCredentials)
	r.Delete("/credentials/{id}", handler.DeleteCredential)
}

// Handler dependencies
type Handler struct {
	WebAuthnService *service.WebAuthnService
	TokenManager    *session.TokenManager
	SessionName     string
	SecureCookie    bool
	CookieDomain    string
}

// NewHandler creates a new WebAuthn handler
func NewHandler(
	webauthnService *service.WebAuthnService,
	tokenMgr *session.TokenManager,
	sessionName string,
	secureCookie bool,
	cookieDomain string,
) *Handler {
	return &Handler{
		WebAuthnService: webauthnService,
		TokenManager:    tokenMgr,
		SessionName:     sessionName,
		SecureCookie:    secureCookie,
		CookieDomain:    cookieDomain,
	}
}
