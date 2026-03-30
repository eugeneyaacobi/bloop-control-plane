package webauthn

import (
	"net/http"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/session"

	"github.com/go-chi/chi/v5"
)

// DeleteCredential handles removing a WebAuthn credential
// DELETE /api/webauthn/credentials/{id}
func (h *Handler) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current session from context
	sess, ok := session.FromContext(ctx)
	if !ok || !sess.IsAuthenticated() || sess.UserID == "" {
		writeJSON(w, http.StatusUnauthorized, models.AuthError{Error: "authentication required"})
		return
	}

	// Get credential ID from URL
	credentialID := chi.URLParam(r, "id")
	if credentialID == "" {
		http.Error(w, "credential id required", http.StatusBadRequest)
		return
	}

	// Extract client info
	ipAddress := session.ClientIP(r)
	userAgent := r.UserAgent()

	// Delete credential
	err := h.WebAuthnService.DeleteCredential(ctx, sess.UserID, credentialID, ipAddress, userAgent)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.AuthError{Error: "credential not found"})
		return
	}

	// Return success
	w.WriteHeader(http.StatusNoContent)
}
