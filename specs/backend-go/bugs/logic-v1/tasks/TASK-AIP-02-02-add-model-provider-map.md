# TASK-AIP-02-02: Add `model_provider_map.go` (`detectProviderFromModel`)

**From Solution:** SOL-AIP-02
**Priority:** P0 — correctness bug
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/usecase/model_provider_map.go` (new)
**Depends on:** none
**Status:** `[x] DONE — model_provider_map.go added; TestDetectProviderFromModel passes (5 cases).`

---

## Context

`ai-provider-service.md` §4 requires the resolution cascade to be
"filtered to accounts of *that* provider," two-pass:
"model-hint-filtered first, then unfiltered" (`ai-provider-service.md:113-116`).
BUG-AIP-02's own finding: "no such table or function exists... anywhere
in backend-go." This is the Go equivalent of the TS resolver's
`MODEL_PROVIDER_MAP` — an in-process static table, zero cross-service
calls, matching §7/§8's "no cross-service call from `Resolve`" / p99 <
20ms budget exactly.

## Changes to make

Create `backend-go/services/ai-provider-service/internal/usecase/model_provider_map.go`:

```go
package usecase

import (
	"strings"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// modelProviderMap maps known model-name prefixes to the ProviderType that
// serves them — the Go equivalent of the TS resolver's MODEL_PROVIDER_MAP
// (ai-provider-service.md §4). Longest-prefix-wins ordering isn't needed
// today since no two prefixes overlap; keep entries specific (e.g. "o1-",
// "o3-") rather than adding a generic catch-all that could misclassify a
// future model family.
var modelProviderMap = []struct {
	prefix   string
	provider domain.ProviderType
}{
	{"claude-", domain.ProviderTypeAnthropic},
	{"gpt-", domain.ProviderTypeOpenAI},
	{"o1-", domain.ProviderTypeOpenAI},
	{"o3-", domain.ProviderTypeOpenAI},
	{"gemini-", domain.ProviderTypeGoogle},
	// azure/aws_bedrock/ollama/vllm models don't have a stable global
	// prefix (they're deployment-name-shaped) — callers targeting those
	// providers must set dev_server_id + rely on scope, or scoped_ref
	// (TASK-AIP-02-06), not model_hint detection.
}

// detectProviderFromModel returns the ProviderType a model name belongs to,
// and false if no known prefix matches — used only to narrow
// ResolveProvider's cascade, never to reject a request outright: a caller
// supplying dev_server_id without model_hint still resolves normally,
// unfiltered by provider.
func detectProviderFromModel(model string) (domain.ProviderType, bool) {
	for _, e := range modelProviderMap {
		if strings.HasPrefix(model, e.prefix) {
			return e.provider, true
		}
	}
	return "", false
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/ai-provider-service/...
go test ./services/ai-provider-service/internal/usecase/... -run TestDetectProviderFromModel
```

Add `TestDetectProviderFromModel` (table-driven): `"claude-3-5-sonnet"` →
Anthropic/true; `"gpt-4o"` → OpenAI/true; `"o1-preview"` → OpenAI/true;
`"gemini-1.5-pro"` → Google/true; `"llama-3-70b"` → `""`/false.
