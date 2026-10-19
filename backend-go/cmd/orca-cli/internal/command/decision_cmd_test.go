package command

import (
	"context"
	"strings"
	"testing"
)

func TestDecisionConfirmCmd_SendsTheTypedTitleAndNeedsOne(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{"POST /v1/decisions/d1/confirm": `{"decision":{"id":"d1","status":"effective"}}`}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newDecisionConfirmCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"d1", "--text", "Plan A"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 || len(g.calls) != 1 || g.bodies[0]["confirmationText"] != "Plan A" {
		t.Fatalf("err=%v exit=%d calls=%v body=%v", err, exit, g.calls, g.bodies)
	}
	var exit2 int
	bare := newDecisionConfirmCmd(flowEnv{factory: nil, json: new(bool), exit: &exit2})
	bare.SetArgs([]string{"d1"})
	_ = bare.ExecuteContext(context.Background())
	if exit2 == 0 {
		t.Fatal("confirming without the title is a usage error")
	}
}

func TestRenderDecisions_ShowsTheChoiceItsRiskAndWhoConfirmed(t *testing.T) {
	got := renderDecisions(parseDoc(t, `{"decisions":[{"id":"d1","subjectKind":"solution","status":"chosen","riskLevel":"high","chosenOptionId":"opt-1","chosenOptionTitle":"Plan A"},{"id":"d2","subjectKind":"solution","status":"effective","confirmedBy":"u1","chosenOptionId":"opt-2"}]}`))
	for _, want := range []string{"d1  solution  chosen  risk high  chose opt-1: Plan A", "confirmed by u1"} {
		if !contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	if renderDecisions(parseDoc(t, `{"decisions":[]}`)) != "no decisions" {
		t.Fatal("an empty log says so")
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
