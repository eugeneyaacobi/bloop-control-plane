package session

import "context"

type Context struct {
	UserID    string
	AccountID string
	Role      string
	Prototype bool
}

func (c Context) IsAuthenticated() bool {
	return c.UserID != "" || c.AccountID != "" || c.Role == "admin"
}

func (c Context) IsAdmin() bool {
	switch c.Role {
	case "admin", "operator", "owner":
		return true
	default:
		return false
	}
}

type contextKey struct{}

func NewContext(ctx context.Context, session Context) context.Context {
	return context.WithValue(ctx, contextKey{}, session)
}

func FromContext(ctx context.Context) (Context, bool) {
	session, ok := ctx.Value(contextKey{}).(Context)
	return session, ok
}
