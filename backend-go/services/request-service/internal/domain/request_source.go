package domain

type SourceProvider string

const (
	SourceProviderJira    SourceProvider = "jira"
	SourceProviderGithub  SourceProvider = "github"
	SourceProviderGitlab  SourceProvider = "gitlab"
	SourceProviderLinear  SourceProvider = "linear"
	SourceProviderMCP     SourceProvider = "mcp"
	SourceProviderManual  SourceProvider = "manual"
	SourceProviderWebhook SourceProvider = "webhook"
)

func ParseSourceProvider(s string) (SourceProvider, error) {
	switch s {
	case "jira", "github", "gitlab", "linear", "mcp", "manual", "webhook":
		return SourceProvider(s), nil
	default:
		return "", ErrRequestInvalidSourceProvider(s)
	}
}

type SourceRef struct {
	Provider SourceProvider
	Site     string
	Ref      string
	URL      string
}
