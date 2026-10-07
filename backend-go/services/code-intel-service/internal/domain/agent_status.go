package domain

import "time"

// RootMismatch represents mismatch between worktree root and index root per PQ-19.
type RootMismatch struct {
	WorktreeRoot string `json:"worktreeRoot"`
	IndexRoot    string `json:"indexRoot"`
}

// BindingStatus mirrors the binding section of codeintel.status.
type BindingStatus struct {
	WorkspaceRoot    string            `json:"workspaceRoot"`
	RepoRoot         string            `json:"repoRoot"`
	LinkedWorktree   bool              `json:"linkedWorktree"`
	WorktreeMismatch bool              `json:"worktreeMismatch"` // boolean per PQ-19
	GitNexus         *GitNexusBinding  `json:"gitnexus,omitempty"`
	CodeGraph        *CodeGraphBinding `json:"codegraph,omitempty"`
}

type GitNexusBinding struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	StoragePath string `json:"storagePath"`
}

type CodeGraphBinding struct {
	ProjectPath string `json:"projectPath"`
	HasDatabase bool   `json:"hasDatabase"`
}

type ToolAvailability struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Supported bool   `json:"supported"`
	Binary    string `json:"binary"`
}

type GitNexusStats struct {
	Files       int `json:"files"`
	Nodes       int `json:"nodes"`
	Edges       int `json:"edges"`
	Communities int `json:"communities"`
	Processes   int `json:"processes"`
}

type GitNexusIndexStatus struct {
	State                  string         `json:"state"` // missing|building|ready|stale|unknown
	IndexedCommit          string         `json:"indexedCommit"`
	IndexedAt              *time.Time     `json:"indexedAt,omitempty"`
	Branch                 string         `json:"branch"`
	Stats                  *GitNexusStats `json:"stats,omitempty"`
	SchemaVersion          int            `json:"schemaVersion"`
	StoragePath            string         `json:"storagePath"`
	Indicators             []string       `json:"indicators,omitempty"`
	IndexRoot              string         `json:"indexRoot"`
	IndexScope             string         `json:"indexScope"` // exact|repo_root|stale|none
	Freshness              string         `json:"freshness"`  // fresh|fresh_base|stale|unknown
	HeadCommit             string         `json:"headCommit"`
	MergeBase              string         `json:"mergeBase"`
	DirtySinceIndex        bool           `json:"dirtySinceIndex"`
	ChangedFilesNotInIndex int            `json:"changedFilesNotInIndex"`
	PendingChanges         *int           `json:"pendingChanges,omitempty"`
}

type CodeGraphPendingChanges struct {
	Added    int `json:"added"`
	Modified int `json:"modified"`
	Removed  int `json:"removed"`
}

type CodeGraphStats struct {
	Files int `json:"files"`
	Nodes int `json:"nodes"`
	Edges int `json:"edges"`
}

type CodeGraphIndexStatus struct {
	State                  string                   `json:"state"`
	IndexedAt              *time.Time               `json:"indexedAt,omitempty"`
	Stats                  *CodeGraphStats          `json:"stats,omitempty"`
	PendingChanges         *CodeGraphPendingChanges `json:"pendingChanges,omitempty"`
	Backend                string                   `json:"backend"`
	JournalMode            string                   `json:"journalMode"`
	DBSizeBytes            int64                    `json:"dbSizeBytes"`
	ExtractionVersion      int                      `json:"extractionVersion"`
	ReindexRecommended     bool                     `json:"reindexRecommended"`
	RootMismatch           *RootMismatch            `json:"rootMismatch,omitempty"` // object or null per PQ-19
	IndexRoot              string                   `json:"indexRoot"`
	IndexScope             string                   `json:"indexScope"`
	Freshness              string                   `json:"freshness"`
	HeadCommit             string                   `json:"headCommit"`
	MergeBase              string                   `json:"mergeBase"`
	DirtySinceIndex        bool                     `json:"dirtySinceIndex"`
	ChangedFilesNotInIndex int                      `json:"changedFilesNotInIndex"`
}

type CompatibilityStatus struct {
	Status  string  `json:"status"` // verified | untested | incompatible
	Reason  *string `json:"reason,omitempty"`
	Details *string `json:"details,omitempty"`
}

type HostStatus struct {
	Platform     string  `json:"platform"`
	Cores        int     `json:"cores"`
	LoadAvg1     float64 `json:"loadavg1"`
	FreeMemBytes int64   `json:"freeMemBytes"`
}

type LimitsStatus struct {
	ToolTimeoutMs      int `json:"toolTimeoutMs"`
	MethodTimeoutMs    int `json:"methodTimeoutMs"`
	ToolMaxOutputBytes int `json:"toolMaxOutputBytes"`
	ResultMaxBytes     int `json:"resultMaxBytes"`
	MaxConcurrentTools int `json:"maxConcurrentTools"`
}

type AgentStatusIndexes struct {
	GitNexus  *GitNexusIndexStatus  `json:"gitnexus,omitempty"`
	CodeGraph *CodeGraphIndexStatus `json:"codegraph,omitempty"`
}

// AgentStatusData mirrors the data field of a codeintel.status result (§4.1).
type AgentStatusData struct {
	Binding             BindingStatus               `json:"binding"`
	Tools               map[string]ToolAvailability `json:"tools"`
	Indexes             AgentStatusIndexes          `json:"indexes"`
	Compatibility       *CompatibilityStatus        `json:"compatibility,omitempty"`
	Warnings            []string                    `json:"warnings,omitempty"`
	SQLiteReadAvailable bool                        `json:"sqliteReadAvailable"`
	Host                *HostStatus                 `json:"host,omitempty"`
	Limits              *LimitsStatus               `json:"limits,omitempty"`
}

// HasWorktreeMismatch reports whether the binding is attached to a linked worktree with mismatch (PQ-19).
func (s *AgentStatusData) HasWorktreeMismatch() bool {
	return s.Binding.WorktreeMismatch
}

// HasRootMismatch reports whether CodeGraph index root diverges from worktree root (PQ-19).
func (s *AgentStatusData) HasRootMismatch() bool {
	return s.Indexes.CodeGraph != nil && s.Indexes.CodeGraph.RootMismatch != nil
}

// IsScopeMismatch reports whether any tool has indexScope == "repo_root" or binding has worktreeMismatch.
func (s *AgentStatusData) IsScopeMismatch() bool {
	if s.Binding.WorktreeMismatch {
		return true
	}
	if s.Indexes.GitNexus != nil && s.Indexes.GitNexus.IndexScope == "repo_root" {
		return true
	}
	if s.Indexes.CodeGraph != nil && s.Indexes.CodeGraph.IndexScope == "repo_root" {
		return true
	}
	return false
}
