package service

import (
	"context"
	"strconv"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/runtime"
)

type CustomerService struct {
	repo    repository.CustomerRepository
	runtime runtime.Repository
}

func NewCustomerService(repo repository.CustomerRepository, runtimeRepo runtime.Repository) *CustomerService {
	if runtimeRepo == nil {
		runtimeRepo = runtime.NewStubRepository()
	}
	return &CustomerService{repo: repo, runtime: runtimeRepo}
}

type CustomerTunnelResponse struct {
	models.Tunnel
	Runtime *repository.RuntimeOverlay `json:"runtime,omitempty"`
}

type CustomerWorkspaceResponse struct {
	AccountName     string                  `json:"accountName"`
	TunnelSummary   string                  `json:"tunnelSummary"`
	Tunnels         []CustomerTunnelResponse `json:"tunnels"`
	Installations   []models.RuntimeInstallation `json:"installations,omitempty"`
	RecentActivity  []runtime.Activity      `json:"recentActivity,omitempty"`
	RuntimeSnapshot runtime.AccountProjection `json:"runtimeSnapshot"`
}

func (s *CustomerService) GetWorkspace(ctx context.Context, accountID string) (*CustomerWorkspaceResponse, error) {
	account, tunnels, err := s.repo.GetWorkspace(ctx, accountID)
	if err != nil {
		return nil, err
	}
	projection, err := s.runtime.ProjectAccount(ctx, account, tunnels)
	if err != nil {
		return nil, err
	}

	installations, err := s.repo.ListInstallations(ctx, accountID)
	if err != nil { return nil, err }
	merged := make([]CustomerTunnelResponse, 0, len(tunnels))
	for _, tunnel := range tunnels {
		overlay, _ := s.repo.GetRuntimeOverlayByTunnel(ctx, accountID, tunnel.ID)
		merged = append(merged, CustomerTunnelResponse{Tunnel: tunnel, Runtime: overlay})
	}
	return &CustomerWorkspaceResponse{
		AccountName:     account.DisplayName,
		TunnelSummary:   summaryString(projection.ActiveRoutes, projection.ProtectedRoutes, projection.DegradedRoutes),
		Tunnels:         merged,
		Installations:   installations,
		RecentActivity:  projection.RecentActivity,
		RuntimeSnapshot: projection,
	}, nil
}

func (s *CustomerService) ListTunnels(ctx context.Context, accountID string) ([]CustomerTunnelResponse, error) {
	tunnels, err := s.repo.ListTunnels(ctx, accountID)
	if err != nil { return nil, err }
	merged := make([]CustomerTunnelResponse, 0, len(tunnels))
	for _, tunnel := range tunnels {
		overlay, _ := s.repo.GetRuntimeOverlayByTunnel(ctx, accountID, tunnel.ID)
		merged = append(merged, CustomerTunnelResponse{Tunnel: tunnel, Runtime: overlay})
	}
	return merged, nil
}

func (s *CustomerService) GetTunnelByID(ctx context.Context, accountID, tunnelID string) (*CustomerTunnelResponse, error) {
	tunnel, err := s.repo.GetTunnelByID(ctx, accountID, tunnelID)
	if err != nil || tunnel == nil { return nil, err }
	overlay, _ := s.repo.GetRuntimeOverlayByTunnel(ctx, accountID, tunnelID)
	return &CustomerTunnelResponse{Tunnel: *tunnel, Runtime: overlay}, nil
}

func summaryString(total, protected, degraded int) string {
	return strconv.Itoa(total) + " active routes / " + strconv.Itoa(protected) + " protected / " + strconv.Itoa(degraded) + " degraded"
}
