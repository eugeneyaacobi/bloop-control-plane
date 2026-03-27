package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const DefaultCookieName = "bloop_session"

var (
	ErrMissingSecret   = errors.New("missing session secret")
	ErrInvalidToken    = errors.New("invalid session token")
	ErrExpiredToken    = errors.New("expired session token")
	ErrMissingClaims   = errors.New("missing required session claims")
	ErrUnsupportedKind = errors.New("unsupported session token kind")
)

type TokenClaims struct {
	Version        int    `json:"v"`
	Kind           string `json:"kind"`
	UserID         string `json:"uid,omitempty"`
	AccountID      string `json:"aid,omitempty"`
	Role           string `json:"role,omitempty"`
	SessionVersion int64  `json:"sv,omitempty"`
	ExpiresAt      int64  `json:"exp"`
}

type TokenManager struct {
	secret []byte
}

type ParsedToken struct {
	Context Context
	Claims  TokenClaims
}

func NewTokenManager(secret string) (*TokenManager, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, ErrMissingSecret
	}
	return &TokenManager{secret: []byte(secret)}, nil
}

func (m *TokenManager) Sign(claims TokenClaims) (string, error) {
	if m == nil || len(m.secret) == 0 {
		return "", ErrMissingSecret
	}
	claims.Version = 1
	if claims.Kind == "" {
		claims.Kind = "session"
	}
	if claims.Kind != "session" {
		return "", ErrUnsupportedKind
	}
	if claims.ExpiresAt == 0 {
		return "", ErrMissingClaims
	}
	if claims.UserID == "" || claims.Role == "" || (claims.AccountID == "" && claims.Role != "admin" && claims.Role != "operator") {
		return "", ErrMissingClaims
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal session claims: %w", err)
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	sig := m.sign(encodedPayload)
	return encodedPayload + "." + sig, nil
}

func (m *TokenManager) ParseDetailed(token string, now time.Time) (*ParsedToken, error) {
	if m == nil || len(m.secret) == 0 {
		return nil, ErrMissingSecret
	}
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, ErrInvalidToken
	}
	if !hmac.Equal([]byte(m.sign(parts[0])), []byte(parts[1])) {
		return nil, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Version != 1 || claims.Kind != "session" {
		return nil, ErrInvalidToken
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if now.Unix() >= claims.ExpiresAt {
		return nil, ErrExpiredToken
	}
	ctx := Context{
		UserID:    claims.UserID,
		AccountID: claims.AccountID,
		Role:      claims.Role,
		Prototype: false,
	}
	if !ctx.IsAuthenticated() || (ctx.AccountID == "" && !ctx.IsAdmin()) {
		return nil, ErrMissingClaims
	}
	return &ParsedToken{Context: ctx, Claims: claims}, nil
}

func (m *TokenManager) Parse(token string, now time.Time) (Context, error) {
	parsed, err := m.ParseDetailed(token, now)
	if err != nil {
		return Context{}, err
	}
	return parsed.Context, nil
}

func (m *TokenManager) sign(encodedPayload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
