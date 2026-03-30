package webauthn

import (
	"encoding/json"
	"net/http"
	"time"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/session"
)

// BeginRegistration handles the start of WebAuthn registration
// POST /api/webauthn/register-begin
func (h *Handler) BeginRegistration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current session from context
	sess, ok := session.FromContext(ctx)
	if !ok || !sess.IsAuthenticated() || sess.UserID == "" {
		writeJSON(w, http.StatusUnauthorized, models.AuthError{Error: "authentication required"})
		return
	}

	// Extract client info
	ipAddress := session.ClientIP(r)
	userAgent := r.UserAgent()

	// Begin registration
	result, err := h.WebAuthnService.BeginRegistration(ctx, sess.UserID, ipAddress, userAgent)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.AuthError{Error: "failed to begin registration"})
		return
	}

	// Return creation options with challenge ID
	writeJSON(w, http.StatusOK, struct {
		ChallengeID                     string `json:"challenge_id"`
		PublicKeyCredentialCreationOptions any    `json:"publicKeyCredentialCreationOptions"`
	}{
		ChallengeID:                     result.ChallengeID,
		PublicKeyCredentialCreationOptions: result.CreationOptions,
	})
}

// writeJSON writes JSON response
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// issueToken creates a signed session token (for login completion)
func (h *Handler) issueToken(userID, accountID, role string) (string, error) {
	return h.TokenManager.Sign(session.TokenClaims{
		Kind:      "session",
		UserID:    userID,
		AccountID: accountID,
		Role:      role,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	})
}
