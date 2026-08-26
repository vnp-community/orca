package domain

import "time"

// Viewer is who a Connect/TestConnection call authenticated as — the
// provider account backing a connection.
type Viewer struct {
	ID          string
	DisplayName string
	Email       string
}

// Workspace is a Jira "site" or Linear "workspace" — the provider-agnostic
// name issue-tracking-service.md §3 unifies both under.
type Workspace struct {
	ID   string
	Name string
	URL  string // Jira site base URL; empty for Linear
}

// Connection is one connected (tenant, user, provider, workspace) row —
// issuetracking_connections' domain shape. A user may have more than one
// connected Workspace per provider (multi-site Jira); each gets its own
// row and its own CredentialID.
type Connection struct {
	TenantID        string
	UserID          string
	Provider        Provider
	Workspace       Workspace
	Viewer          Viewer
	CredentialID    string
	IsSelected      bool
	CredentialError string // set when CredentialID no longer resolves (revoked/decrypt failure)
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ConnectionStatus is GetConnectionStatus's return shape — every connected
// workspace for (tenant, user, provider), plus which one is selected.
type ConnectionStatus struct {
	Connected           bool
	Viewer              Viewer
	Workspaces          []Workspace
	ActiveWorkspaceID   string
	SelectedWorkspaceID string // "" | "all" | a specific workspace id
	CredentialError     string
}
