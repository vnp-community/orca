package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

// RequestReindexUseCase coordinates the 5-step reindex workflow per §2.F.
type RequestReindexUseCase struct {
	resolver  WorktreeTargetResolver
	admission ReindexAdmission
	store     ReindexJobStore
	gateway   AgentCodeIntelGateway
}

// NewRequestReindexUseCase creates a new RequestReindexUseCase.
func NewRequestReindexUseCase(
	resolver WorktreeTargetResolver,
	admission ReindexAdmission,
	store ReindexJobStore,
	gateway AgentCodeIntelGateway,
) *RequestReindexUseCase {
	if admission == nil {
		admission = &DefaultNoopAdmission{}
	}
	return &RequestReindexUseCase{
		resolver:  resolver,
		admission: admission,
		store:     store,
		gateway:   gateway,
	}
}

// Execute runs the reindex admission, DB registration, and agent invocation workflow.
func (u *RequestReindexUseCase) Execute(ctx context.Context, tenantID string, req *codeintelv1.RequestReindexRequest) (*ReindexJob, error) {
	if req == nil || req.Selector == nil {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "CODEINTEL_INVALID_PARAMS", "selector is required", nil)
	}

	mode := req.Mode
	if mode == "" {
		mode = "incremental"
	} else if mode != "incremental" && mode != "full" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "CODEINTEL_INVALID_PARAMS", fmt.Sprintf("invalid reindex mode: %q (expected 'incremental' or 'full')", mode), nil)
	}

	// 1. Resolve Target and RepoBindingID
	target, repoBindingID, err := u.resolver.ResolveTarget(ctx, tenantID, req.Selector)
	if err != nil {
		return nil, err
	}

	// 2. Admission Check (cooldown, quotas per SOL-013)
	if err := u.admission.Admit(ctx, tenantID, repoBindingID); err != nil {
		return nil, err
	}

	// 3. CreateActive in store with transactional outbox event
	now := time.Now().UTC()
	jobID := "ri_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	activeKey := fmt.Sprintf("%s:%s", tenantID, repoBindingID)

	job := ReindexJob{
		JobID:         jobID,
		TenantID:      tenantID,
		RepoBindingID: repoBindingID,
		ActiveKey:     activeKey,
		Mode:          mode,
		Status:        "queued",
		Trigger:       "manual",
		Stage:         "preflight",
		StartedAt:     now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	outboxPayload, _ := json.Marshal(map[string]any{
		"jobId":         jobID,
		"repoBindingId": repoBindingID,
		"mode":          mode,
		"trigger":       "manual",
		"startedAt":     now,
	})

	outbox := OutboxEvent{
		EventID:   uuid.NewString(),
		TenantID:  tenantID,
		EventType: "reindex.started",
		Payload:   outboxPayload,
		CreatedAt: now,
	}

	if err := u.store.CreateActive(ctx, job, outbox); err != nil {
		// Conflict on active job
		if existing, getErr := u.store.GetActiveByBinding(ctx, tenantID, repoBindingID); getErr == nil && existing != nil {
			return nil, apperrors.New(apperrors.KindFailedPrecondition, "CODEINTEL_REINDEX_IN_PROGRESS",
				fmt.Sprintf("reindex already active: jobId=%s stage=%s", existing.JobID, existing.Stage), nil)
		}
		return nil, err
	}

	// 4. Invoke agent codeintel.reindex
	agentParams := ReindexParams{
		Mode:    mode,
		Trigger: "manual",
	}

	rawRes, err := u.gateway.Reindex(ctx, target, agentParams)
	if err != nil {
		// If agent is offline / unavailable, fail immediately without retry
		var appErr *apperrors.AppError
		errorCode := "DEV_SERVER_OFFLINE"
		if errorsAsAppError(err, &appErr) && appErr.Code != "" {
			errorCode = appErr.Code
		}
		finishTime := time.Now().UTC()
		_ = u.store.Finish(ctx, tenantID, jobID, "failed", "", errorCode, finishTime)
		job.Status = "failed"
		job.ErrorCode = errorCode
		job.FinishedAt = &finishTime
		return &job, nil
	}

	// 5. Inspect agent response
	var agentResult struct {
		JobID   string `json:"jobId"`
		State   string `json:"state"`
		Outcome string `json:"outcome"`
	}
	if len(rawRes.Data) > 0 {
		_ = json.Unmarshal(rawRes.Data, &agentResult)
	}

	// If already up-to-date, skipped, or superseded, finish immediately as succeeded
	if agentResult.Outcome != "" {
		finishTime := time.Now().UTC()
		_ = u.store.Finish(ctx, tenantID, jobID, "succeeded", agentResult.Outcome, "", finishTime)
		job.Status = "succeeded"
		job.Outcome = agentResult.Outcome
		job.FinishedAt = &finishTime
		return &job, nil
	}

	// Adopt agent job ID and advance status to running
	agentJobID := agentResult.JobID
	if agentJobID == "" {
		agentJobID = jobID
	}
	_ = u.store.UpdateAgentJobID(ctx, tenantID, jobID, agentJobID)
	job.AgentJobID = agentJobID
	job.Status = "running"
	job.Stage = "running"

	return &job, nil
}

func errorsAsAppError(err error, target **apperrors.AppError) bool {
	if err == nil {
		return false
	}
	if ae, ok := err.(*apperrors.AppError); ok {
		*target = ae
		return true
	}
	return false
}

