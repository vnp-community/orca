package usecase

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// TaskContentEraser clears task content (description, AI context) that references a Request.
// task-service has no such RPC yet, so the wired implementation reports ErrErasureUnsupported.
type TaskContentEraser interface {
	Erase(ctx context.Context, requestID string) error
}

type UnsupportedTaskContentEraser struct{}

func (UnsupportedTaskContentEraser) Erase(context.Context, string) error {
	return domain.ErrErasureUnsupported
}

type ExternalErasure struct {
	System string
	Status string // erased | unsupported | failed
}

type EraseResult struct {
	ErasedAt       time.Time
	External       []ExternalErasure
	NotCoveredNote string
	AlreadyErased  bool
}

// NotCoveredNote is what erasure cannot reach; never promise "completely deleted" to a customer.
const NotCoveredNote = "Copies outside this service are not erased: the issue tracker (Jira), git history, the LLM provider and database backups."

const maxEraseReasonChars = 500

type EraseRequest struct {
	requests RequestReader
	store    RetentionStore
	tx       TxRunner
	audit    *AuditRecorder
	tasks    TaskContentEraser
	key      []byte
	clock    Clock
}

func NewEraseRequest(requests RequestReader, store RetentionStore, tx TxRunner, audit *AuditRecorder, tasks TaskContentEraser, hmacKey []byte, clock Clock) *EraseRequest {
	if clock == nil {
		clock = systemClock{}
	}
	if tasks == nil {
		tasks = UnsupportedTaskContentEraser{}
	}
	return &EraseRequest{requests: requests, store: store, tx: tx, audit: audit, tasks: tasks, key: hmacKey, clock: clock}
}

// Execute anonymizes one Request at once. The admin check repeats the interceptor's: erasure must not depend on wiring.
func (uc *EraseRequest) Execute(ctx context.Context, requestID, reason string) (EraseResult, error) {
	if role, _ := tenant.Role(ctx); role != "admin" || tenant.ActorType(ctx) == tenant.ActorAgent {
		return EraseResult{}, domain.ErrRequestForbidden()
	}
	if len(uc.key) == 0 {
		return EraseResult{}, domain.ErrEraseKeyMissing()
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return EraseResult{}, domain.ErrRequestTenantRequired()
	}
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); n == 0 || n > maxEraseReasonChars {
		return EraseResult{}, domain.ErrEraseReasonRequired()
	}
	req, err := uc.requests.Get(ctx, requestID)
	if err != nil {
		return EraseResult{}, err
	}
	if req.Status == domain.RequestStatusExecuting {
		return EraseResult{}, domain.ErrEraseNotAllowed()
	}

	at := uc.clock.Now()
	actor, _ := tenant.UserID(ctx)
	pseudonym := func(reporter string) string { return domain.PseudonymizeReporter(uc.key, tenantID, reporter) }
	changed := false
	// The erase marker and its audit entry commit together: no erasure without a trace, no trace without an erasure.
	err = uc.tx.InTx(ctx, func(txCtx context.Context) error {
		var aerr error
		if changed, aerr = uc.store.Anonymize(txCtx, requestID, pseudonym, at, actor); aerr != nil || !changed {
			return aerr
		}
		return uc.audit.RecordDurable(txCtx, AuditEvent{
			Action: domain.AuditRequestErase, TargetType: "request", TargetID: requestID, Outcome: "allowed", RequestID: requestID,
			Metadata: map[string]any{"reason": truncateRunes(reason, 200)},
		})
	})
	if err != nil {
		return EraseResult{}, err
	}
	res := EraseResult{ErasedAt: at, NotCoveredNote: NotCoveredNote, AlreadyErased: !changed}
	switch terr := uc.tasks.Erase(ctx, requestID); {
	case terr == nil:
		res.External = append(res.External, ExternalErasure{System: "task-service", Status: "erased"})
	case errors.Is(terr, domain.ErrErasureUnsupported):
		res.External = append(res.External, ExternalErasure{System: "task-service", Status: "unsupported"})
	default:
		res.External = append(res.External, ExternalErasure{System: "task-service", Status: "failed"})
	}
	return res, nil
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
