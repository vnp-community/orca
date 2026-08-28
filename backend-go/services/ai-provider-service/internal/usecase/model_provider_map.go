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
