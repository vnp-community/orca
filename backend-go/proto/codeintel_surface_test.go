package proto_test

import (
	"testing"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

func TestCodeIntelServiceDesc(t *testing.T) {
	desc := codeintelv1.CodeIntelService_ServiceDesc
	if desc.ServiceName != "orca.codeintel.v1.CodeIntelService" {
		t.Fatalf("expected ServiceName 'orca.codeintel.v1.CodeIntelService', got %q", desc.ServiceName)
	}
}

func TestCodeIntelCommonStructures(t *testing.T) {
	sel := &codeintelv1.WorktreeSelector{
		ProjectId:   "proj-1",
		WorktreeRef: "wt-ref-1",
	}
	if sel.GetProjectId() != "proj-1" || sel.GetWorktreeRef() != "wt-ref-1" {
		t.Fatalf("unexpected WorktreeSelector values: %+v", sel)
	}

	meta := &codeintelv1.ResultMeta{
		Repo:        "orca",
		WorktreeId:  "wt-1",
		View:        codeintelv1.ViewKind_VIEW_KIND_STRUCTURE,
		TotalCount:  10,
		NotModified: false,
	}
	if meta.GetView() != codeintelv1.ViewKind_VIEW_KIND_STRUCTURE {
		t.Fatalf("unexpected ViewKind: %v", meta.GetView())
	}
}

func TestQualityGateServiceDesc(t *testing.T) {
	desc := codeintelv1.QualityGateService_ServiceDesc
	if desc.ServiceName != "orca.codeintel.v1.QualityGateService" {
		t.Fatalf("expected QualityGateService ServiceName, got %q", desc.ServiceName)
	}
}

func TestCodeIntelGraphRPCRequestsAndResponses(t *testing.T) {
	sel := &codeintelv1.WorktreeSelector{ProjectId: "p1", WorktreeRef: "w1"}

	// 1. GetStructure
	req1 := &codeintelv1.GetStructureRequest{Selector: sel, Path: "src", Depth: 2, Limit: 50}
	if req1.GetSelector().GetProjectId() != "p1" || req1.GetPath() != "src" {
		t.Fatalf("unexpected GetStructureRequest: %+v", req1)
	}
	resp1 := &codeintelv1.GetStructureResponse{Meta: &codeintelv1.ResultMeta{Repo: "orca"}, Data: &codeintelv1.ModuleGraph{}}
	if resp1.GetMeta().GetRepo() != "orca" {
		t.Fatalf("unexpected GetStructureResponse: %+v", resp1)
	}

	// 2. GetClusterOverview
	req2 := &codeintelv1.GetClusterOverviewRequest{Selector: sel, TopN: 100}
	if req2.GetSelector().GetProjectId() != "p1" || req2.GetTopN() != 100 {
		t.Fatalf("unexpected GetClusterOverviewRequest: %+v", req2)
	}
	resp2 := &codeintelv1.GetClusterOverviewResponse{Meta: &codeintelv1.ResultMeta{Repo: "orca"}, Data: &codeintelv1.ArchitectureGraph{}}
	if resp2.GetData() == nil {
		t.Fatalf("unexpected GetClusterOverviewResponse: %+v", resp2)
	}

	// 3. GetSubgraph
	req3 := &codeintelv1.GetSubgraphRequest{
		Selector: sel,
		Center:   &codeintelv1.GetSubgraphRequest_Symbol{Symbol: "func:main"},
		Depth:    1,
		Limit:    800,
	}
	if req3.GetSymbol() != "func:main" {
		t.Fatalf("unexpected GetSubgraphRequest center: %+v", req3)
	}
	resp3 := &codeintelv1.GetSubgraphResponse{Meta: &codeintelv1.ResultMeta{}, Data: &codeintelv1.SymbolGraph{}}
	if resp3.GetData() == nil {
		t.Fatalf("unexpected GetSubgraphResponse: %+v", resp3)
	}

	// 4. GetImpact
	req4 := &codeintelv1.GetImpactRequest{
		Selector:  sel,
		Target:    &codeintelv1.GetImpactRequest_Key{Key: "func:main"},
		Direction: "upstream",
		Depth:     2,
	}
	if req4.GetKey() != "func:main" || req4.GetDirection() != "upstream" {
		t.Fatalf("unexpected GetImpactRequest: %+v", req4)
	}
	resp4 := &codeintelv1.GetImpactResponse{Meta: &codeintelv1.ResultMeta{}, Data: &codeintelv1.ImpactGraph{}}
	if resp4.GetData() == nil {
		t.Fatalf("unexpected GetImpactResponse: %+v", resp4)
	}

	// 5. GetSymbol
	req5 := &codeintelv1.GetSymbolRequest{
		Selector:      sel,
		Target:        &codeintelv1.GetSymbolRequest_Key{Key: "func:main"},
		IncludeSource: true,
	}
	if req5.GetKey() != "func:main" || !req5.GetIncludeSource() {
		t.Fatalf("unexpected GetSymbolRequest: %+v", req5)
	}
	resp5 := &codeintelv1.GetSymbolResponse{Meta: &codeintelv1.ResultMeta{}, Data: &codeintelv1.SymbolDetail{}}
	if resp5.GetData() == nil {
		t.Fatalf("unexpected GetSymbolResponse: %+v", resp5)
	}

	// 6. GetRouteMap
	req6 := &codeintelv1.GetRouteMapRequest{Selector: sel, Limit: 200}
	if req6.GetLimit() != 200 {
		t.Fatalf("unexpected GetRouteMapRequest: %+v", req6)
	}
	resp6 := &codeintelv1.GetRouteMapResponse{Meta: &codeintelv1.ResultMeta{}, Data: &codeintelv1.RouteMap{}}
	if resp6.GetData() == nil {
		t.Fatalf("unexpected GetRouteMapResponse: %+v", resp6)
	}

	// Check CodeIntelService_ServiceDesc contains all 6 unary methods
	expectedMethods := map[string]bool{
		"GetStructure":       true,
		"GetClusterOverview": true,
		"GetSubgraph":        true,
		"GetImpact":          true,
		"GetSymbol":          true,
		"GetRouteMap":        true,
	}
	for _, m := range codeintelv1.CodeIntelService_ServiceDesc.Methods {
		delete(expectedMethods, m.MethodName)
	}
	if len(expectedMethods) > 0 {
		t.Fatalf("missing expected methods in CodeIntelService_ServiceDesc: %+v", expectedMethods)
	}

	// Check StreamCodeIntelEvents is in Streams
	foundStream := false
	for _, s := range codeintelv1.CodeIntelService_ServiceDesc.Streams {
		if s.StreamName == "StreamCodeIntelEvents" {
			foundStream = true
			if !s.ServerStreams {
				t.Fatalf("expected StreamCodeIntelEvents to have ServerStreams=true")
			}
		}
	}
	if !foundStream {
		t.Fatalf("expected StreamCodeIntelEvents in CodeIntelService_ServiceDesc.Streams")
	}
}

func TestCodeIntelEventsPushMessage(t *testing.T) {
	req := &codeintelv1.StreamCodeIntelEventsRequest{
		Selectors: []*codeintelv1.WorktreeSelector{
			{ProjectId: "p1", WorktreeRef: "w1"},
		},
	}
	if len(req.GetSelectors()) != 1 {
		t.Fatalf("expected 1 selector, got %d", len(req.GetSelectors()))
	}

	pct := int32(75)
	push := &codeintelv1.CodeIntelPush{
		Kind:          "changed",
		EventId:       "event-123",
		WorktreeId:    "wt-1",
		RepoBindingId: "rb-1",
		Reason:        "index_changed",
		Tools:         []string{"gitnexus"},
		Commit:        "c-1",
		Percent:       &pct,
		State:         "running",
	}

	if push.GetKind() != "changed" || push.GetEventId() != "event-123" || push.GetPercent() != 75 {
		t.Fatalf("unexpected push message content: %+v", push)
	}
}


