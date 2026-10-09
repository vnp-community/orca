package usecase_test

import (
	"testing"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
	tt "github.com/stablyai/orca-go/services/mcp-service/internal/usecase/usecasetest"
)

// TASK-E2E-017: updating a policy id that is not a uuid (or names nothing) is NotFound, never MCP_INTERNAL.
func TestUpsertUnknownOrMalformedPolicyID_IsNotFound(t *testing.T) {
	for _, id := range []string{"not-a-uuid", "cccccccc-cccc-cccc-cccc-cccccccccccc"} {
		h := tt.NewHarness(true)
		p := pol(id, "deny", domain.ToolPolicyMatch{Tool: "task_create"})
		p.Version = 1
		_, err := h.Policies.Upsert(tt.Ctx(tenantA, userAdm, "admin"), usecase.UpsertInput{Policy: p})
		if got := code(err); got != domain.CodeNotFound {
			t.Errorf("id %q: code = %q (%v), want %q", id, got, err, domain.CodeNotFound)
		}
	}
}
