package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

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

// T007: CreateTunnel - creates a new tunnel with hostname normalization and uniqueness check
func (s *CustomerService) CreateTunnel(ctx context.Context, accountID string, req models.TunnelCreateRequest) (*models.Tunnel, error) {
	// Normalize hostname: lowercase and strip trailing dot
	hostname := strings.ToLower(strings.TrimSuffix(req.Hostname, "."))

	// Check hostname uniqueness globally
	existing, err := s.repo.GetTunnelByHostname(ctx, hostname)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, &ConflictError{Field: "hostname", Message: "hostname already claimed"}
	}

	// Validate access mode has required fields
	if req.Access == "basic_auth" && req.BasicAuth == nil {
		return nil, &ValidationError{Field: "basic_auth", Message: "basic_auth required when access is basic_auth"}
	}

	now := time.Now()
	tunnel := models.Tunnel{
		ID:        req.ID,
		AccountID: accountID,
		Hostname:  hostname,
		Target:    req.Target,
		Access:    req.Access,
		Status:    "pending",
		Region:    "",
		Owner:     "",
		Risk:      "",
		CreatedAt: now,
		UpdatedAt: now,
	}

	created, err := s.repo.CreateTunnel(ctx, accountID, tunnel)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// T008: UpdateTunnel - partially updates a tunnel with hostname uniqueness check if changed
func (s *CustomerService) UpdateTunnel(ctx context.Context, accountID, tunnelID string, req models.TunnelUpdateRequest) (*models.Tunnel, error) {
	// Get existing tunnel
	existing, err := s.repo.GetTunnelByID(ctx, accountID, tunnelID)
	if err != nil || existing == nil {
		return nil, &NotFoundError{Resource: "tunnel", ID: tunnelID}
	}

	// Build update with partial fields
	update := models.Tunnel{}
	updated := false

	if req.Hostname != nil {
		hostname := strings.ToLower(strings.TrimSuffix(*req.Hostname, "."))
		// Check uniqueness if hostname changed
		if hostname != existing.Hostname {
			conflict, err := s.repo.GetTunnelByHostname(ctx, hostname)
			if err != nil {
				return nil, err
			}
			if conflict != nil {
				return nil, &ConflictError{Field: "hostname", Message: "hostname already claimed"}
			}
		}
		update.Hostname = hostname
		updated = true
	}

	if req.Target != nil {
		update.Target = *req.Target
		updated = true
	}

	if req.Access != nil {
		// Validate access mode has required fields
		if *req.Access == "basic_auth" && req.BasicAuth == nil {
			return nil, &ValidationError{Field: "basic_auth", Message: "basic_auth required when access is basic_auth"}
		}
		update.Access = *req.Access
		updated = true
	}

	if !updated {
		return existing, nil
	}

	updatedTunnel, err := s.repo.UpdateTunnel(ctx, accountID, tunnelID, update)
	if err != nil {
		return nil, err
	}
	return updatedTunnel, nil
}

// T009: DeleteTunnel - deletes a tunnel after verifying ownership
func (s *CustomerService) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	// Verify ownership
	existing, err := s.repo.GetTunnelByID(ctx, accountID, tunnelID)
	if err != nil || existing == nil {
		return &NotFoundError{Resource: "tunnel", ID: tunnelID}
	}

	return s.repo.DeleteTunnel(ctx, accountID, tunnelID)
}

// T010: ValidateTunnel - validates tunnel config with field-level errors
func (s *CustomerService) ValidateTunnel(ctx context.Context, accountID string, req models.TunnelValidationRequest) (*models.TunnelValidationResponse, error) {
	var errors []models.FieldError

	// Validate hostname format
	if req.Hostname == "" {
		errors = append(errors, models.FieldError{Field: "hostname", Message: "hostname is required"})
	} else {
		hostname := strings.ToLower(strings.TrimSuffix(req.Hostname, "."))
		hostnamePattern := regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)
		if !hostnamePattern.MatchString(hostname) {
			errors = append(errors, models.FieldError{Field: "hostname", Message: "invalid hostname format"})
		} else {
			// Check hostname availability
			existing, _ := s.repo.GetTunnelByHostname(ctx, hostname)
			if existing != nil && existing.AccountID != accountID {
				errors = append(errors, models.FieldError{Field: "hostname", Message: "hostname already claimed"})
			}
		}
	}

	// Validate target
	if req.Target == "" {
		errors = append(errors, models.FieldError{Field: "target", Message: "target is required"})
	}

	// Validate port range
	if req.LocalPort != nil {
		if *req.LocalPort < 1 || *req.LocalPort > 65535 {
			errors = append(errors, models.FieldError{Field: "local_port", Message: "port must be between 1 and 65535"})
		}
	}

	// Validate access mode
	validAccessModes := map[string]bool{"public": true, "basic_auth": true, "token_protected": true}
	if req.Access != "" && !validAccessModes[req.Access] {
		errors = append(errors, models.FieldError{Field: "access", Message: "invalid access mode"})
	}

	valid := len(errors) == 0
	return &models.TunnelValidationResponse{Valid: valid, Errors: errors}, nil
}

// T011: GetTunnelStatus - returns runtime status from snapshot table
func (s *CustomerService) GetTunnelStatus(ctx context.Context, accountID, tunnelID string) (*models.TunnelStatusResponse, error) {
	status, degraded, observedAt, err := s.repo.GetRuntimeStatusByTunnelID(ctx, accountID, tunnelID)
	if err != nil {
		return nil, err
	}

	resp := &models.TunnelStatusResponse{
		Status:   status,
		Degraded: degraded,
		Stale:    status == "",
	}

	if observedAt != nil {
		formatted := observedAt.Format(time.RFC3339)
		resp.ObservedAt = &formatted
	}

	if resp.Status == "" {
		resp.Status = "unknown"
	}

	return resp, nil
}

// T012: GetConfigSchema - returns static config defaults
func (s *CustomerService) GetConfigSchema(ctx context.Context) (*models.ConfigSchemaResponse, error) {
	return &models.ConfigSchemaResponse{
		AccessModes: []models.AccessModeInfo{
			{Mode: "public", Description: "Publicly accessible without authentication"},
			{Mode: "basic_auth", Description: "Protected with username and password", Requires: []string{"username", "password"}},
			{Mode: "token_protected", Description: "Protected with bearer token", Requires: []string{"token_env"}},
		},
		HostnamePatterns: []models.HostnamePattern{
			{Pattern: "*.bloop.to", Description: "Bloop subdomain", Example: "api.bloop.to"},
			{Pattern: "custom.*", Description: "Custom domain", Example: "app.example.com"},
		},
		DefaultPorts:    []int{80, 443, 3000, 5000, 8000, 8080},
		DefaultRelayURL: "https://relay.bloop.to",
	}, nil
}

// T013: VerifyEnrollment - validates enrollment token and returns installation credentials
func (s *CustomerService) VerifyEnrollment(ctx context.Context, token string) (*models.EnrollmentVerifyResponse, error) {
	if token == "" {
		return &models.EnrollmentVerifyResponse{Valid: false, Error: "token is required"}, nil
	}

	// Query runtime_installation_tokens via runtime repository
	installationToken, err := s.runtime.VerifyInstallationToken(ctx, token)
	if err != nil || installationToken == nil {
		return &models.EnrollmentVerifyResponse{Valid: false, Error: "invalid_or_expired"}, nil
	}

	// Generate ingest token
	ingestToken, err := s.runtime.CreateIngestToken(ctx, installationToken.InstallationID)
	if err != nil {
		return nil, err
	}

	return &models.EnrollmentVerifyResponse{
		Valid:         true,
		InstallationID: installationToken.InstallationID,
		IngestToken:    ingestToken,
	}, nil
}

// Error types

type ConflictError struct {
	Field   string
	Message string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict on %s: %s", e.Field, e.Message)
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}

type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %s not found", e.Resource, e.ID)
}

// IsConflict checks if error is a conflict error
func IsConflict(err error) bool {
	var conflict *ConflictError
	return errors.As(err, &conflict)
}

// IsValidation checks if error is a validation error
func IsValidation(err error) bool {
	var validation *ValidationError
	return errors.As(err, &validation)
}

// IsNotFound checks if error is a not found error
func IsNotFound(err error) bool {
	var notFound *NotFoundError
	return errors.As(err, &notFound)
}
