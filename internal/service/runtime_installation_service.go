package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/security"
)

type RuntimeInstallationService struct {
	repo  repository.RuntimeInstallationRepository
	nowFn func() time.Time
}

type CreateRuntimeInstallationResult struct {
	Installation         models.RuntimeInstallation `json:"installation"`
	EnrollmentToken      string                     `json:"enrollmentToken"`
	EnrollmentExpiresAt  time.Time                  `json:"enrollmentExpiresAt"`
}

type EnrollmentResult struct {
	Installation models.RuntimeInstallation `json:"installation"`
	IngestToken  string                     `json:"ingestToken"`
}

type RuntimeInstallationDetailResult struct {
	Installation models.RuntimeInstallation `json:"installation"`
	RecentEvents []map[string]any           `json:"recentEvents"`
}

type ResolvedRuntimePrincipal struct {
	InstallationID string
	AccountID      string
}

func NewRuntimeInstallationService(repo repository.RuntimeInstallationRepository) *RuntimeInstallationService {
	return &RuntimeInstallationService{repo: repo, nowFn: func() time.Time { return time.Now().UTC() }}
}

func (s *RuntimeInstallationService) CreateInstallation(ctx context.Context, accountID, name, environment string) (*CreateRuntimeInstallationResult, error) {
	name = strings.TrimSpace(name)
	if name == "" { return nil, errors.New("name is required") }
	now := s.nowFn()
	inst, err := s.repo.CreateInstallation(ctx, accountID, name, strings.TrimSpace(environment), now)
	if err != nil { return nil, err }
	expires := now.Add(15 * time.Minute)
	plain, err := security.GenerateOpaqueToken("bloop_enr_")
	if err != nil { return nil, err }
	if _, err := s.repo.CreateToken(ctx, inst.ID, "enrollment", security.HashToken(plain), &expires, now); err != nil { return nil, err }
	return &CreateRuntimeInstallationResult{Installation: inst, EnrollmentToken: plain, EnrollmentExpiresAt: expires}, nil
}

func (s *RuntimeInstallationService) ListInstallations(ctx context.Context, accountID string) ([]models.RuntimeInstallation, error) {
	return s.repo.ListInstallations(ctx, accountID)
}

func (s *RuntimeInstallationService) Enroll(ctx context.Context, enrollmentToken, clientName, clientVersion string) (*EnrollmentResult, error) {
	now := s.nowFn()
	tok, inst, err := s.repo.GetActiveTokenByHash(ctx, security.HashToken(strings.TrimSpace(enrollmentToken)), "enrollment", now)
	if err != nil { return nil, err }
	if tok == nil || inst == nil { return nil, errors.New("invalid enrollment token") }
	if err := s.repo.MarkTokenUsed(ctx, tok.ID, now); err != nil { return nil, err }
	if err := s.repo.RevokeToken(ctx, tok.ID, now); err != nil { return nil, err }
	ingestPlain, err := security.GenerateOpaqueToken("bloop_ing_")
	if err != nil { return nil, err }
	if _, err := s.repo.CreateToken(ctx, inst.ID, "ingest", security.HashToken(ingestPlain), nil, now); err != nil { return nil, err }
	if err := s.repo.UpdateInstallationStatus(ctx, inst.ID, "active", now); err != nil { return nil, err }
	inst.Status = "active"
	return &EnrollmentResult{Installation: *inst, IngestToken: ingestPlain}, nil
}

func (s *RuntimeInstallationService) ResolveIngestToken(ctx context.Context, token string) (*ResolvedRuntimePrincipal, error) {
	now := s.nowFn()
	tok, inst, err := s.repo.GetActiveTokenByHash(ctx, security.HashToken(strings.TrimSpace(token)), "ingest", now)
	if err != nil { return nil, err }
	if tok == nil || inst == nil { return nil, errors.New("invalid ingest token") }
	if err := s.repo.MarkTokenUsed(ctx, tok.ID, now); err != nil { return nil, err }
	if err := s.repo.UpdateInstallationLastSeen(ctx, inst.ID, now); err != nil { return nil, err }
	return &ResolvedRuntimePrincipal{InstallationID: inst.ID, AccountID: inst.AccountID}, nil
}

func (s *RuntimeInstallationService) GetInstallation(ctx context.Context, accountID, installationID string) (*RuntimeInstallationDetailResult, error) {
	inst, err := s.repo.GetInstallation(ctx, accountID, installationID)
	if err != nil || inst == nil { return nil, err }
	events, err := s.repo.ListInstallationEvents(ctx, accountID, installationID, 10)
	if err != nil { return nil, err }
	return &RuntimeInstallationDetailResult{Installation: *inst, RecentEvents: events}, nil
}

func (s *RuntimeInstallationService) RotateIngestToken(ctx context.Context, accountID, installationID string) (string, error) {
	inst, err := s.repo.GetInstallation(ctx, accountID, installationID)
	if err != nil || inst == nil { return "", errors.New("installation not found") }
	now := s.nowFn()
	if err := s.repo.RevokeActiveTokensByKind(ctx, installationID, "ingest", now); err != nil { return "", err }
	plain, err := security.GenerateOpaqueToken("bloop_ing_")
	if err != nil { return "", err }
	if _, err := s.repo.CreateToken(ctx, installationID, "ingest", security.HashToken(plain), nil, now); err != nil { return "", err }
	return plain, nil
}

func (s *RuntimeInstallationService) RevokeInstallation(ctx context.Context, accountID, installationID string) error {
	inst, err := s.repo.GetInstallation(ctx, accountID, installationID)
	if err != nil || inst == nil { return errors.New("installation not found") }
	now := s.nowFn()
	if err := s.repo.RevokeActiveTokensByKind(ctx, installationID, "ingest", now); err != nil { return err }
	if err := s.repo.RevokeActiveTokensByKind(ctx, installationID, "enrollment", now); err != nil { return err }
	return s.repo.UpdateInstallationStatus(ctx, installationID, "revoked", now)
}
