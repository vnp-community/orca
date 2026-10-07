package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCodeIntelViewSources_PlaceholdersValidation(t *testing.T) {
	r := NewRegistry()
	fake := &fakeGraphCoreClient{}
	deps := codeIntelDeps{
		core:   fake,
		limits: CodeIntelLimits{MaxResponseBytes: 2 << 20},
	}
	registerCodeIntelSourcesPlaceholders(r, deps)

	tests := []struct {
		channel string
		payload string
		errSub  string
	}{
		{"codeIntel.architecture", `{"projectId":"p1","worktreeId":"w1","container":"` + strings.Repeat("x", 513) + `"}`, "container"},
		{"codeIntel.dataFlows", `{"projectId":"p1","worktreeId":"w1","limit":-1}`, "limit"},
		{"codeIntel.dataFlows", `{"projectId":"p1","worktreeId":"w1","limit":101}`, "limit"},
		{"codeIntel.dataFlows", `{"projectId":"p1","worktreeId":"w1","query":"` + strings.Repeat("x", 129) + `"}`, "query"},
		{"codeIntel.dataFlow", `{"projectId":"p1","worktreeId":"w1"}`, "flowId"},
		{"codeIntel.dataFlow", `{"projectId":"p1","worktreeId":"w1","flowId":"f1","maxServiceHops":9}`, "maxServiceHops"},
		{"codeIntel.dataFlow", `{"projectId":"p1","worktreeId":"w1","flowId":"f1","maxSteps":201}`, "maxSteps"},
		{"codeIntel.dataFlow", `{"projectId":"p1","worktreeId":"w1","flowId":"f1","dialect":"oracle"}`, "dialect"},
		{"codeIntel.dataFlow", `{"projectId":"p1","worktreeId":"w1","flowId":"f1","detail":"xyz"}`, "detail"},
		
		// Valid cases should hit the CODEINTEL_UNAVAILABLE error
		{"codeIntel.architecture", `{"projectId":"p1","worktreeId":"w1"}`, "CODEINTEL_UNAVAILABLE"},
		{"codeIntel.dataFlows", `{"projectId":"p1","worktreeId":"w1"}`, "CODEINTEL_UNAVAILABLE"},
		{"codeIntel.dataFlow", `{"projectId":"p1","worktreeId":"w1","flowId":"f1"}`, "CODEINTEL_UNAVAILABLE"},
	}

	ctx := context.Background()
	id := Identity{TenantID: "t-1", UserID: "u-1"}

	for _, tc := range tests {
		t.Run(tc.channel+"_"+tc.errSub, func(t *testing.T) {
			_, err := r.Dispatch(ctx, id, tc.channel, []json.RawMessage{json.RawMessage(tc.payload)})
			if err == nil || !strings.Contains(err.Error(), tc.errSub) {
				t.Fatalf("expected error containing %q, got: %v", tc.errSub, err)
			}
		})
	}
}
