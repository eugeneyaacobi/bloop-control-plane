package authapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
)

// Register handles user registration
// POST /api/auth/register
// Request body: {email, username, password}
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	var req models.RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Email == "" || req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, models.AuthError{Error: "email, username, and password are required"})
		return
	}

	// Extract client info
	ipAddress := session.ClientIP(r)
	userAgent := r.UserAgent()

	// Register user
	result, err := h.AuthService.Register(ctx, req.Email, req.Username, req.Password, ipAddress, userAgent)
	if err != nil {
		var validationErr *service.ValidationError
		var conflictErr *service.ConflictError
		if errors.As(err, &validationErr) {
			writeJSON(w, http.StatusUnprocessableEntity, models.AuthError{Error: validationErr.Error()})
			return
		}
		if errors.As(err, &conflictErr) {
			writeJSON(w, http.StatusConflict, models.AuthError{Error: conflictErr.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, models.AuthError{Error: "registration failed"})
		return
	}

	// If session is nil, user needs to verify email first
	if result.Session == nil {
		writeJSON(w, http.StatusCreated, map[string]any{
			"message":   "registration successful — please check your email to verify your account",
			"user_id":   result.User.ID,
			"email":     result.User.Email,
			"verified":  false,
		})
		return
	}

	// Issue session token (only if verification is not required)
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
		MaxAge:   0,
		Secure:   h.SecureCookie,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Domain:   h.CookieDomain,
	})

	writeJSON(w, http.StatusCreated, models.LoginResponse{
		User: models.UserContext{
			ID:          result.User.ID,
			Email:       result.User.Email,
			Username:    result.User.Username,
			DisplayName: result.User.DisplayName,
			AccountID:   result.Session.AccountID,
			Role:        result.Session.Role,
		},
		RequiresWebAuthn: false,
	})
}
