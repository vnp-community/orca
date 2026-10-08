package secretscan

import "regexp"

type pattern struct {
	kind       Kind
	confidence Confidence
	re         *regexp.Regexp
	valueGroup int
}

// secretPatterns defines all supported redaction patterns.
// Order matters: more specific patterns (e.g. Anthropic key) must come before
// generic ones (e.g. sk-).
// Note: We deliberately exclude the old Vault s.[A-Za-z0-9]{24} pattern
// due to excessively high false positive rates.
var secretPatterns = []pattern{
	// From mcp-service
	{kind: KindGitHubToken, confidence: ConfidenceHigh, re: regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`)},
	{kind: KindGitHubToken, confidence: ConfidenceHigh, re: regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`)},
	{kind: KindAWSAccessKey, confidence: ConfidenceHigh, re: regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{kind: KindBearer, confidence: ConfidenceMedium, re: regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{8,}`)},

	// New patterns
	{kind: KindAnthropicKey, confidence: ConfidenceHigh, re: regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}`)},
	{kind: KindOpenAIKey, confidence: ConfidenceHigh, re: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`)}, // from mcp-service, slightly generic
	{kind: KindSlackToken, confidence: ConfidenceHigh, re: regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`)},
	{kind: KindJWT, confidence: ConfidenceMedium, re: regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`)},
	{kind: KindPrivateKey, confidence: ConfidenceHigh, re: regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?(?:-----END [A-Z ]*PRIVATE KEY-----|$)`)},

	// More new patterns
	{kind: KindGoogleAPIKey, confidence: ConfidenceHigh, re: regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`)},
	{kind: KindVaultToken, confidence: ConfidenceHigh, re: regexp.MustCompile(`hvs\.[A-Za-z0-9_-]{24,}`)},
	// Only the password (group 1) is redacted so the host stays readable.
	// I need to add a capture group for the password.
	{kind: KindConnectionString, confidence: ConfidenceHigh, re: regexp.MustCompile(`\b[a-z][a-z0-9+.-]*://[^\s/:@]+:([^\s/@]+)@[^\s/]+`), valueGroup: 1},

	// .env lines. Group 2 is the secret.
	{kind: KindDotenvSecret, confidence: ConfidenceMedium, re: regexp.MustCompile(`(?m)^\s*(?:export\s+)?([A-Z][A-Z0-9_]*(?:SECRET|TOKEN|KEY|PASSWORD)[A-Z0-9_]*)\s*=\s*["']?([^\s"']{4,})`), valueGroup: 2},

	// mcp-service keeps the key prefix and replaces the value; here the value is group 2 so the key stays visible too.
	{kind: KindSecretAssignment, confidence: ConfidenceMedium, re: regexp.MustCompile(`(?i)(\b(?:password|passwd|secret|token|api[_-]?key|authorization)\b\s*["']?\s*[:=]\s*["']?)([^\s"',;&]{4,})`), valueGroup: 2},
}
