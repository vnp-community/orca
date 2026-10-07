package domain

type CodeIntelBinding struct {
	ID       string
	TenantID string
	RepoID   string
}

type ReindexStatus struct {
	ID     string
	Status string
}
