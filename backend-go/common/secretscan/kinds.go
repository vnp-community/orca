// Package secretscan provides regex-based secret detection and redaction.
// This package duplicates patterns from mcp-service to ensure uniformity
// across the system (e.g., in gateways, evaluators). The common test vectors
// in testdata/vectors.json serve as the contract.
// Changing the patterns table requires incrementing PatternsVersion.
package secretscan

type Kind string

const (
	KindPrivateKey       Kind = "private_key"
	KindGitHubToken      Kind = "github_token"
	KindAWSAccessKey     Kind = "aws_access_key"
	KindBearer           Kind = "bearer_token"
	KindOpenAIKey        Kind = "openai_key"
	KindAnthropicKey     Kind = "anthropic_key"
	KindSlackToken       Kind = "slack_token"
	KindJWT              Kind = "jwt"
	KindGoogleAPIKey     Kind = "google_api_key"
	KindVaultToken       Kind = "vault_token"
	KindConnectionString Kind = "connection_string"
	KindDotenvSecret     Kind = "dotenv_secret"
	KindSecretAssignment Kind = "secret_assignment"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
)

type Finding struct {
	Kind       Kind
	Confidence Confidence
	Start      int
	End        int
}
