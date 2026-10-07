package usecase

import (
	"context"
	"encoding/json"
	"time"
)

// ReindexJobRefresher refreshes an active reindex job by polling agent status (§2.F).
type ReindexJobRefresher interface {
	Refresh(ctx context.Context, target AgentTarget, job *ReindexJob) (*ReindexJob, error)
}

// AgentReindexJobRefresher queries codeintel.reindexStatus and updates the job store.
type AgentReindexJobRefresher struct {
	gateway AgentCodeIntelGateway
	store   ReindexJobStore
}

// NewAgentReindexJobRefresher creates a new refresher.
func NewAgentReindexJobRefresher(gateway AgentCodeIntelGateway, store ReindexJobStore) *AgentReindexJobRefresher {
	return &AgentReindexJobRefresher{
		gateway: gateway,
		store:   store,
	}
}

// Refresh polls agent reindexStatus, maps agent states, and updates progress in the store.
func (r *AgentReindexJobRefresher) Refresh(ctx context.Context, target AgentTarget, job *ReindexJob) (*ReindexJob, error) {
	if job == nil {
		return nil, nil
	}

	queryJobID := job.AgentJobID
	if queryJobID == "" {
		queryJobID = job.JobID
	}

	raw, err := r.gateway.ReindexStatus(ctx, target, ReindexStatusParams{JobID: queryJobID})
	if err != nil {
		// Failure to poll agent status does not fail the job; return current job state
		return job, nil
	}

	type agentError struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	type agentJobInfo struct {
		JobID     string      `json:"jobId"`
		State     string      `json:"state"`
		Stage     string      `json:"stage"`
		Percent   *int32      `json:"percent,omitempty"`
		Message   string      `json:"message"`
		Outcome   string      `json:"outcome"`
		Error     *agentError `json:"error,omitempty"`
	}

	var payload struct {
		Job *agentJobInfo `json:"job"`
	}

	if len(raw.Data) > 0 {
		_ = json.Unmarshal(raw.Data, &payload)
	}

	if payload.Job == nil {
		return job, nil
	}

	aj := payload.Job
	dbStatus, errCode := mapAgentStateToDBStatus(aj.State)
	if aj.Error != nil && aj.Error.Code != "" && errCode == "" {
		errCode = aj.Error.Code
	}

	now := time.Now().UTC()
	job.Stage = aj.Stage
	job.Percent = aj.Percent
	job.Message = aj.Message
	job.Outcome = aj.Outcome
	job.UpdatedAt = now

	if isTerminalStatus(dbStatus) {
		job.Status = dbStatus
		job.ErrorCode = errCode
		job.FinishedAt = &now
		_ = r.store.Finish(ctx, job.TenantID, job.JobID, dbStatus, aj.Outcome, errCode, now)
	} else {
		job.Status = dbStatus
		_ = r.store.UpdateProgress(ctx, job.TenantID, job.JobID, aj.Stage, aj.Percent, aj.Message)
	}

	return job, nil
}

// mapAgentStateToDBStatus maps agent state to DB status per PQ-16 / §2.F:
// - cancelling -> running
// - interrupted -> failed (CODEINTEL_REINDEX_INTERRUPTED)
func mapAgentStateToDBStatus(state string) (status string, errCode string) {
	switch state {
	case "queued":
		return "queued", ""
	case "running":
		return "running", ""
	case "cancelling":
		// PQ-16: cancelling displays as running
		return "running", ""
	case "succeeded":
		return "succeeded", ""
	case "failed":
		return "failed", ""
	case "cancelled":
		return "cancelled", ""
	case "interrupted":
		return "failed", "CODEINTEL_REINDEX_INTERRUPTED"
	default:
		return "running", ""
	}
}

func isTerminalStatus(status string) bool {
	return status == "succeeded" || status == "failed" || status == "cancelled"
}
