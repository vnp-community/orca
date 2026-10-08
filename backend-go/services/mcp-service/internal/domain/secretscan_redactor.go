package domain

import "github.com/stablyai/orca-go/common/secretscan"

// SecretscanRedactor redacts with the shared common/secretscan patterns (same vectors as request-service).
// It is an addition: SecretRedactor above stays the default until the owner of mcp-service switches over.
type SecretscanRedactor struct{}

func (SecretscanRedactor) Redact(s string) (string, bool) { return secretscan.Redact(s) }
