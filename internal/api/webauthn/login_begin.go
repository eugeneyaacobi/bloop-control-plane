package webauthn

import (
	"encoding/json"
	"net/http"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/session"
)

// BeginLoginRequest represents the request to start WebAuthn login
type BeginLoginRequest struct {
	Email string `json:"email"`
}

// BeginLogin handles the start of WebAuthn login ceremony
// POST /api/webauthn/login-begin
func (h *Handler) BeginLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	var req BeginLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, models.AuthError{Error: "email is required"})
		return
	}

	// Extract client info
	ipAddress := session.ClientIP(r)
	userAgent := r.UserAgent()

	// Begin login
	result, err := h.WebAuthnService.BeginLogin(ctx, req.Email, ipAddress, userAgent)
	if err != nil {
		// Don't reveal whether user exists or WebAuthn status
		writeJSON(w, http.StatusBadRequest, models.AuthError{Error: "login begin failed"})
		return
	}

	// Return request options with challenge ID
	writeJSON(w, http.StatusOK, struct {
		ChallengeID                  string `json:"challenge_id"`
		Email                        string `json:"email"`
		PublicKeyCredentialRequestOptions any    `json:"publicKeyCredentialRequestOptions"`
	}{
		ChallengeID:                  result.ChallengeID,
		Email:                        req.Email,
		PublicKeyCredentialRequestOptions: result.RequestOptions,
	})
}
