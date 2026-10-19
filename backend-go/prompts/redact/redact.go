// Package redact cleans text on its way into a prompt.
package redact

import (
	"context"

	"github.com/stablyai/orca-go/common/secretscan"
)

// Secrets masks credentials at every confidence. There is no switch to turn it off.
func Secrets(text string) string {
	out, _ := secretscan.Redact(text)
	return out
}

// TenantSettings reads the per-tenant prompt privacy switch.
type TenantSettings interface {
	RedactPII(ctx context.Context) (bool, error)
}

// PIIMasker masks emails, phone numbers and similar personal data; the PII redactor of the AI gateway plugs in.
type PIIMasker interface {
	Mask(text string) string
}

// Redactor masks secrets always and personal data when the tenant asks for it.
// A settings read error masks PII: failing closed costs some context, failing open leaks.
type Redactor struct {
	settings TenantSettings
	pii      PIIMasker
}

func NewRedactor(settings TenantSettings, pii PIIMasker) *Redactor {
	return &Redactor{settings: settings, pii: pii}
}

func (r *Redactor) Apply(ctx context.Context, text string) string {
	out := Secrets(text)
	if r.pii == nil {
		return out
	}
	maskPII := true
	if r.settings != nil {
		if v, err := r.settings.RedactPII(ctx); err == nil {
			maskPII = v
		}
	}
	if maskPII {
		out = r.pii.Mask(out)
	}
	return out
}
