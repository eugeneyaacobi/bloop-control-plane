package onboarding

import (
	"net/http"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/security"
	"bloop-control-plane/internal/service"
)

type Handler struct {
	Service              *service.OnboardingService
	SignupService        *service.SignupService
	SignupRequestLimiter *security.RateLimiter
	SignupVerifyLimiter  *security.RateLimiter
}

func (h *Handler) Steps(w http.ResponseWriter, r *http.Request) {
	sess, ok := authz.RequireSession(w, r)
	if !ok {
		return
	}

	resp, err := h.Service.ListSteps(r.Context(), sess.AccountID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	authz.WriteJSON(w, http.StatusOK, resp)
}
