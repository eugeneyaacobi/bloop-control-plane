package runtime

import (
	"context"
	"fmt"

	"bloop-control-plane/internal/models"
)

type Activity struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Level   string `json:"level"`
}

type AccountProjection struct {
	ActiveRoutes    int        `json:"activeRoutes"`
	ProtectedRoutes int        `json:"protectedRoutes"`
	DegradedRoutes  int        `json:"degradedRoutes"`
	RecentActivity  []Activity `json:"recentActivity"`
}

type GlobalProjection struct {
	FlaggedExposures int        `json:"flaggedExposures"`
	RecentActivity   []Activity `json:"recentActivity"`
}

type Repository interface {
	ProjectAccount(ctx context.Context, account models.Account, tunnels []models.Tunnel) (AccountProjection, error)
	ProjectGlobal(ctx context.Context, tunnels []models.Tunnel, flags []models.ReviewFlag) (GlobalProjection, error)
	VerifyInstallationToken(ctx context.Context, token string) (*models.RuntimeInstallationToken, error)
	CreateIngestToken(ctx context.Context, installationID string) (string, error)
}

type StubRepository struct{}

func NewStubRepository() *StubRepository { return &StubRepository{} }

func (r *StubRepository) ProjectAccount(ctx context.Context, account models.Account, tunnels []models.Tunnel) (AccountProjection, error) {
	_ = ctx
	projection := AccountProjection{}
	for _, tunnel := range tunnels {
		projection.ActiveRoutes++
		if tunnel.Access != "public" {
			projection.ProtectedRoutes++
		}
		if tunnel.Status != "healthy" {
			projection.DegradedRoutes++
			projection.RecentActivity = append(projection.RecentActivity, Activity{
				ID:      fmt.Sprintf("act-%s", tunnel.ID),
				Level:   levelForStatus(tunnel.Status),
				Message: fmt.Sprintf("%s is %s via %s", tunnel.Hostname, tunnel.Status, tunnel.Target),
			})
		}
	}
	if len(projection.RecentActivity) == 0 {
		projection.RecentActivity = []Activity{{
			ID:      fmt.Sprintf("act-%s-ready", account.ID),
			Level:   "info",
			Message: fmt.Sprintf("%s has no degraded routes right now", account.DisplayName),
		}}
	}
	return projection, nil
}

func (r *StubRepository) ProjectGlobal(ctx context.Context, tunnels []models.Tunnel, flags []models.ReviewFlag) (GlobalProjection, error) {
	_ = ctx
	projection := GlobalProjection{FlaggedExposures: len(flags)}
	for _, flag := range flags {
		projection.RecentActivity = append(projection.RecentActivity, Activity{
			ID:      flag.ID,
			Level:   severityLevel(flag.Severity),
			Message: fmt.Sprintf("Review %s: %s", flag.Item, flag.Reason),
		})
	}
	if len(projection.RecentActivity) == 0 {
		for _, tunnel := range tunnels {
			if tunnel.Access == "public" {
				projection.RecentActivity = append(projection.RecentActivity, Activity{
					ID:      fmt.Sprintf("public-%s", tunnel.ID),
					Level:   "warn",
					Message: fmt.Sprintf("Public route live: %s -> %s", tunnel.Hostname, tunnel.Target),
				})
				break
			}
		}
	}
	return projection, nil
}

func (r *StubRepository) VerifyInstallationToken(ctx context.Context, token string) (*models.RuntimeInstallationToken, error) {
	// Stub implementation - returns a valid token response
	_ = ctx
	_ = token
	return &models.RuntimeInstallationToken{
		ID:             "token_stub",
		InstallationID: "install_stub",
		Kind:           "enrollment",
	}, nil
}

func (r *StubRepository) CreateIngestToken(ctx context.Context, installationID string) (string, error) {
	// Stub implementation - returns a fake ingest token
	_ = ctx
	_ = installationID
	return "ingest_token_stub_" + installationID, nil
}

func levelForStatus(status string) string {
	switch status {
	case "healthy":
		return "info"
	case "guarded":
		return "warn"
	case "hot", "degraded", "down":
		return "critical"
	default:
		return "info"
	}
}

func severityLevel(severity string) string {
	switch severity {
	case "critical", "high", "elevated":
		return "critical"
	case "medium", "warn":
		return "warn"
	default:
		return "info"
	}
}
