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
		// Check error type for appropriate status code
		var validationErr *service.ValidationError
		var conflictErr *service.ConflictError
		if errors.As(err, &validationErr) {
			writeJSON(w, http.StatusUnprocessableEntity, models.AuthError{Error: validationErr.Error()})
			return
		}
		if errors.As(err, &conflictErr) {
			writeJSON(w, http.StatusConflict, models.AuthError{Error: "registration failed"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, models.AuthError{Error: "registration failed"})
		return
	}

	// Issue session token
	token, err := h.issueToken(result.User.ID, result.Session.AccountID, "customer")
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
	writeJSON(w, http.StatusCreated, models.LoginResponse{
		User: models.UserContext{
			ID:          result.User.ID,
			Email:       result.User.Email,
			Username:    result.User.Username,
			DisplayName: result.User.DisplayName,
			AccountID:   "acct_default", // TODO: Get from user's actual account
			Role:        "customer",
		},
		RequiresWebAuthn: false,
	})
}

// writeJSON writes JSON response
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
