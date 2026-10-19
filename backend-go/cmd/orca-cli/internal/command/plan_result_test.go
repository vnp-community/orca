package command

import (
	"context"
	"strings"
	"testing"
)

func TestRenderTaskResult_ShowsChecksAndTheAgentsOwnKnowledgeReport(t *testing.T) {
	out := renderTaskResult(parseDoc(t, `{"taskId":"t1","attempt":2,"parseStatus":"ok","status":"done","summary":"added the cache",
		"filesChanged":["a.go","a_test.go"],"checksRun":[{"id":"go test ./...","exit":0},{"id":"go vet","exit":2}],
		"knowledge":{"available":true,"snapshot":"snap-1","read":["FR-1","ADR-12"],"gaps":["no TTL rule"],"conflicts":["ADR-12 forbids a second cache"]},
		"writeback":{"documents":["docs/tdd/a.md"],"proposalId":"cs-1","proposalOk":true,"touched":[]},"source":"execution_record"}`))
	for _, want := range []string{"attempt 2", "status: done", "check: go test ./...: passed", "check: go vet: failed (exit 2)", "files changed: a.go, a_test.go",
		"read 2 element(s) from snapshot snap-1", "conflict: ADR-12 forbids a second cache", "gap: no TTL rule", "1 document(s) touched, proposal cs-1 (ok)", "agent's report"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderTaskResult_AFailedRunAndARunWithoutABlockSaySoPlainly(t *testing.T) {
	failed := renderTaskResult(parseDoc(t, `{"taskId":"t1","parseStatus":"ok","status":"done","failureClass":"agent_defect","source":"execution_record"}`))
	if !strings.Contains(failed, "FAILED (agent_defect)") {
		t.Fatal(failed)
	}
	bare := renderTaskResult(parseDoc(t, `{"taskId":"t1","parseStatus":"missing","source":"last_execution_output","stdoutTail":"x"}`))
	if !strings.Contains(bare, "result block: missing") || !strings.Contains(bare, "no result block") || strings.Contains(bare, "knowledge") {
		t.Fatal(bare)
	}
}

func TestPlanResultCmd_ReadsTheTaskExecutionRoute(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{"GET /v1/tasks/t1/execution": `{"taskId":"t1","parseStatus":"ok","status":"done","source":"execution_record"}`}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newPlanResultCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"t1"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 || len(g.calls) != 1 {
		t.Fatalf("err=%v exit=%d calls=%v", err, exit, g.calls)
	}
}
