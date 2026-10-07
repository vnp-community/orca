package domain

import "time"

type IndexState int

const (
	IndexStateUnspecified IndexState = iota
	IndexStateMissing
	IndexStateBuilding
	IndexStateReady
	IndexStateStale
	IndexStateUnknown
)

type IndexScope int

const (
	IndexScopeUnspecified IndexScope = iota
	IndexScopeExact
	IndexScopeRepoRoot
	IndexScopeStale
	IndexScopeNone
)

type Freshness int

const (
	FreshnessUnspecified Freshness = iota
	FreshnessFresh
	FreshnessFreshBase
	FreshnessStale
	FreshnessUnknown
)

type IndexStats struct {
	Files       int64
	Nodes       int64
	Edges       int64
	Communities int64
	Processes   int64
}

type PendingChanges struct {
	Added    int32
	Modified int32
	Removed  int32
}

type ToolIndexStatus struct {
	Tool                    string
	Version                 string
	Repo                    string
	State                   IndexState
	IndexedCommit           string
	HeadCommit              string
	IndexedAt               time.Time
	Stats                   IndexStats
	PendingChanges          *PendingChanges
	Languages               []string
	Available               bool
	Supported               bool
	MergeBase               string
	IndexScope              IndexScope
	Freshness               Freshness
	DirtySinceIndex         bool
	ChangedFilesNotInIndex  []string
	Indicators              map[string]string
}
