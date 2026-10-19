package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestOpenSpecEngine_AsksTheWorkspaceForTheRequestsBaseRef(t *testing.T) {
	for name, ref := range map[string]string{"chosen": "release/1.4", "default": ""} {
		t.Run(name, func(t *testing.T) {
			k := newEngineKit(authoringFeatures)
			k.agentWritesTasks(goodTasksMD)
			k.in.Request.BaseRef = ref
			if _, err := k.e.GenerateProposal(context.Background(), k.in); err != nil {
				t.Fatal(err)
			}
			if got := k.ws.lastEnsure; got.BaseRef != ref || got.RequestID != "req-uuid" || got.RequestNumber != 42 || got.ChangeID != "req-42-add-ai" {
				t.Fatalf("Ensure input = %+v", got)
			}
		})
	}
}

// A ref that vanished after it was chosen is the person's to fix, so the run says which ref and what exists instead; any
// other workspace failure stays the generic one.
func TestOpenSpecEngine_BaseRefGone_RunFailsWithItsOwnCodeAndNamesTheAlternatives(t *testing.T) {
	k := newEngineKit(authoringFeatures)
	k.in.Request.BaseRef = "release/gone"
	k.ws.ensureErr = domain.ErrRequestBaseRefNotFound("release/gone", []string{"release/1.4", "main"})
	_, err := k.e.GenerateProposal(context.Background(), k.in)
	f := failureOf(t, err)
	if f.Code != "REQUEST_BASE_REF_NOT_FOUND" {
		t.Fatalf("code = %s", f.Code)
	}
	for _, want := range []string{`"release/gone"`, "release/1.4, main", "choose another base ref"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("message %q lacks %q", f.Message, want)
		}
	}
	if len(k.agent.inputs) != 0 {
		t.Fatal("the agent must not run without a worktree")
	}
}

func TestOpenSpecEngine_BaseRefGone_WithoutSuggestions(t *testing.T) {
	k := newEngineKit(authoringFeatures)
	k.ws.ensureErr = domain.ErrRequestBaseRefNotFound("v9", nil)
	_, err := k.e.GenerateProposal(context.Background(), k.in)
	if f := failureOf(t, err); f.Code != domain.RunErrBaseRefNotFound || strings.Contains(f.Message, "available") {
		t.Fatalf("%+v", f)
	}
}
