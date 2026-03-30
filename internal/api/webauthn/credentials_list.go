package webauthn

import (
	"net/http"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/session"
)

// ListCredentials handles listing a user's WebAuthn credentials
// GET /api/webauthn/credentials
func (h *Handler) ListCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current session from context
	sess, ok := session.FromContext(ctx)
	if !ok || !sess.IsAuthenticated() || sess.UserID == "" {
		writeJSON(w, http.StatusUnauthorized, models.AuthError{Error: "authentication required"})
		return
	}

	// List credentials
	credentials, err := h.WebAuthnService.ListCredentials(ctx, sess.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.AuthError{Error: "failed to list credentials"})
		return
	}

	// Return credential list
	writeJSON(w, http.StatusOK, models.WebAuthnCredentialListResponse{
		Credentials: credentials,
	})
}
