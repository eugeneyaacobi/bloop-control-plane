package unit

import (
	"context"
	"testing"
	"time"

	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/service"
)

type fakeSignupRepo struct {
	signupID             string
	signupEmail          string
	signupState          string
	verificationID       string
	verificationSignupID string
	tokenHash            string
	verificationState    string
	expiresAt            time.Time
	markCalled           bool
	lookupVerification   *repository.EmailVerification
	lookupSignup         *models.SignupRequest
}

func (r *fakeSignupRepo) CreateSignupRequest(ctx context.Context, id, email, state string) error {
	r.signupID, r.signupEmail, r.signupState = id, email, state
	return nil
}

func (r *fakeSignupRepo) CreateEmailVerification(ctx context.Context, id, signupRequestID, tokenHash, state string, expiresAt time.Time) error {
	r.verificationID, r.verificationSignupID, r.tokenHash, r.verificationState, r.expiresAt = id, signupRequestID, tokenHash, state, expiresAt
	return nil
}

func (r *fakeSignupRepo) FindVerificationByTokenHash(ctx context.Context, tokenHash string) (*repository.EmailVerification, *models.SignupRequest, error) {
	if r.lookupVerification == nil || tokenHash == "" {
		return nil, nil, nil
	}
	return r.lookupVerification, r.lookupSignup, nil
}

func (r *fakeSignupRepo) MarkVerificationCompleted(ctx context.Context, verificationID, signupRequestID string, verifiedAt time.Time) error {
	r.markCalled = true
	return nil
}

type fakeEmailSender struct {
	to    string
	token string
	err   error
}

func (f *fakeEmailSender) SendVerificationEmail(ctx context.Context, toEmail, token string) error {
	f.to = toEmail
	f.token = token
	return f.err
}

func TestSignupRequestPersistsAndSendsEmail(t *testing.T) {
	repo := &fakeSignupRepo{}
	email := &fakeEmailSender{}
	cfg := &config.Config{VerificationTokenTTL: time.Hour}
	svc := service.NewSignupService(repo, email, nil, cfg)

	resp, err := svc.RequestSignup(context.Background(), "newuser@example.com")
	if err != nil {
		t.Fatalf("request signup: %v", err)
	}
	if !resp.Accepted {
		t.Fatalf("expected accepted response")
	}
	if repo.signupEmail != "newuser@example.com" {
		t.Fatalf("expected signup email stored, got %q", repo.signupEmail)
	}
	if repo.tokenHash == "" {
		t.Fatalf("expected token hash to be persisted")
	}
	if email.to != "newuser@example.com" {
		t.Fatalf("expected email recipient to be set, got %q", email.to)
	}
	if email.token == "" {
		t.Fatalf("expected raw token to be sent via fake email sender")
	}
}

func TestSignupVerifyMarksCompletedForValidToken(t *testing.T) {
	repo := &fakeSignupRepo{}
	email := &fakeEmailSender{}
	cfg := &config.Config{VerificationTokenTTL: time.Hour}
	svc := service.NewSignupService(repo, email, nil, cfg)

	requestResp, err := svc.RequestSignup(context.Background(), "verifyme@example.com")
	if err != nil || !requestResp.Accepted {
		t.Fatalf("request signup failed: %v", err)
	}
	repo.lookupVerification = &repository.EmailVerification{
		ID:              repo.verificationID,
		SignupRequestID: repo.signupID,
		TokenHash:       repo.tokenHash,
		State:           "pending",
		ExpiresAt:       time.Now().Add(time.Hour),
	}
	repo.lookupSignup = &models.SignupRequest{ID: repo.signupID, Email: repo.signupEmail, State: "pending"}

	resp, err := svc.VerifySignup(context.Background(), email.token)
	if err != nil {
		t.Fatalf("verify signup: %v", err)
	}
	if !resp.Verified || resp.Status != service.SignupVerifyStatusVerified {
		t.Fatalf("expected verification success, got %+v", resp)
	}
	if !repo.markCalled {
		t.Fatalf("expected repo mark completed to be called")
	}
}

func TestSignupVerifyRejectsExpiredToken(t *testing.T) {
	repo := &fakeSignupRepo{}
	email := &fakeEmailSender{}
	cfg := &config.Config{VerificationTokenTTL: time.Hour}
	svc := service.NewSignupService(repo, email, nil, cfg)

	requestResp, err := svc.RequestSignup(context.Background(), "expired@example.com")
	if err != nil || !requestResp.Accepted {
		t.Fatalf("request signup failed: %v", err)
	}
	repo.lookupVerification = &repository.EmailVerification{
		ID:              repo.verificationID,
		SignupRequestID: repo.signupID,
		TokenHash:       repo.tokenHash,
		State:           "pending",
		ExpiresAt:       time.Now().Add(-time.Minute),
	}
	repo.lookupSignup = &models.SignupRequest{ID: repo.signupID, Email: repo.signupEmail, State: "pending"}

	resp, err := svc.VerifySignup(context.Background(), email.token)
	if err != nil {
		t.Fatalf("verify signup: %v", err)
	}
	if resp.Verified || resp.Status != service.SignupVerifyStatusExpired {
		t.Fatalf("expected expired token verification to fail, got %+v", resp)
	}
	if repo.markCalled {
		t.Fatalf("did not expect mark completed for expired token")
	}
}
