package domain

type ContextItem struct {
	ID        string
	RequestID string
	Source    string
	Data      string
	Rank      int
}

func RankContextItems(items []ContextItem) []ContextItem {
	return items
}

func RedactContextEvidence(data string) string {
	return data
}
