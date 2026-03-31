package authapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/security"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
)

// Login handles user login
// POST /api/auth/login
// Request body: {email, password}
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, models.AuthError{Error: "email and password are required"})
		return
	}

	// Validate email format
	if err := security.ValidateEmail(req.Email); err != nil {
		writeJSON(w, http.StatusBadRequest, models.AuthError{Error: err.Error()})
		return
	}

	// Extract client info
	ipAddress := session.ClientIP(r)
	userAgent := r.UserAgent()

	// Authenticate
	result, err := h.AuthService.Login(ctx, req.Email, req.Password, ipAddress, userAgent)
	if err != nil {
		// Check error type for appropriate status code
		var authErr *service.AuthError
		var rateLimitErr *service.RateLimitError
		var lockoutErr *service.LockoutError
		if errors.As(err, &authErr) {
			writeJSON(w, http.StatusUnauthorized, models.AuthError{Error: authErr.Message})
			return
		}
		if errors.As(err, &rateLimitErr) {
			writeJSON(w, http.StatusTooManyRequests, models.AuthError{Error: rateLimitErr.Message})
			return
		}
		if errors.As(err, &lockoutErr) {
			writeJSON(w, http.StatusLocked, models.AuthError{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, models.AuthError{Error: "login failed"})
		return
	}

	// Issue session token
	token, err := h.issueToken(result.Session.UserID, result.Session.AccountID, result.Session.Role)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.AuthError{Error: "failed to create session"})
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     h.SessionName,
		Value:    token,
		Path:     "/",
		MaxAge:   0, // Session cookie
		Secure:   h.SecureCookie,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Domain:   h.CookieDomain,
	})

	// Return user context
	displayName := req.Email
	if result.User != nil {
		displayName = result.User.DisplayName
	}
	email := req.Email
	if result.User != nil {
		email = result.User.Email
	}
	username := ""
	if result.User != nil && result.User.Username != nil {
		username = *result.User.Username
	}
	var usernamePtr *string
	if username != "" {
		usernamePtr = &username
	}
	writeJSON(w, http.StatusOK, models.LoginResponse{
		User: models.UserContext{
			ID:          result.Session.UserID,
			Email:       email,
			Username:    usernamePtr,
			DisplayName: displayName,
			AccountID:   result.Session.AccountID,
			Role:        result.Session.Role,
		},
		RequiresWebAuthn: result.RequiresWebAuthn,
	})
}
