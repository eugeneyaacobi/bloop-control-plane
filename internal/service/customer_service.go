package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/runtime"
	"github.com/jackc/pgx/v5"
)

var ErrTunnelAlreadyExists = errors.New("tunnel already exists")
var ErrTunnelNotFound = errors.New("tunnel not found")

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

type CustomerWorkspaceResponse struct {
	AccountName     string                    `json:"accountName"`
	TunnelSummary   string                    `json:"tunnelSummary"`
	Tunnels         []models.Tunnel           `json:"tunnels"`
	RecentActivity  []runtime.Activity        `json:"recentActivity,omitempty"`
	RuntimeSnapshot runtime.AccountProjection `json:"runtimeSnapshot"`
}

type CreateTunnelInput struct {
	Hostname string `json:"hostname"`
	Target   string `json:"target"`
	Access   string `json:"access"`
	Region   string `json:"region,omitempty"`
}

type UpdateTunnelInput struct {
	Target string `json:"target"`
	Access string `json:"access"`
	Region string `json:"region,omitempty"`
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

	return &CustomerWorkspaceResponse{
		AccountName:     account.DisplayName,
		TunnelSummary:   summaryString(projection.ActiveRoutes, projection.ProtectedRoutes, projection.DegradedRoutes),
		Tunnels:         tunnels,
		RecentActivity:  projection.RecentActivity,
		RuntimeSnapshot: projection,
	}, nil
}

func (s *CustomerService) ListTunnels(ctx context.Context, accountID string) ([]models.Tunnel, error) {
	return s.repo.ListTunnels(ctx, accountID)
}

func (s *CustomerService) GetTunnelByID(ctx context.Context, accountID, tunnelID string) (*models.Tunnel, error) {
	return s.repo.GetTunnelByID(ctx, accountID, tunnelID)
}

func (s *CustomerService) CreateTunnel(ctx context.Context, accountID string, input CreateTunnelInput) (*models.Tunnel, error) {
	access := strings.TrimSpace(input.Access)
	if access == "" {
		access = "token-protected"
	}
	status := "healthy"
	created, err := s.repo.CreateTunnel(ctx, accountID, repository.CreateTunnelParams{
		Hostname: strings.TrimSpace(input.Hostname),
		Target:   strings.TrimSpace(input.Target),
		Access:   access,
		Status:   status,
		Region:   strings.TrimSpace(input.Region),
	})
	if err != nil {
		if err.Error() == ErrTunnelAlreadyExists.Error() || err.Error() == "tunnel already exists" {
			return nil, ErrTunnelAlreadyExists
		}
		return nil, err
	}
	return created, nil
}

func (s *CustomerService) UpdateTunnel(ctx context.Context, accountID, tunnelID string, input UpdateTunnelInput) (*models.Tunnel, error) {
	updated, err := s.repo.UpdateTunnel(ctx, accountID, tunnelID, repository.UpdateTunnelParams{
		Target: strings.TrimSpace(input.Target),
		Access: strings.TrimSpace(input.Access),
		Region: strings.TrimSpace(input.Region),
	})
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrTunnelNotFound
	}
	return updated, nil
}

func (s *CustomerService) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	err := s.repo.DeleteTunnel(ctx, accountID, tunnelID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTunnelNotFound
		}
		return err
	}
	return nil
}

func summaryString(total, protected, degraded int) string {
	return strconv.Itoa(total) + " active routes / " + strconv.Itoa(protected) + " protected / " + strconv.Itoa(degraded) + " degraded"
}
