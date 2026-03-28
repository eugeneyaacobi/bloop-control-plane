package models

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
