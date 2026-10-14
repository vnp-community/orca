package command

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func TestPlanRunTimeout_AnAgentAuthoredChangeGetsLongerThanTheNativeCall(t *testing.T) {
	if planRunTimeout("native") != 4*time.Minute || planRunTimeout("") != 4*time.Minute {
		t.Fatalf("native = %s", planRunTimeout("native"))
	}
	if planRunTimeout("openspec") != 18*time.Minute {
		t.Fatalf("openspec = %s", planRunTimeout("openspec"))
	}
}

func TestPlanProposalWait_FollowsTheRunUntilItHasAProposal(t *testing.T) {
	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if polls.Add(1) < 4 {
			_, _ = w.Write([]byte(`{"runId":"r","status":"running","engine":"openspec"}`))
			return
		}
		_, _ = w.Write([]byte(`{"runId":"r","status":"succeeded","engine":"openspec","proposal":{"title":"P"}}`))
	}))
	defer srv.Close()
	w := waiter{interval: time.Millisecond, progress: io.Discard}
	doc, err := w.planProposal(context.Background(), apiclient.New(srv.URL, "t"), "req", "r", 0)
	if err != nil || doc["proposal"] == nil {
		t.Fatalf("doc=%v err=%v", doc, err)
	}
}

func TestPlanProposalWait_AnExplicitTimeoutOverridesTheEngineBudget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"runId":"r","status":"running","engine":"openspec"}`))
	}))
	defer srv.Close()
	w := waiter{interval: time.Millisecond, progress: io.Discard}
	_, err := w.planProposal(context.Background(), apiclient.New(srv.URL, "t"), "req", "r", 20*time.Millisecond)
	if cliErr, ok := err.(*apiclient.CLIError); !ok || cliErr.Code != "WAIT_TIMEOUT" {
		t.Fatalf("err = %v, want WAIT_TIMEOUT", err)
	}
}
