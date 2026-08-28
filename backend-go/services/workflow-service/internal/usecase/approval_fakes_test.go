package usecase

import (
	"context"
	"errors"
	"sort"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// errUsecaseTestSimulatedFailure is the injected mid-transaction failure
// fakeApprovalRepositoryTx.Update/CreateTx return once failAfterWrites is
// exceeded — used by tests asserting "a failure between two writes leaves
// neither side effect committed."
var errUsecaseTestSimulatedFailure = errors.New("usecase test: simulated mid-transaction failure")

// fakeApprovalRepository is an in-memory ApprovalRepository with the same
// genuine stage-then-commit-or-discard WithTx contract as
// fakeTemplateRepository.WithTx — see that type's doc comment.
type fakeApprovalRepository struct {
	approvals map[string]domain.Approval
	// templates is shared with a fakeTemplateRepository so
	// ApprovalRepositoryTx.Templates() can atomically touch the SAME
	// template rows a real cross-repository transaction would — see
	// PublishTemplate/ResolveApproval, which pass a fakeTemplateRepository
	// wired to the SAME underlying map as this field.
	templates *fakeTemplateRepository

	listErr   error
	withTxErr error
	// txFailAfterWrites injects a failure into THIS tx's own approval
	// writes (Update/CreateTx) — see fakeTemplateRepositoryTx's doc
	// comment for the 1-indexed convention.
	txFailAfterWrites int
	// templatesTxFailAfterWrites injects a failure into the SAME
	// transaction's Templates() writes — a SEPARATE counter from
	// txFailAfterWrites, letting a test express "the approval write
	// succeeds but the paired template write fails" (ResolveApproval's
	// atomic-apply case) independently of the reverse.
	templatesTxFailAfterWrites int
}

func newFakeApprovalRepository(templates *fakeTemplateRepository) *fakeApprovalRepository {
	return &fakeApprovalRepository{approvals: make(map[string]domain.Approval), templates: templates}
}

func (f *fakeApprovalRepository) ListPending(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Approval, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var matched []domain.Approval
	for _, a := range f.approvals {
		if a.TenantID != tenantID || a.Status != domain.ApprovalPending {
			continue
		}
		if a.ID <= pageToken {
			continue
		}
		matched = append(matched, a)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID < matched[j].ID })
	if int32(len(matched)) > pageSize {
		matched = matched[:pageSize]
	}
	next := ""
	if int32(len(matched)) == pageSize && len(matched) > 0 {
		next = matched[len(matched)-1].ID
	}
	return matched, next, nil
}

func (f *fakeApprovalRepository) WithTx(ctx context.Context, fn func(tx ApprovalRepositoryTx) error) error {
	if f.withTxErr != nil {
		return f.withTxErr
	}
	stagedTemplates := &fakeTemplateRepositoryTx{repo: f.templates, writes: make(map[string]domain.WorkflowTemplate), usageIncrements: make(map[string]int32), failAfterWrites: f.templatesTxFailAfterWrites}
	staged := &fakeApprovalRepositoryTx{repo: f, writes: make(map[string]domain.Approval), creates: make(map[string]domain.Approval), templates: stagedTemplates}
	if err := fn(staged); err != nil {
		return err // rollback: nothing in `staged`/`stagedTemplates` is ever applied
	}
	for id, a := range staged.creates {
		f.approvals[id] = a
	}
	for id, a := range staged.writes {
		f.approvals[id] = a
	}
	for id, t := range stagedTemplates.writes {
		f.templates.templates[id] = t
	}
	for id, n := range stagedTemplates.usageIncrements {
		t := f.templates.templates[id]
		t.UsageCount += n
		f.templates.templates[id] = t
	}
	f.templates.executions = append(f.templates.executions, stagedTemplates.executions...)
	return nil
}

// fakeApprovalRepositoryTx is fakeApprovalRepository.WithTx's tx-scoped
// handle.
type fakeApprovalRepositoryTx struct {
	repo            *fakeApprovalRepository
	writes          map[string]domain.Approval
	creates         map[string]domain.Approval
	templates       *fakeTemplateRepositoryTx
	failAfterWrites int
	writeCount      int
}

func (tx *fakeApprovalRepositoryTx) current(id string) (domain.Approval, bool) {
	if a, ok := tx.writes[id]; ok {
		return a, ok
	}
	if a, ok := tx.creates[id]; ok {
		return a, ok
	}
	a, ok := tx.repo.approvals[id]
	return a, ok
}

func (tx *fakeApprovalRepositoryTx) Get(ctx context.Context, approvalID string) (domain.Approval, error) {
	a, ok := tx.current(approvalID)
	if !ok {
		return domain.Approval{}, domain.ErrApprovalNotFound
	}
	return a, nil
}

// checkFailure follows fakeTemplateRepositoryTx's exact 1-indexed
// failAfterWrites convention (see that type's doc comment).
func (tx *fakeApprovalRepositoryTx) checkFailure() error {
	tx.writeCount++
	if tx.failAfterWrites > 0 && tx.writeCount >= tx.failAfterWrites {
		return errUsecaseTestSimulatedFailure
	}
	return nil
}

func (tx *fakeApprovalRepositoryTx) Update(ctx context.Context, approval domain.Approval) error {
	if err := tx.checkFailure(); err != nil {
		return err
	}
	if _, ok := tx.current(approval.ID); !ok {
		return domain.ErrApprovalNotFound
	}
	tx.writes[approval.ID] = approval
	return nil
}

func (tx *fakeApprovalRepositoryTx) Templates() TemplateRepositoryTx {
	return tx.templates
}

func (tx *fakeApprovalRepositoryTx) CreateTx(ctx context.Context, approval domain.Approval) error {
	if err := tx.checkFailure(); err != nil {
		return err
	}
	for _, existing := range tx.repo.approvals {
		if existing.TemplateID == approval.TemplateID && existing.Status == domain.ApprovalPending {
			return domain.ErrApprovalAlreadyPending
		}
	}
	for _, existing := range tx.creates {
		if existing.TemplateID == approval.TemplateID && existing.Status == domain.ApprovalPending {
			return domain.ErrApprovalAlreadyPending
		}
	}
	tx.creates[approval.ID] = approval
	return nil
}

// fakeOPAChecker is an in-memory OPAChecker — a fixed set of admin user ids.
type fakeOPAChecker struct {
	admins map[string]bool
}

func newFakeOPAChecker(adminUserIDs ...string) *fakeOPAChecker {
	admins := make(map[string]bool, len(adminUserIDs))
	for _, id := range adminUserIDs {
		admins[id] = true
	}
	return &fakeOPAChecker{admins: admins}
}

func (f *fakeOPAChecker) IsAdmin(ctx context.Context, userID string) bool {
	return f.admins[userID]
}
