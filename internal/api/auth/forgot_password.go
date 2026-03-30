package authapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
)

// ForgotPasswordRequest represents a forgot-password request
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ForgotPassword handles POST /api/auth/forgot-password
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}

	ipAddress := session.ClientIP(r)
	userAgent := r.UserAgent()

	err := h.PasswordResetService.RequestReset(ctx, req.Email, ipAddress, userAgent)
	if err != nil {
		var rateLimitErr *service.RateLimitError
		if errors.As(err, &rateLimitErr) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": rateLimitErr.Message})
			return
		}
		// Internal error — still return 200 to avoid leaking info
	}

	// Always return the same response regardless of whether the email exists
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "If an account with that email exists, a reset link has been sent.",
	})
}
