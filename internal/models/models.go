package models

import "time"

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type Account struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type Membership struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	AccountID string `json:"accountId"`
	Role      string `json:"role"`
}

type Tunnel struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	Target   string `json:"target"`
	Access   string `json:"access"`
	Status   string `json:"status"`
	Region   string `json:"region,omitempty"`
	Owner    string `json:"owner,omitempty"`
	Risk     string `json:"risk,omitempty"`
}

type ReviewFlag struct {
	ID       string `json:"id"`
	Item     string `json:"item"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
}

type SignupRequest struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	State string `json:"state"`
}

type RuntimeInstallation struct {
	ID          string     `json:"id"`
	AccountID   string     `json:"accountId"`
	Name        string     `json:"name"`
	Environment string     `json:"environment,omitempty"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	LastSeenAt  *time.Time `json:"lastSeenAt,omitempty"`
}

type RuntimeInstallationToken struct {
	ID             string     `json:"id"`
	InstallationID string     `json:"installationId"`
	Kind           string     `json:"kind"`
	TokenHash      string     `json:"-"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	RevokedAt      *time.Time `json:"revokedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	LastUsedAt     *time.Time `json:"lastUsedAt,omitempty"`
}

type RuntimeTunnelBinding struct {
	ID                string    `json:"id"`
	AccountID         string    `json:"accountId"`
	InstallationID    string    `json:"installationId"`
	TunnelID          string    `json:"tunnelId"`
	RuntimeTunnelName string    `json:"runtimeTunnelName"`
	Hostname          string    `json:"hostname"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}
