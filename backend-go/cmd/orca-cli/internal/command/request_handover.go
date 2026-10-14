package command

import (
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

// maxWritebackLines bounds how many tasks `request show` lists under the write-back summary.
const maxWritebackLines = 10

// renderWritebacks says what the agents of a Plan's tasks reported writing back to the design knowledge. It is their own
// report: nothing here was applied by Orca or by anyone. "" when the Request has no Plan or the read failed.
func renderWritebacks(doc apiclient.Document) string {
	if doc == nil {
		return ""
	}
	sum := doc.Doc("summary")
	if sum == nil || sum.Int("withWriteback") == 0 {
		return "design knowledge write-back: none reported by the agents"
	}
	var b strings.Builder
	b.WriteString("design knowledge write-back (the agents' reports; nothing was applied):\n")
	fmt.Fprintf(&b, "  %d of %d task(s) reported one: %d proposal(s) waiting for a person to apply", sum.Int("withWriteback"), sum.Int("tasks"), sum.Int("proposals"))
	if bad := sum.Int("proposalsNotOk"); bad > 0 {
		fmt.Fprintf(&b, " (%d of them marked not ok by the agent itself)", bad)
	}
	fmt.Fprintf(&b, ", %d document(s) changed, %d element(s) touched", sum.Int("documents"), sum.Int("touched"))
	for i, t := range doc.Docs("tasks") {
		if i == maxWritebackLines {
			fmt.Fprintf(&b, "\n  ... %d more task(s)", len(doc.Docs("tasks"))-maxWritebackLines)
			break
		}
		fmt.Fprintf(&b, "\n  %s  %s", t.Str("taskId"), t.Str("title"))
		if id := t.Str("proposalId"); id != "" {
			verdict := "ok"
			if !t.Bool("proposalOk") {
				verdict = "NOT ok"
			}
			fmt.Fprintf(&b, "\n      proposal %s (%s)", id, verdict)
		}
		if docs := t.Strs("documents"); len(docs) > 0 {
			fmt.Fprintf(&b, "\n      documents: %s", strings.Join(docs, ", "))
		}
	}
	return b.String()
}

// renderHandover lists what is left for a person once a Request is completed. Orca never merges code, opens a pull request,
// applies a ChangeSet proposal or regenerates the knowledge graph; this only says what is waiting.
func renderHandover(doc apiclient.Document) string {
	var b strings.Builder
	b.WriteString("handover (what is left for you; Orca merges nothing, opens no pull request and applies no proposal):")
	b.WriteString("\n  - review and merge the code changes of the plan's tasks")
	if runsUseOpenSpec(doc) {
		fmt.Fprintf(&b, "\n  - read the OpenSpec proposal on the local branch request/%d-proposal (`orca plan bundle %s`); it was never pushed", doc.Int("request", "number"), doc.Str("request", "id"))
	}
	wb := doc.Doc("writebacks")
	var proposals []string
	var documents int64
	if wb != nil {
		for _, t := range wb.Docs("tasks") {
			if id := t.Str("proposalId"); id != "" {
				proposals = append(proposals, id)
			}
		}
		documents = wb.Int("summary", "documents")
	}
	if len(proposals) > 0 {
		fmt.Fprintf(&b, "\n  - apply the %d ChangeSet proposal(s) to the knowledge graph yourself: %s", len(proposals), strings.Join(proposals, ", "))
	}
	if len(proposals) > 0 || documents > 0 {
		b.WriteString("\n  - after you accept the document changes, regenerate the knowledge graph: `tdd-agent -C <repo> generate`")
	}
	return b.String()
}

func runsUseOpenSpec(doc apiclient.Document) bool {
	for _, r := range doc.Docs("analysisRuns") {
		if r.Str("engine") == "openspec" {
			return true
		}
	}
	return false
}
