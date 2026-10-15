package command

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func TestPlanRunTimeout_AnAgentAuthoredChangeGetsLongerThanTheNativeCall(t *testing.T) {
	if planRunTimeout("native") != 4*time.Minute || planRunTimeout("") != 4*time.Minute {
		t.Fatalf("native = %s", planRunTimeout("native"))
	}
	if planRunTimeout("openspec") != 64*time.Minute {
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

func TestPlanProposalWait_ATokenThatRunsOutMidWaitSaysTheRunGoesOn(t *testing.T) {
	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if polls.Add(1) < 3 {
			_, _ = w.Write([]byte(`{"runId":"r","status":"running","engine":"openspec"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"UNAUTHENTICATED","message":"authvalidator: signature verification failed"}}`))
	}))
	defer srv.Close()
	w := waiter{interval: time.Millisecond, progress: io.Discard}
	_, err := w.planProposal(context.Background(), apiclient.New(srv.URL, "t"), "req-1", "r", 0)
	cliErr, ok := err.(*apiclient.CLIError)
	if !ok || cliErr.StatusCode != 401 || !strings.Contains(cliErr.Message, "the run goes on") || !strings.Contains(cliErr.Message, "orca plan proposal req-1") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlanCommitCmd_CommitsTheProposalOfAFinishedRunWithoutStartingAnother(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{
		"GET /v1/requests/r1/plan":  `{"status":"succeeded","proposal":{"phases":[]},"rawAiResponse":"raw"}`,
		"POST /v1/requests/r1/plan": `{"planTaskId":"p1"}`,
	}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newPlanCommitCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"r1"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 {
		t.Fatalf("err=%v exit=%d calls=%v", err, exit, g.calls)
	}
	if g.bodies[1]["mode"] != "commit" || g.bodies[1]["rawAiResponse"] != "raw" || g.bodies[1]["proposal"] == nil {
		t.Fatalf("commit body: %v", g.bodies[1])
	}
}

func TestPlanCommitCmd_RefusesARunThatIsStillGoing(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{"GET /v1/requests/r1/plan": `{"status":"running"}`}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newPlanCommitCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"r1"})
	_ = cmd.ExecuteContext(context.Background())
	if exit == 0 || len(g.calls) != 1 {
		t.Fatalf("exit=%d calls=%v", exit, g.calls)
	}
}
