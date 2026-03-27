package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"bloop-control-plane/internal/api/authz"
	"bloop-control-plane/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Pool                 *pgxpool.Pool
	IngestSecret         string
	PrototypeMode        bool
	PrototypeAccountID   string
	RuntimeInstallations *service.RuntimeInstallationService
}

type SnapshotPayload struct {
	CapturedAt string `json:"capturedAt"`
	Tunnels    []struct {
		TunnelID   string `json:"tunnelId"`
		AccountID  string `json:"accountId"`
		Hostname   string `json:"hostname"`
		AccessMode string `json:"accessMode"`
		Status     string `json:"status"`
		Degraded   bool   `json:"degraded"`
		ObservedAt string `json:"observedAt"`
	} `json:"tunnels"`
	Events []struct {
		ID         string `json:"id"`
		AccountID  string `json:"accountId"`
		TunnelID   string `json:"tunnelId"`
		Hostname   string `json:"hostname"`
		Kind       string `json:"kind"`
		Level      string `json:"level"`
		Message    string `json:"message"`
		OccurredAt string `json:"occurredAt"`
	} `json:"events"`
}

func (h *Handler) IngestSnapshot(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	var payload SnapshotPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	for _, tunnel := range payload.Tunnels {
		accountID := principal.AccountID
		installationID := principal.InstallationID
		if accountID == "" {
			accountID = strings.TrimSpace(tunnel.AccountID)
			if normalized, ok := h.normalizeAccountID(ctx, accountID); ok {
				accountID = normalized
			}
		}
		observedAt, err := time.Parse(time.RFC3339, tunnel.ObservedAt)
		if err != nil {
			observedAt = time.Now().UTC()
		}
		_, err = h.Pool.Exec(ctx, `
			INSERT INTO runtime_tunnel_snapshots (id, tunnel_id, account_id, installation_id, hostname, access_mode, status, degraded, observed_at)
			VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8, $9)
			ON CONFLICT (id) DO UPDATE SET
				tunnel_id = EXCLUDED.tunnel_id,
				account_id = EXCLUDED.account_id,
				installation_id = EXCLUDED.installation_id,
				hostname = EXCLUDED.hostname,
				access_mode = EXCLUDED.access_mode,
				status = EXCLUDED.status,
				degraded = EXCLUDED.degraded,
				observed_at = EXCLUDED.observed_at`,
			"rts_"+tunnel.TunnelID, tunnel.TunnelID, accountID, installationID, tunnel.Hostname, tunnel.AccessMode, tunnel.Status, tunnel.Degraded, observedAt,
		)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
	for _, event := range payload.Events {
		accountID := principal.AccountID
		installationID := principal.InstallationID
		if accountID == "" {
			accountID = strings.TrimSpace(event.AccountID)
			if normalized, ok := h.normalizeAccountID(ctx, accountID); ok {
				accountID = normalized
			}
		}
		occurredAt, err := time.Parse(time.RFC3339, event.OccurredAt)
		if err != nil {
			occurredAt = time.Now().UTC()
		}
		_, err = h.Pool.Exec(ctx, `
			INSERT INTO runtime_events (id, account_id, installation_id, tunnel_id, hostname, kind, level, message, occurred_at)
			VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8, $9)
			ON CONFLICT (id) DO UPDATE SET
				account_id = EXCLUDED.account_id,
				installation_id = EXCLUDED.installation_id,
				tunnel_id = EXCLUDED.tunnel_id,
				hostname = EXCLUDED.hostname,
				kind = EXCLUDED.kind,
				level = EXCLUDED.level,
				message = EXCLUDED.message,
				occurred_at = EXCLUDED.occurred_at`,
			event.ID, accountID, installationID, event.TunnelID, event.Hostname, event.Kind, event.Level, event.Message, occurredAt,
		)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
	authz.WriteJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "tunnels": len(payload.Tunnels), "events": len(payload.Events)})
}

type ingestPrincipal struct {
	AccountID      string
	InstallationID string
}

func (h *Handler) resolvePrincipal(r *http.Request) (ingestPrincipal, bool) {
	authzHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authzHeader), "bearer ") && h.RuntimeInstallations != nil {
		token := strings.TrimSpace(authzHeader[7:])
		principal, err := h.RuntimeInstallations.ResolveIngestToken(r.Context(), token)
		if err == nil && principal != nil {
			return ingestPrincipal{AccountID: principal.AccountID, InstallationID: principal.InstallationID}, true
		}
	}
	if strings.TrimSpace(h.IngestSecret) != "" && r.Header.Get("X-Bloop-Runtime-Secret") == h.IngestSecret {
		return ingestPrincipal{}, true
	}
	return ingestPrincipal{}, false
}

func (h *Handler) normalizeAccountID(ctx context.Context, accountID string) (string, bool) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		if h.PrototypeMode && strings.TrimSpace(h.PrototypeAccountID) != "" {
			return h.PrototypeAccountID, true
		}
		return "", false
	}
	var exists bool
	if err := h.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1)`, accountID).Scan(&exists); err == nil && exists {
		return accountID, true
	}
	if h.PrototypeMode && strings.TrimSpace(h.PrototypeAccountID) != "" {
		return h.PrototypeAccountID, true
	}
	return accountID, false
}
