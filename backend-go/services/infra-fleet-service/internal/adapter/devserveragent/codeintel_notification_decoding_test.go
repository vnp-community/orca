package devserveragent

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestDecodeCodeIntelNotification_IndexChanged(t *testing.T) {
	now := time.Now()
	rawParams := `{
		"workspaceRoot": "/work/orca",
		"tool": "gitnexus",
		"commit": "sha-1",
		"indexedAt": "2026-10-06T10:00:00Z",
		"reason": "head_changed",
		"headCommit": "sha-2",
		"stale": true,
		"indexScope": "repo",
		"mergeBase": "sha-base",
		"trigger": "watch"
	}`
	notif := JSONRPCNotification{
		Method: "codeintel.indexChanged",
		Params: json.RawMessage(rawParams),
	}

	ev, ok := decodeCodeIntelNotification(notif, now)
	if !ok {
		t.Fatal("expected decode to succeed")
	}

	if ev.Kind != domain.CodeIntelEventKindIndexChanged {
		t.Errorf("kind = %q, want %q", ev.Kind, domain.CodeIntelEventKindIndexChanged)
	}
	if ev.WorkspaceRoot != "/work/orca" {
		t.Errorf("workspaceRoot = %q, want /work/orca", ev.WorkspaceRoot)
	}
	if ev.Tool != "gitnexus" || ev.Commit != "sha-1" || ev.IndexedAt != "2026-10-06T10:00:00Z" {
		t.Errorf("tool/commit/indexedAt mismatch: %+v", ev)
	}
	if ev.Reason != "head_changed" || ev.HeadCommit != "sha-2" || !ev.Stale {
		t.Errorf("reason/headCommit/stale mismatch: %+v", ev)
	}
	if ev.IndexScope != "repo" || ev.MergeBase != "sha-base" || ev.Trigger != "watch" {
		t.Errorf("indexScope/mergeBase/trigger mismatch: %+v", ev)
	}
	if !ev.ReceivedAt.Equal(now) {
		t.Errorf("receivedAt mismatch: got %v, want %v", ev.ReceivedAt, now)
	}
	if ev.PayloadJSON != rawParams {
		t.Errorf("payloadJSON mismatch")
	}
}

func TestDecodeCodeIntelNotification_IndexChanged_MissingWorkspaceRoot(t *testing.T) {
	notif := JSONRPCNotification{
		Method: "codeintel.indexChanged",
		Params: json.RawMessage(`{"tool": "gitnexus"}`),
	}
	_, ok := decodeCodeIntelNotification(notif, time.Now())
	if ok {
		t.Fatal("expected decode to fail when workspaceRoot is missing")
	}
}

func TestDecodeCodeIntelNotification_ReindexProgress(t *testing.T) {
	now := time.Now()

	// Case 1: percent is 37
	notif1 := JSONRPCNotification{
		Method: "codeintel.reindexProgress",
		Params: json.RawMessage(`{
			"workspaceRoot": "/work/orca",
			"jobId": "job-1",
			"tool": "codegraph",
			"stage": "extracting",
			"percent": 37,
			"message": "indexing files",
			"outcome": "running",
			"errorCode": ""
		}`),
	}
	ev1, ok1 := decodeCodeIntelNotification(notif1, now)
	if !ok1 {
		t.Fatal("expected decode to succeed")
	}
	if ev1.Kind != domain.CodeIntelEventKindReindexProgress {
		t.Errorf("kind = %q, want reindex_progress", ev1.Kind)
	}
	if ev1.JobID != "job-1" || ev1.Tool != "codegraph" || ev1.Stage != "extracting" {
		t.Errorf("field mismatch: %+v", ev1)
	}
	if ev1.Percent == nil || *ev1.Percent != 37 {
		t.Errorf("percent = %v, want 37", ev1.Percent)
	}

	// Case 2: percent is null
	notif2 := JSONRPCNotification{
		Method: "codeintel.reindexProgress",
		Params: json.RawMessage(`{
			"workspaceRoot": "/work/orca",
			"jobId": "job-1",
			"tool": "codegraph",
			"stage": "queued",
			"percent": null
		}`),
	}
	ev2, ok2 := decodeCodeIntelNotification(notif2, now)
	if !ok2 {
		t.Fatal("expected decode to succeed")
	}
	if ev2.Percent != nil {
		t.Errorf("percent = %v, want nil", ev2.Percent)
	}

	// Case 3: missing jobId -> fails
	notif3 := JSONRPCNotification{
		Method: "codeintel.reindexProgress",
		Params: json.RawMessage(`{"workspaceRoot": "/work/orca"}`),
	}
	_, ok3 := decodeCodeIntelNotification(notif3, now)
	if ok3 {
		t.Fatal("expected decode to fail when jobId is missing")
	}
}

func TestDecodeCodeIntelNotification_QualityNotificationsAndLargePayload(t *testing.T) {
	now := time.Now()

	// quality.progress
	qp := JSONRPCNotification{
		Method: "quality.progress",
		Params: json.RawMessage(`{
			"workspaceRoot": "/work/orca",
			"runId": "run-q1",
			"stage": "linting",
			"percent": 50,
			"message": "running oxlint"
		}`),
	}
	evProg, ok := decodeCodeIntelNotification(qp, now)
	if !ok {
		t.Fatal("expected quality.progress decode to succeed")
	}
	if evProg.Kind != domain.CodeIntelEventKindQualityProgress || evProg.RunID != "run-q1" {
		t.Errorf("unexpected quality.progress event: %+v", evProg)
	}

	// quality.finished with huge summary (>64 KiB)
	largeSummary := strings.Repeat("x", 70*1024)
	qfParams, _ := json.Marshal(map[string]any{
		"workspaceRoot": "/work/orca",
		"runId":         "run-q2",
		"headCommit":    "sha-head",
		"errorCode":     "ERR_QUALITY",
		"summary":       largeSummary,
	})
	qf := JSONRPCNotification{
		Method: "quality.finished",
		Params: qfParams,
	}
	evFin, ok := decodeCodeIntelNotification(qf, now)
	if !ok {
		t.Fatal("expected quality.finished decode to succeed")
	}
	if evFin.Kind != domain.CodeIntelEventKindQualityFinished || evFin.RunID != "run-q2" {
		t.Errorf("unexpected quality.finished event: %+v", evFin)
	}
	if evFin.PayloadJSON != "" {
		t.Errorf("expected empty payload_json for payload > 64 KiB, got len = %d", len(evFin.PayloadJSON))
	}
}
