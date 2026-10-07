package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

type mockGateway struct {
	mu           sync.Mutex
	calledMethod map[string]int

	statusRes       RawCodeIntelResult
	statusErr       error
	overviewRes     RawCodeIntelResult
	overviewErr     error
	subgraphRes     RawCodeIntelResult
	subgraphErr     error
	impactRes       RawCodeIntelResult
	impactErr       error
	symbolRes       RawCodeIntelResult
	symbolErr       error
	routesRes       RawCodeIntelResult
	routesErr       error
	changesRes      RawCodeIntelResult
	changesErr      error
	processesRes    RawCodeIntelResult
	processesErr    error
	processRes      RawCodeIntelResult
	processErr      error
	structuralRes   RawCodeIntelResult
	structuralErr   error
	reindexRes      RawCodeIntelResult
	reindexErr      error
	reindexStatRes  RawCodeIntelResult
	reindexStatErr  error
	reindexCancRes  RawCodeIntelResult
	reindexCancErr  error
	watchRes        RawCodeIntelResult
	watchErr        error
	cgSearchRes     RawCodeIntelResult
	cgSearchErr     error
	filesRes        RawCodeIntelResult
	filesErr        error
}

func newMockGateway() *mockGateway {
	return &mockGateway{
		calledMethod: make(map[string]int),
	}
}

func (m *mockGateway) record(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calledMethod[method]++
}

func (m *mockGateway) callCount(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calledMethod[method]
}

func (m *mockGateway) Status(ctx context.Context, target AgentTarget, p StatusParams) (RawCodeIntelResult, error) {
	m.record("status")
	return m.statusRes, m.statusErr
}

func (m *mockGateway) Overview(ctx context.Context, target AgentTarget, p OverviewParams) (RawCodeIntelResult, error) {
	m.record("overview")
	return m.overviewRes, m.overviewErr
}

func (m *mockGateway) Subgraph(ctx context.Context, target AgentTarget, p SubgraphParams) (RawCodeIntelResult, error) {
	m.record("subgraph")
	return m.subgraphRes, m.subgraphErr
}

func (m *mockGateway) Impact(ctx context.Context, target AgentTarget, p ImpactParams) (RawCodeIntelResult, error) {
	m.record("impact")
	return m.impactRes, m.impactErr
}

func (m *mockGateway) Symbol(ctx context.Context, target AgentTarget, p SymbolParams) (RawCodeIntelResult, error) {
	m.record("symbol")
	return m.symbolRes, m.symbolErr
}

func (m *mockGateway) Routes(ctx context.Context, target AgentTarget, p RoutesParams) (RawCodeIntelResult, error) {
	m.record("routes")
	return m.routesRes, m.routesErr
}

func (m *mockGateway) DetectChanges(ctx context.Context, target AgentTarget, p DetectChangesParams) (RawCodeIntelResult, error) {
	m.record("detectChanges")
	return m.changesRes, m.changesErr
}

func (m *mockGateway) Processes(ctx context.Context, target AgentTarget, p ProcessesParams) (RawCodeIntelResult, error) {
	m.record("processes")
	return m.processesRes, m.processesErr
}

func (m *mockGateway) Process(ctx context.Context, target AgentTarget, p ProcessParams) (RawCodeIntelResult, error) {
	m.record("process")
	return m.processRes, m.processErr
}

func (m *mockGateway) StructuralFacts(ctx context.Context, target AgentTarget, p StructuralFactsParams) (RawCodeIntelResult, error) {
	m.record("structuralFacts")
	return m.structuralRes, m.structuralErr
}

func (m *mockGateway) Reindex(ctx context.Context, target AgentTarget, p ReindexParams) (RawCodeIntelResult, error) {
	m.record("reindex")
	return m.reindexRes, m.reindexErr
}

func (m *mockGateway) ReindexStatus(ctx context.Context, target AgentTarget, p ReindexStatusParams) (RawCodeIntelResult, error) {
	m.record("reindexStatus")
	return m.reindexStatRes, m.reindexStatErr
}

func (m *mockGateway) ReindexCancel(ctx context.Context, target AgentTarget, p ReindexCancelParams) (RawCodeIntelResult, error) {
	m.record("reindexCancel")
	return m.reindexCancRes, m.reindexCancErr
}

func (m *mockGateway) Watch(ctx context.Context, target AgentTarget, p WatchParams) (RawCodeIntelResult, error) {
	m.record("watch")
	return m.watchRes, m.watchErr
}

func (m *mockGateway) CodegraphSearch(ctx context.Context, target AgentTarget, p CodegraphSearchParams) (RawCodeIntelResult, error) {
	m.record("codegraphSearch")
	return m.cgSearchRes, m.cgSearchErr
}

func (m *mockGateway) Files(ctx context.Context, target AgentTarget, p FilesParams) (RawCodeIntelResult, error) {
	m.record("files")
	return m.filesRes, m.filesErr
}

func sampleStatusDataJSON() []byte {
	now := time.Now().UTC()
	sd := domain.AgentStatusData{
		Tools: map[string]domain.ToolAvailability{
			"gitnexus":  {Available: true, Version: "1.2.0"},
			"codegraph": {Available: true, Version: "2.4.0"},
		},
		Indexes: domain.AgentStatusIndexes{
			GitNexus: &domain.GitNexusIndexStatus{
				State:         "ready",
				IndexedCommit: "commit-123",
				IndexedAt:     &now,
			},
			CodeGraph: &domain.CodeGraphIndexStatus{
				State:      "ready",
				HeadCommit: "commit-123",
				IndexedAt:  &now,
			},
		},
	}
	b, _ := json.Marshal(sd)
	return b
}

func TestCollectView_AllTable2EViews(t *testing.T) {
	gw := newMockGateway()
	gw.statusRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data:       sampleStatusDataJSON(),
	}

	target := AgentTarget{
		TenantID:      "tenant-1",
		DevServerID:   "dev-1",
		WorkspaceRoot: "/workspace",
		HostPlatform:  "linux",
	}

	collector := NewCollectView(gw, nil, nil)
	ctx := context.Background()

	// 1. STATUS
	res, err := collector.Get(ctx, target, domain.ViewKindStatus, nil)
	if err != nil {
		t.Fatalf("STATUS failed: %v", err)
	}
	if _, ok := res.Data.(domain.AgentStatusData); !ok {
		t.Fatalf("STATUS expected domain.AgentStatusData, got %T", res.Data)
	}
	if len(res.Meta.Sources) != 2 {
		t.Errorf("STATUS sources count expected 2, got %d", len(res.Meta.Sources))
	}

	// 2. CLUSTERS (Overview)
	gw.overviewRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data: []byte(`{
			"nodes": [{"id":"c1","label":"auth","symbolCount":50}],
			"edges": [{"from":"c1","to":"c2","weight":2.0,"kinds":{"CALLS":2}}]
		}`),
	}
	res, err = collector.Get(ctx, target, domain.ViewKindClusters, OverviewParams{TopN: 10})
	if err != nil {
		t.Fatalf("CLUSTERS failed: %v", err)
	}
	arch, ok := res.Data.(domain.ArchitectureGraph)
	if !ok || len(arch.Nodes) != 1 {
		t.Fatalf("CLUSTERS expected ArchitectureGraph with 1 node, got %+v", res.Data)
	}

	// 3. STRUCTURE (converts SymbolGraph to ModuleGraph)
	gw.subgraphRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data: []byte(`{
			"center": {"key":"function:main.go:main","kind":"function","name":"main","filePath":"main.go"},
			"nodes": [
				{"key":"file:main.go:main.go","kind":"file","filePath":"main.go","cluster":"app"},
				{"key":"file:util.go:util.go","kind":"file","filePath":"util.go","cluster":"app"}
			],
			"edges": [
				{"fromKey":"file:main.go:main.go","toKey":"file:util.go:util.go","kind":"imports"}
			],
			"depth": 1
		}`),
	}
	res, err = collector.Get(ctx, target, domain.ViewKindStructure, SubgraphParams{CenterFile: "main.go"})
	if err != nil {
		t.Fatalf("STRUCTURE failed: %v", err)
	}
	mg, ok := res.Data.(domain.ModuleGraph)
	if !ok || len(mg.Nodes) != 2 {
		t.Fatalf("STRUCTURE expected ModuleGraph with 2 nodes, got %+v", res.Data)
	}
	if len(mg.Edges) != 1 || !strings.EqualFold(mg.Edges[0].Kind, "imports") {
		t.Fatalf("STRUCTURE expected ModuleGraph with 1 imports edge, got %+v", mg.Edges)
	}

	// 4. SUBGRAPH
	res, err = collector.Get(ctx, target, domain.ViewKindSubgraph, SubgraphParams{CenterSymbol: "function:main.go:main"})
	if err != nil {
		t.Fatalf("SUBGRAPH failed: %v", err)
	}
	sg, ok := res.Data.(domain.SymbolGraph)
	if !ok || len(sg.Nodes) != 2 {
		t.Fatalf("SUBGRAPH expected SymbolGraph, got %+v", res.Data)
	}

	// 5. IMPACT
	gw.impactRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data: []byte(`{
			"target": {"key":"function:main.go:main","kind":"function","name":"main","filePath":"main.go"},
			"direction": "upstream",
			"risk": "HIGH",
			"impactedCount": 1,
			"levels": [
				{
					"depth": 1,
					"symbols": [
						{"symbol":{"key":"function:api.go:run","kind":"function","name":"run","filePath":"api.go"},"via":"calls","direct":true,"confidence":0.95}
					]
				}
			]
		}`),
	}
	res, err = collector.Get(ctx, target, domain.ViewKindImpact, ImpactParams{Key: "function:main.go:main"})
	if err != nil {
		t.Fatalf("IMPACT failed: %v", err)
	}
	ig, ok := res.Data.(domain.ImpactGraph)
	if !ok || ig.Risk != domain.RiskHigh || len(ig.Levels) != 1 {
		t.Fatalf("IMPACT expected ImpactGraph with RiskHigh, got %+v", res.Data)
	}

	// 6. SYMBOL (with default fail-closed redactor)
	gw.symbolRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data: []byte(`{
			"symbol": {"key":"function:main.go:main","kind":"function","name":"main","filePath":"main.go"},
			"source": {"text":"func main() {}","startLine":1,"endLine":1}
		}`),
	}
	res, err = collector.Get(ctx, target, domain.ViewKindSymbol, SymbolParams{Key: "function:main.go:main"})
	if err != nil {
		t.Fatalf("SYMBOL failed: %v", err)
	}
	symDetail, ok := res.Data.(domain.SymbolDetail)
	if !ok {
		t.Fatalf("SYMBOL expected SymbolDetail, got %T", res.Data)
	}
	if symDetail.Source.Text != "" {
		t.Errorf("expected source to be redacted (empty), got %q", symDetail.Source.Text)
	}
	if symDetail.SourceOmitted != "sensitive_path" {
		t.Errorf("expected sourceOmitted='sensitive_path', got %q", symDetail.SourceOmitted)
	}

	// 7. ROUTES
	gw.routesRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data: []byte(`{
			"routes": [{"id":"r1","path":"/api/login","method":"POST","filePath":"server.go"}],
			"edges": [{"routeId":"r1","handler":"LoginHandler","kind":"handles_route"}]
		}`),
	}
	res, err = collector.Get(ctx, target, domain.ViewKindRoutes, RoutesParams{Limit: 10})
	if err != nil {
		t.Fatalf("ROUTES failed: %v", err)
	}
	rm, ok := res.Data.(domain.RouteMap)
	if !ok || len(rm.Routes) != 1 {
		t.Fatalf("ROUTES expected RouteMap, got %+v", res.Data)
	}

	// 8. CHANGE_OVERLAY
	gw.changesRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data:       []byte(`{"base":{"oid":"abc"},"changedFiles":[]}`),
	}
	res, err = collector.Get(ctx, target, domain.ViewKindChangeOverlay, DetectChangesParams{})
	if err != nil {
		t.Fatalf("CHANGE_OVERLAY failed: %v", err)
	}
	rawOverlay, ok := res.Data.(RawCodeIntelResult)
	if !ok || len(rawOverlay.Data) == 0 {
		t.Fatalf("CHANGE_OVERLAY expected raw result, got %T", res.Data)
	}

	// 9. FLOWS
	gw.processesRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data:       []byte(`{"flows":[{"id":"flow1"}]}`),
	}
	res, err = collector.Get(ctx, target, domain.ViewKindFlows, ProcessesParams{})
	if err != nil {
		t.Fatalf("FLOWS failed: %v", err)
	}

	// 10. FLOW
	gw.processRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data:       []byte(`{"flow":{"id":"flow1"}}`),
	}
	res, err = collector.Get(ctx, target, domain.ViewKindFlow, ProcessParams{ProcessID: "flow1"})
	if err != nil {
		t.Fatalf("FLOW failed: %v", err)
	}
}

func TestCollectView_5000NodesTo1500Truncation(t *testing.T) {
	gw := newMockGateway()
	gw.statusRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data:       sampleStatusDataJSON(),
	}

	// Build a 5,000 node subgraph
	type testNode struct {
		Key       string `json:"key"`
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		FilePath  string `json:"filePath"`
		StartLine int32  `json:"startLine"`
	}

	var nodes []testNode
	for i := 0; i < 5000; i++ {
		nodes = append(nodes, testNode{
			Key:       fmt.Sprintf("function:file%d.go:func%d", i, i),
			Kind:      "function",
			Name:      fmt.Sprintf("func%d", i),
			FilePath:  fmt.Sprintf("file%d.go", i),
			StartLine: int32(i + 1),
		})
	}

	payload := map[string]any{
		"center": map[string]any{
			"key":      "function:file0.go:func0",
			"kind":     "function",
			"name":     "func0",
			"filePath": "file0.go",
		},
		"nodes": nodes,
		"edges": []any{},
		"depth": 1,
	}
	payloadBytes, _ := json.Marshal(payload)

	totalCount := int64(5000)
	gw.subgraphRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		TotalCount: &totalCount,
		Data:       payloadBytes,
	}

	target := AgentTarget{
		TenantID:      "tenant-1",
		DevServerID:   "dev-1",
		WorkspaceRoot: "/workspace",
	}

	collector := NewCollectView(gw, nil, nil)
	res, err := collector.Get(context.Background(), target, domain.ViewKindSubgraph, SubgraphParams{})
	if err != nil {
		t.Fatalf("Get subgraph failed: %v", err)
	}

	sg, ok := res.Data.(domain.SymbolGraph)
	if !ok {
		t.Fatalf("expected SymbolGraph, got %T", res.Data)
	}

	if len(sg.Nodes) != domain.SymbolGraphNodesMax {
		t.Errorf("expected %d nodes after limit, got %d", domain.SymbolGraphNodesMax, len(sg.Nodes))
	}
	if !res.Meta.Truncated {
		t.Errorf("expected meta.Truncated = true, got false")
	}
	if res.Meta.TotalCount != 5000 {
		t.Errorf("expected meta.TotalCount = 5000, got %d", res.Meta.TotalCount)
	}
	if res.Meta.TotalBefore != 5000 {
		t.Errorf("expected meta.TotalBefore = 5000, got %d", res.Meta.TotalBefore)
	}
}

func TestCollectView_StatusFailureDoesNotFailView(t *testing.T) {
	gw := newMockGateway()
	gw.statusErr = errors.New("status probe connection timed out")

	gw.overviewRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data: []byte(`{
			"nodes": [{"id":"c1","label":"auth","symbolCount":10}],
			"edges": []
		}`),
	}

	target := AgentTarget{WorkspaceRoot: "/workspace"}
	collector := NewCollectView(gw, nil, nil)

	res, err := collector.Get(context.Background(), target, domain.ViewKindClusters, OverviewParams{})
	if err != nil {
		t.Fatalf("view should not fail when status probe fails: %v", err)
	}

	hasWarning := false
	for _, w := range res.Meta.Warnings {
		if w == "status_unavailable" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("expected warning 'status_unavailable' in meta.Warnings: %+v", res.Meta.Warnings)
	}
}

func TestCollectView_MainDataFailureFailsView(t *testing.T) {
	gw := newMockGateway()
	gw.statusRes = RawCodeIntelResult{Data: sampleStatusDataJSON()}
	gw.overviewErr = apperrors.New(apperrors.KindUnavailable, "DEV_SERVER_OFFLINE", "dev server is offline", nil)

	target := AgentTarget{WorkspaceRoot: "/workspace"}
	collector := NewCollectView(gw, nil, nil)

	_, err := collector.Get(context.Background(), target, domain.ViewKindClusters, OverviewParams{})
	if err == nil {
		t.Fatalf("expected error when main data call fails, got nil")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "DEV_SERVER_OFFLINE" {
		t.Errorf("expected DEV_SERVER_OFFLINE apperror, got: %v", err)
	}
}

func TestCollectView_AmbiguousSymbolError(t *testing.T) {
	gw := newMockGateway()
	gw.statusRes = RawCodeIntelResult{Data: sampleStatusDataJSON()}
	gw.impactErr = apperrors.New(apperrors.KindInvalidArgument, "AMBIGUOUS_SYMBOL", "symbol target matches 3 candidates", nil)

	target := AgentTarget{WorkspaceRoot: "/workspace"}
	collector := NewCollectView(gw, nil, nil)

	_, err := collector.Get(context.Background(), target, domain.ViewKindImpact, ImpactParams{})
	if err == nil {
		t.Fatalf("expected AMBIGUOUS_SYMBOL error, got nil")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "AMBIGUOUS_SYMBOL" {
		t.Errorf("expected AMBIGUOUS_SYMBOL error, got: %v", err)
	}
}

type allowingRedactor struct{}

func (a *allowingRedactor) RedactSource(ctx context.Context, repo, path string, source *domain.SymbolSource) (*domain.SymbolSource, string, error) {
	return source, "", nil
}

func TestCollectView_SourceRedactorBehavior(t *testing.T) {
	gw := newMockGateway()
	gw.symbolRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data: []byte(`{
			"symbol": {"key":"function:main.go:main","kind":"function","name":"main","filePath":"main.go"},
			"source": {"text":"package main\n\nfunc main() {}","startLine":1,"endLine":3}
		}`),
	}

	target := AgentTarget{WorkspaceRoot: "/workspace"}
	ctx := context.Background()

	// 1. Without redactor (default fail-closed)
	collectorFailClosed := NewCollectView(gw, nil, nil)
	res1, err := collectorFailClosed.Get(ctx, target, domain.ViewKindSymbol, SymbolParams{Key: "function:main.go:main"})
	if err != nil {
		t.Fatalf("symbol view failed: %v", err)
	}
	detail1 := res1.Data.(domain.SymbolDetail)
	if detail1.Source.Text != "" {
		t.Errorf("fail-closed redactor must wipe source text, got %q", detail1.Source.Text)
	}
	if detail1.SourceOmitted != "sensitive_path" {
		t.Errorf("fail-closed redactor must set sourceOmitted='sensitive_path', got %q", detail1.SourceOmitted)
	}

	// 2. With allowing redactor
	collectorAllowed := NewCollectView(gw, &allowingRedactor{}, nil)
	res2, err := collectorAllowed.Get(ctx, target, domain.ViewKindSymbol, SymbolParams{Key: "function:main.go:main"})
	if err != nil {
		t.Fatalf("symbol view failed: %v", err)
	}
	detail2 := res2.Data.(domain.SymbolDetail)
	if detail2.Source.Text != "package main\n\nfunc main() {}" {
		t.Errorf("allowing redactor must preserve source text, got %q", detail2.Source.Text)
	}
	if detail2.SourceOmitted != "" {
		t.Errorf("allowing redactor must clear sourceOmitted, got %q", detail2.SourceOmitted)
	}
}

func TestCollectView_StalePreservation(t *testing.T) {
	gw := newMockGateway()
	// Stale is true in raw envelope
	gw.overviewRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Stale:      true,
		Data:       []byte(`{"nodes":[],"edges":[]}`),
	}
	// Status has matching commit
	gw.statusRes = RawCodeIntelResult{
		HeadCommit: "commit-123",
		Data:       sampleStatusDataJSON(),
	}

	collector := NewCollectView(gw, nil, nil)
	res, err := collector.Get(context.Background(), AgentTarget{}, domain.ViewKindClusters, OverviewParams{})
	if err != nil {
		t.Fatalf("overview failed: %v", err)
	}
	if !res.Meta.Stale {
		t.Errorf("stale from raw envelope must never be downgraded to false")
	}

	// Another case: raw Stale is false, but headCommit differs from index commit
	gw.overviewRes = RawCodeIntelResult{
		HeadCommit: "commit-456", // different!
		Stale:      false,
		Data:       []byte(`{"nodes":[],"edges":[]}`),
	}
	res2, err := collector.Get(context.Background(), AgentTarget{}, domain.ViewKindClusters, OverviewParams{})
	if err != nil {
		t.Fatalf("overview failed: %v", err)
	}
	if !res2.Meta.Stale {
		t.Errorf("stale must be true when headCommit differs from source commit")
	}
}
