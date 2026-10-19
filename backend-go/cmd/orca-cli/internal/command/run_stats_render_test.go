package command

import (
	"context"
	"strings"
	"testing"
)

const statsJSON = `{"toolCalls":{"mcp__tdd-agent__kg_search":5,"mcp__tdd-agent__kg_get_element":9,"Read":12,"Bash":3},"knowledgeToolCalls":14,"turns":38,"durationMs":422000,"costUsd":0.81}`

func TestRenderRunStats_SeparatesTheGraphsToolsFromTheRestAndNamesNoContent(t *testing.T) {
	got := renderRunStats(parseDoc(t, statsJSON), 0)
	for _, want := range []string{"14 knowledge tool call(s) (kg_get_element x9, kg_search x5)", "15 other tool call(s)", "38 turn(s)", "7m2s", "$0.81"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestRenderRunStats_ACostThatWasNotReportedIsSaidSoAndNeverShownAsZero(t *testing.T) {
	got := renderRunStats(parseDoc(t, `{"toolCalls":{"Read":1},"knowledgeToolCalls":0,"turns":2}`), 90000)
	if !strings.Contains(got, "cost not reported") || strings.Contains(got, "$0") || !strings.Contains(got, "1m30s") {
		t.Fatalf("%q", got)
	}
	if paid := renderRunStats(parseDoc(t, `{"toolCalls":{},"knowledgeToolCalls":0,"costUsd":0}`), 0); !strings.Contains(paid, "$0.00") {
		t.Fatalf("a reported zero is a real zero: %q", paid)
	}
}

func TestRenderRunStats_ARunWithNothingToReportPrintsNothing(t *testing.T) {
	if got := renderRunStats(nil, 0); got != "" {
		t.Fatalf("%q", got)
	}
	if got := renderRunStats(nil, 5000); got != "5s" {
		t.Fatalf("a native plan still shows how long it took: %q", got)
	}
}

func TestRenderPlanProposalAndSolutions_ShowTheAgentLine(t *testing.T) {
	plan := renderPlanProposal(parseDoc(t, `{"runId":"r","status":"succeeded","engine":"openspec","elapsedMs":83000,"stats":`+statsJSON+`}`))
	if !strings.Contains(plan, "agent: 14 knowledge tool call(s)") || !strings.Contains(plan, "$0.81") {
		t.Fatalf("plan proposal:\n%s", plan)
	}
	sols := renderSolutions(parseDoc(t, `{"solutions":[],"runs":[{"id":"run1","kind":"solution","status":"succeeded","stats":`+statsJSON+`},{"id":"run2","kind":"solution","status":"succeeded"}]}`))
	if strings.Count(sols, "agent:") != 1 || !strings.Contains(sols, "kg_search x5") {
		t.Fatalf("solutions:\n%s", sols)
	}
}

func TestRequestShowCmd_AddsTheRunsWithTheirAgentFiguresAndSurvivesARunsReadThatFails(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{
		"GET /v1/requests/r1":           `{"request":{"id":"r1","number":7,"title":"T","status":"planning","version":3}}`,
		"GET /v1/requests/r1/solutions": `{"solutions":[],"runs":[{"id":"run9","kind":"plan","status":"succeeded","engine":"openspec","stats":` + statsJSON + `}]}`,
	}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newRequestShowCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"r1"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 || len(g.calls) != 2 {
		t.Fatalf("err=%v exit=%d calls=%v", err, exit, g.calls)
	}
	out := renderRequestShow(parseDoc(t, `{"request":{"id":"r1","number":7,"title":"T","status":"planning","version":3},"analysisRuns":[{"id":"run9","kind":"plan","status":"succeeded","engine":"openspec","stats":`+statsJSON+`}]}`))
	for _, want := range []string{"#7", "runs:", "run9  plan  succeeded  engine openspec", "agent: 14 knowledge tool call(s)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if bare := renderRequestShow(parseDoc(t, `{"request":{"id":"r1","number":7,"title":"T","status":"planning","version":3}}`)); strings.Contains(bare, "runs:") {
		t.Fatalf("a Request with no run list prints none:\n%s", bare)
	}
	g2 := &gateway{t: t, reply: map[string]string{
		"GET /v1/requests/r1":           `{"request":{"id":"r1","number":7,"title":"T","status":"planning","version":3}}`,
		"GET /v1/requests/r1/solutions": "!400 REQUEST_SOLUTION_KIND_NOT_ALLOWED no solutions for this type",
	}}
	srv2, factory2 := g2.server()
	defer srv2.Close()
	var exit2 int
	cmd2 := newRequestShowCmd(flowEnv{factory: factory2, json: new(bool), exit: &exit2})
	cmd2.SetArgs([]string{"r1"})
	if err := cmd2.ExecuteContext(context.Background()); err != nil || exit2 != 0 {
		t.Fatalf("a failed runs read must not fail the show: err=%v exit=%d", err, exit2)
	}
}
