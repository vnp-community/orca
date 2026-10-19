package command

import (
	"context"
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

// handoverItem is one thing left for a person once a Request is completed. Command is a suggestion shown, never run.
type handoverItem struct {
	Kind    string `json:"kind"`
	Text    string `json:"text"`
	Command string `json:"command,omitempty"`
}

// maxHandoverNames bounds how many ids or names one handover line spells out.
const maxHandoverNames = 8

// handoverItems works out what is left for a person from what `request show` already read: the Plan's task worktrees, the
// proposal branch and the state of its archive, the write-backs the agents reported, and the conflicts the Solution found.
// Orca merges no code, opens no pull request, applies no proposal and regenerates nothing; every item is a suggestion.
func handoverItems(doc apiclient.Document) []handoverItem {
	var items []handoverItem
	switch worktrees, known := taskWorktrees(doc); {
	case !known:
		items = append(items, handoverItem{Kind: "code", Text: "review and merge the code changes of the plan's tasks"})
	case len(worktrees) > 0:
		items = append(items, handoverItem{Kind: "code", Text: fmt.Sprintf("review and merge the code of the plan's task worktree(s), checking which are already merged: %s", namesOf(worktrees))})
	}
	if runsUseOpenSpec(doc) {
		items = append(items, proposalItems(doc)...)
	}
	items = append(items, writebackItems(doc)...)
	if conflicts := solutionConflicts(doc); len(conflicts) > 0 {
		items = append(items, handoverItem{Kind: "conflicts", Text: fmt.Sprintf("decide the %d Decision/Constraint conflict(s) the Solution reported: %s", len(conflicts), namesOf(conflicts))})
	}
	return items
}

// taskWorktrees lists the distinct worktree ids the Plan's tasks ran in. known is false when the Plan's tasks could not be read.
func taskWorktrees(doc apiclient.Document) (ids []string, known bool) {
	tree := doc.Doc("planTasks")
	if tree == nil {
		return nil, false
	}
	seen := map[string]bool{}
	for _, t := range tree.Docs("tasks") {
		if id := t.Str("worktreeId"); id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, true
}

func proposalItems(doc apiclient.Document) []handoverItem {
	branch := fmt.Sprintf("request/%d-proposal", doc.Int("request", "number"))
	bundle := doc.Doc("planBundle")
	read := handoverItem{Kind: "proposal", Text: fmt.Sprintf("read the OpenSpec proposal on the local branch %s (`orca plan bundle %s`); it was never pushed", branch, doc.Str("request", "id"))}
	if bundle == nil {
		return []handoverItem{read}
	}
	if b := bundle.Str("branch"); b != "" {
		branch = b
	}
	merge := fmt.Sprintf("git merge %s", branch)
	switch bundle.Str("status") {
	case "archived":
		return []handoverItem{{Kind: "proposal", Text: fmt.Sprintf("the OpenSpec change was archived on the local branch %s (commit %s, never pushed); merge it when you want its specs in your main branch", branch, shortSHA(bundle.Str("commit"))), Command: merge}}
	case "ready":
		if bundle.Str("syncState") == "failed" {
			return []handoverItem{read, {Kind: "archive", Text: fmt.Sprintf("WARNING the archive step gave up after %d attempt(s): %s. Archive by hand on that branch; the Request stays completed", bundle.Int("syncAttempts"), firstOr(bundle.Str("syncError"), "no reason recorded")),
				Command: fmt.Sprintf("openspec archive %s --yes", bundle.Str("changeId"))}}
		}
		return []handoverItem{read, {Kind: "archive", Text: "the archive of the OpenSpec change is still pending; Orca retries it by itself, run this command again later"}}
	}
	return []handoverItem{read}
}

func writebackItems(doc apiclient.Document) []handoverItem {
	wb := doc.Doc("writebacks")
	if wb == nil {
		return nil
	}
	var proposals []string
	for _, t := range wb.Docs("tasks") {
		if id := t.Str("proposalId"); id != "" {
			proposals = append(proposals, id)
		}
	}
	var items []handoverItem
	if len(proposals) > 0 {
		items = append(items, handoverItem{Kind: "writeback", Text: fmt.Sprintf("apply the %d ChangeSet proposal(s) to the knowledge graph yourself: %s", len(proposals), namesOf(proposals))})
	}
	if len(proposals) > 0 || wb.Int("summary", "documents") > 0 {
		items = append(items, handoverItem{Kind: "regenerate", Text: "after you accept the document changes, regenerate the knowledge graph", Command: "tdd-agent -C <repo> generate"})
	}
	return items
}

// solutionConflicts are the Decision/Constraint conflicts the approved Solution's agent said it found in the knowledge graph.
func solutionConflicts(doc apiclient.Document) []string {
	var out []string
	for _, s := range doc.Docs("solutions") {
		if s.Str("status") != "approved" {
			continue
		}
		out = append(out, s.Strs("options", "design_references", "conflicts")...)
	}
	return out
}

func namesOf(in []string) string {
	if len(in) <= maxHandoverNames {
		return strings.Join(in, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(in[:maxHandoverNames], ", "), len(in)-maxHandoverNames)
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return firstOr(sha, "unknown")
}

// attachHandover puts the items into the document so `--json` carries them too.
func attachHandover(doc apiclient.Document) {
	items := handoverItems(doc)
	out := make([]any, 0, len(items))
	for _, it := range items {
		m := map[string]any{"kind": it.Kind, "text": it.Text}
		if it.Command != "" {
			m["command"] = it.Command
		}
		out = append(out, m)
	}
	doc["handover"] = map[string]any{"items": out, "nothingLeft": len(out) == 0}
}

// renderHandover lists what is left for a person once a Request is completed, or says that nothing is.
func renderHandover(doc apiclient.Document) string {
	items := handoverItems(doc)
	if len(items) == 0 {
		return "handover: nothing is left for you (no task worktrees, no proposal branch, no write-back, no open conflict)"
	}
	var b strings.Builder
	b.WriteString("handover (what is left for you; Orca merges nothing, opens no pull request and applies no proposal):")
	for _, it := range items {
		fmt.Fprintf(&b, "\n  - %s", it.Text)
		if it.Command != "" {
			fmt.Fprintf(&b, "\n      suggested, not run: %s", it.Command)
		}
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

// readHandoverFacts reads what the handover needs beyond the Request itself: the Plan's tasks with their worktrees and, when
// an OpenSpec change was authored, its state. A failed read leaves that part out; it never hides the Request.
func readHandoverFacts(ctx context.Context, cli *apiclient.Client, doc apiclient.Document, requestID, planTaskID string) {
	if tree, err := cli.GetTaskSubtree(ctx, planTaskID); err == nil {
		doc["planTasks"] = tree
	}
	if runsUseOpenSpec(doc) {
		if plan, err := cli.GetPlanProposal(ctx, requestID, ""); err == nil {
			doc["planBundle"] = plan["bundle"]
		}
	}
}
