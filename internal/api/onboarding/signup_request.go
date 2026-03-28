package onboarding

import (
	"encoding/json"
	"net/http"
	"time"
)

type SignupRequestPayload struct {
	Email string `json:"email"`
}

func (h *Handler) SignupRequest(w http.ResponseWriter, r *http.Request) {
	var payload SignupRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if h.SignupRequestLimiter != nil && !h.SignupRequestLimiter.Allow("request:"+payload.Email+":"+r.RemoteAddr, time.Now().UTC()) {
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
	resp, err := h.SignupService.RequestSignup(r.Context(), payload.Email)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(resp)
}
