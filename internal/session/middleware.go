package session

import (
	"net"
	"net/http"
	"strings"
	"time"
)

type Resolver struct {
	PrototypeAccountID string
	PrototypeUserID    string
	PrototypeRole      string
	AllowPrototype     bool
	CookieName         string
	Tokens             *TokenManager
	Now                func() time.Time
}

func (r Resolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		resolved := Context{}
		if token := r.sessionTokenFromRequest(req); token != "" && r.Tokens != nil {
			parsed, err := r.Tokens.Parse(token, r.now())
			if err == nil {
				resolved = parsed
			}
		}
		if !resolved.IsAuthenticated() && r.AllowPrototype {
			if resolved.UserID == "" {
				resolved.UserID = r.PrototypeUserID
			}
			if resolved.AccountID == "" {
				resolved.AccountID = r.PrototypeAccountID
			}
			if resolved.Role == "" {
				resolved.Role = r.PrototypeRole
			}
			resolved.Prototype = resolved.UserID == r.PrototypeUserID && resolved.AccountID == r.PrototypeAccountID && resolved.Role == r.PrototypeRole
		}
		next.ServeHTTP(w, req.WithContext(NewContext(req.Context(), resolved)))
	})
}

func (r Resolver) sessionTokenFromRequest(req *http.Request) string {
	authz := strings.TrimSpace(req.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	cookieName := strings.TrimSpace(r.CookieName)
	if cookieName == "" {
		cookieName = DefaultCookieName
	}
	cookie, err := req.Cookie(cookieName)
	if err == nil {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
}

func (r Resolver) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func ClientIP(req *http.Request) string {
	forwarded := strings.TrimSpace(strings.Split(req.Header.Get("X-Forwarded-For"), ",")[0])
	if forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(req.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(req.RemoteAddr)
}
