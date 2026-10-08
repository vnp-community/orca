package stubs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const classifierPrompt = "You classify a software work request. title: %s"

func classify(t *testing.T, s *AgentScript, title string) domain.ClassificationProposal {
	t.Helper()
	out, err := s.Answer("dev-1", "ai.complete", mustJSON(map[string]any{"prompt": strings.Replace(classifierPrompt, "%s", title, 1)}))
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	p, err := domain.ParseClassificationProposal([]byte(res.Content))
	if err != nil {
		t.Fatalf("the stub's answer must satisfy the real proposal contract: %v (%s)", err, res.Content)
	}
	return p
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestAgentScript_ClassificationFollowsTheMarkerAndPassesTheRealParser(t *testing.T) {
	s := NewAgentScript()
	p := classify(t, s, "Crash on save "+MarkerFor("bug", "S"))
	if p.Type != domain.RequestTypeBug || p.Size != domain.RequestSizeS || p.Urgency != domain.UrgencyNormal {
		t.Fatalf("%+v", p)
	}
	p = classify(t, s, "Prod down [e2e:type=hotfix size=M urgency=urgent confidence=0.95]")
	if p.Type != domain.RequestTypeHotfix || p.Urgency != domain.UrgencyUrgent || p.Confidence != 0.95 {
		t.Fatalf("%+v", p)
	}
	p = classify(t, s, "No marker at all")
	if p.Type != domain.RequestTypeChangeRequest || p.Size != domain.RequestSizeM {
		t.Fatalf("default: %+v", p)
	}
}

func TestAgentScript_FailClassificationReturnsUnparseableText(t *testing.T) {
	s := NewAgentScript()
	s.FailClassification = true
	out, err := s.Answer("", "ai.complete", mustJSON(map[string]any{"prompt": "You classify a software work request."}))
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Content string `json:"content"`
	}
	_ = json.Unmarshal([]byte(out), &res)
	if _, perr := domain.ParseClassificationProposal([]byte(res.Content)); perr == nil {
		t.Fatal("FailClassification must produce text the parser rejects")
	}
}

func TestAgentScript_UnwrittenAnswersAreErrorsAndCallsAreRecorded(t *testing.T) {
	s := NewAgentScript()
	if _, err := s.Answer("dev-1", "agent.execPrompt", `{"prompt":"x"}`); err == nil {
		t.Fatal("a method with no answer must fail instead of inventing one")
	}
	s.AddHandler(func(method string, _ map[string]any) (any, bool) {
		if method == "agent.execPrompt" {
			return map[string]any{"output": "done"}, true
		}
		return nil, false
	})
	out, err := s.Answer("dev-1", "agent.execPrompt", `{"prompt":"x"}`)
	if err != nil || !strings.Contains(out, "done") {
		t.Fatalf("handler answer: %q %v", out, err)
	}
	if calls := s.Calls(); len(calls) != 2 || calls[1].DevServerID != "dev-1" || calls[1].Method != "agent.execPrompt" {
		t.Fatalf("calls %+v", calls)
	}
}
