package unit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"bloop-control-plane/internal/audit"
	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/security"
	"bloop-control-plane/internal/service"
)

type fakeAuditRecorder struct {
	events []string
	meta   []string
}

func TestVerifySignupMissingTokenReturnsError(t *testing.T) {
	repo := &fakeSignupRepo{}
	email := &fakeEmailSender{}
	cfg := &config.Config{VerificationTokenTTL: time.Hour}
	svc := newTestSignupService(repo, email, cfg)

	resp, err := svc.VerifySignup(context.Background(), "")
	if err == nil {
		t.Fatalf("expected missing token error")
	}
	if resp != nil {
		t.Fatalf("expected nil response on missing token")
	}
}

func TestRequestSignupEmailFailureStillAcceptedAndNoRawTokenLeak(t *testing.T) {
	repo := &fakeSignupRepo{}
	email := &fakeEmailSender{err: context.DeadlineExceeded}
	cfg := &config.Config{VerificationTokenTTL: time.Hour}
	svc := newTestSignupService(repo, email, cfg)

	resp, err := svc.RequestSignup(context.Background(), "failing@example.com")
	if err != nil {
		t.Fatalf("request signup: %v", err)
	}
	if !resp.Accepted {
		t.Fatalf("expected accepted response")
	}
	payload, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if string(payload) == "" {
		t.Fatalf("expected json payload")
	}
	if email.token != "" && containsString(string(payload), email.token) {
		t.Fatalf("raw token leaked in response payload: %s", string(payload))
	}
	if repo.tokenHash == email.token {
		t.Fatalf("raw token should not equal persisted hash")
	}
}

func TestVerifySignupExpiredTokenReturnsFalse(t *testing.T) {
	repo := &fakeSignupRepo{}
	email := &fakeEmailSender{}
	cfg := &config.Config{VerificationTokenTTL: time.Hour}
	svc := newTestSignupService(repo, email, cfg)

	_, err := svc.RequestSignup(context.Background(), "expired2@example.com")
	if err != nil {
		t.Fatalf("request signup: %v", err)
	}
	repo.lookupVerification = &repository.EmailVerification{
		ID:              repo.verificationID,
		SignupRequestID: repo.signupID,
		TokenHash:       repo.tokenHash,
		State:           "pending",
		ExpiresAt:       time.Now().Add(-time.Second),
	}
	repo.lookupSignup = &models.SignupRequest{ID: repo.signupID, Email: repo.signupEmail, State: "pending"}

	resp, err := svc.VerifySignup(context.Background(), email.token)
	if err != nil {
		t.Fatalf("verify signup: %v", err)
	}
	if resp.Verified || resp.Status != service.SignupVerifyStatusExpired {
		t.Fatalf("expected expired token verification to fail, got %+v", resp)
	}
}

func TestTokenHashingNeverStoresRawToken(t *testing.T) {
	repo := &fakeSignupRepo{}
	email := &fakeEmailSender{}
	cfg := &config.Config{VerificationTokenTTL: time.Hour}
	svc := newTestSignupService(repo, email, cfg)

	_, err := svc.RequestSignup(context.Background(), "hashcheck@example.com")
	if err != nil {
		t.Fatalf("request signup: %v", err)
	}
	if repo.tokenHash == "" || email.token == "" {
		t.Fatalf("expected both stored hash and email token")
	}
	if repo.tokenHash == email.token {
		t.Fatalf("raw token was stored instead of hash")
	}
	if repo.tokenHash != security.HashVerificationToken(email.token) {
		t.Fatalf("stored hash did not match derived token hash")
	}
}

func newTestSignupService(repo *fakeSignupRepo, email *fakeEmailSender, cfg *config.Config) *service.SignupService {
	return service.NewSignupService(repo, email, audit.New(nil), cfg, nil, nil)
}

func containsString(haystack, needle string) bool {
	return needle != "" && len(haystack) >= len(needle) && (haystack == needle || len(needle) > 0 && stringContains(haystack, needle))
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
