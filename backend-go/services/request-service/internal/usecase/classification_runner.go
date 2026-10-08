package usecase

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type EnqueueClassificationInput struct {
	RequestID string
	EventID   string
	Trigger   string
	Manual    bool
}

type EnqueueClassificationResult struct {
	Run     domain.ClassificationRun
	Started bool // false: an existing run (same event or already live) is returned instead
}

// ClassificationRunner owns the background half of classification (decision D3): callers
// get a run id at once, the AI call runs on a leased worker, and a sweeper re-claims runs
// whose instance died. Neither JetStream redelivery nor a held RPC is needed for durability.
type ClassificationRunner struct {
	repo    RequestRepository
	runs    ClassificationRunRepository
	propose *ProposeRequestClassification
	tx      TxRunner
	owner   string
	ttl     time.Duration
	now     func() time.Time

	stopCtx context.Context
	stop    context.CancelFunc
	wg      sync.WaitGroup
}

func NewClassificationRunner(repo RequestRepository, runs ClassificationRunRepository, propose *ProposeRequestClassification, tx TxRunner, owner string, leaseTTL time.Duration) *ClassificationRunner {
	if leaseTTL <= 0 {
		leaseTTL = 90 * time.Second // longer than the 60s AI timeout plus the result write
	}
	stopCtx, stop := context.WithCancel(context.Background())
	return &ClassificationRunner{repo: repo, runs: runs, propose: propose, tx: tx, owner: owner, ttl: leaseTTL, now: time.Now, stopCtx: stopCtx, stop: stop}
}

// Close stops heartbeats and the sweeper and waits for in-flight runs to return.
func (r *ClassificationRunner) Close() {
	r.stop()
	r.wg.Wait()
}

func (r *ClassificationRunner) Enqueue(ctx context.Context, in EnqueueClassificationInput) (EnqueueClassificationResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return EnqueueClassificationResult{}, domain.ErrRequestTenantRequired()
	}
	req, err := r.repo.Get(ctx, in.RequestID)
	if err != nil {
		return EnqueueClassificationResult{}, err
	}
	if err := r.propose.Validate(req, in.Manual); err != nil {
		return EnqueueClassificationResult{}, err
	}
	now := r.now().UTC()
	run := domain.ClassificationRun{
		ID: uuid.NewString(), TenantID: tenantID, RequestID: req.ID, Trigger: in.Trigger, SourceEventID: in.EventID,
		Manual: in.Manual, ActorID: req.ReporterID, Status: domain.ClassificationRunRunning, Claims: 1,
		LeaseOwner: r.owner, LeaseExpiresAt: now.Add(r.ttl), StartedAt: now,
	}
	existing, started, err := r.runs.Start(ctx, run)
	if err != nil {
		return EnqueueClassificationResult{}, err
	}
	if !started {
		return EnqueueClassificationResult{Run: existing}, nil
	}
	r.spawn(run)
	return EnqueueClassificationResult{Run: run, Started: true}, nil
}

// Run is the consumer-facing trigger: it only enqueues, so the event is acked once the run is durable.
// A request that left the classifiable states or no longer exists is a permanent condition, not an error.
func (r *ClassificationRunner) Run(ctx context.Context, requestID, eventID, trigger string) error {
	_, err := r.Enqueue(ctx, EnqueueClassificationInput{RequestID: requestID, EventID: eventID, Trigger: trigger})
	var ae *apperrors.AppError
	if errors.As(err, &ae) && (ae.Code == "REQUEST_NOT_CLASSIFIABLE" || ae.Code == "REQUEST_NOT_FOUND") {
		slog.InfoContext(ctx, "classification skipped", slog.String("request_id", requestID), slog.String("code", ae.Code))
		return nil
	}
	return err
}

func (r *ClassificationRunner) spawn(run domain.ClassificationRun) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.execute(run)
	}()
}

func (r *ClassificationRunner) runCtx(run domain.ClassificationRun) context.Context {
	return tenant.WithUserID(tenant.WithTenantID(r.stopCtx, run.TenantID), run.ActorID)
}

func (r *ClassificationRunner) execute(run domain.ClassificationRun) {
	ctx, cancel := context.WithCancel(r.runCtx(run))
	defer cancel()
	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		t := time.NewTicker(r.ttl / 3)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ok, err := r.runs.Renew(ctx, run.ID, r.owner, r.now().UTC().Add(r.ttl))
				if err == nil && !ok {
					cancel() // another instance took the run over
					return
				}
			}
		}
	}()
	err := r.propose.Execute(ctx, ProposeInput{RequestID: run.RequestID, EventID: run.SourceEventID, Trigger: run.Trigger, Manual: run.Manual, RunID: run.ID})
	cancel()
	<-hbDone
	if err == nil {
		return
	}
	var ae *apperrors.AppError
	if errors.As(err, &ae) && ae.Kind != apperrors.KindInternal {
		// Permanent (no longer classifiable, limit reached): end the run instead of re-claiming it forever.
		fctx := r.runCtx(run)
		if ferr := r.tx.InTx(fctx, func(txCtx context.Context) error {
			return r.runs.Finish(txCtx, run.ID, domain.ClassificationRunFailed, ae.Code)
		}); ferr != nil {
			slog.WarnContext(fctx, "classification run finish failed", slog.String("run_id", run.ID), slog.Any("error", ferr))
		}
		return
	}
	// Transient: leave the run running; its lease expires and RecoverLoop retries (bounded by claims).
	slog.WarnContext(ctx, "classification run will be retried after lease expiry", slog.String("run_id", run.ID), slog.Any("error", err))
}

// RecoverOnce claims runs whose lease expired and restarts them; returns how many were taken.
func (r *ClassificationRunner) RecoverOnce(ctx context.Context) (int, error) {
	runs, err := r.runs.ClaimExpired(ctx, r.owner, r.now().UTC().Add(r.ttl), 10)
	if err != nil {
		return 0, err
	}
	for _, run := range runs {
		r.spawn(run)
	}
	return len(runs), nil
}

// RecoverLoop sweeps until ctx or Close; the first pass runs at once to pick up a crashed instance's work.
func (r *ClassificationRunner) RecoverLoop(ctx context.Context, interval time.Duration) {
	r.wg.Add(1)
	defer r.wg.Done()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if _, err := r.RecoverOnce(ctx); err != nil {
			slog.WarnContext(ctx, "classification recovery sweep failed", slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return
		case <-r.stopCtx.Done():
			return
		case <-t.C:
		}
	}
}
