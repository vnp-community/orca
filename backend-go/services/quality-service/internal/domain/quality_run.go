package domain

type QualityRun struct {
	ID       string
	TenantID string
	RepoID   string
}

type Finding struct {
	ID       string
	Message  string
	Severity string
}
