package command

import (
	"context"
	"strings"
	"testing"
)

const writebacksJSON = `{"tasks":[
 {"taskId":"t1","title":"Add the cache","proposalId":"cs-1","proposalOk":true,"documents":["docs/tdd/a.md"],"touched":["FR-1"]},
 {"taskId":"t2","title":"Wire the flag","proposalId":"cs-2","proposalOk":false,"documents":[],"touched":["BRL-3"]}],
 "summary":{"tasks":5,"withWriteback":2,"proposals":2,"proposalsNotOk":1,"documents":1,"touched":2}}`

func TestRenderWritebacks_SaysWhatWasReportedAndThatNothingWasApplied(t *testing.T) {
	got := renderWritebacks(parseDoc(t, writebacksJSON))
	for _, want := range []string{"nothing was applied", "2 of 5 task(s) reported one", "2 proposal(s) waiting for a person to apply", "(1 of them marked not ok by the agent itself)",
		"1 document(s) changed, 2 element(s) touched", "proposal cs-1 (ok)", "proposal cs-2 (NOT ok)", "documents: docs/tdd/a.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderWritebacks_ARequestWithNoneSaysSoAndOneWithoutAPlanSaysNothing(t *testing.T) {
	none := renderWritebacks(parseDoc(t, `{"tasks":[],"summary":{"tasks":3,"withWriteback":0}}`))
	if !strings.Contains(none, "none reported by the agents") {
		t.Fatal(none)
	}
	if renderWritebacks(nil) != "" {
		t.Fatal("a Request without a plan has no write-back line")
	}
}

func TestRenderWritebacks_LongListsAreCut(t *testing.T) {
	var tasks []string
	for i := 0; i < 14; i++ {
		tasks = append(tasks, `{"taskId":"t`+string(rune('a'+i))+`","title":"x","documents":["d"]}`)
	}
	got := renderWritebacks(parseDoc(t, `{"tasks":[`+strings.Join(tasks, ",")+`],"summary":{"tasks":14,"withWriteback":14,"documents":14}}`))
	if !strings.Contains(got, "... 4 more task(s)") {
		t.Fatal(got)
	}
}

func TestRenderRequestShow_AHandoverAppearsOnlyOnceTheRequestIsCompleted(t *testing.T) {
	base := `{"request":{"id":"r1","number":7,"title":"T","status":"%s","version":3},"analysisRuns":[{"id":"run1","kind":"plan","status":"succeeded","engine":"openspec"}],"writebacks":` + writebacksJSON + `}`
	running := renderRequestShow(parseDoc(t, strings.Replace(base, "%s", "executing", 1)))
	if strings.Contains(running, "handover") || !strings.Contains(running, "design knowledge write-back") {
		t.Fatalf("executing:\n%s", running)
	}
	done := renderRequestShow(parseDoc(t, strings.Replace(base, "%s", "completed", 1)))
	for _, want := range []string{"handover (what is left for you", "review and merge the code changes", "request/7-proposal", "`orca plan bundle r1`",
		"apply the 2 ChangeSet proposal(s) to the knowledge graph yourself: cs-1, cs-2", "tdd-agent -C <repo> generate"} {
		if !strings.Contains(done, want) {
			t.Errorf("missing %q in:\n%s", want, done)
		}
	}
}

func TestRenderHandover_ANativeRequestWithNoWritebackOnlyAsksForTheCode(t *testing.T) {
	got := renderHandover(parseDoc(t, `{"request":{"id":"r1","number":7,"status":"completed"},"analysisRuns":[{"id":"run1","kind":"solution","status":"succeeded","engine":"native"}]}`))
	if strings.Count(got, "\n  - ") != 1 || strings.Contains(got, "ChangeSet") || strings.Contains(got, "OpenSpec") || strings.Contains(got, "generate") {
		t.Fatalf("%s", got)
	}
}

func TestRequestShowCmd_ReadsWhatTheHandoverNeedsOnlyOnceTheRequestIsCompleted(t *testing.T) {
	reply := func(status string) map[string]string {
		return map[string]string{
			"GET /v1/requests/r1":             `{"request":{"id":"r1","number":7,"title":"T","status":"` + status + `","version":3,"planTaskId":"plan-1"}}`,
			"GET /v1/requests/r1/solutions":   `{"solutions":[],"runs":[{"id":"run1","kind":"plan","status":"succeeded","engine":"openspec"}]}`,
			"GET /v1/tasks/plan-1/writebacks": writebacksJSON,
			"GET /v1/tasks/plan-1/subtree":    `{"tasks":[{"id":"t1","worktreeId":"wt-a"},{"id":"t2","worktreeId":"wt-a"},{"id":"t3"}]}`,
			"GET /v1/requests/r1/plan":        `{"bundle":{"changeId":"req-7-x","branch":"request/7-proposal","status":"archived","commit":"0123456789abcdef"}}`,
		}
	}
	for status, wantCalls := range map[string]int{"executing": 3, "completed": 5} {
		g := &gateway{t: t, reply: reply(status)}
		srv, factory := g.server()
		var exit int
		cmd := newRequestShowCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
		cmd.SetArgs([]string{"r1"})
		err := cmd.ExecuteContext(context.Background())
		srv.Close()
		if err != nil || exit != 0 || len(g.calls) != wantCalls {
			t.Fatalf("%s: err=%v exit=%d calls=%v", status, err, exit, g.calls)
		}
	}
}

func TestAttachHandover_PutsTheItemsInTheDocumentForScripts(t *testing.T) {
	doc := parseDoc(t, `{"request":{"id":"r1","number":7,"status":"completed"},"analysisRuns":[],"planTasks":{"tasks":[{"worktreeId":"wt-a"}]}}`)
	attachHandover(doc)
	h := doc.Doc("handover")
	items := h.Docs("items")
	if h == nil || h.Bool("nothingLeft") || len(items) != 1 || items[0].Str("kind") != "code" || !strings.Contains(items[0].Str("text"), "wt-a") {
		t.Fatalf("%v", doc["handover"])
	}
	empty := parseDoc(t, `{"request":{"id":"r1","number":7,"status":"completed"},"analysisRuns":[],"planTasks":{"tasks":[{"id":"t"}]}}`)
	attachHandover(empty)
	if !empty.Doc("handover").Bool("nothingLeft") || len(empty.Doc("handover").Docs("items")) != 0 {
		t.Fatalf("%v", empty["handover"])
	}
}

func TestRenderHandover_NothingLeftSaysSo(t *testing.T) {
	got := renderHandover(parseDoc(t, `{"request":{"id":"r1","number":7,"status":"completed"},"analysisRuns":[{"engine":"native"}],"planTasks":{"tasks":[{"id":"t1"}]},"writebacks":{"tasks":[],"summary":{"tasks":1,"withWriteback":0}}}`))
	if !strings.Contains(got, "nothing is left for you") || strings.Contains(got, "\n  - ") {
		t.Fatal(got)
	}
}

func TestRenderHandover_ListsTheTaskWorktreesOnceEachAndCutsALongList(t *testing.T) {
	var tasks []string
	for i := 0; i < 12; i++ {
		tasks = append(tasks, `{"worktreeId":"wt-`+string(rune('a'+i))+`"}`, `{"worktreeId":"wt-`+string(rune('a'+i))+`"}`)
	}
	got := renderHandover(parseDoc(t, `{"request":{"id":"r1","number":7,"status":"completed"},"planTasks":{"tasks":[`+strings.Join(tasks, ",")+`]}}`))
	if !strings.Contains(got, "wt-a, wt-b") || strings.Count(got, "wt-a") != 1 || !strings.Contains(got, "and 4 more") {
		t.Fatal(got)
	}
}

func TestRenderHandover_TheArchiveStateOfTheProposalIsSaid(t *testing.T) {
	base := `{"request":{"id":"r1","number":7,"status":"completed"},"analysisRuns":[{"engine":"openspec"}],"planTasks":{"tasks":[]},"planBundle":%s}`
	cases := []struct {
		name, bundle string
		want         []string
		notWant      []string
	}{
		{"archived", `{"changeId":"req-7-x","branch":"request/7-proposal","status":"archived","commit":"0123456789abcdef"}`,
			[]string{"archived on the local branch request/7-proposal (commit 01234567, never pushed)", "suggested, not run: git merge request/7-proposal"}, []string{"WARNING"}},
		{"failed", `{"changeId":"req-7-x","branch":"request/7-proposal","status":"ready","syncState":"failed","syncAttempts":5,"syncError":"spec delta does not apply"}`,
			[]string{"WARNING the archive step gave up after 5 attempt(s): spec delta does not apply", "the Request stays completed", "suggested, not run: openspec archive req-7-x --yes"}, nil},
		{"pending", `{"changeId":"req-7-x","branch":"request/7-proposal","status":"ready","syncState":"pending"}`,
			[]string{"still pending", "read the OpenSpec proposal on the local branch request/7-proposal"}, []string{"WARNING"}},
	}
	for _, c := range cases {
		got := renderHandover(parseDoc(t, strings.Replace(base, "%s", c.bundle, 1)))
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: missing %q in:\n%s", c.name, w, got)
			}
		}
		for _, w := range c.notWant {
			if strings.Contains(got, w) {
				t.Errorf("%s: unexpected %q in:\n%s", c.name, w, got)
			}
		}
	}
}

func TestRenderHandover_ConflictsOfTheApprovedSolutionAreListedAndOthersAreNot(t *testing.T) {
	got := renderHandover(parseDoc(t, `{"request":{"id":"r1","number":7,"status":"completed"},"planTasks":{"tasks":[]},"solutions":[
 {"status":"approved","options":{"design_references":{"available":true,"conflicts":["ADR-12 vs AC-5","BR-3 vs option 2"]}}},
 {"status":"rejected","options":{"design_references":{"available":true,"conflicts":["OLD-1"]}}}]}`))
	if !strings.Contains(got, "decide the 2 Decision/Constraint conflict(s) the Solution reported: ADR-12 vs AC-5, BR-3 vs option 2") || strings.Contains(got, "OLD-1") {
		t.Fatal(got)
	}
}
