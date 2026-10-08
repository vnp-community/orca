package domain

import (
	"strings"
	"unicode/utf8"

	"github.com/stablyai/orca-go/common/secretscan"
)

type AnalysisDocument struct {
	ID      string
	Content string
}

// secretScanWindow mirrors secretscan's scan limit: bytes past it are never inspected.
const secretScanWindow = 1024 * 1024

const secretScanTruncatedNote = "\n[TRUNCATED: unscanned remainder dropped]"

// RedactSecrets masks detected credentials. Text longer than the scan window is cut at the window because
// the scanner passes unscanned bytes through unchanged, which would leak a secret placed after it.
func RedactSecrets(content string) string {
	truncated := false
	if len(content) > secretScanWindow {
		content, truncated = content[:secretScanWindow], true
		for len(content) > 0 && !utf8.ValidString(content) {
			content = content[:len(content)-1]
		}
	}
	out := secretscan.RedactKinds(content, secretscan.ConfidenceMedium).Text
	if truncated {
		out = strings.TrimRight(out, "\n") + secretScanTruncatedNote
	}
	return out
}
