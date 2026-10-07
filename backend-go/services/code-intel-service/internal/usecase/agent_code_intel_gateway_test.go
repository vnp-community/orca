package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

type mockAgentRPCCaller struct {
	calledMethod string
	calledTarget AgentTarget
	calledParams map[string]any
	retResult    RawCodeIntelResult
	retErr       error
}

func (m *mockAgentRPCCaller) Call(ctx context.Context, target AgentTarget, method string, params map[string]any) (RawCodeIntelResult, error) {
	m.calledTarget = target
	m.calledMethod = method
	m.calledParams = params
	return m.retResult, m.retErr
}

func TestAgentParams_ReflectionNoProhibitedFieldNames(t *testing.T) {
	prohibited := []string{
		"args", "argv", "command", "cmd", "cwd", "env", "repo", "cypher", "shell", "timeout", "tool",
	}

	paramStructs := []any{
		StatusParams{},
		OverviewParams{},
		ProcessesParams{},
		ProcessParams{},
		SubgraphParams{},
		ImpactParams{},
		SymbolParams{},
		RoutesParams{},
		DetectChangesParams{},
		StructuralFactsParams{},
		ReindexParams{},
		ReindexStatusParams{},
		ReindexCancelParams{},
		WatchParams{},
		CodegraphSearchParams{},
		FilesParams{},
	}

	for _, s := range paramStructs {
		val := reflect.ValueOf(s)
		typ := val.Type()
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			fieldNameLower := strings.ToLower(field.Name)
			for _, bad := range prohibited {
				if fieldNameLower == bad {
					t.Errorf("struct %s has prohibited field name %q", typ.Name(), field.Name)
				}
			}
		}
	}
}

func TestAgentParams_BoundaryAndValidationChecks(t *testing.T) {
	ctx := context.Background()
	caller := &mockAgentRPCCaller{}
	gw := NewAgentCodeIntelGateway(caller)
	target := AgentTarget{
		TenantID:      "t-1",
		DevServerID:   "ds-1",
		WorkspaceRoot: "/workspace/project",
	}

	assertCodeIntelInvalidParams := func(t *testing.T, err error, caseName string) {
		t.Helper()
		if err == nil {
			t.Fatalf("[%s] expected error, got nil", caseName)
		}
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Code != "CODEINTEL_INVALID_PARAMS" {
			if !strings.Contains(err.Error(), "CODEINTEL_INVALID_PARAMS") {
				t.Errorf("[%s] error code = %v, want CODEINTEL_INVALID_PARAMS", caseName, err)
			}
		}
	}

	// 1. StatusParams
	t.Run("StatusParams", func(t *testing.T) {
		err := (StatusParams{BaseRef: "-invalid"}).Validate()
		assertCodeIntelInvalidParams(t, err, "BaseRef starting with -")

		_, err = gw.Status(ctx, target, StatusParams{BaseRef: "origin/main"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["workspaceRoot"] != "/workspace/project" || caller.calledParams["baseRef"] != "origin/main" {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 2. OverviewParams
	t.Run("OverviewParams", func(t *testing.T) {
		err := (OverviewParams{TopN: 501}).Validate()
		assertCodeIntelInvalidParams(t, err, "TopN=501")

		err = (OverviewParams{MaxEdges: 5001}).Validate()
		assertCodeIntelInvalidParams(t, err, "MaxEdges=5001")

		err = (OverviewParams{EdgeKinds: []string{"INVALID_KIND"}}).Validate()
		assertCodeIntelInvalidParams(t, err, "invalid edge kind")

		withTop := true
		_, err = gw.Overview(ctx, target, OverviewParams{TopN: 100, MaxEdges: 1000, EdgeKinds: []string{"CALLS"}, WithTopFiles: &withTop})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["workspaceRoot"] != "/workspace/project" || caller.calledParams["topN"] != 100 {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 3. ProcessesParams
	t.Run("ProcessesParams", func(t *testing.T) {
		err := (ProcessesParams{Limit: 101}).Validate()
		assertCodeIntelInvalidParams(t, err, "Limit=101")

		err = (ProcessesParams{Offset: -1}).Validate()
		assertCodeIntelInvalidParams(t, err, "Offset=-1")

		_, err = gw.Processes(ctx, target, ProcessesParams{Limit: 20, Offset: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["workspaceRoot"] != "/workspace/project" || caller.calledParams["limit"] != 20 {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 4. ProcessParams
	t.Run("ProcessParams", func(t *testing.T) {
		err := (ProcessParams{ProcessID: ""}).Validate()
		assertCodeIntelInvalidParams(t, err, "empty processId")

		err = (ProcessParams{ProcessID: "-bad"}).Validate()
		assertCodeIntelInvalidParams(t, err, "processId starting with -")

		_, err = gw.Process(ctx, target, ProcessParams{ProcessID: "proc_1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["processId"] != "proc_1" {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 5. SubgraphParams
	t.Run("SubgraphParams", func(t *testing.T) {
		err := (SubgraphParams{Depth: 4, CenterSymbol: "sym1"}).Validate()
		assertCodeIntelInvalidParams(t, err, "depth=4")

		err = (SubgraphParams{Limit: 1501, CenterSymbol: "sym1"}).Validate()
		assertCodeIntelInvalidParams(t, err, "limit=1501")

		err = (SubgraphParams{CenterFile: "../escape.go"}).Validate()
		assertCodeIntelInvalidParams(t, err, "relative path containing ..")

		err = (SubgraphParams{CenterSymbol: "-flag"}).Validate()
		assertCodeIntelInvalidParams(t, err, "centerSymbol with -")

		err = (SubgraphParams{}).Validate()
		assertCodeIntelInvalidParams(t, err, "missing center")

		_, err = gw.Subgraph(ctx, target, SubgraphParams{CenterSymbol: "sym1", Depth: 2, Limit: 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["depth"] != 2 || caller.calledParams["limit"] != 100 {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 6. ImpactParams
	t.Run("ImpactParams", func(t *testing.T) {
		err := (ImpactParams{Depth: 4, UID: "uid1"}).Validate()
		assertCodeIntelInvalidParams(t, err, "depth=4")

		err = (ImpactParams{Limit: 301, UID: "uid1"}).Validate()
		assertCodeIntelInvalidParams(t, err, "limit=301")

		err = (ImpactParams{File: "../bad.go", UID: "uid1"}).Validate()
		assertCodeIntelInvalidParams(t, err, "file containing ..")

		err = (ImpactParams{UID: "-flag"}).Validate()
		assertCodeIntelInvalidParams(t, err, "uid starting with -")

		_, err = gw.Impact(ctx, target, ImpactParams{UID: "uid1", Direction: "upstream", Depth: 2})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["direction"] != "upstream" {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 7. SymbolParams
	t.Run("SymbolParams", func(t *testing.T) {
		err := (SymbolParams{RelationLimit: 51, UID: "sym1"}).Validate()
		assertCodeIntelInvalidParams(t, err, "relationLimit=51")

		err = (SymbolParams{File: "../bad.go", Name: "Fn"}).Validate()
		assertCodeIntelInvalidParams(t, err, "file containing ..")

		err = (SymbolParams{}).Validate()
		assertCodeIntelInvalidParams(t, err, "empty identifier")

		_, err = gw.Symbol(ctx, target, SymbolParams{UID: "sym1", RelationLimit: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["uid"] != "sym1" {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 8. RoutesParams
	t.Run("RoutesParams", func(t *testing.T) {
		err := (RoutesParams{Limit: 501}).Validate()
		assertCodeIntelInvalidParams(t, err, "limit=501")

		_, err = gw.Routes(ctx, target, RoutesParams{Limit: 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["limit"] != 100 {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 9. DetectChangesParams
	t.Run("DetectChangesParams", func(t *testing.T) {
		err := (DetectChangesParams{Base: "-flag"}).Validate()
		assertCodeIntelInvalidParams(t, err, "base starting with -")

		err = (DetectChangesParams{Head: "-bad"}).Validate()
		assertCodeIntelInvalidParams(t, err, "head starting with -")

		_, err = gw.DetectChanges(ctx, target, DetectChangesParams{Base: "main"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["base"] != "main" {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 10. StructuralFactsParams
	t.Run("StructuralFactsParams", func(t *testing.T) {
		err := (StructuralFactsParams{FactKind: "invalidKind"}).Validate()
		assertCodeIntelInvalidParams(t, err, "invalid fact kind")

		err = (StructuralFactsParams{FactKind: "cycles", PathPrefixes: []string{"../escape/"}}).Validate()
		assertCodeIntelInvalidParams(t, err, "pathPrefix containing ..")

		err = (StructuralFactsParams{FactKind: "fileSizes", Limit: 5001}).Validate()
		assertCodeIntelInvalidParams(t, err, "limit=5001")

		_, err = gw.StructuralFacts(ctx, target, StructuralFactsParams{FactKind: "cycles", Limit: 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["kind"] != "cycles" {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 11. ReindexParams
	t.Run("ReindexParams", func(t *testing.T) {
		err := (ReindexParams{Mode: "badMode"}).Validate()
		assertCodeIntelInvalidParams(t, err, "badMode")

		err = (ReindexParams{Trigger: "badTrigger"}).Validate()
		assertCodeIntelInvalidParams(t, err, "badTrigger")

		err = (ReindexParams{SelectedTools: []string{"unknownTool"}}).Validate()
		assertCodeIntelInvalidParams(t, err, "unknownTool")

		_, err = gw.Reindex(ctx, target, ReindexParams{Mode: "incremental", SelectedTools: []string{"gitnexus"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tools, ok := caller.calledParams["tools"].([]string)
		if !ok || len(tools) != 1 || tools[0] != "gitnexus" {
			t.Errorf("unexpected tools param: %v", caller.calledParams["tools"])
		}
	})

	// 12. ReindexStatusParams
	t.Run("ReindexStatusParams", func(t *testing.T) {
		err := (ReindexStatusParams{JobID: "-job"}).Validate()
		assertCodeIntelInvalidParams(t, err, "jobId starting with -")

		_, err = gw.ReindexStatus(ctx, target, ReindexStatusParams{JobID: "job-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["jobId"] != "job-1" {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 13. ReindexCancelParams
	t.Run("ReindexCancelParams", func(t *testing.T) {
		err := (ReindexCancelParams{JobID: ""}).Validate()
		assertCodeIntelInvalidParams(t, err, "empty jobId")

		_, err = gw.ReindexCancel(ctx, target, ReindexCancelParams{JobID: "job-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["jobId"] != "job-1" {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 14. WatchParams
	t.Run("WatchParams", func(t *testing.T) {
		_, err := gw.Watch(ctx, target, WatchParams{Enabled: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["enabled"] != true {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 15. CodegraphSearchParams
	t.Run("CodegraphSearchParams", func(t *testing.T) {
		err := (CodegraphSearchParams{Query: ""}).Validate()
		assertCodeIntelInvalidParams(t, err, "empty query")

		err = (CodegraphSearchParams{Query: "-bad"}).Validate()
		assertCodeIntelInvalidParams(t, err, "query starting with -")

		err = (CodegraphSearchParams{Query: "search", Limit: 51}).Validate()
		assertCodeIntelInvalidParams(t, err, "limit=51")

		_, err = gw.CodegraphSearch(ctx, target, CodegraphSearchParams{Query: "myFunc", Limit: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["search"] != "myFunc" || caller.calledParams["limit"] != 10 {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})

	// 16. FilesParams
	t.Run("FilesParams", func(t *testing.T) {
		err := (FilesParams{Filter: "../bad"}).Validate()
		assertCodeIntelInvalidParams(t, err, "filter containing ..")

		err = (FilesParams{Limit: 5001}).Validate()
		assertCodeIntelInvalidParams(t, err, "limit=5001")

		_, err = gw.Files(ctx, target, FilesParams{Filter: "src/main.ts", Limit: 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if caller.calledParams["filter"] != "src/main.ts" || caller.calledParams["limit"] != 100 {
			t.Errorf("unexpected params: %v", caller.calledParams)
		}
	})
}

func TestAgentStatus_Helpers(t *testing.T) {
	now := time.Now()
	data := domain.AgentStatusData{
		Binding: domain.BindingStatus{
			WorkspaceRoot:    "/worktree/main",
			RepoRoot:         "/worktree/main",
			LinkedWorktree:   true,
			WorktreeMismatch: true,
		},
		Indexes: domain.AgentStatusIndexes{
			GitNexus: &domain.GitNexusIndexStatus{
				State:      "ready",
				IndexScope: "repo_root",
				IndexedAt:  &now,
			},
			CodeGraph: &domain.CodeGraphIndexStatus{
				State: "ready",
				RootMismatch: &domain.RootMismatch{
					WorktreeRoot: "/worktree/main",
					IndexRoot:    "/worktree/secondary",
				},
				IndexScope: "repo_root",
			},
		},
	}

	if !data.HasWorktreeMismatch() {
		t.Error("expected HasWorktreeMismatch = true")
	}
	if !data.HasRootMismatch() {
		t.Error("expected HasRootMismatch = true")
	}
	if !data.IsScopeMismatch() {
		t.Error("expected IsScopeMismatch = true")
	}

	// Test clean status
	cleanData := domain.AgentStatusData{
		Binding: domain.BindingStatus{
			WorktreeMismatch: false,
		},
		Indexes: domain.AgentStatusIndexes{
			GitNexus: &domain.GitNexusIndexStatus{
				IndexScope: "exact",
			},
			CodeGraph: &domain.CodeGraphIndexStatus{
				IndexScope:   "exact",
				RootMismatch: nil,
			},
		},
	}
	if cleanData.HasWorktreeMismatch() {
		t.Error("expected clean HasWorktreeMismatch = false")
	}
	if cleanData.HasRootMismatch() {
		t.Error("expected clean HasRootMismatch = false")
	}
	if cleanData.IsScopeMismatch() {
		t.Error("expected clean IsScopeMismatch = false")
	}

	// JSON unmarshal verification
	rawJSON := `{
		"binding": {
			"workspaceRoot": "/app",
			"repoRoot": "/app",
			"linkedWorktree": false,
			"worktreeMismatch": false
		},
		"tools": {
			"gitnexus": { "available": true, "version": "1.6.9", "supported": true, "binary": "/bin/gitnexus" }
		},
		"indexes": {
			"gitnexus": {
				"state": "ready",
				"indexedCommit": "abc",
				"indexScope": "exact",
				"freshness": "fresh",
				"headCommit": "abc",
				"schemaVersion": 5,
				"storagePath": "/app/.gitnexus"
			}
		},
		"sqliteReadAvailable": true
	}`
	var decoded domain.AgentStatusData
	if err := json.Unmarshal([]byte(rawJSON), &decoded); err != nil {
		t.Fatalf("unmarshaling sample status JSON: %v", err)
	}
	if decoded.Binding.WorkspaceRoot != "/app" || decoded.Indexes.GitNexus.State != "ready" {
		t.Errorf("unexpected decoded values: %+v", decoded)
	}
}
