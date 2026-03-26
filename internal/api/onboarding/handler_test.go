package onboarding

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
)

func withOnboardingSession(req *http.Request, accountID string) *http.Request {
	return req.WithContext(session.NewContext(req.Context(), session.Context{UserID: "user_test", AccountID: accountID, Role: "customer"}))
}

type fakeOnboardingRepo struct {
	steps         []repository.OnboardingStep
	lastAccountID string
	err           error
}

func (f *fakeOnboardingRepo) ListSteps(ctx context.Context, accountID string) ([]repository.OnboardingStep, error) {
	f.lastAccountID = accountID
	return f.steps, f.err
}

type fakeHandlerSignupRepo struct {
	requestEmail string
	verification *repository.EmailVerification
	signup       *models.SignupRequest
	markCalled   bool
}

func (r *fakeHandlerSignupRepo) CreateSignupRequest(ctx context.Context, id, email, state string) error {
	r.requestEmail = email
	r.signup = &models.SignupRequest{ID: id, Email: email, State: state}
	return nil
}

func (r *fakeHandlerSignupRepo) CreateEmailVerification(ctx context.Context, id, signupRequestID, tokenHash, state string, expiresAt time.Time) error {
	r.verification = &repository.EmailVerification{
		ID:              id,
		SignupRequestID: signupRequestID,
		TokenHash:       tokenHash,
		State:           state,
		ExpiresAt:       expiresAt,
	}
	return nil
}

func (r *fakeHandlerSignupRepo) FindVerificationByTokenHash(ctx context.Context, tokenHash string) (*repository.EmailVerification, *models.SignupRequest, error) {
	if r.verification == nil || r.signup == nil || r.verification.TokenHash != tokenHash {
		return nil, nil, nil
	}
	return r.verification, r.signup, nil
}

func (r *fakeHandlerSignupRepo) MarkVerificationCompleted(ctx context.Context, verificationID, signupRequestID string, verifiedAt time.Time) error {
	r.markCalled = true
	if r.verification != nil {
		r.verification.State = "verified"
		r.verification.VerifiedAt = &verifiedAt
	}
	if r.signup != nil {
		r.signup.State = "verified"
	}
	return nil
}

type fakeEmailSender struct{}

func (f *fakeEmailSender) SendVerificationEmail(ctx context.Context, toEmail, token string) error {
	return nil
}

func TestSignupRequestReturnsAcceptedJSON(t *testing.T) {
	repo := &fakeHandlerSignupRepo{}
	h := &Handler{
		SignupService: service.NewSignupService(repo, &fakeEmailSender{}, nil, &config.Config{VerificationTokenTTL: time.Hour}),
	}
	body := bytes.NewBufferString(`{"email":"handler@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/signup/request", body)
	w := httptest.NewRecorder()

	h.SignupRequest(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected %d got %d", http.StatusAccepted, w.Code)
	}
	var resp map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp["accepted"] {
		t.Fatalf("expected accepted=true")
	}
	if repo.requestEmail != "handler@example.com" {
		t.Fatalf("expected request email recorded, got %q", repo.requestEmail)
	}
}

func TestSignupRequestRejectsBadJSON(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/signup/request", bytes.NewBufferString("{"))
	w := httptest.NewRecorder()

	h.SignupRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, w.Code)
	}
}

func TestSignupVerifyReturnsInvalidStatusForUnknownToken(t *testing.T) {
	repo := &fakeHandlerSignupRepo{}
	h := &Handler{
		SignupService: service.NewSignupService(repo, &fakeEmailSender{}, nil, &config.Config{VerificationTokenTTL: time.Hour}),
	}
	req := httptest.NewRequest(http.MethodPost, "/signup/verify", bytes.NewBufferString(`{"token":"unknown-token"}`))
	w := httptest.NewRecorder()

	h.SignupVerify(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}
	var resp struct {
		Verified bool   `json:"verified"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Verified || resp.Status != string(service.SignupVerifyStatusInvalid) {
		t.Fatalf("expected invalid status for unknown token, got %+v", resp)
	}
}

func TestSignupVerifyRejectsBadJSON(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/signup/verify", bytes.NewBufferString("{"))
	w := httptest.NewRecorder()

	h.SignupVerify(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, w.Code)
	}
}

func TestStepsRequiresSession(t *testing.T) {
	h := &Handler{Service: service.NewOnboardingService(&fakeOnboardingRepo{})}
	w := httptest.NewRecorder()
	h.Steps(w, httptest.NewRequest(http.MethodGet, "/steps", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestStepsReturnsJSON(t *testing.T) {
	repo := &fakeOnboardingRepo{steps: []repository.OnboardingStep{{ID: "ob_1", StepKey: "connect", Title: "Connect", Detail: "Do thing", State: "active"}}}
	h := &Handler{Service: service.NewOnboardingService(repo)}
	req := withOnboardingSession(httptest.NewRequest(http.MethodGet, "/steps", nil), "acct_from_header")
	w := httptest.NewRecorder()

	h.Steps(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}
	var resp []repository.OnboardingStep
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 1 || resp[0].ID != "ob_1" {
		t.Fatalf("unexpected steps response: %+v", resp)
	}
	if repo.lastAccountID != "acct_from_header" {
		t.Fatalf("expected session account id to be used, got %q", repo.lastAccountID)
	}
}
