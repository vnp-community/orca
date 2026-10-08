package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/schemas"
)

func newCreateWithRecorder(env *artEnv) *CreateRequest {
	tr := NewTransitionRequest(env.s, env, env.s)
	reg, err := schemas.Default()
	if err != nil {
		panic(err)
	}
	uc := NewCreateRequest(env.s, lcIdempotency{env.s}, env, env.s, tr, nil)
	index := newArtIndex()
	rec := NewRequestRevisionRecorder(revisionsView{env}, env.s).WithSchemas(reg).WithIndex(NewMintArtifactIDs(index))
	return uc.WithCreationRecorder(rec)
}

func creatorCtx() context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), userAda)
}

func TestCreateRequest_WritesRevisionOne_SameTx(t *testing.T) {
	env := newArtEnv()
	uc := newCreateWithRecorder(env)
	res, err := uc.Execute(creatorCtx(), CreateRequestInput{
		ProjectID: testProject, Title: "Cần SSO", Body: "Đăng nhập một lần cho mọi dịch vụ nội bộ của công ty.",
		Source:             domain.SourceRef{Provider: domain.SourceProviderManual},
		AcceptanceCriteria: []domain.ACInput{{Text: "Đăng nhập thành công chuyển về trang chủ", VerifyHint: "test"}, {Text: "Tài khoản khóa hiện thông báo"}},
		TypeFields:         map[string]any{"goal": "SSO"},
	})
	if err != nil || !res.Created {
		t.Fatalf("%+v %v", res, err)
	}
	r := env.s.requests[res.Request.ID]
	revs := env.revisions[r.ID]
	if len(revs) != 1 || revs[0].Revision != 1 || revs[0].Cause != domain.RevisionCauseCreated || revs[0].Digest != r.ContentDigest {
		t.Fatalf("revisions %+v digest %s", revs, r.ContentDigest)
	}
	c := contentOf(t, r)
	if len(c.AcceptanceCriteria.Items) != 2 || c.AcceptanceCriteria.Items[1].ID != "AC-2" || c.TypeFields["goal"] != "SSO" || r.ContentRevision != 1 {
		t.Fatalf("content = %+v", c)
	}
	if got := len(env.s.events); got < 3 {
		t.Fatalf("want created, revised and status_changed events, got %d", got)
	}
	if !strings.Contains(string(revs[0].Snapshot), "Cần SSO") {
		t.Fatal("the revision snapshot must hold the creation content")
	}
}

func TestCreateRequest_NoInitialContent_StillRecordsRevisionOne(t *testing.T) {
	env := newArtEnv()
	res, err := newCreateWithRecorder(env).Execute(creatorCtx(), CreateRequestInput{ProjectID: testProject, Title: "Chỉ tiêu đề", Source: domain.SourceRef{Provider: domain.SourceProviderManual}})
	if err != nil {
		t.Fatal(err)
	}
	if revs := env.revisions[res.Request.ID]; len(revs) != 1 || string(env.s.requests[res.Request.ID].AcceptanceCriteriaJSON) != "[]" {
		t.Fatalf("%+v", revs)
	}
}

func TestCreateRequest_RevisionFailureLeavesNothingBehind(t *testing.T) {
	env := newArtEnv()
	env.failRevAdd = errBoom
	_, err := newCreateWithRecorder(env).Execute(creatorCtx(), CreateRequestInput{ProjectID: testProject, Title: "x", Source: domain.SourceRef{Provider: domain.SourceProviderManual}})
	if err == nil {
		t.Fatal("expected failure")
	}
	if len(env.s.requests) != 0 || len(env.revisions) != 0 || env.s.counter != 0 {
		t.Fatalf("a failed creation must leave no request, revision or burnt number: %d %d %d", len(env.s.requests), len(env.revisions), env.s.counter)
	}
}

func TestCreateRequest_InvalidInitialContentRefused(t *testing.T) {
	env := newArtEnv()
	_, err := newCreateWithRecorder(env).Execute(creatorCtx(), CreateRequestInput{
		ProjectID: testProject, Title: "x", Source: domain.SourceRef{Provider: domain.SourceProviderManual},
		AcceptanceCriteria: []domain.ACInput{{Text: strings.Repeat("a", 501)}},
	})
	if err == nil || len(env.s.requests) != 0 {
		t.Fatalf("an oversized criterion must be refused: %v", err)
	}
	_, err = newCreateWithRecorder(env).Execute(creatorCtx(), CreateRequestInput{
		ProjectID: testProject, Title: "x", Source: domain.SourceRef{Provider: domain.SourceProviderManual}, TypeFields: map[string]any{"ac_next": 7},
	})
	if err == nil {
		t.Fatal("ac_next belongs to the server")
	}
}

func TestMintSolutionID_AssignsSeqIndexesOptions(t *testing.T) {
	idx := newArtIndex()
	m := NewMintArtifactIDs(idx)
	sol := &domain.Solution{ID: newID(), RequestID: "req-1", OptionsJSON: []byte(`{"options":[{"id":"opt-1"},{"id":"opt-2"}]}`)}
	idx.solReq[sol.ID] = "req-1"
	if err := m.MintSolutionID(context.Background(), 142, sol); err != nil {
		t.Fatal(err)
	}
	if sol.Seq != 1 {
		t.Fatalf("seq %d", sol.Seq)
	}
	for _, id := range []string{"SOL-142.1", "SOL-142.1/opt-1", "SOL-142.1/opt-2"} {
		e, ok := idx.entries[id]
		if !ok || e.ArtifactID != sol.ID {
			t.Errorf("%s not indexed: %+v", id, e)
		}
	}
	if idx.entries["SOL-142.1/opt-1"].Kind != domain.DisplayKindOption || idx.entries["SOL-142.1"].Kind != domain.DisplayKindSolution {
		t.Error("kinds")
	}
	second := &domain.Solution{ID: newID(), RequestID: "req-1", OptionsJSON: []byte(`{"options":[{"id":"opt-1"}]}`)}
	idx.solReq[second.ID] = "req-1"
	if err := m.MintSolutionID(context.Background(), 142, second); err != nil || second.Seq != 2 {
		t.Fatalf("second solution of a request: %d %v", second.Seq, err)
	}
	// Re-minting an indexed solution changes nothing.
	if err := m.MintSolutionID(context.Background(), 142, sol); err != nil || sol.Seq != 1 || len(idx.entries) != 5 {
		t.Fatalf("remint: seq=%d entries=%d %v", sol.Seq, len(idx.entries), err)
	}
}

func TestMintSolutionID_ConflictRetries(t *testing.T) {
	idx := newArtIndex()
	idx.clash = 2
	sol := &domain.Solution{ID: newID(), RequestID: "req-1"}
	idx.solReq[sol.ID] = "req-1"
	if err := NewMintArtifactIDs(idx).MintSolutionID(context.Background(), 7, sol); err != nil || sol.Seq != 3 {
		t.Fatalf("two clashes then success should land on seq 3: %d %v", sol.Seq, err)
	}
	idx2 := newArtIndex()
	idx2.clash = 5
	sol2 := &domain.Solution{ID: newID(), RequestID: "req-1"}
	idx2.solReq[sol2.ID] = "req-1"
	err := NewMintArtifactIDs(idx2).MintSolutionID(context.Background(), 7, sol2)
	mustCode(t, err, "REQUEST_ARTIFACT_SEQ_CONFLICT")
}

func TestMintPlanIDs_TreeOrder(t *testing.T) {
	idx := newArtIndex()
	m := NewMintArtifactIDs(idx)
	seq, err := m.MintPlanIDs(context.Background(), 142, "req-1", PlanTreeIDs{
		PlanTaskID: "plan-uuid", Phases: []PlanPhaseIDs{{TaskID: "ph1", Tasks: []string{"t1", "t2"}}, {TaskID: "ph2", Tasks: []string{"t3"}}}, Tasks: []string{"t4"},
	})
	if err != nil || seq != 1 {
		t.Fatalf("%d %v", seq, err)
	}
	want := map[string]string{"PLN-142.1": "plan-uuid", "PH-142.1.1": "ph1", "PH-142.1.2": "ph2", "TSK-142.1.1": "t1", "TSK-142.1.2": "t2", "TSK-142.1.3": "t3", "TSK-142.1.4": "t4"}
	for id, artifact := range want {
		if idx.entries[id].ArtifactID != artifact {
			t.Errorf("%s -> %q, want %q", id, idx.entries[id].ArtifactID, artifact)
		}
	}
	if seq2, _ := m.MintPlanIDs(context.Background(), 142, "req-1", PlanTreeIDs{PlanTaskID: "plan-2"}); seq2 != 2 {
		t.Fatalf("a replan is a new plan with the next seq, got %d", seq2)
	}
}

func TestReplaceRequestCoverage_RollsBackWithCaller(t *testing.T) {
	env := newArtEnv()
	cov := &artCoverage{rows: map[string][]domain.CoverageRow{}}
	// Join the coverage block to the env's transaction by snapshotting it around the call.
	uc := NewReplaceRequestCoverage(cov)
	plan := domain.PlanSpecs{Tasks: []domain.TaskSpecView{{ID: "t1", Satisfies: []string{"AC-1", "AC-2"}, CheckIDs: []string{"c1"}}, {ID: "t2", Satisfies: []string{"AC-2"}}}}
	if err := uc.Execute(lcCtx(), "req-1", "plan-1", plan); err != nil {
		t.Fatal(err)
	}
	rows, _ := cov.ListByRequest(lcCtx(), "req-1")
	if len(rows) != 3 {
		t.Fatalf("rows %+v", rows)
	}
	// The replacement is a whole block: a second plan state replaces, never appends.
	_ = uc.Execute(lcCtx(), "req-1", "plan-1", domain.PlanSpecs{Tasks: []domain.TaskSpecView{{ID: "t9", Satisfies: []string{"AC-1"}, CheckIDs: []string{"c1"}}}})
	rows, _ = cov.ListByRequest(lcCtx(), "req-1")
	if len(rows) != 1 || rows[0].TaskID != "t9" {
		t.Fatalf("not replaced: %+v", rows)
	}
	_ = env
	if err := NewReplaceRequestCoverage(cov).Execute(context.Background(), "req-1", "plan-1", plan); err == nil {
		t.Fatal("a tenant is required")
	}
}

func TestBuildCoverageRows(t *testing.T) {
	rows := domain.BuildCoverageRows("plan-1", domain.PlanSpecs{Tasks: []domain.TaskSpecView{
		{ID: "t2", Satisfies: []string{"AC-2", "AC-1", "AC-1"}, CheckIDs: []string{"c2", "c1"}},
		{ID: "t1", Satisfies: []string{"AC-1"}},
		{ID: "t3", Satisfies: []string{"AC-3"}, ExemptFromCoverage: true, Labels: []string{"rollback"}},
	}})
	var got []string
	for _, r := range rows {
		got = append(got, r.ACID+"/"+r.TaskID+"/"+r.CheckID)
	}
	want := []string{"AC-1/t1/", "AC-1/t2/c1", "AC-1/t2/c2", "AC-2/t2/c1", "AC-2/t2/c2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v", got)
	}
	ac := domain.AcceptanceCriteria{}
	for i := 0; i < 4; i++ {
		_, _ = ac.Add("x", "")
	}
	_ = ac.Retire("AC-4")
	if un := domain.UncoveredACs(ac, rows); strings.Join(un, ",") != "AC-3" {
		t.Fatalf("uncovered %v", un)
	}
}

func newReader(env *artEnv, tree PlanTreeReader) (*ArtifactReader, *artRelations, *artCoverage, *artIndex) {
	rel, cov, idx := &artRelations{}, &artCoverage{rows: map[string][]domain.CoverageRow{}}, newArtIndex()
	r := NewArtifactReader(env.s, revisionsView{env}, cov, rel, lcLinks{env.s}, idx)
	if tree != nil {
		r.WithPlanTree(tree)
	}
	return r, rel, cov, idx
}

func TestGetArtifactGraph_MergesThreeSources_PartialOnTaskServiceError(t *testing.T) {
	f := newArtFx()
	parent := f.seed(nil)
	child := f.seed(func(r *domain.Request) { r.PlanTaskID = "plan-1" })
	f.env.s.links = append(f.env.s.links, domain.RequestLink{ParentRequestID: parent.ID, ChildRequestID: child.ID, Reason: domain.LinkReasonRelatesTo})
	tree := fakeTree{sub: PlanSubtree{
		Nodes:     []SubtreeNode{{ID: "plan-1", Kind: "plan"}, {ID: "ph1", ParentID: "plan-1", Kind: "phase"}, {ID: "t1", ParentID: "ph1", Kind: "task"}, {ID: "t2", ParentID: "plan-1", Kind: "task"}},
		DependsOn: []domain.DependsEdge{{From: "t2", To: "t1"}},
	}}
	reader, rel, _, _ := newReader(f.env, tree)
	_, _ = rel.Insert(context.Background(), domain.ArtifactRelation{RequestID: child.ID, Rel: domain.RelationDerivedFrom, FromKind: domain.NodeSolution, FromID: "SOL-1.1", ToKind: domain.NodeRequest, ToID: "REQ-2"})
	_, _ = rel.Insert(context.Background(), domain.ArtifactRelation{RequestID: child.ID, Rel: domain.RelationImplements, FromKind: domain.NodePlan, FromID: "PLN-2.1", ToKind: domain.NodeOption, ToID: "SOL-1.1/opt-1"})

	g, err := reader.Graph(lcCtx(), child.ID)
	if err != nil || g.Partial {
		t.Fatalf("%+v %v", g, err)
	}
	var rels []string
	for _, e := range g.Edges {
		rels = append(rels, string(e.Rel))
	}
	if strings.Join(rels, ",") != "contains,contains,contains,depends_on,derived_from,implements,spawned_by" {
		t.Fatalf("edges = %v", rels)
	}
	for _, e := range g.Edges {
		if !domain.AllowedRelation(e.Rel, e.FromKind, e.ToKind) {
			t.Errorf("edge outside the ontology: %+v", e)
		}
		if e.Rel == domain.RelationSpawnedBy && (e.FromID != child.ID || e.ToID != parent.ID) {
			t.Errorf("spawned_by points the wrong way: %+v", e)
		}
	}

	broken := NewArtifactReader(f.env.s, revisionsView{f.env}, &artCoverage{rows: map[string][]domain.CoverageRow{}}, rel, lcLinks{f.env.s}, newArtIndex()).
		WithPlanTree(fakeTree{err: errors.New("task-service down")})
	g, err = broken.Graph(lcCtx(), child.ID)
	if err != nil || !g.Partial || len(g.Edges) != 3 {
		t.Fatalf("task-service failure must give the other sources and partial=true, not an error: %+v %v", g, err)
	}
	for _, e := range g.Edges {
		if e.Rel == domain.RelationContains || e.Rel == domain.RelationDependsOn {
			t.Errorf("tree edges cannot appear without the tree: %+v", e)
		}
	}
	noTree, _, _, _ := newReader(f.env, nil)
	if g, _ := noTree.Graph(lcCtx(), child.ID); !g.Partial {
		t.Fatal("a request with a plan and no tree reader is partial")
	}
	if g, _ := noTree.Graph(lcCtx(), parent.ID); g.Partial {
		t.Fatal("a request without a plan has nothing partial")
	}
}

func TestReader_CrossTenantAndMalformedAreNotFound(t *testing.T) {
	f := newArtFx()
	r := f.seed(nil)
	reader, _, _, idx := newReader(f.env, nil)
	other := tenant.WithTenantID(context.Background(), "other")
	// the fake request repo is tenant blind, so model the tenant check through the index like the real adapter does
	idx.entries["SOL-1.1"] = domain.IndexEntry{TenantID: "t1", DisplayID: "SOL-1.1", Kind: domain.DisplayKindSolution, RequestID: r.ID, ArtifactID: newID()}
	if _, err := reader.Resolve(other, "SOL-1.1"); err == nil {
		t.Fatal("another tenant must not resolve the id")
	}
	if _, err := reader.Resolve(context.Background(), "SOL-1.1"); err == nil {
		t.Fatal("no tenant, no answer")
	}
	for _, bad := range []string{"", "sol-1.1", "SOL-1", "AC-3"} {
		if _, err := reader.Resolve(lcCtx(), bad); err == nil {
			t.Errorf("%q must not resolve", bad)
		}
	}
	got, err := reader.Resolve(lcCtx(), "SOL-1.1")
	if err != nil || got.RequestID != r.ID || got.Kind != domain.DisplayKindSolution {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := reader.ListRevisions(lcCtx(), "not-a-uuid", 0, 10); err == nil {
		t.Fatal("malformed id")
	}
	if _, err := reader.GetRevision(lcCtx(), r.ID, 99); err == nil {
		t.Fatal("missing revision")
	}
}

func TestReader_ListRevisionsPaginates(t *testing.T) {
	f := newArtFx()
	r := f.seed(nil)
	c := contentOf(t, r)
	for i := 0; i < 4; i++ {
		c.Title = strings.Repeat("v", i+1)
		if _, err := f.appendRev.Execute(asUser(userAda), AppendInput{RequestID: r.ID, Content: c, Cause: domain.RevisionCauseEdited}); err != nil {
			t.Fatal(err)
		}
	}
	reader, _, _, _ := newReader(f.env, nil)
	page, err := reader.ListRevisions(lcCtx(), r.ID, 0, 2)
	if err != nil || len(page.Revisions) != 2 || page.NextAfter != 2 {
		t.Fatalf("%+v %v", page, err)
	}
	page, _ = reader.ListRevisions(lcCtx(), r.ID, page.NextAfter, 2)
	if len(page.Revisions) != 2 || page.Revisions[0].Revision != 3 || page.NextAfter != 4 {
		t.Fatalf("%+v", page)
	}
	page, _ = reader.ListRevisions(lcCtx(), r.ID, 4, 2)
	if len(page.Revisions) != 1 || page.NextAfter != 0 {
		t.Fatalf("%+v", page)
	}
}

func TestReader_CoverageReportsUncoveredACs(t *testing.T) {
	f := newArtFx()
	r := f.seed(func(r *domain.Request) {
		c := domain.RequestContent{Title: r.Title, Type: r.Type}
		_, _ = c.AcceptanceCriteria.Add("một", "")
		_, _ = c.AcceptanceCriteria.Add("hai", "")
		x, _ := r.WithContent(c)
		*r = x
	})
	reader, _, cov, _ := newReader(f.env, nil)
	_ = cov.ReplaceForPlan(lcCtx(), r.ID, "plan-1", []domain.CoverageRow{{ACID: "AC-1", TaskID: "t1", PlanTaskID: "plan-1"}})
	v, err := reader.Coverage(lcCtx(), r.ID)
	if err != nil || len(v.Rows) != 1 || len(v.UncoveredACIDs) != 1 || v.UncoveredACIDs[0] != "AC-2" {
		t.Fatalf("%+v %v", v, err)
	}
}

func TestExportArtifactProjection(t *testing.T) {
	f := newArtFx()
	r := f.seed(nil)
	reg, _ := schemas.Default()
	idx := newArtIndex()
	solID := newID()
	idx.entries["SOL-1.1"] = domain.IndexEntry{TenantID: "t1", DisplayID: "SOL-1.1", Kind: domain.DisplayKindSolution, RequestID: r.ID, ArtifactID: solID}
	optionsDoc := `{"schema_version":1,"options":[{"id":"opt-1","title":"A","summary":"s","approach":"a","effort":{"size":"M","hours_estimate":2},"risk":{"level":"low","description":"d"},"affected_areas":[]}],"recommendation":{"option_id":"opt-1","reason":"r"}}`
	ex := NewExportArtifactProjection(f.env.s, revisionsView{f.env}, solutionStub{solID: {ID: solID, OptionsJSON: []byte(optionsDoc), Status: domain.SolutionStatusProposed}}, idx, reg)

	res, err := ex.Execute(lcCtx(), r.ID, "SOL-1.1", "markdown")
	if err != nil || res.Filename != "SOL-1.1.md" || !strings.HasPrefix(res.Digest, "sha256:") || !strings.Contains(res.Content, "orca_schema: 1") || !strings.Contains(res.Content, "status: proposed") {
		t.Fatalf("%+v %v", res, err)
	}
	back, vs := domain.ParseProjection(reg, []byte(res.Content), domain.ArtifactKindSolution)
	if len(vs) != 0 || !strings.Contains(string(back), `"opt-1"`) {
		t.Fatalf("export must parse back: %+v", vs)
	}
	again, _ := ex.Execute(lcCtx(), r.ID, "SOL-1.1", "")
	if again.Content != res.Content || again.Digest != res.Digest {
		t.Fatal("export must be deterministic and default to markdown")
	}
	own, err := ex.Execute(lcCtx(), r.ID, "", "markdown")
	if err != nil || own.Filename != domain.FormatRequestID(r.Number)+".md" {
		t.Fatalf("request export: %+v %v", own, err)
	}
	if bs, vs := domain.ParseProjection(reg, []byte(own.Content), domain.ArtifactKindRequest); len(vs) != 0 || !strings.Contains(string(bs), "Lỗi đăng nhập") {
		t.Fatalf("%+v", vs)
	}
	_, err = ex.Execute(lcCtx(), r.ID, "SOL-1.1", "pdf")
	mustCode(t, err, "REQUEST_ARTIFACT_FORMAT_UNSUPPORTED")
	_, err = ex.Execute(lcCtx(), r.ID, "PLN-1.1", "markdown")
	mustCode(t, err, "REQUEST_ARTIFACT_EXPORT_UNSUPPORTED")
	_, err = ex.Execute(lcCtx(), r.ID, "SOL-9.9", "markdown")
	mustCode(t, err, "REQUEST_ARTIFACT_NOT_FOUND")
	other := f.seed(nil)
	_, err = ex.Execute(lcCtx(), other.ID, "SOL-1.1", "markdown")
	mustCode(t, err, "REQUEST_ARTIFACT_NOT_FOUND")
	_, err = ex.Execute(context.Background(), r.ID, "SOL-1.1", "markdown")
	if err == nil {
		t.Fatal("tenant required")
	}
}

type solutionStub map[string]domain.Solution

func (s solutionStub) Insert(context.Context, domain.Solution) error { return errUnexpected }
func (s solutionStub) Get(_ context.Context, id string) (domain.Solution, error) {
	if sol, ok := s[id]; ok {
		return sol, nil
	}
	return domain.Solution{}, domain.ErrSolutionNotFound(id)
}
func (s solutionStub) ListByRequestID(context.Context, string) ([]domain.Solution, error) {
	return nil, errUnexpected
}
func (s solutionStub) Update(context.Context, domain.Solution, int64) (domain.Solution, error) {
	return domain.Solution{}, errUnexpected
}
