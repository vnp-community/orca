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

func TestRequestShowCmd_ReadsTheWritebacksOfThePlanWhenThereIsOne(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{
		"GET /v1/requests/r1":             `{"request":{"id":"r1","number":7,"title":"T","status":"completed","version":3,"planTaskId":"plan-1"}}`,
		"GET /v1/requests/r1/solutions":   `{"solutions":[],"runs":[]}`,
		"GET /v1/tasks/plan-1/writebacks": writebacksJSON,
	}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newRequestShowCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"r1"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 || len(g.calls) != 3 {
		t.Fatalf("err=%v exit=%d calls=%v", err, exit, g.calls)
	}
}
