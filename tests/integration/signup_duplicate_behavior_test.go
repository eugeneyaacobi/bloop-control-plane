package integration

import (
	"context"
	"testing"
	"time"

	"bloop-control-plane/internal/audit"
	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/service"
)

type duplicateCaptureEmailSender struct {
	tokens []string
}

func (d *duplicateCaptureEmailSender) SendVerificationEmail(ctx context.Context, toEmail, token string) error {
	d.tokens = append(d.tokens, token)
	return nil
}

func TestDuplicateSignupRequestsAreBlandlyAccepted(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	repo := repository.NewPostgresSignupRepository(pool)
	email := &duplicateCaptureEmailSender{}
	svc := service.NewSignupService(repo, email, audit.New(pool), &config.Config{VerificationTokenTTL: time.Hour}, nil, nil)

	first, err := svc.RequestSignup(context.Background(), "dupe@example.com")
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	second, err := svc.RequestSignup(context.Background(), "dupe@example.com")
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	if !first.Accepted || !second.Accepted {
		t.Fatalf("expected both requests to be accepted")
	}

	var signupCount, verificationCount int
	if err := pool.QueryRow(context.Background(), "select count(*) from signup_requests where email = 'dupe@example.com'").Scan(&signupCount); err != nil {
		t.Fatalf("count signup requests: %v", err)
	}
	if err := pool.QueryRow(context.Background(), "select count(*) from email_verifications ev join signup_requests sr on sr.id = ev.signup_request_id where sr.email = 'dupe@example.com'").Scan(&verificationCount); err != nil {
		t.Fatalf("count verifications: %v", err)
	}
	if signupCount != 2 || verificationCount != 2 {
		t.Fatalf("expected duplicate requests to persist two rows, got signup=%d verification=%d", signupCount, verificationCount)
	}
	if len(email.tokens) != 2 {
		t.Fatalf("expected two email attempts, got %d", len(email.tokens))
	}
	if email.tokens[0] == email.tokens[1] {
		t.Fatalf("expected distinct tokens for duplicate requests")
	}
	var emailSentCount int
	if err := pool.QueryRow(context.Background(), "select count(*) from audit_events where event_type = 'signup.email_sent' and target_id in (select id from signup_requests where email = 'dupe@example.com')").Scan(&emailSentCount); err != nil {
		t.Fatalf("count email sent audit events: %v", err)
	}
	if emailSentCount != 2 {
		t.Fatalf("expected two email_sent audit events, got %d", emailSentCount)
	}
}
