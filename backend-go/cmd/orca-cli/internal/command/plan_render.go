package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

// renderPlanTree prints the plan, its Phases and their tasks indented by parent, with ids (a Phase id is what
// `orca plan start-phase` takes) and the phases each Phase waits for.
func renderPlanTree(doc apiclient.Document) string {
	tasks := doc.Docs("tasks")
	children := map[string][]apiclient.Document{}
	byID := map[string]apiclient.Document{}
	for _, t := range tasks {
		byID[t.Str("id")] = t
		children[t.Str("parent_id")] = append(children[t.Str("parent_id")], t)
	}
	waits := map[string][]string{}
	for _, e := range doc.Docs("depends_on_edges") {
		from, to := e.Str("from_task_id"), e.Str("to_task_id")
		if byID[from].Str("task_type") == "phase" && byID[to].Str("task_type") == "phase" {
			waits[from] = append(waits[from], to)
		}
	}
	var b strings.Builder
	var walk func(parent string, depth int)
	walk = func(parent string, depth int) {
		kids := children[parent]
		// The service gives no position, and its order changes between calls: title keeps the output stable.
		sort.SliceStable(kids, func(i, j int) bool { return kids[i].Str("title") < kids[j].Str("title") })
		for _, t := range kids {
			fmt.Fprintf(&b, "%s%-7s %-10s %s  %s", strings.Repeat("  ", depth), t.Str("task_type"), t.Str("status"), t.Str("id"), t.Str("title"))
			if after := waits[t.Str("id")]; len(after) > 0 {
				fmt.Fprintf(&b, "  (after %s)", strings.Join(after, ", "))
			}
			b.WriteString("\n")
			walk(t.Str("id"), depth+1)
		}
	}
	// The root is the task whose parent is not in the subtree.
	for _, t := range tasks {
		if _, ok := byID[t.Str("parent_id")]; !ok {
			fmt.Fprintf(&b, "%-7s %-10s %s  %s\n", t.Str("task_type"), t.Str("status"), t.Str("id"), t.Str("title"))
			walk(t.Str("id"), 1)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
