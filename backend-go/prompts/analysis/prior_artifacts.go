package analysis

import (
	"unicode/utf8"
)

// PriorArtifactsRunes caps the whole <prior_artifacts> block. Revisions are unlimited, so without a total the prompt
// would grow with every round.
const PriorArtifactsRunes = 40000

// priorView is the earlier attempts that fit the cap, each already rendered.
type priorView struct {
	Omitted int
	Entries []string
}

// newPriorView renders the earlier attempts. When they do not fit PriorArtifactsRunes the oldest are dropped first,
// because the newest version and its feedback say the most about what the reviewer wants now.
func newPriorView(arts []PriorArtifact) (priorView, error) {
	entries := make([]string, len(arts))
	for i, a := range arts {
		text, err := RenderPriorArtifact(i+1, a)
		if err != nil {
			return priorView{}, err
		}
		entries[i] = text
	}
	first := firstEntryWithinCap(entries)
	return priorView{Omitted: first, Entries: entries[first:]}, nil
}

// RenderPriorArtifact renders earlier attempt number n (counting from 1) as it appears inside <prior_artifacts>.
func RenderPriorArtifact(n int, a PriorArtifact) (string, error) {
	return set.Execute("_prior_artifact_entry.tmpl", struct {
		N int
		PriorArtifact
	}{n, a})
}

// firstEntryWithinCap is the index of the oldest entry kept. The newest entry is always kept: one entry is at most about
// twice request_context.ArtifactRunes, far under the cap.
func firstEntryWithinCap(entries []string) int {
	total := 0
	for i := len(entries) - 1; i >= 0; i-- {
		total += utf8.RuneCountInString(entries[i])
		if total > PriorArtifactsRunes && i < len(entries)-1 {
			return i + 1
		}
	}
	return 0
}
