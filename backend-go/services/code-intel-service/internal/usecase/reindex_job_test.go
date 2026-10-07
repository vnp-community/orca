package usecase

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

// inMemoryReindexJobStore implements ReindexJobStore with active_key exclusivity and outbox tracking.
type inMemoryReindexJobStore struct {
	mu         sync.Mutex
	jobs       map[string]ReindexJob // key: tenantID + ":" + jobID
	activeKeys map[string]string     // key: activeKey -> jobID
	outbox     []OutboxEvent
}

func newInMemoryReindexJobStore() *inMemoryReindexJobStore {
	return &inMemoryReindexJobStore{
		jobs:       make(map[string]ReindexJob),
		activeKeys: make(map[string]string),
	}
}

func (s *inMemoryReindexJobStore) CreateActive(ctx context.Context, job ReindexJob, outbox OutboxEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job.ActiveKey != "" {
		if existingID, exists := s.activeKeys[job.ActiveKey]; exists {
			return fmt.Errorf("%w: job=%s", ErrActiveJobExists, existingID)
		}
		s.activeKeys[job.ActiveKey] = job.JobID
	}

	key := job.TenantID + ":" + job.JobID
	s.jobs[key] = job
	s.outbox = append(s.outbox, outbox)
	return nil
}

func (s *inMemoryReindexJobStore) Get(ctx context.Context, tenantID, jobID string) (*ReindexJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tenantID + ":" + jobID
	j, ok := s.jobs[key]
	if !ok {
		return nil, nil
	}
	cpy := j
	return &cpy, nil
}

func (s *inMemoryReindexJobStore) GetActiveByBinding(ctx context.Context, tenantID, repoBindingID string) (*ReindexJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	activeKey := fmt.Sprintf("%s:%s", tenantID, repoBindingID)
	jobID, ok := s.activeKeys[activeKey]
	if !ok {
		return nil, nil
	}
	key := tenantID + ":" + jobID
	j, ok := s.jobs[key]
	if !ok {
		return nil, nil
	}
	cpy := j
	return &cpy, nil
}

func (s *inMemoryReindexJobStore) UpdateProgress(ctx context.Context, tenantID, jobID, stage string, percent *int32, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tenantID + ":" + jobID
	j, ok := s.jobs[key]
	if !ok {
		return fmt.Errorf("job not found")
	}
	j.Stage = stage
	j.Percent = percent
	j.Message = message
	j.UpdatedAt = time.Now().UTC()
	s.jobs[key] = j
	return nil
}

func (s *inMemoryReindexJobStore) Finish(ctx context.Context, tenantID, jobID, status, outcome, errorCode string, finishedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tenantID + ":" + jobID
	j, ok := s.jobs[key]
	if !ok {
		return fmt.Errorf("job not found")
	}
	j.Status = status
	j.Outcome = outcome
	j.ErrorCode = errorCode
	j.FinishedAt = &finishedAt
	j.UpdatedAt = finishedAt
	if j.ActiveKey != "" {
		delete(s.activeKeys, j.ActiveKey)
		j.ActiveKey = ""
	}
	s.jobs[key] = j
	return nil
}

func (s *inMemoryReindexJobStore) UpdateAgentJobID(ctx context.Context, tenantID, jobID, agentJobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tenantID + ":" + jobID
	j, ok := s.jobs[key]
	if !ok {
		return fmt.Errorf("job not found")
	}
	j.AgentJobID = agentJobID
	j.Status = "running"
	j.UpdatedAt = time.Now().UTC()
	s.jobs[key] = j
	return nil
}

type staticResolver struct {
	target  AgentTarget
	binding string
	err     error
}

func (r *staticResolver) ResolveTarget(ctx context.Context, tenantID string, sel *codeintelv1.WorktreeSelector) (AgentTarget, string, error) {
	if r.err != nil {
		return AgentTarget{}, "", r.err
	}
	return r.target, r.binding, nil
}

type mockAdmission struct {
	err error
}

func (a *mockAdmission) Admit(ctx context.Context, tenantID, repoBindingID string) error {
	return a.err
}

func TestRequestReindex_SuccessWorkflow(t *testing.T) {
	store := newInMemoryReindexJobStore()
	gw := newMockGateway()
	gw.reindexRes = RawCodeIntelResult{
		Data: []byte(`{"jobId":"agent_ri_999","state":"running"}`),
	}

	target := AgentTarget{
		TenantID:      "tenant-1",
		DevServerID:   "dev-1",
		WorkspaceRoot: "/workspace",
	}
	resolver := &staticResolver{target: target, binding: "binding-1"}
	usecase := NewRequestReindexUseCase(resolver, nil, store, gw)

	req := &codeintelv1.RequestReindexRequest{
		Selector: &codeintelv1.WorktreeSelector{ProjectId: "p1", WorktreeRef: "w1"},
		Mode:     "incremental",
	}

	job, err := usecase.Execute(context.Background(), "tenant-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Status != "running" {
		t.Errorf("expected job status 'running', got %q", job.Status)
	}
	if job.AgentJobID != "agent_ri_999" {
		t.Errorf("expected AgentJobID 'agent_ri_999', got %q", job.AgentJobID)
	}
	if len(store.outbox) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(store.outbox))
	}
	if store.outbox[0].EventType != "reindex.started" {
		t.Errorf("expected outbox event type 'reindex.started', got %q", store.outbox[0].EventType)
	}

	// Verify proto conversion works
	pb := ToProtoReindexJob(job)
	if pb.GetJobId() != job.JobID || pb.GetStatus() != "running" {
		t.Fatalf("unexpected proto job: %+v", pb)
	}
}

func TestRequestReindex_ImmediateOutcomeSucceeded(t *testing.T) {
	store := newInMemoryReindexJobStore()
	gw := newMockGateway()
	gw.reindexRes = RawCodeIntelResult{
		Data: []byte(`{"outcome":"already_up_to_date"}`),
	}

	resolver := &staticResolver{
		target:  AgentTarget{WorkspaceRoot: "/workspace"},
		binding: "binding-1",
	}
	usecase := NewRequestReindexUseCase(resolver, nil, store, gw)

	req := &codeintelv1.RequestReindexRequest{
		Selector: &codeintelv1.WorktreeSelector{ProjectId: "p1"},
		Mode:     "incremental",
	}

	job, err := usecase.Execute(context.Background(), "tenant-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Status != "succeeded" {
		t.Errorf("expected status 'succeeded', got %q", job.Status)
	}
	if job.Outcome != "already_up_to_date" {
		t.Errorf("expected outcome 'already_up_to_date', got %q", job.Outcome)
	}
	if job.FinishedAt == nil {
		t.Errorf("expected FinishedAt to be non-nil")
	}
}

func TestRequestReindex_OfflineFailsImmediately(t *testing.T) {
	store := newInMemoryReindexJobStore()
	gw := newMockGateway()
	gw.reindexErr = apperrors.New(apperrors.KindUnavailable, "DEV_SERVER_OFFLINE", "dev server disconnected", nil)

	resolver := &staticResolver{
		target:  AgentTarget{WorkspaceRoot: "/workspace"},
		binding: "binding-1",
	}
	usecase := NewRequestReindexUseCase(resolver, nil, store, gw)

	req := &codeintelv1.RequestReindexRequest{
		Selector: &codeintelv1.WorktreeSelector{ProjectId: "p1"},
		Mode:     "full",
	}

	job, err := usecase.Execute(context.Background(), "tenant-1", req)
	if err != nil {
		t.Fatalf("unexpected error, should return failed job: %v", err)
	}

	if job.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", job.Status)
	}
	if job.ErrorCode != "DEV_SERVER_OFFLINE" {
		t.Errorf("expected ErrorCode 'DEV_SERVER_OFFLINE', got %q", job.ErrorCode)
	}
}

func TestRequestReindex_ConcurrentActiveKeyExclusivity(t *testing.T) {
	store := newInMemoryReindexJobStore()
	gw := newMockGateway()
	gw.reindexRes = RawCodeIntelResult{
		Data: []byte(`{"jobId":"agent_1"}`),
	}

	resolver := &staticResolver{
		target:  AgentTarget{WorkspaceRoot: "/workspace"},
		binding: "binding-concurrent",
	}
	usecase := NewRequestReindexUseCase(resolver, nil, store, gw)

	req := &codeintelv1.RequestReindexRequest{
		Selector: &codeintelv1.WorktreeSelector{ProjectId: "p1"},
		Mode:     "incremental",
	}

	// First request succeeds
	job1, err1 := usecase.Execute(context.Background(), "tenant-1", req)
	if err1 != nil {
		t.Fatalf("first request failed: %v", err1)
	}
	if job1.Status != "running" {
		t.Fatalf("expected running, got %s", job1.Status)
	}

	// Second request for same tenant & binding fails with CODEINTEL_REINDEX_IN_PROGRESS
	_, err2 := usecase.Execute(context.Background(), "tenant-1", req)
	if err2 == nil {
		t.Fatalf("second concurrent request should have failed")
	}
	var appErr *apperrors.AppError
	if !errorsAsAppError(err2, &appErr) || appErr.Code != "CODEINTEL_REINDEX_IN_PROGRESS" {
		t.Fatalf("expected CODEINTEL_REINDEX_IN_PROGRESS, got: %v", err2)
	}
}

func TestGetReindexJob_TenantIsolationAndRefresh(t *testing.T) {
	store := newInMemoryReindexJobStore()
	gw := newMockGateway()

	refresher := NewAgentReindexJobRefresher(gw, store)
	resolver := &staticResolver{
		target:  AgentTarget{WorkspaceRoot: "/workspace"},
		binding: "binding-1",
	}
	usecase := NewGetReindexJobUseCase(resolver, store, refresher, 1*time.Millisecond)

	// Seed a running job in store
	past := time.Now().UTC().Add(-1 * time.Minute)
	job := ReindexJob{
		JobID:         "job-100",
		TenantID:      "tenant-A",
		RepoBindingID: "binding-1",
		Status:        "running",
		Stage:         "preflight",
		AgentJobID:    "agent-job-100",
		UpdatedAt:     past,
	}
	_ = store.CreateActive(context.Background(), job, OutboxEvent{})

	ctx := context.Background()

	// 1. Another tenant cannot access it -> NOT_FOUND
	_, err := usecase.Execute(ctx, "tenant-B", &codeintelv1.GetReindexJobRequest{
		Selector: &codeintelv1.WorktreeSelector{ProjectId: "p1"},
		JobId:    "job-100",
	})
	if err == nil {
		t.Fatalf("expected error for different tenant")
	}
	var appErr *apperrors.AppError
	if !errorsAsAppError(err, &appErr) || appErr.Code != "CODEINTEL_NOT_FOUND" {
		t.Errorf("expected CODEINTEL_NOT_FOUND for other tenant, got: %v", err)
	}

	// 2. Matching tenant reads job and refreshes from agent
	percent50 := int32(50)
	gw.reindexStatRes = RawCodeIntelResult{
		Data: []byte(`{
			"job": {
				"jobId": "agent-job-100",
				"state": "running",
				"stage": "gitnexus.analyze",
				"percent": 50,
				"message": "indexing 500 files"
			}
		}`),
	}

	gotJob, err := usecase.Execute(ctx, "tenant-A", &codeintelv1.GetReindexJobRequest{
		Selector: &codeintelv1.WorktreeSelector{ProjectId: "p1"},
		JobId:    "job-100",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotJob.Stage != "gitnexus.analyze" {
		t.Errorf("expected stage refreshed to 'gitnexus.analyze', got %q", gotJob.Stage)
	}
	if gotJob.Percent == nil || *gotJob.Percent != percent50 {
		t.Errorf("expected percent 50, got %v", gotJob.Percent)
	}
}

func TestReindexJobRefresh_StateMappings(t *testing.T) {
	store := newInMemoryReindexJobStore()
	gw := newMockGateway()
	refresher := NewAgentReindexJobRefresher(gw, store)

	ctx := context.Background()
	target := AgentTarget{WorkspaceRoot: "/workspace"}

	// 1. Cancelling maps to running per PQ-16
	job := &ReindexJob{
		JobID:      "j1",
		TenantID:   "t1",
		Status:     "queued",
		AgentJobID: "aj1",
	}
	gw.reindexStatRes = RawCodeIntelResult{
		Data: []byte(`{"job":{"jobId":"aj1","state":"cancelling","stage":"verify"}}`),
	}
	refreshed, err := refresher.Refresh(ctx, target, job)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if refreshed.Status != "running" {
		t.Errorf("cancelling must map to running, got %q", refreshed.Status)
	}

	// 2. Interrupted maps to failed with CODEINTEL_REINDEX_INTERRUPTED
	gw.reindexStatRes = RawCodeIntelResult{
		Data: []byte(`{"job":{"jobId":"aj1","state":"interrupted"}}`),
	}
	refreshed, err = refresher.Refresh(ctx, target, job)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if refreshed.Status != "failed" {
		t.Errorf("interrupted must map to failed, got %q", refreshed.Status)
	}
	if refreshed.ErrorCode != "CODEINTEL_REINDEX_INTERRUPTED" {
		t.Errorf("expected ErrorCode 'CODEINTEL_REINDEX_INTERRUPTED', got %q", refreshed.ErrorCode)
	}
}
