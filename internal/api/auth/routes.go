package authapi

import (
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
	"time"

	"github.com/go-chi/chi/v5"
)

// Mount sets up the auth routes
func Mount(r chi.Router, handler *Handler) {
	r.Post("/register", handler.Register)
	r.Post("/login", handler.Login)
	r.Post("/refresh", handler.Refresh)
	r.Post("/forgot-password", handler.ForgotPassword)
	r.Post("/reset-password", handler.ResetPassword)
}

// Handler dependencies
type Handler struct {
	AuthService          *service.AuthService
	PasswordResetService *service.PasswordResetService
	TokenManager         *session.TokenManager
	SessionName          string
	SecureCookie         bool
	CookieDomain         string
}

// NewHandler creates a new auth handler
func NewHandler(authService *service.AuthService, passwordResetService *service.PasswordResetService, tokenMgr *session.TokenManager, sessionName string, secureCookie bool, cookieDomain string) *Handler {
	return &Handler{
		AuthService:          authService,
		PasswordResetService: passwordResetService,
		TokenManager:         tokenMgr,
		SessionName:          sessionName,
		SecureCookie:         secureCookie,
		CookieDomain:         cookieDomain,
	}
}

// issueToken creates a signed session token
func (h *Handler) issueToken(userID, accountID, role string) (string, error) {
	return h.TokenManager.Sign(session.TokenClaims{
		Kind:      "session",
		UserID:    userID,
		AccountID: accountID,
		Role:      role,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	})
}
