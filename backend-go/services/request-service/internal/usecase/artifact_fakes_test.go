package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// artEnv is an in-memory world for the artifact and clarification use cases: the lifecycle store plus
// revisions, clarifications and decisions. Its InTx rolls all of them back together, like one database.
type artEnv struct {
	s           *lcStore
	revisions   map[string][]domain.RequestRevision
	clars       map[string]*domain.Clarification
	decisions   map[string]*domain.Decision
	decHistory  []domain.DecisionHistory
	locked      map[string]bool
	failRevAdd  error
	failMarkExp error
	reqNumbers  map[string]int64
}

func newArtEnv() *artEnv {
	return &artEnv{
		s: newLcStore(), revisions: map[string][]domain.RequestRevision{}, clars: map[string]*domain.Clarification{},
		decisions: map[string]*domain.Decision{}, locked: map[string]bool{},
	}
}

type artSnapshot struct {
	revisions map[string][]domain.RequestRevision
	clars     map[string]domain.Clarification
	decisions map[string]domain.Decision
	hist      int
}

func (e *artEnv) snapshot() artSnapshot {
	sn := artSnapshot{revisions: map[string][]domain.RequestRevision{}, clars: map[string]domain.Clarification{}, decisions: map[string]domain.Decision{}, hist: len(e.decHistory)}
	for k, v := range e.revisions {
		sn.revisions[k] = append([]domain.RequestRevision(nil), v...)
	}
	for k, v := range e.clars {
		sn.clars[k] = cloneClar(*v)
	}
	for k, v := range e.decisions {
		sn.decisions[k] = *v
	}
	return sn
}

func cloneClar(c domain.Clarification) domain.Clarification {
	c.Questions = append([]domain.ClarificationQuestion(nil), c.Questions...)
	c.Assignees = append([]domain.Principal(nil), c.Assignees...)
	return c
}

func (e *artEnv) restore(sn artSnapshot) {
	e.revisions = sn.revisions
	e.clars = map[string]*domain.Clarification{}
	for k, v := range sn.clars {
		c := v
		e.clars[k] = &c
	}
	e.decisions = map[string]*domain.Decision{}
	for k, v := range sn.decisions {
		d := v
		e.decisions[k] = &d
	}
	e.decHistory = e.decHistory[:sn.hist]
}

func (e *artEnv) InTransaction(ctx context.Context) bool { return e.s.InTransaction(ctx) }

func (e *artEnv) InTx(ctx context.Context, fn func(context.Context) error) error {
	if e.s.InTransaction(ctx) {
		return fn(ctx)
	}
	sn := e.snapshot()
	err := e.s.InTx(ctx, fn)
	if err != nil {
		e.restore(sn)
	}
	e.locked = map[string]bool{} // row locks end with the outermost transaction
	return err
}

// ---- RequestContentWriter

func (e *artEnv) UpdateContent(_ context.Context, req domain.Request, expected int64) (domain.Request, error) {
	cur, ok := e.s.requests[req.ID]
	if !ok {
		return domain.Request{}, domain.ErrRequestNotFound(req.ID)
	}
	if cur.Version != expected || cur.ContentRevision != req.ContentRevision-1 {
		return domain.Request{}, domain.ErrRequestVersionConflict(req.ID, expected)
	}
	cur.Title, cur.Body = req.Title, req.Body
	cur.AcceptanceCriteriaJSON, cur.TypeFieldsJSON = req.AcceptanceCriteriaJSON, req.TypeFieldsJSON
	cur.ContentSchemaVersion, cur.ContentRevision, cur.ContentDigest = req.ContentSchemaVersion, req.ContentRevision, req.ContentDigest
	cur.Version++
	e.s.requests[cur.ID] = cur
	return cur, nil
}

// ---- RequestRevisionRepository

func (e *artEnv) Append(_ context.Context, rev domain.RequestRevision) error {
	if e.failRevAdd != nil {
		return e.failRevAdd
	}
	for _, r := range e.revisions[rev.RequestID] {
		if r.Revision == rev.Revision {
			return domain.ErrRequestVersionConflict(rev.RequestID, int64(rev.Revision))
		}
	}
	e.revisions[rev.RequestID] = append(e.revisions[rev.RequestID], rev)
	return nil
}

func (e *artEnv) Get(ctx context.Context, requestID string, revision int) (domain.RequestRevision, error) {
	for _, r := range e.revisions[requestID] {
		if r.Revision == revision {
			return r, nil
		}
	}
	return domain.RequestRevision{}, domain.ErrRequestRevisionNotFound(requestID, revision)
}

func (e *artEnv) List(_ context.Context, requestID string, after, limit int) ([]domain.RequestRevision, error) {
	var out []domain.RequestRevision
	for _, r := range e.revisions[requestID] {
		if r.Revision > after {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Revision < out[j].Revision })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// revisionsView adapts artEnv.Get/List/Append to RequestRevisionRepository without a name clash with ClarificationRepository.
type revisionsView struct{ e *artEnv }

func (v revisionsView) Append(ctx context.Context, r domain.RequestRevision) error {
	return v.e.Append(ctx, r)
}
func (v revisionsView) Get(ctx context.Context, id string, rev int) (domain.RequestRevision, error) {
	return v.e.Get(ctx, id, rev)
}
func (v revisionsView) List(ctx context.Context, id string, after, limit int) ([]domain.RequestRevision, error) {
	return v.e.List(ctx, id, after, limit)
}

// ---- ClarificationRepository

type clarView struct{ e *artEnv }

func (v clarView) Insert(_ context.Context, c domain.Clarification) error {
	for _, x := range v.e.clars {
		if x.RequestID == c.RequestID && x.Status == domain.ClarificationStatusOpen && c.Status == domain.ClarificationStatusOpen {
			return domain.ErrClarificationStateNotAllowed("this request already has an open clarification")
		}
		if x.RequestID == c.RequestID && x.Seq == c.Seq {
			return domain.ErrRequestVersionConflict(c.RequestID, int64(c.Seq))
		}
	}
	cp := cloneClar(c)
	v.e.clars[c.ID] = &cp
	return nil
}

func (v clarView) Get(_ context.Context, id string) (domain.Clarification, error) {
	c, ok := v.e.clars[id]
	if !ok {
		return domain.Clarification{}, domain.ErrClarificationNotFound(id)
	}
	return cloneClar(*c), nil
}

func (v clarView) GetOpenByRequest(_ context.Context, requestID string) (*domain.Clarification, error) {
	for _, c := range v.e.clars {
		if c.RequestID == requestID && c.Status == domain.ClarificationStatusOpen {
			cp := cloneClar(*c)
			return &cp, nil
		}
	}
	return nil, nil
}

func (v clarView) List(_ context.Context, f ClarificationListFilter) ([]domain.Clarification, error) {
	var out []domain.Clarification
	for _, c := range v.e.clars {
		if c.RequestID == f.RequestID && c.Seq > f.AfterSeq && (f.Status == "" || c.Status == f.Status) {
			out = append(out, cloneClar(*c))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (v clarView) NextSeq(_ context.Context, requestID string) (int, error) {
	max := 0
	for _, c := range v.e.clars {
		if c.RequestID == requestID && c.Seq > max {
			max = c.Seq
		}
	}
	return max + 1, nil
}

func (v clarView) MaxRound(_ context.Context, requestID string, source domain.ClarificationSource, ref string) (int, error) {
	max := 0
	for _, c := range v.e.clars {
		if c.RequestID == requestID && c.Source == source && c.SourceRef == ref && c.Round > max {
			max = c.Round
		}
	}
	return max, nil
}

func (v clarView) guard(id string, expected int64) (*domain.Clarification, error) {
	c, ok := v.e.clars[id]
	if !ok {
		return nil, domain.ErrClarificationNotFound(id)
	}
	if c.Status != domain.ClarificationStatusOpen {
		return nil, domain.ErrClarificationNotOpen(id, c.Status)
	}
	if expected != 0 && c.Version != expected {
		return nil, domain.ErrClarificationVersionConflict(id, expected)
	}
	return c, nil
}

func (v clarView) UpdateAnswers(_ context.Context, id string, answers []AnswerRecord, expected int64) error {
	c, err := v.guard(id, expected)
	if err != nil {
		return err
	}
	c.Version++
	for _, a := range answers {
		for i := range c.Questions {
			if c.Questions[i].ID == a.QuestionID {
				c.Questions[i].Answer, c.Questions[i].AnswerSource, c.Questions[i].AnsweredBy = a.Value, a.Source, a.By
			}
		}
	}
	return nil
}

func (v clarView) MarkAnswered(_ context.Context, id string, rev int, at time.Time, expected int64) error {
	c, err := v.guard(id, expected)
	if err != nil {
		return err
	}
	c.Status, c.AnsweredAt, c.AnsweredRequestRevision, c.Version = domain.ClarificationStatusAnswered, &at, &rev, c.Version+1
	return nil
}

func (v clarView) MarkCancelled(_ context.Context, id, reason string, _ time.Time, expected int64) error {
	c, err := v.guard(id, expected)
	if err != nil {
		return err
	}
	c.Status, c.CancelReason, c.Version = domain.ClarificationStatusCancelled, reason, c.Version+1
	return nil
}

func (v clarView) ListDueRefs(_ context.Context, now time.Time, batch int) ([]ClarificationRef, error) {
	var out []ClarificationRef
	for _, c := range v.e.clars {
		if c.Status == domain.ClarificationStatusOpen && !c.DueAt.After(now) {
			out = append(out, ClarificationRef{TenantID: c.TenantID, ID: c.ID})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > batch {
		out = out[:batch]
	}
	return out, nil
}

func (v clarView) ListRemindableRefs(_ context.Context, now time.Time, batch int) ([]ClarificationRef, error) {
	var out []ClarificationRef
	for _, c := range v.e.clars {
		half := c.CreatedAt.Add(c.DueAt.Sub(c.CreatedAt) / 2)
		if c.Status == domain.ClarificationStatusOpen && c.RemindedAt == nil && c.DueAt.After(now) && !now.Before(half) {
			out = append(out, ClarificationRef{TenantID: c.TenantID, ID: c.ID})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > batch {
		out = out[:batch]
	}
	return out, nil
}

// LockOpenDue models SKIP LOCKED: a row another transaction holds is skipped, and the lock lives until the transaction ends.
func (v clarView) LockOpenDue(_ context.Context, id string, now time.Time) (*domain.Clarification, error) {
	c, ok := v.e.clars[id]
	if !ok || c.Status != domain.ClarificationStatusOpen || c.DueAt.After(now) || v.e.locked[id] {
		return nil, nil
	}
	v.e.locked[id] = true
	cp := cloneClar(*c)
	return &cp, nil
}

func (v clarView) MarkExpired(_ context.Context, id string, _ time.Time) error {
	if v.e.failMarkExp != nil {
		return v.e.failMarkExp
	}
	c, err := v.guard(id, 0)
	if err != nil {
		return err
	}
	c.Status, c.Version = domain.ClarificationStatusExpired, c.Version+1
	return nil
}

func (v clarView) MarkReminded(_ context.Context, id string, at time.Time) (bool, error) {
	c, ok := v.e.clars[id]
	if !ok || c.Status != domain.ClarificationStatusOpen || c.RemindedAt != nil {
		return false, nil
	}
	c.RemindedAt = &at
	return true, nil
}

func (v clarView) ListPendingForUser(_ context.Context, f PendingFilter) ([]domain.Clarification, string, error) {
	var out []domain.Clarification
	for _, c := range v.e.clars {
		if c.Status != domain.ClarificationStatusOpen {
			continue
		}
		if f.IsAdmin {
			out = append(out, cloneClar(*c))
			continue
		}
		req := v.e.s.requests[c.RequestID]
		for _, a := range c.Assignees {
			if (a.Kind == domain.PrincipalKindUser && a.ID == f.UserID) || (a.Kind == domain.PrincipalKindReporter && req.ReporterID == f.UserID) ||
				(a.Kind == domain.PrincipalKindTeam && containsString(f.Teams, a.ID)) || (a.Kind == domain.PrincipalKindRole && containsString(f.Roles, a.ID)) {
				out = append(out, cloneClar(*c))
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, "", nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ---- DecisionRepository

type decView struct{ e *artEnv }

func (v decView) Insert(_ context.Context, d domain.Decision) error {
	for _, x := range v.e.decisions {
		if x.SubjectKind == d.SubjectKind && x.SubjectID == d.SubjectID && x.IsLive() && d.IsLive() {
			return domain.ErrDecisionLiveExists(string(d.SubjectKind) + ":" + d.SubjectID)
		}
	}
	cp := d
	v.e.decisions[d.ID] = &cp
	return nil
}

func (v decView) Get(_ context.Context, id string) (domain.Decision, error) {
	d, ok := v.e.decisions[id]
	if !ok {
		return domain.Decision{}, domain.ErrDecisionNotFound(id)
	}
	return *d, nil
}

func (v decView) GetLiveBySubject(_ context.Context, kind domain.DecisionSubjectKind, subject string) (*domain.Decision, error) {
	for _, d := range v.e.decisions {
		if d.SubjectKind == kind && d.SubjectID == subject && d.IsLive() {
			cp := *d
			return &cp, nil
		}
	}
	return nil, nil
}

func (v decView) Update(_ context.Context, d domain.Decision, expected int64) (domain.Decision, error) {
	cur, ok := v.e.decisions[d.ID]
	if !ok {
		return domain.Decision{}, domain.ErrDecisionNotFound(d.ID)
	}
	if cur.Version != expected {
		return domain.Decision{}, domain.ErrDecisionVersionConflict(d.ID, expected)
	}
	d.Version = expected + 1
	*cur = d
	return d, nil
}

func (v decView) AppendHistory(_ context.Context, h domain.DecisionHistory) error {
	v.e.decHistory = append(v.e.decHistory, h)
	return nil
}

func (v decView) ListHistory(_ context.Context, id string) ([]domain.DecisionHistory, error) {
	var out []domain.DecisionHistory
	for _, h := range v.e.decHistory {
		if h.DecisionID == id {
			out = append(out, h)
		}
	}
	return out, nil
}

func (v decView) List(_ context.Context, f DecisionListFilter) ([]domain.Decision, error) {
	var out []domain.Decision
	for _, d := range v.e.decisions {
		if d.RequestID == f.RequestID && d.Seq > f.AfterSeq && (f.Status == "" || d.Status == f.Status) {
			out = append(out, *d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out, nil
}

func (v decView) NextSeq(_ context.Context, requestID string) (int, error) {
	max := 0
	for _, d := range v.e.decisions {
		if d.RequestID == requestID && d.Seq > max {
			max = d.Seq
		}
	}
	return max + 1, nil
}

func (v decView) supersede(match func(*domain.Decision) bool) []domain.Decision {
	var out []domain.Decision
	for _, d := range v.e.decisions {
		if d.IsLive() && match(d) {
			d.Status = domain.DecisionStatusSuperseded
			d.Version++
			out = append(out, *d)
		}
	}
	return out
}

func (v decView) SupersedeLiveBySubject(_ context.Context, kind domain.DecisionSubjectKind, subject string) ([]domain.Decision, error) {
	return v.supersede(func(d *domain.Decision) bool { return d.SubjectKind == kind && d.SubjectID == subject }), nil
}

func (v decView) SupersedeLiveByRequest(_ context.Context, requestID string) ([]domain.Decision, error) {
	return v.supersede(func(d *domain.Decision) bool { return d.RequestID == requestID }), nil
}

// ---- index / relations / coverage

type artIndex struct {
	entries map[string]domain.IndexEntry
	seqs    map[string]int // solution id -> seq
	maxSeq  map[string]int // request id -> highest seq
	solReq  map[string]string
	clash   int // SetSolutionSeq fails with a seq conflict this many times
	plans   map[string]int
}

func newArtIndex() *artIndex {
	return &artIndex{entries: map[string]domain.IndexEntry{}, seqs: map[string]int{}, maxSeq: map[string]int{}, plans: map[string]int{}, solReq: map[string]string{}}
}

func (x *artIndex) Insert(_ context.Context, e domain.IndexEntry) (bool, error) {
	if _, ok := x.entries[e.DisplayID]; ok {
		return false, nil
	}
	x.entries[e.DisplayID] = e
	if e.Kind == domain.DisplayKindPlan {
		x.plans[e.RequestID]++
	}
	return true, nil
}

func (x *artIndex) Resolve(ctx context.Context, id string) (domain.IndexEntry, error) {
	e, ok := x.entries[id]
	if !ok {
		return domain.IndexEntry{}, domain.ErrArtifactNotFound(id)
	}
	if t, _ := tenant.TenantID(ctx); e.TenantID != "" && t != e.TenantID {
		return domain.IndexEntry{}, domain.ErrArtifactNotFound(id)
	}
	return e, nil
}

func (x *artIndex) NextSolutionSeq(_ context.Context, requestID string) (int, error) {
	return x.maxSeq[requestID] + 1, nil
}

func (x *artIndex) NextPlanSeq(_ context.Context, requestID string) (int, error) {
	return x.plans[requestID] + 1, nil
}

func (x *artIndex) SetSolutionSeq(_ context.Context, solutionID string, seq int) error {
	req := x.solReq[solutionID]
	if x.clash > 0 {
		x.clash--
		x.maxSeq[req]++ // another writer took the number between the read and the write
		return domain.ErrArtifactSeqConflict()
	}
	x.seqs[solutionID] = seq
	if seq > x.maxSeq[req] {
		x.maxSeq[req] = seq
	}
	return nil
}

type artRelations struct{ rows []domain.ArtifactRelation }

func (r *artRelations) Insert(_ context.Context, rel domain.ArtifactRelation) (bool, error) {
	for _, x := range r.rows {
		if x.Rel == rel.Rel && x.FromKind == rel.FromKind && x.FromID == rel.FromID && x.ToKind == rel.ToKind && x.ToID == rel.ToID {
			return false, nil
		}
	}
	r.rows = append(r.rows, rel)
	return true, nil
}

func (r *artRelations) ListByRequest(_ context.Context, requestID string) ([]domain.ArtifactRelation, error) {
	var out []domain.ArtifactRelation
	for _, x := range r.rows {
		if x.RequestID == requestID {
			out = append(out, x)
		}
	}
	return out, nil
}

// artCoverage keeps rows per plan and joins the env's rollback through the snapshot hook.
type artCoverage struct {
	rows map[string][]domain.CoverageRow
}

func (c *artCoverage) ReplaceForPlan(_ context.Context, requestID, planTaskID string, rows []domain.CoverageRow) error {
	key := requestID + "|" + planTaskID
	c.rows[key] = append([]domain.CoverageRow(nil), rows...)
	return nil
}

func (c *artCoverage) ListByRequest(_ context.Context, requestID string) ([]domain.CoverageRow, error) {
	var out []domain.CoverageRow
	for k, rows := range c.rows {
		if len(k) > len(requestID) && k[:len(requestID)] == requestID {
			out = append(out, rows...)
		}
	}
	return out, nil
}

type fakeTree struct {
	sub PlanSubtree
	err error
}

func (f fakeTree) GetSubtree(context.Context, string) (PlanSubtree, error) { return f.sub, f.err }

var errUnexpected = errors.New("unexpected call")

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprint(err))
	}
	return b
}

func newID() string { return uuid.NewString() }
