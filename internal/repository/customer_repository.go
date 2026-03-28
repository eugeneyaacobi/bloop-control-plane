package repository

import (
	"context"
	"strings"

	"bloop-control-plane/internal/models"
)

type CreateTunnelParams struct {
	Hostname string
	Target   string
	Access   string
	Status   string
	Region   string
	Owner    string
	Risk     string
}

type UpdateTunnelParams struct {
	Target string
	Access string
	Region string
}

type CustomerRepository interface {
	GetWorkspace(ctx context.Context, accountID string) (models.Account, []models.Tunnel, error)
	ListTunnels(ctx context.Context, accountID string) ([]models.Tunnel, error)
	GetTunnelByID(ctx context.Context, accountID, tunnelID string) (*models.Tunnel, error)
	CreateTunnel(ctx context.Context, accountID string, params CreateTunnelParams) (*models.Tunnel, error)
	UpdateTunnel(ctx context.Context, accountID, tunnelID string, params UpdateTunnelParams) (*models.Tunnel, error)
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

func (r *InMemoryCustomerRepository) CreateTunnel(ctx context.Context, accountID string, params CreateTunnelParams) (*models.Tunnel, error) {
	id := strings.NewReplacer(".", "-", ":", "-", "/", "-").Replace(params.Hostname)
	tunnel := models.Tunnel{
		ID:       id,
		Hostname: params.Hostname,
		Target:   params.Target,
		Access:   params.Access,
		Status:   params.Status,
		Region:   params.Region,
		Owner:    params.Owner,
		Risk:     params.Risk,
	}
	r.Tunnels = append(r.Tunnels, tunnel)
	return &tunnel, nil
}

func (r *InMemoryCustomerRepository) UpdateTunnel(ctx context.Context, accountID, tunnelID string, params UpdateTunnelParams) (*models.Tunnel, error) {
	for i, tunnel := range r.Tunnels {
		if tunnel.ID != tunnelID {
			continue
		}
		tunnel.Target = params.Target
		tunnel.Access = params.Access
		tunnel.Region = params.Region
		r.Tunnels[i] = tunnel
		return &tunnel, nil
	}
	return nil, nil
}
