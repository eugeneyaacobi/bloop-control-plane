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
	ListTunnels(ctx context.Context, accountID string) ([]models.Tunnel, error)
	GetTunnelByID(ctx context.Context, accountID, tunnelID string) (*models.Tunnel, error)
	CreateTunnel(ctx context.Context, actorID, accountID string, input service.CreateTunnelInput) (*models.Tunnel, error)
	UpdateTunnel(ctx context.Context, actorID, accountID, tunnelID string, input service.UpdateTunnelInput) (*models.Tunnel, error)
	DeleteTunnel(ctx context.Context, actorID, accountID, tunnelID string) error
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
