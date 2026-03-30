package webauthn

import (
	"encoding/json"
	"net/http"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/session"
)

// FinishRegistrationRequest represents the request to complete WebAuthn registration
type FinishRegistrationRequest struct {
	ChallengeID     string `json:"challenge_id"`
	Credential      any    `json:"credential"`
	CredentialName  string `json:"credential_name"`
}

// FinishRegistration handles the completion of WebAuthn registration
// POST /api/webauthn/register-finish
func (h *Handler) FinishRegistration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current session from context
	sess, ok := session.FromContext(ctx)
	if !ok || !sess.IsAuthenticated() || sess.UserID == "" {
		writeJSON(w, http.StatusUnauthorized, models.AuthError{Error: "authentication required"})
		return
	}

	// Parse request
	var req FinishRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ChallengeID == "" {
		writeJSON(w, http.StatusBadRequest, models.AuthError{Error: "challenge_id is required"})
		return
	}

	// Default credential name
	credentialName := req.CredentialName
	if credentialName == "" {
		credentialName = "Security Key"
	}

	// Extract client info
	ipAddress := session.ClientIP(r)
	userAgent := r.UserAgent()

	// Finish registration (serialize credential to JSON for service)
	var credentialJSON []byte
	var err error
	if req.Credential != nil {
		credentialJSON, err = json.Marshal(req.Credential)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, models.AuthError{Error: "invalid credential data"})
			return
		}
	}

	result, err := h.WebAuthnService.FinishRegistration(ctx, sess.UserID, req.ChallengeID, string(credentialJSON), credentialName, ipAddress, userAgent)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.AuthError{Error: err.Error()})
		return
	}

	// Return credential info
	writeJSON(w, http.StatusOK, result.Credential)
}
