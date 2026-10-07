package devserveragent

import (
	"encoding/json"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const maxPayloadJSONBytes = 64 * 1024 // 64 KiB

type indexChangedParams struct {
	WorkspaceRoot string `json:"workspaceRoot"`
	Tool          string `json:"tool"`
	Commit        string `json:"commit"`
	IndexedAt     string `json:"indexedAt"`
	Reason        string `json:"reason"`
	HeadCommit    string `json:"headCommit"`
	Stale         bool   `json:"stale"`
	IndexScope    string `json:"indexScope"`
	MergeBase     string `json:"mergeBase"`
	Trigger       string `json:"trigger"`
}

type reindexProgressParams struct {
	WorkspaceRoot string `json:"workspaceRoot"`
	JobID         string `json:"jobId"`
	Tool          string `json:"tool"`
	Stage         string `json:"stage"`
	Percent       *int32 `json:"percent"`
	Message       string `json:"message"`
	Outcome       string `json:"outcome"`
	ErrorCode     string `json:"errorCode"`
}

type qualityProgressParams struct {
	WorkspaceRoot string `json:"workspaceRoot"`
	RunID         string `json:"runId"`
	Stage         string `json:"stage"`
	Percent       *int32 `json:"percent"`
	Message       string `json:"message"`
}

type qualityFinishedParams struct {
	WorkspaceRoot string `json:"workspaceRoot"`
	RunID         string `json:"runId"`
	HeadCommit    string `json:"headCommit"`
	ErrorCode     string `json:"errorCode"`
}

// decodeCodeIntelNotification decodes incoming JSON-RPC notifications for codeintel and quality methods.
// Returns (event, true) on successful decode, or (event, false) if required fields are missing or invalid.
func decodeCodeIntelNotification(n JSONRPCNotification, now time.Time) (domain.CodeIntelEvent, bool) {
	var payloadJSON string
	if len(n.Params) > 0 && len(n.Params) <= maxPayloadJSONBytes {
		payloadJSON = string(n.Params)
	}

	switch n.Method {
	case "codeintel.indexChanged":
		var p indexChangedParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return domain.CodeIntelEvent{}, false
		}
		if p.WorkspaceRoot == "" {
			return domain.CodeIntelEvent{}, false
		}
		return domain.CodeIntelEvent{
			Kind:          domain.CodeIntelEventKindIndexChanged,
			WorkspaceRoot: p.WorkspaceRoot,
			Tool:          p.Tool,
			Commit:        p.Commit,
			IndexedAt:     p.IndexedAt,
			Reason:        p.Reason,
			HeadCommit:    p.HeadCommit,
			Stale:         p.Stale,
			IndexScope:    p.IndexScope,
			MergeBase:     p.MergeBase,
			Trigger:       p.Trigger,
			ReceivedAt:    now,
			PayloadJSON:   payloadJSON,
		}, true

	case "codeintel.reindexProgress":
		var p reindexProgressParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return domain.CodeIntelEvent{}, false
		}
		if p.WorkspaceRoot == "" || p.JobID == "" {
			return domain.CodeIntelEvent{}, false
		}
		return domain.CodeIntelEvent{
			Kind:          domain.CodeIntelEventKindReindexProgress,
			WorkspaceRoot: p.WorkspaceRoot,
			JobID:         p.JobID,
			Tool:          p.Tool,
			Stage:         p.Stage,
			Percent:       p.Percent,
			Message:       p.Message,
			Outcome:       p.Outcome,
			ErrorCode:     p.ErrorCode,
			ReceivedAt:    now,
			PayloadJSON:   payloadJSON,
		}, true

	case "quality.progress":
		var p qualityProgressParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return domain.CodeIntelEvent{}, false
		}
		if p.WorkspaceRoot == "" || p.RunID == "" {
			return domain.CodeIntelEvent{}, false
		}
		return domain.CodeIntelEvent{
			Kind:          domain.CodeIntelEventKindQualityProgress,
			WorkspaceRoot: p.WorkspaceRoot,
			RunID:         p.RunID,
			Stage:         p.Stage,
			Percent:       p.Percent,
			Message:       p.Message,
			ReceivedAt:    now,
			PayloadJSON:   payloadJSON,
		}, true

	case "quality.finished":
		var p qualityFinishedParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return domain.CodeIntelEvent{}, false
		}
		if p.WorkspaceRoot == "" || p.RunID == "" {
			return domain.CodeIntelEvent{}, false
		}
		return domain.CodeIntelEvent{
			Kind:          domain.CodeIntelEventKindQualityFinished,
			WorkspaceRoot: p.WorkspaceRoot,
			RunID:         p.RunID,
			HeadCommit:    p.HeadCommit,
			ErrorCode:     p.ErrorCode,
			ReceivedAt:    now,
			PayloadJSON:   payloadJSON,
		}, true

	default:
		return domain.CodeIntelEvent{}, false
	}
}
