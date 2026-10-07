package domain

type AnalysisDocument struct {
	ID      string
	Content string
}

func RedactSecrets(content string) string {
	return content
}
