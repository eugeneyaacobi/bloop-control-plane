package service

import (
	"context"
	"time"

	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/session"
)

type SessionService struct {
	repo  repository.SessionRepository
	nowFn func() time.Time
}

func NewSessionService(repo repository.SessionRepository) *SessionService {
	return &SessionService{repo: repo, nowFn: func() time.Time { return time.Now().UTC() }}
}

type SessionMeResponse struct {
	Authenticated bool   `json:"authenticated"`
	Prototype     bool   `json:"prototype"`
	Role          string `json:"role,omitempty"`
	UserID        string `json:"userId,omitempty"`
	AccountID     string `json:"accountId,omitempty"`
	User          any    `json:"user,omitempty"`
	Account       any    `json:"account,omitempty"`
	Membership    any    `json:"membership,omitempty"`
}

func (s *SessionService) Logout(ctx context.Context) error {
	sess, _ := session.FromContext(ctx)
	if s.repo == nil || !sess.IsAuthenticated() {
		return nil
	}
	_, err := s.repo.BumpSessionVersion(ctx, sess, s.nowFn())
	return err
}

func (s *SessionService) GetMe(ctx context.Context) (*SessionMeResponse, error) {
	sess, _ := session.FromContext(ctx)
	resp := &SessionMeResponse{
		Authenticated: sess.IsAuthenticated(),
		Prototype:     sess.Prototype,
		Role:          sess.Role,
		UserID:        sess.UserID,
		AccountID:     sess.AccountID,
	}
	if s.repo == nil || !resp.Authenticated {
		return resp, nil
	}
	identity, err := s.repo.ResolveIdentity(ctx, sess)
	if err != nil {
		return nil, err
	}
	if identity != nil {
		resp.User = identity.User
		resp.Account = identity.Account
		resp.Membership = identity.Membership
	}
	return resp, nil
}
