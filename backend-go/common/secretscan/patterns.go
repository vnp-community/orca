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
	
	// Connection string with password. Group 1 is the password part. Wait, the spec says to redact via group? 
	// spec: \b[a-z][a-z0-9+.-]*://[^\s/:@]+:[^\s/@]+@[^\s/]+ (connection_string, che mật khẩu qua nhóm)
	// I need to add a capture group for the password.
	{kind: KindConnectionString, confidence: ConfidenceHigh, re: regexp.MustCompile(`\b[a-z][a-z0-9+.-]*://[^\s/:@]+:([^\s/@]+)@[^\s/]+`), valueGroup: 1},
	
	// .env lines. Group 2 is the secret.
	{kind: KindDotenvSecret, confidence: ConfidenceMedium, re: regexp.MustCompile(`(?m)^\s*(?:export\s+)?([A-Z][A-Z0-9_]*(?:SECRET|TOKEN|KEY|PASSWORD)[A-Z0-9_]*)\s*=\s*["']?([^\s"']{4,})`), valueGroup: 2},
	
	// Assignment from mcp-service.
	// (?i)(\b(?:password|passwd|secret|token|api[_-]?key|authorization)\b\s*["']?\s*[:=]\s*["']?)[^\s"',;&]{4,}
	// Here group 1 is the prefix, the secret itself is NOT in a group in mcp-service, it uses ${1}[REDACTED].
	// To match the `valueGroup` semantics where `valueGroup` is the secret, I will capture the secret in group 2.
	{kind: KindSecretAssignment, confidence: ConfidenceMedium, re: regexp.MustCompile(`(?i)(\b(?:password|passwd|secret|token|api[_-]?key|authorization)\b\s*["']?\s*[:=]\s*["']?)([^\s"',;&]{4,})`), valueGroup: 2},
}
