package onboarding

import (
	"encoding/json"
	"net/http"
	"time"
)

type SignupVerifyPayload struct {
	Token string `json:"token"`
}

func (h *Handler) SignupVerify(w http.ResponseWriter, r *http.Request) {
	var payload SignupVerifyPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if h.SignupVerifyLimiter != nil && !h.SignupVerifyLimiter.Allow("verify:"+r.RemoteAddr, time.Now().UTC()) {
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
	resp, err := h.SignupService.VerifySignup(r.Context(), payload.Token)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if resp != nil && resp.Session != nil && resp.Session.Token != "" {
		cookieName := h.SessionCookieName
		if cookieName == "" {
			cookieName = "bloop_session"
		}
		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    resp.Session.Token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   h.SessionCookieSecure,
			Domain:   h.SessionCookieDomain,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
