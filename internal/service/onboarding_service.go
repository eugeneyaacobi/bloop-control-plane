package service

import (
	"context"

	"bloop-control-plane/internal/repository"
)

type OnboardingService struct {
	repo repository.OnboardingRepository
}

func NewOnboardingService(repo repository.OnboardingRepository) *OnboardingService {
	return &OnboardingService{repo: repo}
}

func (s *OnboardingService) ListSteps(ctx context.Context, accountID string) ([]repository.OnboardingStep, error) {
	return s.repo.ListSteps(ctx, accountID)
}
