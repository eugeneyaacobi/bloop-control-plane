package session

import "time"

type Issuer struct {
	tokens     *TokenManager
	cookieName string
	ttl        time.Duration
}

func NewIssuer(tokens *TokenManager, cookieName string, ttl time.Duration) *Issuer {
	if cookieName == "" {
		cookieName = DefaultCookieName
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &Issuer{tokens: tokens, cookieName: cookieName, ttl: ttl}
}

func (i *Issuer) Issue(ctx Context, now time.Time) (string, time.Time, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	expiresAt := now.Add(i.ttl)
	token, err := i.tokens.Sign(TokenClaims{
		UserID:    ctx.UserID,
		AccountID: ctx.AccountID,
		Role:      ctx.Role,
		ExpiresAt: expiresAt.Unix(),
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func (i *Issuer) CookieName() string { return i.cookieName }
