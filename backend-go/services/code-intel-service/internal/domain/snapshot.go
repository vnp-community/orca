package domain

import "time"

// GraphModelVersion tracks the data shape version. Change this to invalidate all cache if shape changes.
const GraphModelVersion = "v1"

// SnapshotKey represents the unique identifier for a cached graph view.
type SnapshotKey struct {
	Tenant     string
	Binding    string
	View       ViewKind
	HeadCommit string
	ParamsHash string
}

// Snapshot represents a cached result.
type Snapshot struct {
	Key         SnapshotKey
	ContentETag string
	TotalCount  int
	Data        any
	ExpiresAt   time.Time
	CreatedAt   time.Time
}
