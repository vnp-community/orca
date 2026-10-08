package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type promptCompleter struct {
	replies []string
	prompts []string
	err     error
}

func (c *promptCompleter) Complete(_ context.Context, prompt string) (string, error) {
	c.prompts = append(c.prompts, prompt)
	if c.err != nil {
		return "", c.err
	}
	return c.replies[min(len(c.prompts)-1, len(c.replies)-1)], nil
}

func TestNativeEngine_GenerateAnalysisIsReal(t *testing.T) {
	c := &promptCompleter{replies: []string{oneOptionReply(), fenced(validSolutionReply(t))}}
	e := NewNativeEngine(c, nil)
	out, err := e.GenerateAnalysis(context.Background(), ProjectRef{}, AnalysisInput{
		Request: sampleRequest("body"), Feedback: "thử cách khác",
		PriorArtifacts: []PriorArtifact{{Kind: "solution", Status: "rejected", Content: `{"old":1}`}},
	})
	if err != nil || len(out.OptionsJSON) == 0 || out.Raw == "" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if _, err := domain.ParseSolutionOptions(out.OptionsJSON); err != nil {
		t.Fatal(err)
	}
	if len(c.prompts) != 2 {
		t.Fatalf("prompts = %d", len(c.prompts))
	}
	for _, want := range []string{"thử cách khác", `{"old":1}`} {
		if !containsStr(c.prompts[0], want) {
			t.Errorf("first prompt lacks %q", want)
		}
	}
	if !containsStr(c.prompts[1], "number of options") {
		t.Error("retry prompt lacks the validation error")
	}
}

func TestNativeEngine_GenerateAnalysisFailures(t *testing.T) {
	bad := &promptCompleter{replies: []string{"nope"}}
	_, err := NewNativeEngine(bad, nil).GenerateAnalysis(context.Background(), ProjectRef{}, AnalysisInput{Request: sampleRequest("b")})
	if !errors.Is(err, domain.ErrSolutionOptionsInvalid) || len(bad.prompts) != 2 {
		t.Fatalf("err=%v calls=%d", err, len(bad.prompts))
	}
	boom := errors.New("down")
	_, err = NewNativeEngine(&promptCompleter{err: boom}, nil).GenerateAnalysis(context.Background(), ProjectRef{}, AnalysisInput{Request: sampleRequest("b")})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, err := NewNativeEngine(nil, nil).GenerateAnalysis(context.Background(), ProjectRef{}, AnalysisInput{}); err == nil {
		t.Fatal("missing completer must error")
	}
}

func containsStr(s, sub string) bool { return strings.Contains(s, sub) }
