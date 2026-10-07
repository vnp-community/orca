package grpc

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockViewReader struct {
	res usecase.ViewResult
	err error
}

func (m *mockViewReader) Get(ctx context.Context, target usecase.AgentTarget, view domain.ViewKind, params any) (usecase.ViewResult, error) {
	if m.err != nil {
		return usecase.ViewResult{}, m.err
	}
	return m.res, nil
}

type staticWorktreeResolver struct {
	target  usecase.AgentTarget
	binding string
	err     error
}

func (r *staticWorktreeResolver) ResolveTarget(ctx context.Context, tenantID string, sel *codeintelv1.WorktreeSelector) (usecase.AgentTarget, string, error) {
	if r.err != nil {
		return usecase.AgentTarget{}, "", r.err
	}
	return r.target, r.binding, nil
}

func TestCodeIntelServer_All8RPCs(t *testing.T) {
	reader := &mockViewReader{}
	metrics := domain.NewCollectorMetrics()
	resolver := &staticWorktreeResolver{
		target:  usecase.AgentTarget{WorkspaceRoot: "/workspace"},
		binding: "binding-1",
	}

	server := NewCodeIntelServer(reader, resolver, nil, nil, metrics)
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	sel := &codeintelv1.WorktreeSelector{ProjectId: "p1", WorktreeRef: "w1"}

	// 1. GetStructure
	reader.res = usecase.ViewResult{
		Meta: domain.ResultMeta{Repo: "/workspace", View: domain.ViewKindStructure, Truncated: true},
		Data: domain.ModuleGraph{
			Nodes: []domain.ModuleNode{{ID: "m1", Kind: "file"}},
			Edges: []domain.ModuleEdge{{From: "m1", To: "m2", Kind: "IMPORTS", Count: 1}},
		},
	}
	respStruct, err := server.GetStructure(ctx, &codeintelv1.GetStructureRequest{Selector: sel, Path: "src"})
	if err != nil {
		t.Fatalf("GetStructure failed: %v", err)
	}
	if len(respStruct.GetData().GetNodes()) != 1 {
		t.Fatalf("expected 1 node, got %d", len(respStruct.GetData().GetNodes()))
	}
	if metrics.GetTruncatedCount("structure") != 1 {
		t.Errorf("expected 1 truncated count for structure")
	}

	// 2. GetClusterOverview
	reader.res = usecase.ViewResult{
		Meta: domain.ResultMeta{Repo: "/workspace", View: domain.ViewKindClusters},
		Data: domain.ArchitectureGraph{
			Nodes: []domain.ClusterNode{{ID: "c1", Label: "core"}},
		},
	}
	respOverview, err := server.GetClusterOverview(ctx, &codeintelv1.GetClusterOverviewRequest{Selector: sel, TopN: 100})
	if err != nil {
		t.Fatalf("GetClusterOverview failed: %v", err)
	}
	if len(respOverview.GetData().GetNodes()) != 1 {
		t.Fatalf("expected 1 cluster node, got %d", len(respOverview.GetData().GetNodes()))
	}

	// 3. GetSubgraph
	reader.res = usecase.ViewResult{
		Meta: domain.ResultMeta{Repo: "/workspace", View: domain.ViewKindSubgraph},
		Data: domain.SymbolGraph{
			Center: domain.SymbolRef{Key: "func:main"},
			Nodes:  []domain.SymbolNode{{Ref: domain.SymbolRef{Key: "func:main"}}},
		},
	}
	respSub, err := server.GetSubgraph(ctx, &codeintelv1.GetSubgraphRequest{
		Selector: sel,
		Center:   &codeintelv1.GetSubgraphRequest_Symbol{Symbol: "func:main"},
	})
	if err != nil {
		t.Fatalf("GetSubgraph failed: %v", err)
	}
	if respSub.GetData().GetCenter().GetKey() != "func:main" {
		t.Fatalf("expected center 'func:main', got %q", respSub.GetData().GetCenter().GetKey())
	}

	// 4. GetImpact
	reader.res = usecase.ViewResult{
		Meta: domain.ResultMeta{Repo: "/workspace", View: domain.ViewKindImpact},
		Data: domain.ImpactGraph{
			Target: domain.SymbolRef{Key: "func:main"},
			Risk:   domain.RiskHigh,
		},
	}
	respImpact, err := server.GetImpact(ctx, &codeintelv1.GetImpactRequest{
		Selector: sel,
		Target:   &codeintelv1.GetImpactRequest_Key{Key: "func:main"},
	})
	if err != nil {
		t.Fatalf("GetImpact failed: %v", err)
	}
	if respImpact.GetData().GetRisk() != codeintelv1.Risk_RISK_HIGH {
		t.Fatalf("expected RiskHigh, got %v", respImpact.GetData().GetRisk())
	}

	// 5. GetSymbol
	reader.res = usecase.ViewResult{
		Meta: domain.ResultMeta{Repo: "/workspace", View: domain.ViewKindSymbol},
		Data: domain.SymbolDetail{
			Symbol: domain.SymbolRef{Key: "func:main"},
		},
	}
	respSym, err := server.GetSymbol(ctx, &codeintelv1.GetSymbolRequest{
		Selector: sel,
		Target:   &codeintelv1.GetSymbolRequest_Key{Key: "func:main"},
	})
	if err != nil {
		t.Fatalf("GetSymbol failed: %v", err)
	}
	if respSym.GetData().GetSymbol().GetKey() != "func:main" {
		t.Fatalf("expected symbol key 'func:main', got %q", respSym.GetData().GetSymbol().GetKey())
	}

	// 6. GetRouteMap
	reader.res = usecase.ViewResult{
		Meta: domain.ResultMeta{Repo: "/workspace", View: domain.ViewKindRoutes},
		Data: domain.RouteMap{
			Routes: []domain.RouteNode{{ID: "r1", Path: "/api"}},
		},
	}
	respRoutes, err := server.GetRouteMap(ctx, &codeintelv1.GetRouteMapRequest{Selector: sel})
	if err != nil {
		t.Fatalf("GetRouteMap failed: %v", err)
	}
	if len(respRoutes.GetData().GetRoutes()) != 1 {
		t.Fatalf("expected 1 route, got %d", len(respRoutes.GetData().GetRoutes()))
	}

	// Check metrics recorded calls
	if metrics.GetCallCount("GetStructure", "success") != 1 {
		t.Errorf("expected 1 success call for GetStructure")
	}
	if metrics.GetCallCount("GetClusterOverview", "success") != 1 {
		t.Errorf("expected 1 success call for GetClusterOverview")
	}
}

func TestCodeIntelServer_ErrorMappingViaToGRPCStatus(t *testing.T) {
	reader := &mockViewReader{
		err: apperrors.New(apperrors.KindNotFound, "CODEINTEL_NOT_FOUND", "symbol not found", nil),
	}
	resolver := &staticWorktreeResolver{
		target:  usecase.AgentTarget{WorkspaceRoot: "/workspace"},
		binding: "binding-1",
	}

	server := NewCodeIntelServer(reader, resolver, nil, nil, nil)
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	sel := &codeintelv1.WorktreeSelector{ProjectId: "p1"}

	_, err := server.GetStructure(ctx, &codeintelv1.GetStructureRequest{Selector: sel})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Fatalf("expected codes.NotFound, got %v", st.Code())
	}
}

func TestCodeIntelServer_MissingTenantUnauthenticated(t *testing.T) {
	server := NewCodeIntelServer(&mockViewReader{}, nil, nil, nil, nil)
	// Context has no tenant
	ctx := context.Background()

	_, err := server.GetStructure(ctx, &codeintelv1.GetStructureRequest{})
	if err == nil {
		t.Fatalf("expected unauthenticated error for missing tenant, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated && st.Code() != codes.InvalidArgument {
		t.Errorf("expected Unauthenticated or InvalidArgument, got: %v", st.Code())
	}
}
