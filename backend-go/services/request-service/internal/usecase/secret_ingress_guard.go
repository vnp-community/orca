package usecase

import (
	"github.com/stablyai/orca-go/common/secretscan"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// SecretIngressGuard masks high-confidence secrets in title and body before they are stored. A Request that
// pasted a credential is already an incident, so keeping the original in the database only widens the leak.
type SecretIngressGuard struct{}

// Apply returns the masked text, whether anything was masked, and the kinds found (never the values).
// Text beyond the scan window cannot be checked, so it is refused rather than stored unscanned.
func (SecretIngressGuard) Apply(title, body string) (newTitle, newBody string, suspected bool, kinds []secretscan.Kind, err error) {
	t := secretscan.RedactKinds(title, secretscan.ConfidenceHigh)
	b := secretscan.RedactKinds(body, secretscan.ConfidenceHigh)
	if t.Truncated {
		return "", "", false, nil, domain.ErrRequestPayloadTooLarge("title")
	}
	if b.Truncated {
		return "", "", false, nil, domain.ErrRequestPayloadTooLarge("body")
	}
	kinds = append(append(kinds, t.Kinds...), b.Kinds...)
	return t.Text, b.Text, len(kinds) > 0, kinds, nil
}
