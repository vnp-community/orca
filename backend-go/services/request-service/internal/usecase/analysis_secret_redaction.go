package usecase

import (
	"encoding/json"

	"github.com/stablyai/orca-go/common/secretscan"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RedactSecrets replaces secrets found by common/secretscan with [REDACTED] and counts them.
// It reduces leakage; it does not guarantee that no secret survives (pattern coverage is unmeasured).
func RedactSecrets(s string) (string, int) {
	n := len(secretscan.Scan(s))
	if n == 0 {
		return s, 0
	}
	out, _ := secretscan.Redact(s)
	return out, n
}

// RedactRaw bounds the stored raw model output first: secretscan only scans a window, so unbounded input would leave a tail unredacted.
func RedactRaw(s string) string {
	out, _ := RedactSecrets(domain.TruncateRaw(s))
	return out
}

// RedactDocument redacts every string value of a validated analysis document before it is stored, and
// returns how many secrets were replaced. Walking the generic tree covers excerpts, answers and any field added later.
func RedactDocument(raw []byte) ([]byte, int, error) {
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, 0, err
	}
	total := 0
	tree = redactTree(tree, &total)
	out, err := json.Marshal(tree)
	return out, total, err
}

func redactTree(v any, total *int) any {
	switch x := v.(type) {
	case string:
		out, n := RedactSecrets(x)
		*total += n
		return out
	case []any:
		for i := range x {
			x[i] = redactTree(x[i], total)
		}
		return x
	case map[string]any:
		for k, item := range x {
			x[k] = redactTree(item, total)
		}
		return x
	}
	return v
}
