package repository

import (
	"context"

	"bloop-control-plane/internal/models"
)

type RuntimeOverlay struct {
	InstallationID   string
	InstallationName string
	Status           string
	Degraded         bool
	LastSeenAt       *string
	ObservedHostname string
	Drift            string
}

type CustomerRepository interface {
	GetWorkspace(ctx context.Context, accountID string) (models.Account, []models.Tunnel, error)
	ListTunnels(ctx context.Context, accountID string) ([]models.Tunnel, error)
	GetTunnelByID(ctx context.Context, accountID, tunnelID string) (*models.Tunnel, error)
	ListInstallations(ctx context.Context, accountID string) ([]models.RuntimeInstallation, error)
	GetRuntimeOverlayByTunnel(ctx context.Context, accountID, tunnelID string) (*RuntimeOverlay, error)
}

type InMemoryCustomerRepository struct {
	Account models.Account
	Tunnels []models.Tunnel
}

func NewInMemoryCustomerRepository() *InMemoryCustomerRepository {
	return &InMemoryCustomerRepository{
		Account: models.Account{ID: "acct_default", DisplayName: "Gene / default-org"},
		Tunnels: []models.Tunnel{
			{ID: "api", Hostname: "api.bloop.to", Target: "app-server:8080", Access: "token-protected", Status: "healthy", Region: "iad-1"},
			{ID: "admin", Hostname: "admin.bloop.to", Target: "backoffice:3000", Access: "basic-auth", Status: "guarded", Region: "iad-1"},
			{ID: "hooks", Hostname: "hooks.bloop.to", Target: "webhook-gateway:8787", Access: "public", Status: "hot", Region: "ord-1"},
		},
	}
}

func (r *InMemoryCustomerRepository) GetWorkspace(ctx context.Context, accountID string) (models.Account, []models.Tunnel, error) {
	return r.Account, r.Tunnels, nil
}

func (r *InMemoryCustomerRepository) ListTunnels(ctx context.Context, accountID string) ([]models.Tunnel, error) {
	return r.Tunnels, nil
}

func (r *InMemoryCustomerRepository) GetTunnelByID(ctx context.Context, accountID, tunnelID string) (*models.Tunnel, error) {
	for _, tunnel := range r.Tunnels {
		if tunnel.ID == tunnelID {
			t := tunnel
			return &t, nil
		}
	}
	return nil, nil
}
