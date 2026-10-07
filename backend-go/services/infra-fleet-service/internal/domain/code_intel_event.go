package domain

import "time"

const (
	CodeIntelEventKindIndexChanged    = "index_changed"
	CodeIntelEventKindReindexProgress = "reindex_progress"
	CodeIntelEventKindQualityProgress = "quality_progress"
	CodeIntelEventKindQualityFinished = "quality_finished"
	CodeIntelEventKindResync          = "resync"
	CodeIntelEventKindOverflow        = "overflow"
)

// CodeIntelEvent represents an event received from an agent or synthesized by the fleet transport.
// Covers index_changed, reindex_progress, quality_progress, quality_finished, resync, overflow.
type CodeIntelEvent struct {
	Kind          string
	WorkspaceRoot string
	Tool          string
	Commit        string
	IndexedAt     string
	JobID         string
	Stage         string
	Percent       *int32
	Message       string
	ReceivedAt    time.Time
	Reason        string
	HeadCommit    string
	Stale         bool
	IndexScope    string
	MergeBase     string
	Trigger       string
	Outcome       string
	ErrorCode     string
	RunID         string
	PayloadJSON   string
}
