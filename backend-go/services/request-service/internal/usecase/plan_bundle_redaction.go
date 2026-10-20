package usecase

import (
	"strings"
)

// bundleFile is one file of the authored change as read from the worktree, path relative to the change directory.
type bundleFile struct {
	Path    string
	Content []byte
}

// redactBundle builds the stored copy of a change: every file has its secrets masked, and a file that no longer fits the
// byte budget becomes a marker of its raw size and hash. It returns how many secrets were masked in the files it kept.
// The branch keeps the agent's raw text; only the database copy, which the UI and CLI read, is masked.
func redactBundle(files []bundleFile, budget int) (map[string]string, int) {
	bundle := make(map[string]string, len(files))
	total := 0
	for _, f := range files {
		text, n := RedactSecrets(string(f.Content))
		if len(text) > budget {
			bundle[f.Path] = omittedMarker(f.Content)
			continue
		}
		budget -= len(text)
		total += n
		bundle[f.Path] = text
	}
	return bundle, total
}

// isOmittedMarker tells a bundle entry that stands for a file too large to keep from stored text.
func isOmittedMarker(content string) bool {
	return strings.HasPrefix(content, "<omitted size=")
}
