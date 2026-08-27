package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// hasAllTags reports whether every tag in want is present in have — the
// AND-filter semantics ListTemplates' Tags input requires.
func hasAllTags(have, want []string) bool {
	set := make(map[string]bool, len(have))
	for _, t := range have {
		set[t] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

// fakeTemplateRepository is an in-memory TemplateRepository — never touches
// real Postgres, used by create_template_test.go, list_templates_test.go,
// and resolve_template_test.go.
type fakeTemplateRepository struct {
	templates  map[string]domain.WorkflowTemplate
	getErr     error
	createErr  error
	listErr    error
	resolveErr error
	updateErr  error

	// updateCalls counts Update invocations — update_template_test.go
	// asserts on this to confirm the cycle check short-circuits before any
	// write is attempted.
	updateCalls int
	// lastUpdateExpectedVersion captures the last expectedVersion Update
	// was called with, so tests can confirm it's forwarded unchanged.
	lastUpdateExpectedVersion int32
	// lastUpdateBumpVersion captures the last bumpVersion Update was called
	// with — TASK-WF-01-06's breaking-change + active-usage gate.
	lastUpdateBumpVersion bool

	// withTxErr, if set, makes WithTx fail before fn is even called (a
	// begin-tx failure) — distinct from fn itself returning an error
	// (rollback), which withTx below handles via its own staged-write
	// discard.
	withTxErr error
	// executions records every CreateExecution call made through a
	// COMMITTED transaction — staged writes from a rolled-back tx never
	// land here, mirroring a real Postgres ROLLBACK.
	executions []domain.WorkflowExecution
	// txFailAfterWrites configures the NEXT WithTx call's tx to fail
	// starting with its (1-indexed) Nth write op — 0 means "never fail" —
	// the "inject a failure between two writes" mechanism TASK-WF-03-05's
	// tests need.
	txFailAfterWrites int
	// ratings is committed (template_id|user_id) -> stars — TASK-WF-03-07's
	// one-rating-per-(user,template) store, mirroring workflow.ratings'
	// UNIQUE constraint. Staged writes from a rolled-back tx never land
	// here, same commit discipline as templates/executions above.
	ratings map[string]int32
}

func newFakeTemplateRepository() *fakeTemplateRepository {
	return &fakeTemplateRepository{templates: make(map[string]domain.WorkflowTemplate), ratings: make(map[string]int32)}
}

// ratingKey is fakeTemplateRepository.ratings' composite key shape.
func ratingKey(templateID, userID string) string { return templateID + "|" + userID }

func (f *fakeTemplateRepository) CreateTemplate(ctx context.Context, tmpl domain.WorkflowTemplate) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.templates[tmpl.ID] = tmpl
	return nil
}

func (f *fakeTemplateRepository) GetTemplate(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, error) {
	if f.getErr != nil {
		return domain.WorkflowTemplate{}, f.getErr
	}
	t, ok := f.templates[id]
	if !ok || t.TenantID != tenantID {
		return domain.WorkflowTemplate{}, domain.ErrTemplateNotFound
	}
	return t, nil
}

// ListTemplates mirrors the real repository's keyset-pagination contract
// (id-ordered, cursor = last-seen id) closely enough to exercise
// usecase.ListTemplates without needing real Postgres.
// ListTemplates supports query/tags filtering and the "trending" sort
// meaningfully (both operate on real domain.WorkflowTemplate fields); the
// "recent" sort is accepted (never errors) but falls back to id order —
// domain.WorkflowTemplate has no UpdatedAt field for a fake to sort by,
// so real "recent" ordering is verified at the postgres integration-test
// layer instead (see repository_test.go), not here.
func (f *fakeTemplateRepository) ListTemplates(ctx context.Context, tenantID, scope, query string, tags []string, sortOrder, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var matched []domain.WorkflowTemplate
	for _, t := range f.templates {
		if t.TenantID != tenantID {
			continue
		}
		if scope != "" && string(t.Scope) != scope {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(t.Name), strings.ToLower(query)) && !strings.Contains(strings.ToLower(t.Description), strings.ToLower(query)) {
			continue
		}
		if len(tags) > 0 && !hasAllTags(t.Tags, tags) {
			continue
		}
		if sortOrder == "" && t.ID <= pageToken {
			continue // id-keyset only applies to the default (id-ordered) sort
		}
		matched = append(matched, t)
	}
	switch sortOrder {
	case "trending":
		sort.Slice(matched, func(i, j int) bool {
			if matched[i].UsageCount != matched[j].UsageCount {
				return matched[i].UsageCount > matched[j].UsageCount
			}
			if matched[i].RatingSum != matched[j].RatingSum {
				return matched[i].RatingSum > matched[j].RatingSum
			}
			return matched[i].ID < matched[j].ID
		})
	default:
		sort.Slice(matched, func(i, j int) bool { return matched[i].ID < matched[j].ID })
	}
	if int32(len(matched)) > pageSize {
		matched = matched[:pageSize]
	}
	next := ""
	if int32(len(matched)) == pageSize && len(matched) > 0 {
		next = matched[len(matched)-1].ID
	}
	return matched, next, nil
}

// ResolveChain mirrors the real repository's root-first ordering
// (index 0 = topmost ancestor, last = templateID itself), depth-capped.
func (f *fakeTemplateRepository) ResolveChain(ctx context.Context, tenantID, templateID string, maxDepth int) ([]domain.WorkflowTemplate, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	var chain []domain.WorkflowTemplate
	cur, ok := f.templates[templateID]
	if !ok || cur.TenantID != tenantID {
		return nil, domain.ErrTemplateNotFound
	}
	for depth := 0; depth <= maxDepth; depth++ {
		// Prepend: each newly-visited node is an ancestor of everything
		// already in chain, so it belongs before them — this walk starts
		// at the requested template and moves outward, but the result
		// must read root-first (see the real repository's ORDER BY depth
		// DESC, which achieves the same order via SQL instead).
		chain = append([]domain.WorkflowTemplate{cur}, chain...)
		if cur.ParentTemplateID == "" {
			break
		}
		parent, ok := f.templates[cur.ParentTemplateID]
		if !ok {
			break
		}
		cur = parent
	}
	return chain, nil
}

// Update mirrors the real repository's version-bump-on-write contract: on
// success it returns tmpl with Version = expectedVersion+1 (the bumped
// value a real conditional UPDATE's RETURNING clause would produce).
func (f *fakeTemplateRepository) Update(ctx context.Context, tmpl domain.WorkflowTemplate, expectedVersion int32, bumpVersion bool) (domain.WorkflowTemplate, error) {
	f.updateCalls++
	f.lastUpdateExpectedVersion = expectedVersion
	f.lastUpdateBumpVersion = bumpVersion
	if f.updateErr != nil {
		return domain.WorkflowTemplate{}, f.updateErr
	}
	if bumpVersion {
		tmpl.Version = expectedVersion + 1
	} else {
		tmpl.Version = expectedVersion
	}
	f.templates[tmpl.ID] = tmpl
	return tmpl, nil
}

// GetByShareToken scans f.templates for a matching ShareToken — a linear
// scan is fine for a fake with a handful of fixture rows.
func (f *fakeTemplateRepository) GetByShareToken(ctx context.Context, shareToken string) (domain.WorkflowTemplate, error) {
	for _, t := range f.templates {
		if t.ShareToken != "" && t.ShareToken == shareToken {
			return t, nil
		}
	}
	return domain.WorkflowTemplate{}, domain.ErrTemplateNotFound
}

// SetShareToken mirrors the real repository's not-tenant-scoped contract
// (the caller already confirmed ownership via GetTemplate).
func (f *fakeTemplateRepository) SetShareToken(ctx context.Context, templateID, token string) error {
	t, ok := f.templates[templateID]
	if !ok {
		return domain.ErrTemplateNotFound
	}
	t.ShareToken = token
	f.templates[templateID] = t
	return nil
}

// WithTx implements a genuine stage-then-commit-or-discard transaction
// over f.templates/f.executions — a rolled-back write (fn returns a
// non-nil error) must be provably invisible afterward, which the real
// tests (TASK-WF-03-05's "inject a failure between two writes, assert
// neither side effect landed") depend on; a shared in-memory map mutated
// directly, with no staging, couldn't express that.
func (f *fakeTemplateRepository) WithTx(ctx context.Context, fn func(tx TemplateRepositoryTx) error) error {
	if f.withTxErr != nil {
		return f.withTxErr
	}
	staged := &fakeTemplateRepositoryTx{repo: f, writes: make(map[string]domain.WorkflowTemplate), usageIncrements: make(map[string]int32), ratingWrites: make(map[string]int32), failAfterWrites: f.txFailAfterWrites}
	if err := fn(staged); err != nil {
		return err // rollback: nothing in `staged` is ever applied to f
	}
	for id, t := range staged.writes {
		f.templates[id] = t
	}
	for id, n := range staged.usageIncrements {
		t := f.templates[id]
		t.UsageCount += n
		f.templates[id] = t
	}
	for key, stars := range staged.ratingWrites {
		f.ratings[key] = stars
	}
	f.executions = append(f.executions, staged.executions...)
	return nil
}

// fakeTemplateRepositoryTx is fakeTemplateRepository.WithTx's tx-scoped
// handle — see WithTx's doc comment for the stage-then-commit rationale.
type fakeTemplateRepositoryTx struct {
	repo            *fakeTemplateRepository
	writes          map[string]domain.WorkflowTemplate
	usageIncrements map[string]int32
	// ratingWrites is UpsertRating's staged (template_id|user_id) -> stars
	// write — separate from writes/usageIncrements since it commits into
	// fakeTemplateRepository.ratings, a different map than templates.
	ratingWrites    map[string]int32
	executions      []domain.WorkflowExecution
	failAfterWrites int // if > 0, the (1-indexed) write call after which every subsequent call errors — simulates "crash mid-transaction"
	writeCount      int
}

// current returns id's current value, preferring an already-staged write
// within this same tx over the committed repo state — later ops within one
// WithTx call see earlier ops' effects, matching real transaction semantics.
func (tx *fakeTemplateRepositoryTx) current(id string) (domain.WorkflowTemplate, bool) {
	if t, ok := tx.writes[id]; ok {
		return t, true
	}
	t, ok := tx.repo.templates[id]
	return t, ok
}

// checkFailure returns a simulated error starting with this tx's
// failAfterWrites'th write call (1-indexed; 0 disables failure injection
// entirely) — e.g. failAfterWrites=1 fails on the very first write,
// failAfterWrites=2 lets the first succeed and fails the second onward.
func (tx *fakeTemplateRepositoryTx) checkFailure() error {
	tx.writeCount++
	if tx.failAfterWrites > 0 && tx.writeCount >= tx.failAfterWrites {
		return errors.New("fakeTemplateRepositoryTx: simulated failure")
	}
	return nil
}

func (tx *fakeTemplateRepositoryTx) UpdateVisibility(ctx context.Context, tmpl domain.WorkflowTemplate) (domain.WorkflowTemplate, error) {
	if err := tx.checkFailure(); err != nil {
		return domain.WorkflowTemplate{}, err
	}
	if _, ok := tx.current(tmpl.ID); !ok {
		return domain.WorkflowTemplate{}, domain.ErrTemplateNotFound
	}
	tx.writes[tmpl.ID] = tmpl
	return tmpl, nil
}

func (tx *fakeTemplateRepositoryTx) SetVisibility(ctx context.Context, templateID string, v domain.Visibility) error {
	if err := tx.checkFailure(); err != nil {
		return err
	}
	t, ok := tx.current(templateID)
	if !ok {
		return domain.ErrTemplateNotFound
	}
	t.Visibility = v
	tx.writes[templateID] = t
	return nil
}

func (tx *fakeTemplateRepositoryTx) CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error {
	if err := tx.checkFailure(); err != nil {
		return err
	}
	tx.executions = append(tx.executions, exec)
	return nil
}

func (tx *fakeTemplateRepositoryTx) IncrementUsageCount(ctx context.Context, templateID string) error {
	if err := tx.checkFailure(); err != nil {
		return err
	}
	if _, ok := tx.current(templateID); !ok {
		return domain.ErrTemplateNotFound
	}
	tx.usageIncrements[templateID]++
	return nil
}

// UpsertRating mirrors postgres.templateTx.UpsertRating's contract: one
// rating per (templateID, userID) — a second call from the same user
// UPDATES their prior stars (delta-adjusts rating_sum, leaves rating_count
// alone) rather than double-counting (delta = full stars, rating_count+1
// for a genuinely new rating).
func (tx *fakeTemplateRepositoryTx) UpsertRating(ctx context.Context, templateID, userID string, stars int32) (RateTemplateResult, error) {
	if err := tx.checkFailure(); err != nil {
		return RateTemplateResult{}, err
	}
	t, ok := tx.current(templateID)
	if !ok {
		return RateTemplateResult{}, domain.ErrTemplateNotFound
	}

	key := ratingKey(templateID, userID)
	oldStars, isExisting := tx.repo.ratings[key]
	if staged, ok := tx.ratingWrites[key]; ok {
		oldStars, isExisting = staged, true
	}

	sumDelta := stars
	if isExisting {
		sumDelta = stars - oldStars
	} else {
		t.RatingCount++
	}
	t.RatingSum += sumDelta

	tx.ratingWrites[key] = stars
	tx.writes[templateID] = t
	return RateTemplateResult{RatingSum: t.RatingSum, RatingCount: t.RatingCount}, nil
}
