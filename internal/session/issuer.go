package session

import (
	"context"
	"time"
)

type Issuer struct {
	tokens          *TokenManager
	cookieName      string
	ttl             time.Duration
	sessionVersions interface{ GetSessionVersion(ctx context.Context, scopeKey string) (int64, error) }
}

func NewIssuer(tokens *TokenManager, cookieName string, ttl time.Duration, sessionVersions interface{ GetSessionVersion(ctx context.Context, scopeKey string) (int64, error) }) *Issuer {
	if cookieName == "" {
		cookieName = DefaultCookieName
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &Issuer{tokens: tokens, cookieName: cookieName, ttl: ttl, sessionVersions: sessionVersions}
}

func (i *Issuer) Issue(ctx Context, now time.Time) (string, time.Time, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	expiresAt := now.Add(i.ttl)
	sessionVersion := int64(1)
	if i.sessionVersions != nil {
		if version, err := i.sessionVersions.GetSessionVersion(context.Background(), ctx.UserID+"|"+ctx.AccountID+"|"+ctx.Role); err == nil {
			sessionVersion = version
		}
	}
	token, err := i.tokens.Sign(TokenClaims{
		UserID:         ctx.UserID,
		AccountID:      ctx.AccountID,
		Role:           ctx.Role,
		SessionVersion: sessionVersion,
		ExpiresAt:      expiresAt.Unix(),
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func (i *Issuer) CookieName() string { return i.cookieName }
