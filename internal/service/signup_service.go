package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bloop-control-plane/internal/audit"
	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/security"
	"bloop-control-plane/internal/session"
)

type SignupService struct {
	repo         repository.SignupRepository
	email        VerificationEmailSender
	audit        *audit.Recorder
	cfg          *config.Config
	issuer       *session.Issuer
	provisioning repository.ProvisioningRepository
	nowFn        func() time.Time
}

func NewSignupService(repo repository.SignupRepository, email VerificationEmailSender, audit *audit.Recorder, cfg *config.Config, issuer *session.Issuer, provisioning repository.ProvisioningRepository) *SignupService {
	return &SignupService{
		repo:         repo,
		email:        email,
		audit:        audit,
		cfg:          cfg,
		issuer:       issuer,
		provisioning: provisioning,
		nowFn:        func() time.Time { return time.Now().UTC() },
	}
}

type SignupRequestResponse struct {
	Accepted bool `json:"accepted"`
}

type SignupVerifyStatus string

const (
	SignupVerifyStatusVerified SignupVerifyStatus = "verified"
	SignupVerifyStatusInvalid  SignupVerifyStatus = "invalid"
	SignupVerifyStatusExpired  SignupVerifyStatus = "expired"
	SignupVerifyStatusUsed     SignupVerifyStatus = "used"
)

type SignupVerifyResponse struct {
	Verified bool               `json:"verified"`
	Status   SignupVerifyStatus `json:"status"`
	Session  *SessionIssue      `json:"session,omitempty"`
}

type SessionIssue struct {
	Token      string          `json:"token,omitempty"`
	CookieName string          `json:"cookieName,omitempty"`
	ExpiresAt  string          `json:"expiresAt,omitempty"`
	Context    *session.Context `json:"context,omitempty"`
}

func (s *SignupService) RequestSignup(ctx context.Context, email string) (*SignupRequestResponse, error) {
	now := s.nowFn()
	token, err := security.NewVerificationToken(s.cfg.VerificationTokenTTL, now)
	if err != nil {
		return nil, err
	}
	signupID := fmt.Sprintf("sr_%d", now.UnixNano())
	verificationID := fmt.Sprintf("sv_%d", now.UnixNano())

	if err := s.repo.CreateSignupRequest(ctx, signupID, email, "pending"); err != nil {
		return nil, err
	}
	if err := s.repo.CreateEmailVerification(ctx, verificationID, signupID, token.Hash, "pending", token.ExpiresAt); err != nil {
		return nil, err
	}

	meta, _ := json.Marshal(map[string]any{"email": email, "status": "requested"})
	_ = s.audit.Record(ctx, "signup.requested", "", "signup_request", signupID, string(meta))

	if err := s.email.SendVerificationEmail(ctx, email, token.Raw); err != nil {
		meta, _ := json.Marshal(map[string]any{"email": email, "status": "email_failed", "reason": "delivery_attempt_failed"})
		_ = s.audit.Record(ctx, "signup.email_failed", "", "signup_request", signupID, string(meta))
		return &SignupRequestResponse{Accepted: true}, nil
	}

	meta, _ = json.Marshal(map[string]any{"email": email, "status": "email_sent"})
	_ = s.audit.Record(ctx, "signup.email_sent", "", "signup_request", signupID, string(meta))
	return &SignupRequestResponse{Accepted: true}, nil
}

func (s *SignupService) VerifySignup(ctx context.Context, rawToken string) (*SignupVerifyResponse, error) {
	if rawToken == "" {
		return nil, errors.New("missing token")
	}
	tokenHash := security.HashVerificationToken(rawToken)
	verification, signup, err := s.repo.FindVerificationByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if verification == nil || signup == nil {
		return &SignupVerifyResponse{Verified: false, Status: SignupVerifyStatusInvalid}, nil
	}
	now := s.nowFn()
	if verification.ExpiresAt.Before(now) {
		return &SignupVerifyResponse{Verified: false, Status: SignupVerifyStatusExpired}, nil
	}
	if verification.State != "pending" {
		resp := &SignupVerifyResponse{Verified: false, Status: SignupVerifyStatusUsed}
		if s.issuer != nil && s.provisioning != nil {
			if existing, err := s.provisioning.FindProvisionedIdentityByEmail(ctx, signup.Email); err == nil && existing != nil {
				sessCtx := session.Context{UserID: existing.UserID, AccountID: existing.AccountID, Role: existing.Role, Prototype: false}
				if token, expiresAt, err := s.issuer.Issue(sessCtx, now); err == nil {
					resp.Session = &SessionIssue{Token: token, CookieName: s.issuer.CookieName(), ExpiresAt: expiresAt.UTC().Format(time.RFC3339), Context: &sessCtx}
				}
			}
		}
		return resp, nil
	}
	if err := s.repo.MarkVerificationCompleted(ctx, verification.ID, signup.ID, now); err != nil {
		return nil, err
	}
	meta, _ := json.Marshal(map[string]any{"email": signup.Email, "status": "verified"})
	_ = s.audit.Record(ctx, "signup.verified", "", "signup_request", signup.ID, string(meta))

	resp := &SignupVerifyResponse{Verified: true, Status: SignupVerifyStatusVerified}
	if s.issuer != nil {
		identity := &repository.ProvisionedIdentity{UserID: "user_gene", AccountID: "acct_default", Role: "owner", DisplayName: "Gene", Email: signup.Email, AccountName: "Gene / default-org"}
		if s.provisioning != nil {
			if provisioned, err := s.provisioning.ProvisionSignupIdentity(ctx, signup.Email, now); err == nil && provisioned != nil {
				identity = provisioned
			}
		}
		sessCtx := session.Context{UserID: identity.UserID, AccountID: identity.AccountID, Role: identity.Role, Prototype: false}
		token, expiresAt, err := s.issuer.Issue(sessCtx, now)
		if err == nil {
			resp.Session = &SessionIssue{
				Token:      token,
				CookieName: s.issuer.CookieName(),
				ExpiresAt:  expiresAt.UTC().Format(time.RFC3339),
				Context:    &sessCtx,
			}
		}
	}
	return resp, nil
}
