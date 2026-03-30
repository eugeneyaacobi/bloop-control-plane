package customer

import (
	"context"
	"net/http"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/models"
	"bloop-control-plane/internal/service"
)

type Handler struct {
	Service CustomerWorkspaceService
}

type CustomerWorkspaceService interface {
	GetWorkspace(ctx context.Context, accountID string) (*service.CustomerWorkspaceResponse, error)
	ListTunnels(ctx context.Context, accountID string) ([]service.CustomerTunnelResponse, error)
	GetTunnelByID(ctx context.Context, accountID, tunnelID string) (*service.CustomerTunnelResponse, error)
	CreateTunnel(ctx context.Context, accountID string, req models.TunnelCreateRequest) (*models.Tunnel, error)
	UpdateTunnel(ctx context.Context, accountID, tunnelID string, req models.TunnelUpdateRequest) (*models.Tunnel, error)
	DeleteTunnel(ctx context.Context, accountID, tunnelID string) error
	ValidateTunnel(ctx context.Context, accountID string, req models.TunnelValidationRequest) (*models.TunnelValidationResponse, error)
	GetTunnelStatus(ctx context.Context, accountID, tunnelID string) (*models.TunnelStatusResponse, error)
	GetConfigSchema(ctx context.Context) (*models.ConfigSchemaResponse, error)
	VerifyEnrollment(ctx context.Context, token string) (*models.EnrollmentVerifyResponse, error)
}

func (h *Handler) Workspace(w http.ResponseWriter, r *http.Request) {
	sess, ok := authz.RequireSession(w, r)
	if !ok {
		return
	}

	resp, err := h.Service.GetWorkspace(r.Context(), sess.AccountID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	authz.WriteJSON(w, http.StatusOK, resp)
}
