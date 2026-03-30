package tokens

import (
	"net/http"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/session"
)

// List handles listing user's tokens
// GET /api/tokens
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current session from context
	sess, ok := session.FromContext(ctx)
	if !ok || !sess.IsAuthenticated() || sess.UserID == "" {
		writeJSON(w, http.StatusUnauthorized, models.AuthError{Error: "authentication required"})
		return
	}

	// List tokens for user
	tokens, err := h.TokenService.ListTokens(ctx, sess.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.AuthError{Error: "failed to list tokens"})
		return
	}

	// Return token list (without plaintext values!)
	writeJSON(w, http.StatusOK, models.TokenListResponse{
		Tokens: tokens,
	})
}
