package wscompat

import (
	"context"
	"encoding/json"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/secretscan"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// executionResultView is what the task sheet's Result tab reads (frontend/src/shared/request-artifact-parsers.ts
// parseExecutionResult): camelCase keys, parseStatus ok|missing|invalid. verdict is never set here, it belongs to the
// verification that request-service makes after a run.
type executionResultView struct {
	TaskID       string         `json:"taskId"`
	Attempt      int32          `json:"attempt,omitempty"`
	ParseStatus  string         `json:"parseStatus"`
	Status       string         `json:"status,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	FilesChanged []string       `json:"filesChanged"`
	ChecksRun    []checkRunView `json:"checksRun"`
	FailureClass string         `json:"failureClass,omitempty"`
	StdoutTail   string         `json:"stdoutTail,omitempty"`
	Outputs      map[string]any `json:"outputs,omitempty"`
	// Source says where the data came from: "execution_record" (a contract run) or "last_execution_output" (the raw
	// stdout of a run that reported no result block, shown as parseStatus "missing").
	Source    string `json:"source"`
	Redacted  bool   `json:"redacted,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

type checkRunView struct {
	ID   string `json:"id"`
	Exit int    `json:"exit"`
}

func registerTaskExecutionChannels(r *Registry, client taskv1.TaskServiceClient) {
	r.Register("execution.get", func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[struct {
			TaskID     string `json:"taskId"`
			LatestOnly bool   `json:"latestOnly"`
		}](args, 0)
		if err != nil {
			return nil, err
		}
		if in.TaskID == "" {
			return nil, errors.New("TASK_INVALID_ARGUMENT: taskId is required")
		}
		return executionResultFor(ctx, client, in.TaskID)
	})
}

// executionResultFor answers with the newest ExecutionRecord when the run reported a result block, and otherwise with
// the stored stdout of the last successful run. Task-service checks read access on the task in both calls.
func executionResultFor(ctx context.Context, client taskv1.TaskServiceClient, taskID string) (*executionResultView, error) {
	recs, err := client.ListExecutionRecords(ctx, &taskv1.ListExecutionRecordsRequest{TaskIds: []string{taskID}, LatestOnly: true, Limit: 1})
	if err != nil && status.Code(err) != codes.Unimplemented {
		return nil, executionChannelError(err)
	}
	if rec := newestRecord(recs.GetRecords()); rec != nil {
		return executionRecordView(rec), nil
	}
	out, err := client.GetTaskExecutionOutput(ctx, &taskv1.GetTaskExecutionOutputRequest{TaskId: taskID})
	if err != nil && status.Code(err) != codes.Unimplemented {
		return nil, executionChannelError(err)
	}
	if out.GetFound() && out.GetOutput() != "" {
		return &executionResultView{
			TaskID: taskID, ParseStatus: "missing", FilesChanged: []string{}, ChecksRun: []checkRunView{},
			StdoutTail: out.GetOutput(), Source: "last_execution_output", Redacted: out.GetRedacted(), Truncated: out.GetTruncated(),
		}, nil
	}
	return nil, errors.New("TASK_NOT_FOUND: this task has no execution result yet")
}

func newestRecord(recs []*taskv1.ExecutionRecord) *taskv1.ExecutionRecord {
	var best *taskv1.ExecutionRecord
	for _, r := range recs {
		if best == nil || r.GetCreatedAt().AsTime().After(best.GetCreatedAt().AsTime()) {
			best = r
		}
	}
	return best
}

// executionChannelError keeps task-service's message out of the client; the code is enough for the UI to branch on.
func executionChannelError(err error) error {
	switch status.Code(err) {
	case codes.PermissionDenied, codes.Unauthenticated:
		return errors.New("TASK_FORBIDDEN: you may not read this task's execution result")
	case codes.NotFound:
		return errors.New("TASK_NOT_FOUND: task not found")
	case codes.InvalidArgument:
		return errors.New("TASK_INVALID_ARGUMENT: invalid execution result request")
	}
	return errors.New("TASK_UNAVAILABLE: could not read the execution result")
}

func executionRecordView(rec *taskv1.ExecutionRecord) *executionResultView {
	view := &executionResultView{
		TaskID: rec.GetTaskId(), Attempt: rec.GetAttempt(), ParseStatus: rec.GetParseStatus(), FailureClass: rec.GetFailureClass(),
		FilesChanged: []string{}, ChecksRun: []checkRunView{}, Source: "execution_record",
	}
	tail, redacted := secretscan.Redact(rec.GetStdoutTail())
	view.StdoutTail, view.Redacted = tail, redacted

	var result struct {
		Status       string                     `json:"status"`
		Summary      string                     `json:"summary"`
		FilesChanged []string                   `json:"files_changed"`
		ChecksRun    []checkRunView             `json:"checks_run"`
		Outputs      map[string]json.RawMessage `json:"outputs"`
	}
	if rec.GetResultJson() != "" && json.Unmarshal([]byte(rec.GetResultJson()), &result) == nil {
		view.Status = result.Status
		view.Summary, _ = secretscan.Redact(result.Summary)
		view.ChecksRun = append(view.ChecksRun, result.ChecksRun...)
		view.FilesChanged = appendUnique(view.FilesChanged, result.FilesChanged...)
		view.Outputs = redactedOutputs(result.Outputs)
	}
	// What the agent saw change in the worktree is better evidence than what it says it changed, so it comes first.
	var changes struct {
		ChangedFiles []struct {
			Path string `json:"path"`
		} `json:"changedFiles"`
	}
	if rec.GetChangesJson() != "" && json.Unmarshal([]byte(rec.GetChangesJson()), &changes) == nil {
		observed := make([]string, 0, len(changes.ChangedFiles))
		for _, f := range changes.ChangedFiles {
			observed = append(observed, f.Path)
		}
		view.FilesChanged = appendUnique(observed, view.FilesChanged...)
	}
	return view
}

func appendUnique(dst []string, more ...string) []string {
	seen := make(map[string]bool, len(dst)+len(more))
	for _, s := range dst {
		seen[s] = true
	}
	for _, s := range more {
		if s != "" && !seen[s] {
			seen[s] = true
			dst = append(dst, s)
		}
	}
	return dst
}

// redactedOutputs decodes each declared output and masks secrets inside its strings; a value that is not JSON is dropped.
func redactedOutputs(raw map[string]json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]any, len(raw))
	for name, v := range raw {
		var decoded any
		if json.Unmarshal(v, &decoded) == nil {
			out[name] = redactJSONStrings(decoded)
		}
	}
	return out
}

func redactJSONStrings(v any) any {
	switch t := v.(type) {
	case string:
		s, _ := secretscan.Redact(t)
		return s
	case []any:
		for i := range t {
			t[i] = redactJSONStrings(t[i])
		}
	case map[string]any:
		for k := range t {
			t[k] = redactJSONStrings(t[k])
		}
	}
	return v
}
