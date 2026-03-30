package tokens

import (
	"errors"
	"net/http"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"

	"github.com/go-chi/chi/v5"
)

// Refresh handles token rotation
// POST /api/tokens/{id}/refresh
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current session from context
	sess, ok := session.FromContext(ctx)
	if !ok || !sess.IsAuthenticated() || sess.UserID == "" {
		writeJSON(w, http.StatusUnauthorized, models.AuthError{Error: "authentication required"})
		return
	}

	// Get token ID from URL
	tokenID := chi.URLParam(r, "id")
	if tokenID == "" {
		http.Error(w, "token id required", http.StatusBadRequest)
		return
	}

	// Refresh token
	result, err := h.TokenService.RefreshToken(ctx, sess.UserID, tokenID)
	if err != nil {
		var notFoundErr *service.NotFoundError
		if errors.As(err, &notFoundErr) {
			writeJSON(w, http.StatusNotFound, models.AuthError{Error: "token not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, models.AuthError{Error: "failed to refresh token"})
		return
	}

	// Return new token with plaintext (only shown once!)
	writeJSON(w, http.StatusOK, models.TokenRefreshResponse{
		ID:          result.Token.ID,
		Name:        result.Token.Name,
		Token:       result.Plaintext,
		TokenPrefix: result.TokenPrefix,
		ExpiresAt:   result.Token.ExpiresAt,
		CreatedAt:   result.Token.CreatedAt,
	})
}
