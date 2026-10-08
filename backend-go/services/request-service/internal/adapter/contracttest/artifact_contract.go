package contracttest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"github.com/stablyai/orca-go/services/request-service/schemas"
)

// ArtifactEnv adds the CR-REQ-027 and CR-REQ-028 repositories to the intake environment.
type ArtifactEnv struct {
	IntakeEnv
	Content        usecase.RequestContentWriter
	Revisions      usecase.RequestRevisionRepository
	Index          usecase.ArtifactIndexRepository
	Relations      usecase.ArtifactRelationRepository
	Coverage       usecase.RequestCoverageRepository
	Clarifications usecase.ClarificationRepository
	Decisions      usecase.DecisionRepository
}

type failOnSubject struct {
	usecase.OutboxWriter
	subject string
}

func (f failOnSubject) InsertOutboxEvent(ctx context.Context, ev domain.OutboxEvent) error {
	if ev.Subject == f.subject {
		return errors.New("forced outbox failure")
	}
	return f.OutboxWriter.InsertOutboxEvent(ctx, ev)
}

func userIn(tenantID, userID string) context.Context {
	return tenant.WithUserID(CtxForTenant(tenantID), userID)
}

// RunArtifactContract covers CR-REQ-027 on the dialect's real database.
func RunArtifactContract(t *testing.T, newEnv func(t *testing.T) ArtifactEnv) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env ArtifactEnv)
	}{
		{"CreateWritesRevisionOneSameTx", artCreateRevisionOne},
		{"CreateFailureLeavesNoRevisionAndNoNumberGap", artCreateFailureAtomic},
		{"AppendConcurrentOneWins", artAppendConcurrent},
		{"VietnameseSurvivesJSONColumns", artVietnameseRoundTrip},
		{"RetiredACKeepsOldRevisionReadable", artRetiredACReadable},
		{"RevisionUniquePerNumber", artRevisionUnique},
		{"IndexResolveTenantScopedAndIdempotent", artIndexTenantScoped},
		{"Mint20SolutionsConcurrentDistinctSeq", artMintConcurrent},
		{"RelationUniqueTupleIdempotent", artRelationIdempotent},
		{"CoverageReplacesWholeBlock", artCoverageReplace},
		{"CoverageRollsBackWithCaller", artCoverageRollback},
		{"GraphMergesRelationsAndLinks", artGraphMerge},
		{"ContentWriteTenantIsolation", artContentTenantIsolation},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, env) })
	}
}

func artCreate(env ArtifactEnv, outbox usecase.OutboxWriter) *usecase.CreateRequest {
	reg, err := schemas.Default()
	if err != nil {
		panic(err)
	}
	tr := usecase.NewTransitionRequest(env.Requests, env.Tx.(usecase.TxScope), outbox)
	rec := usecase.NewRequestRevisionRecorder(env.Revisions, outbox).WithSchemas(reg).WithIndex(usecase.NewMintArtifactIDs(env.Index))
	return usecase.NewCreateRequest(env.Requests, env.Idempotency, env.Tx, outbox, tr, nil).WithCreationRecorder(rec)
}

func artAppend(env ArtifactEnv) *usecase.AppendRequestRevision {
	reg, _ := schemas.Default()
	return usecase.NewAppendRequestRevision(env.Requests, env.Content, env.Revisions, env.Tx, env.Outbox).WithSchemas(reg)
}

func artSeed(t *testing.T, env ArtifactEnv, ctx context.Context, mod func(r *domain.Request)) domain.Request {
	t.Helper()
	return createRequest(t, env.Env, ctx, mod)
}

func manualCreate(project, title string) usecase.CreateRequestInput {
	return usecase.CreateRequestInput{ProjectID: project, Title: title, Body: "Mô tả đủ dài để vượt qua ngưỡng hai mươi ký tự.", Source: domain.SourceRef{Provider: domain.SourceProviderManual}}
}

func artCreateRevisionOne(t *testing.T, env ArtifactEnv) {
	tenantID := newTenant()
	ctx := userIn(tenantID, uuid.NewString())
	in := manualCreate(uuid.NewString(), "Đăng nhập một lần")
	in.AcceptanceCriteria = []domain.ACInput{{Text: "Đăng nhập thành công về trang chủ", VerifyHint: "test"}}
	in.TypeFields = map[string]any{"goal": "SSO"}
	res, err := artCreate(env, env.Outbox).Execute(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	r, err := env.Requests.Get(ctx, res.Request.ID)
	if err != nil {
		t.Fatal(err)
	}
	rev, err := env.Revisions.Get(ctx, r.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := domain.ContentFromRequest(r)
	digest, _ := c.Digest()
	if rev.Cause != domain.RevisionCauseCreated || rev.Digest != digest || digest != r.ContentDigest || r.ContentRevision != 1 {
		t.Fatalf("revision %+v request digest %s content digest %s", rev, r.ContentDigest, digest)
	}
	if len(c.AcceptanceCriteria.Items) != 1 || c.AcceptanceCriteria.Items[0].ID != "AC-1" || c.TypeFields["goal"] != "SSO" {
		t.Fatalf("stored content %+v", c)
	}
	if !strings.Contains(string(rev.Snapshot), "Đăng nhập một lần") {
		t.Fatalf("snapshot %s", rev.Snapshot)
	}
	subjects := env.OutboxSubjects(t, tenantID)
	if indexOf(subjects, domain.SubjectRequestRevised) < 0 || indexOf(subjects, domain.SubjectRequestCreated) < 0 {
		t.Fatalf("events %v", subjects)
	}
	got, err := env.Index.Resolve(ctx, domain.FormatRequestID(r.Number))
	if err != nil || got.ArtifactID != r.ID || got.Kind != domain.DisplayKindRequest {
		t.Fatalf("REQ id must be indexed: %+v %v", got, err)
	}
}

func indexOf(list []string, s string) int {
	for i, x := range list {
		if x == s {
			return i
		}
	}
	return -1
}

func artCreateFailureAtomic(t *testing.T, env ArtifactEnv) {
	tenantID := newTenant()
	ctx := userIn(tenantID, uuid.NewString())
	// request.created is written after the recorder, so failing it proves the revision rolls back with the request.
	uc := artCreate(env, failOnSubject{OutboxWriter: env.Outbox, subject: domain.SubjectRequestCreated})
	if _, err := uc.Execute(ctx, manualCreate(uuid.NewString(), "sẽ thất bại")); err == nil {
		t.Fatal("expected failure")
	}
	if n := env.CountRows(t, "request_revisions", tenantID); n != 0 {
		t.Fatalf("%d orphan revisions", n)
	}
	if n := env.CountRows(t, "requests", tenantID); n != 0 {
		t.Fatalf("%d requests", n)
	}
	if env.NextNumberPeek(t, tenantID) != 0 {
		t.Fatal("a number was burnt")
	}
	if n := env.CountRows(t, "artifact_index", tenantID); n != 0 {
		t.Fatalf("%d orphan index rows", n)
	}
}

func artAppendConcurrent(t *testing.T, env ArtifactEnv) {
	tenantID := newTenant()
	ctx := userIn(tenantID, uuid.NewString())
	r := artSeed(t, env, ctx, nil)
	uc := artAppend(env)
	base, _ := domain.ContentFromRequest(r)
	type outcome struct {
		res usecase.AppendResult
		err error
	}
	out := make(chan outcome, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		c := base
		c.Title = fmt.Sprintf("tiêu đề của người %d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			res, err := uc.Execute(ctx, usecase.AppendInput{RequestID: r.ID, Content: c, Cause: domain.RevisionCauseEdited, ActorID: "u", ActorKind: domain.ActorKindUser, ExpectedVersion: r.Version})
			out <- outcome{res, err}
		}()
	}
	close(start)
	wg.Wait()
	close(out)
	wins, losses := 0, 0
	for o := range out {
		switch {
		case o.err == nil && o.res.Applied:
			wins++
		case o.err != nil && (errCode(o.err) == "REQUEST_VERSION_CONFLICT" || containsAny(o.err.Error(), "1213", "deadlock detected", "could not serialize")):
			losses++
		default:
			t.Fatalf("unexpected outcome: %+v %v", o.res, o.err)
		}
	}
	if wins != 1 || losses != 1 {
		t.Fatalf("wins=%d losses=%d", wins, losses)
	}
	got, _ := env.Requests.Get(ctx, r.ID)
	if got.ContentRevision != 2 {
		t.Fatalf("content_revision = %d", got.ContentRevision)
	}
	revs, _ := env.Revisions.List(ctx, r.ID, 0, 10)
	if len(revs) != 1 || revs[0].Revision != 2 {
		// the seeded request has no revision 1 row; only the winner's row exists
		t.Fatalf("revisions %+v", revs)
	}
	revised := 0
	for _, s := range env.OutboxSubjects(t, tenantID) {
		if s == domain.SubjectRequestRevised {
			revised++
		}
	}
	if revised != 1 {
		t.Fatalf("%d revised events", revised)
	}
}

func artVietnameseRoundTrip(t *testing.T, env ArtifactEnv) {
	ctx := userIn(newTenant(), uuid.NewString())
	r := artSeed(t, env, ctx, nil)
	c, _ := domain.ContentFromRequest(r)
	c.Title = "Đăng nhập bằng tài khoản Nguyễn Văn Đức"
	c.Body = "Lỗi: người dùng “Trần Thị Mỹ” không đăng nhập được ✓ — ơ ư ê ô ạ ặ ậ ề ế"
	c.TypeFields = map[string]any{"environment": "máy chủ thử nghiệm Hà Nội"}
	_, _ = c.AcceptanceCriteria.Add("Hiển thị thông báo “Mật khẩu không đúng”", "test")
	if _, err := artAppend(env).Execute(ctx, usecase.AppendInput{RequestID: r.ID, Content: c, Cause: domain.RevisionCauseEdited}); err != nil {
		t.Fatal(err)
	}
	got, _ := env.Requests.Get(ctx, r.ID)
	back, err := domain.ContentFromRequest(got)
	if err != nil {
		t.Fatal(err)
	}
	if back.Title != c.Title || back.Body != c.Body || back.TypeFields["environment"] != "máy chủ thử nghiệm Hà Nội" || back.AcceptanceCriteria.Items[0].Text != "Hiển thị thông báo “Mật khẩu không đúng”" {
		t.Fatalf("lost in the JSON columns: %+v", back)
	}
	rev, err := env.Revisions.Get(ctx, r.ID, got.ContentRevision)
	if err != nil {
		t.Fatal(err)
	}
	canon, _ := domain.CanonicalJSON(rev.Snapshot)
	if string(canon) != string(rev.Snapshot) || domain.DigestOfCanonical(rev.Snapshot) != rev.Digest {
		t.Fatalf("the stored snapshot must read back canonical and match its digest (%s)", rev.Digest)
	}
}

func artRetiredACReadable(t *testing.T, env ArtifactEnv) {
	tenantID := newTenant()
	ctx := userIn(tenantID, uuid.NewString())
	res, err := artCreate(env, env.Outbox).Execute(ctx, func() usecase.CreateRequestInput {
		in := manualCreate(uuid.NewString(), "có tiêu chí")
		in.AcceptanceCriteria = []domain.ACInput{{Text: "tiêu chí đầu"}}
		return in
	}())
	if err != nil {
		t.Fatal(err)
	}
	uc := artAppend(env)
	r := res.Request
	c, _ := domain.ContentFromRequest(r)
	next, err := c.Apply(domain.ContentPatch{AcceptanceCriteria: &[]domain.ACInput{{Text: "tiêu chí thay thế"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(ctx, usecase.AppendInput{RequestID: r.ID, Content: next, Cause: domain.RevisionCauseEdited}); err != nil {
		t.Fatal(err)
	}
	now, _ := env.Requests.Get(ctx, r.ID)
	cur, _ := domain.ContentFromRequest(now)
	if ac, _ := cur.AcceptanceCriteria.Find("AC-1"); ac.Status != domain.ACStatusRetired {
		t.Fatalf("AC-1 must be retired: %+v", cur.AcceptanceCriteria.Items)
	}
	if ac, ok := cur.AcceptanceCriteria.Find("AC-2"); !ok || ac.Status != domain.ACStatusActive {
		t.Fatalf("the replacement takes AC-2: %+v", cur.AcceptanceCriteria.Items)
	}
	old, err := env.Revisions.Get(ctx, r.ID, 1)
	if err != nil || !strings.Contains(string(old.Snapshot), `"id":"AC-1","status":"active"`) && !strings.Contains(string(old.Snapshot), `"status":"active"`) {
		t.Fatalf("revision 1 must still show AC-1 active: %v %s", err, old.Snapshot)
	}
}

func artRevisionUnique(t *testing.T, env ArtifactEnv) {
	ctx := userIn(newTenant(), uuid.NewString())
	r := artSeed(t, env, ctx, nil)
	rev := domain.RequestRevision{RequestID: r.ID, Revision: 1, Cause: domain.RevisionCauseCreated, Snapshot: []byte(`{"schema_version":1}`), Digest: strings.Repeat("a", 64), ActorKind: domain.RevisionActorSystem}
	if err := env.Revisions.Append(ctx, rev); err != nil {
		t.Fatal(err)
	}
	requireCode(t, env.Revisions.Append(ctx, rev), "REQUEST_VERSION_CONFLICT")
	rev.Revision = 2
	rev.Cause = "bogus"
	if err := env.Revisions.Append(ctx, rev); err == nil {
		t.Fatal("the cause CHECK must refuse an unknown cause")
	}
	_, err := env.Revisions.Get(ctx, r.ID, 9)
	requireCode(t, err, "REQUEST_REVISION_NOT_FOUND")
}

func artIndexTenantScoped(t *testing.T, env ArtifactEnv) {
	tA, tB := newTenant(), newTenant()
	ctxA, ctxB := userIn(tA, uuid.NewString()), userIn(tB, uuid.NewString())
	r := artSeed(t, env, ctxA, nil)
	e := domain.IndexEntry{DisplayID: "SOL-5.1", Kind: domain.DisplayKindSolution, RequestID: r.ID, ArtifactID: uuid.NewString()}
	if ok, err := env.Index.Insert(ctxA, e); err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	if ok, err := env.Index.Insert(ctxA, e); err != nil || ok {
		t.Fatalf("a repeat is a no-op, not an error: %v %v", ok, err)
	}
	if ok, err := env.Index.Insert(ctxB, domain.IndexEntry{DisplayID: "SOL-5.1", Kind: domain.DisplayKindSolution, RequestID: uuid.NewString(), ArtifactID: uuid.NewString()}); err != nil || !ok {
		t.Fatalf("the same display id in another tenant is its own row: %v %v", ok, err)
	}
	got, err := env.Index.Resolve(ctxA, "SOL-5.1")
	if err != nil || got.ArtifactID != e.ArtifactID {
		t.Fatalf("%+v %v", got, err)
	}
	_, err = env.Index.Resolve(CtxForTenant(newTenant()), "SOL-5.1")
	requireCode(t, err, "REQUEST_ARTIFACT_NOT_FOUND")
	if _, err := env.Index.Insert(ctxA, domain.IndexEntry{DisplayID: "X-1", Kind: "weird", RequestID: r.ID, ArtifactID: uuid.NewString()}); err == nil {
		t.Fatal("the kind CHECK must refuse an unknown kind")
	}
}

func artMintConcurrent(t *testing.T, env ArtifactEnv) {
	tenantID := newTenant()
	ctx := userIn(tenantID, uuid.NewString())
	r := artSeed(t, env, ctx, nil)
	const n = 20
	mint := usecase.NewMintArtifactIDs(env.Index)
	now := time.Now().UTC().Truncate(time.Microsecond)
	// The repository assigns seq on insert (solutions.seq is NOT NULL); concurrent inserts for one request must still get 1..n.
	var insWG sync.WaitGroup
	insErrs := make(chan error, n)
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		s := domain.Solution{ID: uuid.NewString(), TenantID: tenantID, RequestID: r.ID, OptionsJSON: []byte(`{"options":[{"id":"opt-1"}]}`), Version: 1, CreatedAt: now.Add(time.Duration(i) * time.Millisecond), UpdatedAt: now}
		ids[i] = s.ID
		insWG.Add(1)
		go func() {
			defer insWG.Done()
			if err := retryDeadlock(func() error { return env.Solutions.Insert(ctx, s) }); err != nil {
				insErrs <- err
			}
		}()
	}
	insWG.Wait()
	close(insErrs)
	for err := range insErrs {
		t.Fatalf("insert failed: %v", err)
	}
	var sols []domain.Solution
	inserted := map[int]bool{}
	for _, id := range ids {
		s, err := env.Solutions.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if s.Seq < 1 || s.Seq > n || inserted[s.Seq] {
			t.Fatalf("insert gave seq %d (duplicated or out of range)", s.Seq)
		}
		inserted[s.Seq] = true
		sols = append(sols, s)
	}
	var wg sync.WaitGroup
	seqs := make(chan int, n)
	errs := make(chan error, n)
	for i := range sols {
		s := sols[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := retryDeadlock(func() error {
				return env.Tx.InTx(ctx, func(txCtx context.Context) error {
					cp := s
					if err := mint.MintSolutionID(txCtx, r.Number, &cp); err != nil {
						return err
					}
					seqs <- cp.Seq
					return nil
				})
			})
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(seqs)
	close(errs)
	for err := range errs {
		t.Fatalf("mint failed: %v", err)
	}
	seen := map[int]bool{}
	for s := range seqs {
		if s < 1 || s > n || seen[s] {
			t.Fatalf("seq %d duplicated or out of range", s)
		}
		seen[s] = true
	}
	if len(seen) != n {
		t.Fatalf("%d distinct seqs", len(seen))
	}
	for k := 1; k <= n; k++ {
		if _, err := env.Index.Resolve(ctx, domain.FormatSolutionID(r.Number, k)); err != nil {
			t.Fatalf("SOL-%d.%d not indexed: %v", r.Number, k, err)
		}
	}
	if got, err := env.Index.Resolve(ctx, domain.FormatOptionID(domain.FormatSolutionID(r.Number, 1), 1)); err != nil || got.Kind != domain.DisplayKindOption {
		t.Fatalf("option id: %+v %v", got, err)
	}
}

func artRelationIdempotent(t *testing.T, env ArtifactEnv) {
	ctx := userIn(newTenant(), uuid.NewString())
	r := artSeed(t, env, ctx, nil)
	rel, err := domain.NewArtifactRelation(uuid.NewString(), "", r.ID, domain.RelationDerivedFrom, domain.NodeSolution, "SOL-1.1", domain.NodeRequest, "REQ-1")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := env.Relations.Insert(ctx, rel); err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	rel.ID = uuid.NewString()
	if ok, err := env.Relations.Insert(ctx, rel); err != nil || ok {
		t.Fatalf("a repeated tuple is a no-op: %v %v", ok, err)
	}
	other, _ := domain.NewArtifactRelation(uuid.NewString(), "", r.ID, domain.RelationImplements, domain.NodePlan, "PLN-1.1", domain.NodeOption, "SOL-1.1/opt-1")
	if _, err := env.Relations.Insert(ctx, other); err != nil {
		t.Fatal(err)
	}
	list, err := env.Relations.ListByRequest(ctx, r.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("%+v %v", list, err)
	}
	bad := other
	bad.ID, bad.Rel = uuid.NewString(), "invented"
	if _, err := env.Relations.Insert(ctx, bad); err == nil {
		t.Fatal("the rel CHECK must refuse an unknown relation")
	}
	if l, _ := env.Relations.ListByRequest(CtxForTenant(newTenant()), r.ID); len(l) != 0 {
		t.Fatal("another tenant sees nothing")
	}
}

func artCoverageReplace(t *testing.T, env ArtifactEnv) {
	ctx := userIn(newTenant(), uuid.NewString())
	r := artSeed(t, env, ctx, nil)
	plan := uuid.NewString()
	task1, task2 := uuid.NewString(), uuid.NewString()
	first := []domain.CoverageRow{{ACID: "AC-1", TaskID: task1, CheckID: "c1", PlanTaskID: plan}, {ACID: "AC-2", TaskID: task2, PlanTaskID: plan}}
	if err := env.Coverage.ReplaceForPlan(ctx, r.ID, plan, first); err != nil {
		t.Fatal(err)
	}
	otherPlan := uuid.NewString()
	if err := env.Coverage.ReplaceForPlan(ctx, r.ID, otherPlan, []domain.CoverageRow{{ACID: "AC-9", TaskID: task1, PlanTaskID: otherPlan}}); err != nil {
		t.Fatal(err)
	}
	if err := env.Coverage.ReplaceForPlan(ctx, r.ID, plan, []domain.CoverageRow{{ACID: "AC-3", TaskID: task2, CheckID: "c7", PlanTaskID: plan}}); err != nil {
		t.Fatal(err)
	}
	rows, err := env.Coverage.ListByRequest(ctx, r.ID)
	if err != nil || len(rows) != 2 {
		t.Fatalf("%+v %v", rows, err)
	}
	acs := rows[0].ACID + "," + rows[1].ACID
	if acs != "AC-3,AC-9" {
		t.Fatalf("the plan's rows are replaced as a block and other plans keep theirs: %s", acs)
	}
	if err := env.Coverage.ReplaceForPlan(ctx, r.ID, plan, nil); err != nil {
		t.Fatal(err)
	}
	if rows, _ := env.Coverage.ListByRequest(ctx, r.ID); len(rows) != 1 {
		t.Fatalf("an empty replacement clears the plan: %+v", rows)
	}
}

func artCoverageRollback(t *testing.T, env ArtifactEnv) {
	ctx := userIn(newTenant(), uuid.NewString())
	r := artSeed(t, env, ctx, nil)
	plan := uuid.NewString()
	keep := []domain.CoverageRow{{ACID: "AC-1", TaskID: uuid.NewString(), PlanTaskID: plan}}
	if err := env.Coverage.ReplaceForPlan(ctx, r.ID, plan, keep); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("commit step failed")
	err := env.Tx.InTx(ctx, func(txCtx context.Context) error {
		if err := env.Coverage.ReplaceForPlan(txCtx, r.ID, plan, []domain.CoverageRow{{ACID: "AC-7", TaskID: uuid.NewString(), PlanTaskID: plan}}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("%v", err)
	}
	rows, _ := env.Coverage.ListByRequest(ctx, r.ID)
	if len(rows) != 1 || rows[0].ACID != "AC-1" {
		t.Fatalf("the delete and insert must both roll back with the caller: %+v", rows)
	}
}

type staticTree struct{ sub usecase.PlanSubtree }

func (s staticTree) GetSubtree(context.Context, string) (usecase.PlanSubtree, error) {
	return s.sub, nil
}

func artGraphMerge(t *testing.T, env ArtifactEnv) {
	ctx := userIn(newTenant(), uuid.NewString())
	parent := artSeed(t, env, ctx, nil)
	plan := uuid.NewString()
	child := artSeed(t, env, ctx, func(r *domain.Request) { r.PlanTaskID = plan })
	link, err := domain.NewRequestLink(parent.ID, child.ID, domain.LinkReasonRelatesTo, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if err := env.Links.Insert(ctx, link); err != nil {
		t.Fatal(err)
	}
	rel, _ := domain.NewArtifactRelation(uuid.NewString(), "", child.ID, domain.RelationDerivedFrom, domain.NodeSolution, "SOL-9.1", domain.NodeRequest, "REQ-9")
	if _, err := env.Relations.Insert(ctx, rel); err != nil {
		t.Fatal(err)
	}
	t1 := uuid.NewString()
	reader := usecase.NewArtifactReader(env.Requests, env.Revisions, env.Coverage, env.Relations, env.Links, env.Index).
		WithPlanTree(staticTree{sub: usecase.PlanSubtree{Nodes: []usecase.SubtreeNode{{ID: plan, Kind: "plan"}, {ID: t1, ParentID: plan, Kind: "task"}}}})
	g, err := reader.Graph(ctx, child.ID)
	if err != nil || g.Partial || len(g.Edges) != 3 {
		t.Fatalf("%+v %v", g, err)
	}
	have := map[domain.Relation]bool{}
	for _, e := range g.Edges {
		have[e.Rel] = true
	}
	if !have[domain.RelationContains] || !have[domain.RelationDerivedFrom] || !have[domain.RelationSpawnedBy] {
		t.Fatalf("%+v", g.Edges)
	}
}

func artContentTenantIsolation(t *testing.T, env ArtifactEnv) {
	tA, tB := newTenant(), newTenant()
	ctxA, ctxB := userIn(tA, uuid.NewString()), userIn(tB, uuid.NewString())
	r := artSeed(t, env, ctxA, nil)
	next := r
	next.ContentRevision = 2
	_, err := env.Content.UpdateContent(ctxB, next, r.Version)
	requireCode(t, err, "REQUEST_NOT_FOUND")
	if _, err := env.Revisions.Get(ctxB, r.ID, 1); err == nil {
		t.Fatal("another tenant must not read a revision")
	}
	c, _ := domain.ContentFromRequest(r)
	c.Title = "đổi"
	_, err = artAppend(env).Execute(ctxB, usecase.AppendInput{RequestID: r.ID, Content: c, Cause: domain.RevisionCauseEdited})
	requireCode(t, err, "REQUEST_NOT_FOUND")
}
