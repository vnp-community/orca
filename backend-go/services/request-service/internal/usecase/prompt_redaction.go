package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/secretscan"
)

// TenantSecuritySettings reads the per-tenant prompt privacy switch.
type TenantSecuritySettings interface {
	RedactPII(ctx context.Context) (bool, error)
}

// PIIMasker masks emails, phone numbers and similar personal data; the PII redactor of the AI gateway plugs in.
type PIIMasker interface {
	Mask(text string) string
}

// PromptRedactor is the single place that cleans text on its way into a prompt: secrets are always masked
// (every confidence, no switch to turn it off) and personal data too when the tenant asks for it.
// A settings read error masks PII: failing closed costs some context, failing open leaks.
type PromptRedactor struct {
	settings TenantSecuritySettings
	pii      PIIMasker
}

func NewPromptRedactor(settings TenantSecuritySettings, pii PIIMasker) *PromptRedactor {
	return &PromptRedactor{settings: settings, pii: pii}
}

func (p *PromptRedactor) Apply(ctx context.Context, text string) string {
	out, _ := secretscan.Redact(text)
	if p.pii == nil {
		return out
	}
	maskPII := true
	if p.settings != nil {
		if v, err := p.settings.RedactPII(ctx); err == nil {
			maskPII = v
		}
	}
	if maskPII {
		out = p.pii.Mask(out)
	}
	return out
}
