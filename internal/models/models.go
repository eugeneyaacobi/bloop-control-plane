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
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Hostname  string    `json:"hostname"`
	Target    string    `json:"target"`
	Access    string    `json:"access"`
	Status    string    `json:"status"`
	Region    string    `json:"region,omitempty"`
	Owner     string    `json:"owner,omitempty"`
	Risk      string    `json:"risk,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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

// Tunnel management request/response types

type TunnelCreateRequest struct {
	ID        string `json:"id"`
	Hostname  string `json:"hostname"`
	Target    string `json:"target"`
	Access    string `json:"access"`
	BasicAuth *struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"basic_auth,omitempty"`
	TokenEnv string `json:"token_env,omitempty"`
}

type TunnelUpdateRequest struct {
	Hostname  *string `json:"hostname,omitempty"`
	Target    *string `json:"target,omitempty"`
	Access    *string `json:"access,omitempty"`
	BasicAuth *struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"basic_auth,omitempty"`
	TokenEnv *string `json:"token_env,omitempty"`
}

type TunnelValidationRequest struct {
	Hostname string `json:"hostname"`
	Target   string `json:"target"`
	Access   string `json:"access"`
	LocalPort *int  `json:"local_port,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type TunnelValidationResponse struct {
	Valid  bool         `json:"valid"`
	Errors []FieldError `json:"errors,omitempty"`
}

type TunnelStatusResponse struct {
	Status     string  `json:"status"`
	ObservedAt *string `json:"observed_at,omitempty"`
	Degraded   bool    `json:"degraded"`
	Stale      bool    `json:"stale"`
}

type AccessModeInfo struct {
	Mode        string `json:"mode"`
	Description string `json:"description"`
	Requires    []string `json:"requires,omitempty"`
}

type HostnamePattern struct {
	Pattern        string `json:"pattern"`
	Description    string `json:"description"`
	Example        string `json:"example"`
}

type ConfigSchemaResponse struct {
	AccessModes      []AccessModeInfo   `json:"access_modes"`
	HostnamePatterns []HostnamePattern `json:"hostname_patterns"`
	DefaultPorts     []int              `json:"default_ports"`
	DefaultRelayURL  string             `json:"default_relay_url"`
}

type EnrollmentVerifyRequest struct {
	Token string `json:"token"`
}

type EnrollmentVerifyResponse struct {
	Valid        bool   `json:"valid"`
	InstallationID string `json:"installation_id,omitempty"`
	IngestToken    string `json:"ingest_token,omitempty"`
	Error          string `json:"error,omitempty"`
}
