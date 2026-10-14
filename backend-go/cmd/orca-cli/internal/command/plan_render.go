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

// renderPlanProposal prints the run's state; for an openspec plan it adds where the change lives, the findings an approver
// should see and what the agent says it read. A native plan prints no bundle section, so nothing reads as "found nothing".
func renderPlanProposal(doc apiclient.Document) string {
	var b strings.Builder
	fmt.Fprintf(&b, "run %s  %s", doc.Str("runId"), doc.Str("status"))
	if e := doc.Str("engine"); e != "" {
		fmt.Fprintf(&b, "  engine %s", e)
	}
	b.WriteString("\n")
	if code := doc.Str("errorCode"); code != "" {
		fmt.Fprintf(&b, "error %s: %s\n", code, doc.Str("errorMessage"))
	}
	if p := doc.Doc("proposal"); p != nil {
		fmt.Fprintf(&b, "plan  %s\n", p.Str("title"))
		phases, tasks := doc.Docs("proposal", "phases"), 0
		for _, ph := range phases {
			tasks += len(ph.Docs("tasks"))
		}
		tasks += len(doc.Docs("proposal", "tasks"))
		fmt.Fprintf(&b, "      %d phases, %d tasks\n", len(phases), tasks)
	}
	renderBundleSection(&b, doc)
	return strings.TrimRight(b.String(), "\n")
}

func renderPlanBundle(doc apiclient.Document) string {
	if doc.Doc("bundle") == nil {
		return "no authored OpenSpec change behind this plan (engine " + firstOr(doc.Str("engine"), "native") + ")"
	}
	var b strings.Builder
	renderBundleSection(&b, doc)
	return strings.TrimRight(b.String(), "\n")
}

func renderBundleSection(b *strings.Builder, doc apiclient.Document) {
	bundle := doc.Doc("bundle")
	if bundle == nil {
		return
	}
	fmt.Fprintf(b, "change  %s\nbranch  %s  commit %s\n", bundle.Str("changeId"), bundle.Str("branch"), bundle.Str("commit"))
	for _, f := range bundle.Docs("files") {
		note := ""
		if f.Str("omitted") == "true" {
			note = "  (too large to keep; read it from the branch)"
		}
		fmt.Fprintf(b, "  %-40s %6d bytes%s\n", f.Str("path"), f.Int("size"), note)
	}
	for _, c := range doc.Docs("checks") {
		where := ""
		if w := c.Str("where"); w != "" {
			where = " (" + w + ")"
		}
		fmt.Fprintf(b, "%-8s %s%s: %s\n", c.Str("severity"), c.Str("code"), where, c.Str("message"))
	}
	if k := doc.Doc("knowledge"); k != nil {
		fmt.Fprintf(b, "knowledge  available=%s snapshot=%s read=%d gaps=%d conflicts=%d\n", k.Str("available"), firstOr(k.Str("snapshot"), "-"), len(list(k, "read")), len(list(k, "gaps")), len(list(k, "conflicts")))
		for _, g := range list(k, "conflicts") {
			fmt.Fprintf(b, "  conflict: %s\n", g)
		}
	}
}

func renderBundleFile(doc apiclient.Document) string {
	head := "# " + doc.Str("path")
	if doc.Str("omitted") == "true" {
		head += "  (not stored: too large)"
	}
	return head + "\n" + strings.TrimRight(doc.Str("content"), "\n")
}

func list(d apiclient.Document, key string) []string {
	raw, _ := d[key].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func firstOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
