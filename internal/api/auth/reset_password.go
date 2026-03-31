package authapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
)

// ResetPasswordRequest represents a reset-password request
type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// ResetPassword handles POST /api/auth/reset-password
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Token == "" || req.NewPassword == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token and new_password are required"})
		return
	}

	ipAddress := session.ClientIP(r)
	userAgent := r.UserAgent()

	err := h.PasswordResetService.ResetPassword(ctx, req.Token, req.NewPassword, ipAddress, userAgent)
	if err != nil {
		var validationErr *service.ValidationError
		if errors.As(err, &validationErr) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": validationErr.Message})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "password reset failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Password has been reset. Please log in.",
	})
}
