package tokens

import (
	"encoding/json"
	"net/http"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/session"
)

// Create handles token creation
// POST /api/tokens
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current session from context
	sess, ok := session.FromContext(ctx)
	if !ok || !sess.IsAuthenticated() || sess.UserID == "" {
		writeJSON(w, http.StatusUnauthorized, models.AuthError{Error: "authentication required"})
		return
	}

	// Parse request
	var req models.TokenCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, models.AuthError{Error: "name is required"})
		return
	}

	// Use account from session if not specified
	accountID := req.AccountID
	if accountID == "" {
		accountID = sess.AccountID
	}

	// Create token
	result, err := h.TokenService.CreateToken(ctx, sess.UserID, accountID, req.Name, req.ExpiresIn)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.AuthError{Error: "failed to create token"})
		return
	}

	// Return token with plaintext (only shown once!)
	writeJSON(w, http.StatusCreated, models.TokenCreateResponse{
		ID:          result.Token.ID,
		Name:        result.Token.Name,
		Token:       result.Plaintext,
		TokenPrefix: result.TokenPrefix,
		AccountID:   result.Token.AccountID,
		ExpiresAt:   result.Token.ExpiresAt,
		CreatedAt:   result.Token.CreatedAt,
	})
}

// writeJSON writes JSON response
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
