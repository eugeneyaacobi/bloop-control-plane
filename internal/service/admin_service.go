package service

import (
	"context"
	"strconv"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/runtime"
)

type AdminService struct {
	repo    repository.AdminRepository
	runtime runtime.Repository
}

func NewAdminService(repo repository.AdminRepository, runtimeRepo runtime.Repository) *AdminService {
	if runtimeRepo == nil {
		runtimeRepo = runtime.NewStubRepository()
	}
	return &AdminService{repo: repo, runtime: runtimeRepo}
}

type Stat struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type AdminOverviewResponse struct {
	OverviewStats   []Stat             `json:"overviewStats"`
	Exposures       []models.Tunnel    `json:"exposures"`
	RecentActivity  []runtime.Activity `json:"recentActivity,omitempty"`
	RuntimeSnapshot runtime.GlobalProjection `json:"runtimeSnapshot"`
}

func (s *AdminService) GetOverview(ctx context.Context) (*AdminOverviewResponse, error) {
	accounts, publicRoutes, flagged, err := s.repo.OverviewStats(ctx)
	if err != nil {
		return nil, err
	}
	runtimeInstallations, runtimeActive, runtimeRevoked, runtimeStale, err := s.repo.RuntimeInstallationStats(ctx)
	if err != nil {
		return nil, err
	}
	exposures, err := s.repo.ListTunnels(ctx)
	if err != nil {
		return nil, err
	}
	flags, err := s.repo.ListReviewFlags(ctx)
	if err != nil {
		return nil, err
	}
	projection, err := s.runtime.ProjectGlobal(ctx, exposures, flags)
	if err != nil {
		return nil, err
	}
	if projection.FlaggedExposures > flagged {
		flagged = projection.FlaggedExposures
	}
	return &AdminOverviewResponse{
		OverviewStats: []Stat{
			{Label: "Accounts", Value: strconv.Itoa(accounts)},
			{Label: "Open public routes", Value: strconv.Itoa(publicRoutes)},
			{Label: "Flagged exposures", Value: strconv.Itoa(flagged)},
			{Label: "Installations", Value: strconv.Itoa(runtimeInstallations)},
			{Label: "Active installs", Value: strconv.Itoa(runtimeActive)},
			{Label: "Revoked installs", Value: strconv.Itoa(runtimeRevoked)},
			{Label: "Stale heartbeats", Value: strconv.Itoa(runtimeStale)},
		},
		Exposures:       exposures,
		RecentActivity:  projection.RecentActivity,
		RuntimeSnapshot: projection,
	}, nil
}

func (s *AdminService) ListUsers(ctx context.Context) ([]models.User, error) {
	return s.repo.ListUsers(ctx)
}

func (s *AdminService) ListTunnels(ctx context.Context) ([]models.Tunnel, error) {
	return s.repo.ListTunnels(ctx)
}

func (s *AdminService) ListReviewQueue(ctx context.Context) ([]models.ReviewFlag, error) {
	return s.repo.ListReviewFlags(ctx)
}
