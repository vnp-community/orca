package command

import (
	"context"
	"strings"
	"testing"
)

const askSolutions = `{"solutions":[{"id":"sol-1","options":{"open_questions":[
 {"id":"Q-1","text":"Does ADR-09 extend to LLM providers?","blocking":true},
 {"id":"Q-2","text":"Who grants the exception?"}]}}]}`

func TestOpenQuestionsOf_OnlyTheBlockingOnesUnlessAllIsAsked(t *testing.T) {
	doc := parseDoc(t, askSolutions)
	qs, found := openQuestionsOf(doc.Docs("solutions"), "sol-1", false)
	if !found || len(qs) != 1 || qs[0].Str("questionKey") != "open_question:Q-1" || !strings.Contains(qs[0].Str("prompt"), "ADR-09") || !qs[0].Bool("required") {
		t.Fatalf("%v %v", qs, found)
	}
	all, _ := openQuestionsOf(doc.Docs("solutions"), "sol-1", true)
	if len(all) != 2 || all[1].Str("questionKey") != "open_question:Q-2" {
		t.Fatalf("%v", all)
	}
	if _, found := openQuestionsOf(doc.Docs("solutions"), "other", false); found {
		t.Fatal("an unknown Solution must not be found")
	}
}

func TestClarificationAskCmd_PostsTheQuestionsAboutTheSolution(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{
		"GET /v1/requests/r1/solutions":       askSolutions,
		"POST /v1/requests/r1/clarifications": `{"clarification":{"id":"c1","displayId":"CLR-7.2"}}`,
	}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newClarificationAskCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"r1", "--solution", "sol-1"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 || len(g.calls) != 2 {
		t.Fatalf("err=%v exit=%d calls=%v", err, exit, g.calls)
	}
	body := g.bodies[1]
	qs, _ := body["questions"].([]any)
	if body["source"] != "solution_open_question" || body["sourceRef"] != "sol-1" || len(qs) != 1 || qs[0].(map[string]any)["questionKey"] != "open_question:Q-1" {
		t.Fatalf("%v", body)
	}
}

func TestClarificationAskCmd_NeedsASolution(t *testing.T) {
	var exit int
	cmd := newClarificationAskCmd(flowEnv{factory: nil, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"r1"})
	_ = cmd.ExecuteContext(context.Background())
	if exit == 0 {
		t.Fatal("a missing --solution is a usage error")
	}
}
