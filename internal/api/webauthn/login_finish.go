package webauthn

import (
	"encoding/json"
	"errors"
	"net/http"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
)

// FinishLoginRequest represents the request to complete WebAuthn login
type FinishLoginRequest struct {
	ChallengeID string `json:"challenge_id"`
	Email       string `json:"email"`
	Credential  any    `json:"credential"`
}

// FinishLogin handles the completion of WebAuthn login ceremony
// POST /api/webauthn/login-finish
func (h *Handler) FinishLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	var req FinishLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ChallengeID == "" || req.Email == "" {
		writeJSON(w, http.StatusBadRequest, models.AuthError{Error: "challenge_id and email are required"})
		return
	}

	// Extract client info
	ipAddress := session.ClientIP(r)
	userAgent := r.UserAgent()

	// Serialize credential to JSON for service
	var credentialJSON []byte
	var err error
	if req.Credential != nil {
		credentialJSON, err = json.Marshal(req.Credential)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, models.AuthError{Error: "invalid credential data"})
			return
		}
	}

	// Finish login
	userID, err := h.WebAuthnService.FinishLogin(ctx, req.Email, req.ChallengeID, string(credentialJSON), ipAddress, userAgent)
	if err != nil {
		var authErr *service.AuthError
		if errors.As(err, &authErr) {
			writeJSON(w, http.StatusUnauthorized, models.AuthError{Error: authErr.Message})
			return
		}
		writeJSON(w, http.StatusUnauthorized, models.AuthError{Error: "authentication failed"})
		return
	}

	// Issue session token
	// Note: In a real implementation, you'd look up the user's account and role
	token, err := h.issueToken(userID, "acct_default", "customer")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.AuthError{Error: "failed to create session"})
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     h.SessionName,
		Value:    token,
		Path:     "/",
		MaxAge:   0, // Session cookie
		Secure:   h.SecureCookie,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Domain:   h.CookieDomain,
	})

	// Return success
	writeJSON(w, http.StatusOK, models.LoginResponse{
		User: models.UserContext{
			ID:          userID,
			Email:       req.Email,
			DisplayName: req.Email,
			AccountID:   "acct_default",
			Role:        "customer",
		},
		RequiresWebAuthn: true,
	})
}
