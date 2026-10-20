package command

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

// planAskWidth is how much of the request for changes a row shows; the full text stays in the UI.
const planAskWidth = 80

// renderPlanRevisions prints one row per version of the plan, newest first. The files column counts the files added, changed
// or removed since the version before; a version whose documents were never kept shows "-" there.
func renderPlanRevisions(doc apiclient.Document) string {
	rows := doc.Docs("revisions")
	if len(rows) == 0 {
		return "this Request has no plan version yet"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-8s %-11s %-17s %-6s %s\n", "VERSION", "STATUS", "GENERATED", "FILES", "ASKED")
	for _, r := range rows {
		files := "-"
		if r.Str("engine") == "openspec" && r.Str("textKept") != "false" {
			files = fmt.Sprint(r.Int("filesChanged"))
		}
		fmt.Fprintf(&b, "%-8d %-11s %-17s %-6s %s\n", r.Int("revision"), r.Str("status"), shortTime(r.Str("generatedAt")), files, askExcerpt(r.Str("revisionFeedback")))
	}
	if next := doc.Str("nextPageToken"); next != "" {
		fmt.Fprintf(&b, "more: --page-token %s\n", next)
	}
	return strings.TrimRight(b.String(), "\n")
}

func shortTime(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return "-"
	}
	return t.UTC().Format("2006-01-02 15:04")
}

// askExcerpt is the first words of a request for changes on one line, cut on a character boundary.
func askExcerpt(ask string) string {
	ask = strings.Join(strings.Fields(ask), " ")
	if ask == "" {
		return "-"
	}
	if utf8.RuneCountInString(ask) <= planAskWidth {
		return ask
	}
	return string([]rune(ask)[:planAskWidth]) + "…"
}
