package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// memArtifacts is an in-memory ReviewArtifactRepository with the rules of the real ones: revisions per key, one active drawing per
// slot, content kept once per Request and digest.
type memArtifacts struct {
	mu    sync.Mutex
	rows  []domain.ReviewArtifact
	blobs map[string]domain.ReviewBlob
	saves int
}

func newMemArtifacts() *memArtifacts { return &memArtifacts{blobs: map[string]domain.ReviewBlob{}} }

func (m *memArtifacts) Save(_ context.Context, a domain.ReviewArtifact, blob *domain.ReviewBlob) (domain.ReviewArtifact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if blob != nil {
		if _, ok := m.blobs[a.RequestID+"|"+blob.Digest]; !ok {
			m.blobs[a.RequestID+"|"+blob.Digest] = *blob
		}
	}
	rev := 1
	for _, x := range m.rows {
		if x.RequestID == a.RequestID && x.Key == a.Key && x.Revision >= rev {
			rev = x.Revision + 1
		}
	}
	a.Revision, a.CreatedAt = rev, time.Now().UTC()
	if a.State == domain.ArtifactActive {
		for i := range m.rows {
			if m.rows[i].RequestID == a.RequestID && m.rows[i].Key.Slot() == a.Key.Slot() && m.rows[i].State == domain.ArtifactActive {
				m.rows[i].State = domain.ArtifactSuperseded
			}
		}
	}
	m.rows = append(m.rows, a)
	m.saves++
	return a, nil
}
func (m *memArtifacts) FindByKey(_ context.Context, req string, k domain.ArtifactKey) (domain.ReviewArtifact, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var best domain.ReviewArtifact
	found := false
	for _, x := range m.rows {
		if x.RequestID == req && x.Key == k && (!found || x.Revision > best.Revision) {
			best, found = x, true
		}
	}
	return best, found, nil
}
func (m *memArtifacts) Active(_ context.Context, req string, s domain.ReviewSection, v, name string) (domain.ReviewArtifact, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.rows {
		if x.RequestID == req && x.Key.Section == s && x.Key.Variant == v && x.Key.Name == name && x.State == domain.ArtifactActive {
			return x, true, nil
		}
	}
	return domain.ReviewArtifact{}, false, nil
}
func (m *memArtifacts) Activate(_ context.Context, req, id string) (domain.ReviewArtifact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.rows {
		if m.rows[i].ID == id && m.rows[i].RequestID == req {
			if m.rows[i].State == domain.ArtifactFailed {
				return domain.ReviewArtifact{}, domain.ErrReviewArtifactUnsafe("a failed drawing cannot be restored")
			}
			for j := range m.rows {
				if m.rows[j].RequestID == req && m.rows[j].Key.Slot() == m.rows[i].Key.Slot() && m.rows[j].State == domain.ArtifactActive {
					m.rows[j].State = domain.ArtifactSuperseded
				}
			}
			m.rows[i].State = domain.ArtifactActive
			return m.rows[i], nil
		}
	}
	return domain.ReviewArtifact{}, domain.ErrReviewArtifactNotFound(id)
}
func (m *memArtifacts) Get(_ context.Context, req, id string) (domain.ReviewArtifact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.rows {
		if x.ID == id && x.RequestID == req {
			return x, nil
		}
	}
	return domain.ReviewArtifact{}, domain.ErrReviewArtifactNotFound(id)
}
func (m *memArtifacts) ListRevisions(_ context.Context, req string, s domain.ReviewSection, v, name string) ([]domain.ReviewArtifact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.ReviewArtifact
	for i := len(m.rows) - 1; i >= 0; i-- {
		x := m.rows[i]
		if x.RequestID == req && x.Key.Section == s && x.Key.Variant == v && x.Key.Name == name {
			out = append(out, x)
		}
	}
	return out, nil
}
func (m *memArtifacts) ListActive(_ context.Context, req string) ([]domain.ReviewArtifact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.ReviewArtifact
	for _, x := range m.rows {
		if x.RequestID == req && x.State == domain.ArtifactActive {
			out = append(out, x)
		}
	}
	return out, nil
}
func (m *memArtifacts) Blob(_ context.Context, req, digest string) (domain.ReviewBlob, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.blobs[req+"|"+digest]
	return b, ok, nil
}
func (m *memArtifacts) StoredBytes(context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for _, b := range m.blobs {
		n += int64(len(b.Content))
	}
	return n, nil
}
func (m *memArtifacts) DeleteSupersededBefore(_ context.Context, before time.Time, limit int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept, n := m.rows[:0:0], 0
	for _, x := range m.rows {
		if x.State != domain.ArtifactActive && x.CreatedAt.Before(before) && n < limit {
			n++
			continue
		}
		kept = append(kept, x)
	}
	m.rows = kept
	return n, nil
}
func (m *memArtifacts) DeleteOrphanBlobs(_ context.Context, olderThan time.Time, limit int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for key, b := range m.blobs {
		if n >= limit || !b.CreatedAt.Before(olderThan) && !b.CreatedAt.IsZero() {
			continue
		}
		used := false
		for _, x := range m.rows {
			if strings.HasSuffix(key, "|"+x.BlobDigest) && x.BlobDigest == b.Digest {
				used = true
			}
		}
		if !used {
			delete(m.blobs, key)
			n++
		}
	}
	return n, nil
}

// drawingHTML is a page the skill could return for a model: the labels of its nodes and edges are the only words in the svg.
func drawingHTML(m domain.DiagramModel) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>d</title><style>svg{font-family:system-ui,sans-serif}</style></head><body>`)
	fmt.Fprintf(&b, `<svg id="rv-%s-%s" viewBox="0 0 800 400" xmlns="http://www.w3.org/2000/svg"><title id="rv-t">%s view</title><desc id="rv-d">Drawing of the model</desc>`, m.Section, m.Variant, m.Section)
	for i, n := range m.Nodes {
		fmt.Fprintf(&b, `<g id="rv-n%d"><rect x="%d" y="10" width="100" height="40"/><text x="%d" y="30">%s</text></g>`, i, i*110, i*110, n.Label)
	}
	for _, e := range m.Edges {
		fmt.Fprintf(&b, `<text x="1" y="1">%s %s</text>`, e.Protocol, e.Label)
	}
	b.WriteString(`</svg></body></html>`)
	return b.String()
}

func drawReply(mutate func(html string, m domain.DiagramModel) string) func(AgentPromptInput) (AgentPromptResult, error) {
	return func(in AgentPromptInput) (AgentPromptResult, error) {
		models := modelsInPrompt(in.Prompt)
		res := domain.DrawResult{Schema: domain.ReviewArtifactSchemaName}
		for _, m := range models {
			html := drawingHTML(m.model)
			if mutate != nil {
				html = mutate(html, m.model)
			}
			res.Artifacts = append(res.Artifacts, domain.DrawnArtifact{Section: m.model.Section, Variant: m.model.Variant, Name: m.model.Name, InputDigest: m.digest, HTML: html})
		}
		raw, _ := json.Marshal(res)
		return AgentPromptResult{ResultBlock: raw, AppliedAccessMode: "readonly", Stats: domain.RunStats{Usage: &domain.RunUsage{CostUSD: ptr(0.1)}}}, nil
	}
}

type promptedModel struct {
	model  domain.DiagramModel
	digest string
}

// modelsInPrompt reads back the <diagram> blocks of a draw prompt, which is what an agent would do.
func modelsInPrompt(prompt string) []promptedModel {
	var out []promptedModel
	for _, block := range strings.Split(prompt, "<diagram ")[1:] {
		head := block[:strings.Index(block, ">")]
		attr := func(name string) string {
			i := strings.Index(head, name+`="`)
			if i < 0 {
				return ""
			}
			rest := head[i+len(name)+2:]
			return rest[:strings.Index(rest, `"`)]
		}
		start, end := strings.Index(block, "<model>\n"), strings.Index(block, "\n</model>")
		var m domain.DiagramModel
		_ = json.Unmarshal([]byte(strings.ReplaceAll(block[start+len("<model>\n"):end], `<\/model`, `</model`)), &m)
		m.InputDigest = attr("input_digest")
		out = append(out, promptedModel{model: m, digest: attr("input_digest")})
	}
	return out
}

type drawEnv struct {
	*runEnv
	draw      *ReviewDrawRunner
	artifacts *memArtifacts
	store     *ReviewArtifactStore
	profiles  *switchProfiles
	drawAgent *reviewScriptedAgent
}

type switchProfiles struct{ p domain.ResolvedProfile }

func (s *switchProfiles) Resolve(context.Context, string) (domain.ResolvedProfile, error) {
	return s.p, nil
}

const drawSkillSHA = "d1d1d1d1d1d1d1d1"

func drawProfile(t *testing.T) domain.DevServerCapability {
	return capabilityProfile(t, []string{domain.FeatureKnowledgeHub, domain.FeatureReadonly},
		`[{"name":"tdd-knowledge","installed":true,"sha256":"aa"},{"name":"diagram-design","installed":true,"sha256":"`+drawSkillSHA+`"}]`)
}

// newDrawEnv has a review that already went through its context run, so its diagram models are in the cache.
func newDrawEnv(t *testing.T, status domain.RequestStatus) *drawEnv {
	t.Helper()
	e := &drawEnv{runEnv: newRunEnv(t, status)}
	e.caps.profile = drawProfile(t)
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(goodDesignResult())}
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	e.artifacts = newMemArtifacts()
	e.store = &ReviewArtifactStore{Repo: e.artifacts}
	e.profiles = &switchProfiles{p: domain.DefaultDrawingProfile()}
	e.drawAgent = &reviewScriptedAgent{}
	e.draw = &ReviewDrawRunner{
		Service: e.svc, Runs: e.runs, Contexts: e.contexts, Artifacts: e.store, Profiles: e.profiles, Conns: e.runner.Conns, Agent: e.drawAgent,
		Caps: e.caps, Config: e.runner.Config, Spawn: func(f func()) { f() }, Nonce: func() string { return "nonce0123456789012345678901234567" },
	}
	return e
}

func (e *drawEnv) start(t *testing.T, in StartDrawInput) StartDrawResult {
	t.Helper()
	in.RequestID = e.req.ID
	res, err := e.draw.Start(e.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestDrawRun_DrawsWhatIsMissingOnceAndTheSecondOpenCostsNothing(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	e.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){drawReply(nil)}

	first := e.start(t, StartDrawInput{})
	if first.Run == nil || first.Pending != 2 || e.drawAgent.count() != 1 {
		t.Fatalf("first: %+v calls=%d", first, e.drawAgent.count())
	}
	call := e.drawAgent.calls[0]
	if !call.ReadOnlyEnforced || call.KnowledgeRoot != "" || call.WriteProfile != "" || call.ResultNonce == "" {
		t.Errorf("a draw run is read-only and reads no graph: %+v", call)
	}
	if run := e.runs.last(); run.Kind != ReviewRunDraw || run.Status != ReviewRunSucceeded || run.CostActual != "0.1000 USD" || run.Scope != "design" {
		t.Errorf("run: %+v", run)
	}
	active, _ := e.artifacts.ListActive(e.ctx, e.req.ID)
	if len(active) != 2 {
		t.Fatalf("active drawings: %d", len(active))
	}
	for _, a := range active {
		if a.ViewBox != "0 0 800 400" || a.Title == "" || a.Key.SkillVersion != "diagram-design@"+drawSkillSHA || a.Key.PresetDigest == "" || a.BlobDigest == "" {
			t.Errorf("drawing: %+v", a)
		}
	}

	second := e.start(t, StartDrawInput{})
	if second.Run != nil || second.Cached != 2 || second.Pending != 0 || e.drawAgent.count() != 1 {
		t.Errorf("second open: %+v calls=%d", second, e.drawAgent.count())
	}

	view, _, _ := e.svc.Latest(e.ctx, e.req.ID)
	refs, err := e.store.Annotate(e.ctx, e.req.ID, view.Review.Diagrams, e.profiles.p)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range refs {
		if r.Status != "cached" || r.ArtifactID == "" {
			t.Errorf("annotated: %+v", r)
		}
	}
	content, err := e.store.Read(e.ctx, e.req.ID, refs[0].ArtifactID, "")
	if err != nil || !strings.Contains(string(content.Blob.Content), "<svg") || content.NotModified {
		t.Fatalf("read: %+v %v", content, err)
	}
	if again, _ := e.store.Read(e.ctx, e.req.ID, refs[0].ArtifactID, content.Artifact.BlobDigest); !again.NotModified || len(again.Blob.Content) != 0 {
		t.Errorf("if_digest: %+v", again)
	}
}

func TestDrawRun_AfterCodeOnlyTheDeltaIsDrawnBecauseTheDesignIsAlreadyKept(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	e.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){drawReply(nil)}
	e.start(t, StartDrawInput{})

	finishAll(e.reviewEnv)
	e.setStatus(domain.RequestStatusCompleted)
	e.agent.replies = append(e.agent.replies, agentReply(goodDeliveredResult()))
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	res := e.start(t, StartDrawInput{})
	if res.Cached != 2 || res.Pending != 1 || e.drawAgent.count() != 2 {
		t.Fatalf("only the delta is new: %+v calls=%d", res, e.drawAgent.count())
	}
	prompt := e.drawAgent.calls[1].Prompt
	if strings.Count(prompt, "<diagram ") != 1 || !strings.Contains(prompt, `variant="delta"`) || strings.Contains(prompt, `variant="design"`) {
		t.Errorf("the second run draws one diagram, the delta:\n%s", prompt)
	}
	if run := e.runs.last(); run.Scope != "delivered" {
		t.Errorf("run: %+v", run)
	}
}

func TestDrawRun_OneRetryForOnlyTheDiagramThatFailed(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	first := true
	e.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){func(in AgentPromptInput) (AgentPromptResult, error) {
		return drawReply(func(h string, m domain.DiagramModel) string {
			if first && m.Section == domain.SectionFlow {
				return strings.Replace(h, "</svg>", `<script>alert(1)</script></svg>`, 1)
			}
			return h
		})(in)
	}}
	e.drawAgent.replies = append(e.drawAgent.replies, func(in AgentPromptInput) (AgentPromptResult, error) {
		first = false
		return drawReply(nil)(in)
	})
	// The first reply is used once, then the second for the retry.
	res := e.start(t, StartDrawInput{})
	if res.Pending != 2 || e.drawAgent.count() != 2 {
		t.Fatalf("calls = %d", e.drawAgent.count())
	}
	retry := e.drawAgent.calls[1].Prompt
	if strings.Count(retry, "<diagram ") != 1 || !strings.Contains(retry, `section="flow"`) || !strings.Contains(retry, "<script> is not allowed") {
		t.Errorf("the retry is for the flow only and says why:\n%s", retry)
	}
	if active, _ := e.artifacts.ListActive(e.ctx, e.req.ID); len(active) != 2 {
		t.Errorf("both drawings end up stored: %d", len(active))
	}
	if run := e.runs.last(); run.Status != ReviewRunSucceeded || run.ErrorCode != "" {
		t.Errorf("run: %+v", run)
	}
}

func TestDrawRun_FailsTwiceIsKeptAndNotRetriedByItself(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	e.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){drawReply(func(h string, m domain.DiagramModel) string {
		if m.Section == domain.SectionFlow {
			return strings.Replace(h, "</svg>", `<foreignObject id="rv-x"></foreignObject></svg>`, 1)
		}
		return h
	})}
	res := e.start(t, StartDrawInput{})
	if res.Pending != 2 || e.drawAgent.count() != 2 {
		t.Fatalf("one attempt and one retry: calls=%d", e.drawAgent.count())
	}
	run := e.runs.last()
	if run.Status != ReviewRunSucceeded || run.ErrorCode != "REQUEST_REVIEW_PARTIAL" {
		t.Errorf("one drawing stored and one failed is a partial run: %+v", run)
	}
	flows, _ := e.artifacts.ListRevisions(e.ctx, e.req.ID, domain.SectionFlow, domain.DiagramVariantDesign, "Bug status")
	if len(flows) != 1 || flows[0].State != domain.ArtifactFailed || flows[0].ErrorCode != "REQUEST_REVIEW_ARTIFACT_UNSAFE" || flows[0].BlobDigest != "" {
		t.Fatalf("the failure is recorded: %+v", flows)
	}
	again := e.start(t, StartDrawInput{})
	if again.Run != nil || again.Failed != 1 || again.Cached != 1 || e.drawAgent.count() != 2 {
		t.Errorf("a failed drawing is not retried by itself: %+v calls=%d", again, e.drawAgent.count())
	}
	e.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){drawReply(nil)}
	forced := e.start(t, StartDrawInput{Force: true, Sections: []domain.ReviewSection{domain.SectionFlow}, Reason: "try again", Origin: domain.ArtifactOriginRegenerated})
	if forced.Run == nil || forced.Pending != 1 {
		t.Fatalf("force draws it again: %+v", forced)
	}
	flows, _ = e.artifacts.ListRevisions(e.ctx, e.req.ID, domain.SectionFlow, domain.DiagramVariantDesign, "Bug status")
	if flows[0].State != domain.ArtifactActive || flows[0].Origin != domain.ArtifactOriginRegenerated || flows[0].Reason != "try again" || flows[0].Revision != 2 {
		t.Errorf("the redrawn flow: %+v", flows[0])
	}
}

func TestDrawRun_ReplyProblems(t *testing.T) {
	cases := map[string]func(AgentPromptInput) (AgentPromptResult, error){
		"a question instead of a drawing": func(AgentPromptInput) (AgentPromptResult, error) {
			return AgentPromptResult{Stdout: "Which style guide should I use?", AppliedAccessMode: "readonly"}, nil
		},
		"another schema": func(AgentPromptInput) (AgentPromptResult, error) {
			return AgentPromptResult{ResultBlock: []byte(`{"schema":"x/1","artifacts":[]}`), AppliedAccessMode: "readonly"}, nil
		},
		"no drawing for a diagram": func(AgentPromptInput) (AgentPromptResult, error) {
			return AgentPromptResult{ResultBlock: []byte(`{"schema":"outcome-review-draw/1","artifacts":[]}`), AppliedAccessMode: "readonly"}, nil
		},
		"a drawing of something else": drawReply(func(h string, m domain.DiagramModel) string {
			return strings.Replace(h, "</svg>", `<text x="1" y="1">Quarterly revenue forecast pipeline conversion funnel</text></svg>`, 1)
		}),
	}
	for name, reply := range cases {
		e := newDrawEnv(t, domain.RequestStatusExecuting)
		e.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){reply}
		e.start(t, StartDrawInput{})
		run := e.runs.last()
		if run.Status != ReviewRunFailed || e.drawAgent.count() != 2 {
			t.Errorf("%s: run=%+v calls=%d", name, run, e.drawAgent.count())
		}
		if active, _ := e.artifacts.ListActive(e.ctx, e.req.ID); len(active) != 0 {
			t.Errorf("%s: nothing may be stored: %d", name, len(active))
		}
		rows, _ := e.artifacts.ListRevisions(e.ctx, e.req.ID, domain.SectionArchitecture, domain.DiagramVariantDesign, "")
		if len(rows) != 1 || rows[0].State != domain.ArtifactFailed {
			t.Errorf("%s: the failure is kept: %+v", name, rows)
		}
	}
}

func TestDrawRun_AgentProblemsAbortWithoutKeepingAnythingFromTheRun(t *testing.T) {
	good, _ := json.Marshal(domain.DrawResult{Schema: domain.ReviewArtifactSchemaName})
	for name, res := range map[string]AgentPromptResult{
		"violation":    {ResultBlock: good, AppliedAccessMode: "readonly", Warnings: []string{"READONLY_VIOLATION"}},
		"not readonly": {ResultBlock: good},
		"timeout":      {ResultBlock: good, AppliedAccessMode: "readonly", TimedOut: true},
		"moved head":   {ResultBlock: good, AppliedAccessMode: "readonly", HeadMoved: true},
	} {
		e := newDrawEnv(t, domain.RequestStatusExecuting)
		res := res
		e.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){func(AgentPromptInput) (AgentPromptResult, error) { return res, nil }}
		e.start(t, StartDrawInput{})
		if run := e.runs.last(); run.Status != ReviewRunFailed || e.drawAgent.count() != 1 || e.artifacts.saves != 0 {
			t.Errorf("%s: run=%+v calls=%d saves=%d", name, run, e.drawAgent.count(), e.artifacts.saves)
		}
	}
}

func TestDrawRun_RefusesBeforeSpending(t *testing.T) {
	cases := map[string]struct {
		profile domain.DevServerCapability
		code    string
	}{
		"skill not installed": {capabilityProfile(t, []string{domain.FeatureReadonly}, `[{"name":"diagram-design","installed":false}]`), domain.CodeReviewSkillMissing},
		"skills not reported": {capabilityProfile(t, []string{domain.FeatureReadonly}, ""), domain.CodeReviewSkillMissing},
		"no read-only":        {capabilityProfile(t, []string{}, `[{"name":"diagram-design","installed":true,"sha256":"aa"}]`), domain.CodeReviewAgentTooOld},
	}
	for name, c := range cases {
		e := newDrawEnv(t, domain.RequestStatusExecuting)
		e.caps.profile = c.profile
		before := len(e.runs.ord)
		_, err := e.draw.Start(e.ctx, StartDrawInput{RequestID: e.req.ID})
		if !codeIs(err, c.code) || e.drawAgent.count() != 0 || len(e.runs.ord) != before {
			t.Errorf("%s: err=%v calls=%d", name, err, e.drawAgent.count())
		}
	}
	// A different version of the skill than the one Orca pins is a missing skill.
	old := domain.PinnedDiagramDesignSHA
	domain.PinnedDiagramDesignSHA = "ffffffffffffffff"
	defer func() { domain.PinnedDiagramDesignSHA = old }()
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	if _, err := e.draw.Start(e.ctx, StartDrawInput{RequestID: e.req.ID}); !codeIs(err, domain.CodeReviewSkillMissing) {
		t.Errorf("pinned version: %v", err)
	}
	// A review that has no context yet has nothing to draw.
	n := newRunEnv(t, domain.RequestStatusExecuting)
	n.caps.profile = drawProfile(t)
	if _, err := n.svc.Generate(n.ctx, GenerateInput{RequestID: n.req.ID}); err != nil {
		t.Fatal(err)
	}
	d := &ReviewDrawRunner{Service: n.svc, Runs: n.runs, Contexts: n.contexts, Artifacts: &ReviewArtifactStore{Repo: newMemArtifacts()}, Conns: n.runner.Conns, Agent: &reviewScriptedAgent{}, Caps: n.caps, Config: n.runner.Config}
	if res, err := d.Start(n.ctx, StartDrawInput{RequestID: n.req.ID}); err != nil || res.Skipped != "no_context" {
		t.Errorf("no context: %+v %v", res, err)
	}
}

func TestDrawRun_ChangingOneSectionRedrawsOnlyThatSectionAndGoingBackIsFree(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	e.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){drawReply(nil)}
	e.start(t, StartDrawInput{})

	override, err := domain.ParseDrawingProfile([]byte(`{"schema":"review-drawing-profile/1","sections":{"flow":{"maxLanes":4}}}`))
	if err != nil {
		t.Fatal(err)
	}
	e.profiles.p = domain.ResolveDrawingProfile(domain.Layer{Scope: domain.SourceProject, Profile: override})
	res := e.start(t, StartDrawInput{})
	if res.Cached != 1 || res.Pending != 1 || e.drawAgent.count() != 2 {
		t.Fatalf("only the flow is redrawn: %+v calls=%d", res, e.drawAgent.count())
	}
	if prompt := e.drawAgent.calls[1].Prompt; strings.Count(prompt, "<diagram ") != 1 || !strings.Contains(prompt, `section="flow"`) {
		t.Errorf("prompt:\n%s", prompt)
	}

	e.profiles.p = domain.DefaultDrawingProfile()
	back := e.start(t, StartDrawInput{})
	if back.Cached != 2 || back.Pending != 0 || e.drawAgent.count() != 2 {
		t.Errorf("going back to the first profile costs nothing: %+v calls=%d", back, e.drawAgent.count())
	}
	revs, _ := e.artifacts.ListRevisions(e.ctx, e.req.ID, domain.SectionFlow, domain.DiagramVariantDesign, "Bug status")
	active := 0
	for _, r := range revs {
		if r.State == domain.ArtifactActive {
			active++
		}
	}
	if len(revs) != 2 || active != 1 {
		t.Errorf("two revisions and one active: %+v", revs)
	}

	// A section the profile turns off is not drawn and not counted.
	off, _ := domain.ParseDrawingProfile([]byte(`{"schema":"review-drawing-profile/1","sections":{"flow":{"enabled":false}}}`))
	e.profiles.p = domain.ResolveDrawingProfile(domain.Layer{Scope: domain.SourceTenant, Profile: off})
	if r := e.start(t, StartDrawInput{}); r.Cached != 1 || r.Pending != 0 || e.drawAgent.count() != 2 {
		t.Errorf("disabled section: %+v", r)
	}
}

func TestDrawRun_MoreThanFourDiagramsAreDrawnInBatches(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	view, _, _ := e.svc.Latest(e.ctx, e.req.ID)
	rec, _, _ := e.contexts.Get(e.ctx, e.req.ID, ReviewContextResult, view.Stored.ContextDigest, time.Now())
	var payload contextCachePayload
	_ = json.Unmarshal(rec.Payload, &payload)
	// Six flows: the design drew one, so the cache now holds seven diagrams.
	base := payload.Context.Diagrams
	for _, m := range base {
		if m.Section == domain.SectionFlow {
			for i := 0; i < 6; i++ {
				c := m
				c.Name = fmt.Sprintf("Flow %d", i)
				c.InputDigest, _ = c.ComputeInputDigest()
				payload.Context.Diagrams = append(payload.Context.Diagrams, c)
			}
			break
		}
	}
	raw, _ := json.Marshal(payload)
	rec.Payload = raw
	_, _ = e.contexts.Put(e.ctx, rec)

	e.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){drawReply(nil)}
	res := e.start(t, StartDrawInput{})
	if res.Pending != 8 || e.drawAgent.count() != 2 {
		t.Fatalf("eight diagrams in two batches: %+v calls=%d", res, e.drawAgent.count())
	}
	for i, c := range e.drawAgent.calls {
		if n := strings.Count(c.Prompt, "<diagram "); n > domain.MaxDrawingsPerRun {
			t.Errorf("batch %d has %d diagrams", i, n)
		}
	}
	if active, _ := e.artifacts.ListActive(e.ctx, e.req.ID); len(active) != 8 {
		t.Errorf("active drawings: %d", len(active))
	}
	names := []string{}
	for _, c := range e.drawAgent.calls {
		for _, m := range modelsInPrompt(c.Prompt) {
			names = append(names, string(m.model.Section)+"/"+m.model.Name)
		}
	}
	sort.Strings(names)
	if len(names) != 8 {
		t.Errorf("each diagram is asked for once: %v", names)
	}
}

func TestDrawRun_OnlyOneDrawRunAtATimeAndTheQuotaHolds(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	e.draw.Spawn = func(func()) {}
	if _, err := e.draw.Start(e.ctx, StartDrawInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.draw.Start(e.ctx, StartDrawInput{RequestID: e.req.ID, Force: true}); !codeIs(err, domain.CodeReviewRunInProgress) {
		t.Errorf("a second draw run: %v", err)
	}
	// A draw run does not block a context run: the kinds have their own slots.
	if run, ok, _ := e.runs.ActiveForRequest(e.ctx, e.req.ID, ReviewRunContext); ok {
		t.Errorf("the context run is over: %+v", run)
	}

	q := newDrawEnv(t, domain.RequestStatusExecuting)
	q.store.QuotaBytes = 100
	q.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){drawReply(nil)}
	q.start(t, StartDrawInput{})
	if run := q.runs.last(); run.Status != ReviewRunFailed || run.ErrorCode != "REQUEST_REVIEW_BUDGET_EXCEEDED" {
		t.Errorf("quota: %+v", run)
	}
}

func TestArtifactStore_RefusesContentThatChangedAfterItWasChecked(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	e.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){drawReply(nil)}
	e.start(t, StartDrawInput{})
	active, _ := e.artifacts.ListActive(e.ctx, e.req.ID)
	a := active[0]
	e.artifacts.mu.Lock()
	b := e.artifacts.blobs[e.req.ID+"|"+a.BlobDigest]
	b.Content = append(b.Content, []byte("<script>x</script>")...)
	e.artifacts.blobs[e.req.ID+"|"+a.BlobDigest] = b
	e.artifacts.mu.Unlock()
	if _, err := e.store.Read(e.ctx, e.req.ID, a.ID, ""); !codeIs(err, domain.CodeReviewArtifactCorrupt) {
		t.Errorf("a changed drawing must not be served: %v", err)
	}
	if _, err := e.store.Read(e.ctx, e.req.ID, "00000000-0000-4000-8000-000000000000", ""); !codeIs(err, domain.CodeReviewArtifactNotFound) {
		t.Errorf("missing: %v", err)
	}
}

func TestDrawPromptVersionMatchesTheDomain(t *testing.T) {
	if domain.DrawPromptVersion == "" || domain.DrawPromptVersion != drawPromptVersionOfModule() {
		t.Errorf("the preset digest and the prompt module must name the same version: %q vs %q", domain.DrawPromptVersion, drawPromptVersionOfModule())
	}
}

func TestDrawRun_ChainedAfterTheContextRun(t *testing.T) {
	e := newRunEnv(t, domain.RequestStatusExecuting)
	e.caps.profile = drawProfile(t)
	arts := newMemArtifacts()
	store := &ReviewArtifactStore{Repo: arts}
	drawAgent := &reviewScriptedAgent{replies: []func(AgentPromptInput) (AgentPromptResult, error){drawReply(nil)}}
	e.svc.Artifacts, e.svc.Runs = store, e.runs
	e.runner.Draw = &ReviewDrawRunner{
		Service: e.svc, Runs: e.runs, Contexts: e.contexts, Artifacts: store, Conns: e.runner.Conns, Agent: drawAgent, Caps: e.caps, Config: e.runner.Config,
		Spawn: func(f func()) { f() }, Nonce: func() string { return "nonce0123456789012345678901234567" },
	}
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(goodDesignResult())}
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	if drawAgent.count() != 1 {
		t.Fatalf("the drawings follow the context: %d draw calls", drawAgent.count())
	}
	view, _, _ := e.svc.Latest(e.ctx, e.req.ID)
	for _, d := range view.Review.Diagrams {
		if d.Status != "cached" || d.ArtifactID == "" {
			t.Errorf("a read says which drawings exist: %+v", d)
		}
	}
	var fromModel domain.OutcomeReview
	if err := json.Unmarshal(view.Stored.Model, &fromModel); err != nil || len(fromModel.Diagrams) != len(view.Review.Diagrams) || fromModel.Diagrams[0].Status != "cached" {
		t.Errorf("the model that travels carries the same statuses: %v %+v", err, fromModel.Diagrams)
	}

	// Asking again: the context is cached and so are the drawings; nothing is called.
	again, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID})
	if err != nil || !again.ContextCached || again.Draw == nil || again.Draw.Cached != 2 || again.Draw.Run != nil || drawAgent.count() != 1 || e.agent.count() != 1 {
		t.Errorf("again: cached=%v draw=%+v calls=%d/%d err=%v", again.ContextCached, again.Draw, drawAgent.count(), e.agent.count(), err)
	}
}

func TestDrawRun_ASkillInstalledLaterIsEnoughAndAMissingOneNeverFailsTheContext(t *testing.T) {
	e := newRunEnv(t, domain.RequestStatusExecuting) // the readiness profile has no diagram-design skill
	arts := newMemArtifacts()
	store := &ReviewArtifactStore{Repo: arts}
	drawAgent := &reviewScriptedAgent{replies: []func(AgentPromptInput) (AgentPromptResult, error){drawReply(nil)}}
	e.svc.Artifacts, e.svc.Runs = store, e.runs
	e.runner.Draw = &ReviewDrawRunner{
		Service: e.svc, Runs: e.runs, Contexts: e.contexts, Artifacts: store, Conns: e.runner.Conns, Agent: drawAgent, Caps: e.caps, Config: e.runner.Config,
		Spawn: func(f func()) { f() }, Nonce: func() string { return "nonce0123456789012345678901234567" },
	}
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(goodDesignResult())}
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	if run := e.runs.last(); run.Kind != ReviewRunContext || run.Status != ReviewRunSucceeded || drawAgent.count() != 0 {
		t.Fatalf("the context run is unaffected: %+v draws=%d", run, drawAgent.count())
	}
	view, _, _ := e.svc.Latest(e.ctx, e.req.ID)
	for _, d := range view.Review.Diagrams {
		if d.Status != "missing" {
			t.Errorf("without a skill the diagrams are missing: %+v", d)
		}
	}
	// The skill is installed; asking again applies the cached context and draws.
	e.caps.profile = drawProfile(t)
	got, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID})
	if err != nil || !got.ContextCached || got.Draw == nil || got.Draw.Run == nil || drawAgent.count() != 1 || e.agent.count() != 1 {
		t.Fatalf("draw after the skill appears: %+v %v calls=%d/%d", got.Draw, err, drawAgent.count(), e.agent.count())
	}
}

func TestDrawRun_AReadShowsDrawingWhileARunIsActive(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	e.svc.Artifacts, e.svc.Runs = e.store, e.runs
	e.draw.Spawn = func(func()) {} // the run starts and does not finish
	if _, err := e.draw.Start(e.ctx, StartDrawInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	view, _, _ := e.svc.Latest(e.ctx, e.req.ID)
	for _, d := range view.Review.Diagrams {
		if d.Status != "drawing" {
			t.Errorf("status %q while the run is active", d.Status)
		}
	}
}
