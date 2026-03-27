package session

import (
	"context"
)

type SessionVersionLookup interface {
	GetSessionVersion(ctx context.Context, scopeKey string) (int64, error)
}

func ValidateSessionVersion(ctx context.Context, repo SessionVersionLookup, parsed *ParsedToken) bool {
	if repo == nil || parsed == nil {
		return true
	}
	scopeKey := parsed.Context.UserID + "|" + parsed.Context.AccountID + "|" + parsed.Context.Role
	version, err := repo.GetSessionVersion(ctx, scopeKey)
	if err != nil {
		return false
	}
	if parsed.Claims.SessionVersion == 0 {
		return version <= 1
	}
	return parsed.Claims.SessionVersion == version
}
