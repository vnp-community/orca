package command

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/config"
)

// gateway is a scripted api-gateway: it records each call and answers with the next canned body for that route.
type gateway struct {
	t      *testing.T
	calls  []string
	bodies []map[string]any
	reply  map[string]string
}

func (g *gateway) server() (*httptest.Server, func() (*apiclient.Client, error)) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := r.Method + " " + r.URL.Path
		g.calls = append(g.calls, route)
		var body map[string]any
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		g.bodies = append(g.bodies, body)
		out, ok := g.reply[route]
		if !ok {
			g.t.Errorf("unexpected call %s", route)
			http.Error(w, `{"error":{"code":"UNEXPECTED","message":"no script"}}`, http.StatusTeapot)
			return
		}
		if strings.HasPrefix(out, "!") { // "!409 CODE message" scripts an error
			f := strings.SplitN(out[1:], " ", 3)
			w.WriteHeader(map[string]int{"409": 409, "400": 400}[f[0]])
			_, _ = w.Write([]byte(`{"error":{"code":"` + f[1] + `","message":"` + f[2] + `"}}`))
			return
		}
		_, _ = w.Write([]byte(out))
	}))
	return srv, func() (*apiclient.Client, error) { return apiclient.New(srv.URL, "tok"), nil }
}

func TestConfirmType_UsesTheCurrentVersionWhenNoneIsGiven(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{
		"GET /v1/requests/r1":               `{"request":{"id":"r1","version":7,"status":"awaiting_type_confirmation"}}`,
		"POST /v1/requests/r1/confirm-type": `{"request":{"id":"r1","version":8,"status":"analyzing"}}`,
	}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	env := flowEnv{factory: factory, json: new(bool), exit: &exit}
	cmd := newRequestConfirmTypeCmd(env)
	cmd.SetArgs([]string{"r1", "--type", "change_request", "--size", "S"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 {
		t.Fatalf("err=%v exit=%d", err, exit)
	}
	sent := g.bodies[1]
	if sent["type"] != "change_request" || sent["size"] != "S" || sent["expectedVersion"] != float64(7) {
		t.Errorf("confirm body = %v", sent)
	}
	if _, has := sent["urgency"]; has {
		t.Errorf("an unset flag must not be sent: %v", sent)
	}
}

func TestConfirmType_MissingTypeIsAUsageErrorWithoutAnyCall(t *testing.T) {
	g := &gateway{t: t}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newRequestConfirmTypeCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"r1"})
	_ = cmd.ExecuteContext(context.Background())
	if exit != 2 || len(g.calls) != 0 {
		t.Fatalf("exit=%d calls=%v", exit, g.calls)
	}
}

func TestApprove_ReadsVersionAndDigestThenDecidesAgainstThem(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{
		"GET /v1/approvals/a1":          `{"approval":{"id":"a1","subjectType":"plan","version":3,"subjectDigest":"dg"}}`,
		"POST /v1/approvals/a1/approve": `{"approval":{"id":"a1","status":"approved"},"requestStatus":"executing"}`,
	}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newApprovalDecideCmd(flowEnv{factory: factory, json: new(bool), exit: &exit}, "approve")
	cmd.SetArgs([]string{"a1", "--comment", "ok"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 {
		t.Fatalf("err=%v exit=%d", err, exit)
	}
	if b := g.bodies[1]; b["version"] != float64(3) || b["digest"] != "dg" || b["comment"] != "ok" {
		t.Errorf("approve body = %v", b)
	}
}

func TestReject_NeedsACommentAndHalfAPairIsRefused(t *testing.T) {
	for name, args := range map[string][]string{
		"no comment":   {"a1"},
		"version only": {"a1", "--comment", "x", "--version", "2"},
	} {
		g := &gateway{t: t}
		srv, factory := g.server()
		var exit int
		cmd := newApprovalDecideCmd(flowEnv{factory: factory, json: new(bool), exit: &exit}, "reject")
		cmd.SetArgs(args)
		_ = cmd.ExecuteContext(context.Background())
		srv.Close()
		if exit != 2 || len(g.calls) != 0 {
			t.Errorf("%s: exit=%d calls=%v", name, exit, g.calls)
		}
	}
}

func TestServiceRefusalKeepsItsCodeAndExits1(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{
		"POST /v1/requests/r1/phases/p1/start": "!409 REQUEST_PHASE_NOT_APPROVED phase p1 has no approved phase approval",
	}}
	srv, factory := g.server()
	defer srv.Close()
	exit := RunFlowCall(context.Background(), factory, true, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
		return cli.StartPhase(ctx, "r1", "p1")
	}, nil)
	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
}

func TestAnswerItem_TextIsAJSONStringAndRawJSONIsKept(t *testing.T) {
	text, err := answerItem(`q1=line one`+"\n"+`line two`, false)
	if err != nil || text["valueJson"] != `"line one\nline two"` {
		t.Fatalf("text = %v err=%v", text, err)
	}
	raw, err := answerItem(`q2={"a":1}`, true)
	if err != nil || raw["valueJson"] != `{"a":1}` {
		t.Fatalf("raw = %v err=%v", raw, err)
	}
	if _, err := answerItem(`q3=not json`, true); err == nil {
		t.Error("invalid raw JSON must be refused")
	}
	if _, err := answerItem(`=x`, false); err == nil {
		t.Error("a missing question id must be refused")
	}
}

func TestRequestStatusWait_StopsAtTheTargetAndTimesOut(t *testing.T) {
	statuses := []string{"classifying", "classifying", "awaiting_type_confirmation"}
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := statuses[min(i, len(statuses)-1)]
		i++
		_, _ = w.Write([]byte(`{"request":{"id":"r1","status":"` + s + `"}}`))
	}))
	defer srv.Close()
	w := waiter{interval: time.Millisecond, progress: io.Discard}
	cli := apiclient.New(srv.URL, "tok")
	doc, err := w.requestStatus(context.Background(), cli, "r1", map[string]bool{"awaiting_type_confirmation": true}, time.Second)
	if err != nil || doc.Str("request", "status") != "awaiting_type_confirmation" {
		t.Fatalf("doc=%v err=%v", doc, err)
	}
	_, err = w.requestStatus(context.Background(), cli, "r1", map[string]bool{"never": true}, 20*time.Millisecond)
	if cliErr, ok := err.(*apiclient.CLIError); !ok || cliErr.Code != "WAIT_TIMEOUT" {
		t.Fatalf("err = %v, want WAIT_TIMEOUT", err)
	}
}

func TestAnalysisRunWait_ReportsAFailedRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"solutions":[],"runs":[{"id":"run1","status":"failed","finishedAt":"2026-10-13T00:00:00Z","errorCode":"REQUEST_AI_UNAVAILABLE","errorMessage":"no agent"}]}`))
	}))
	defer srv.Close()
	w := waiter{interval: time.Millisecond, progress: io.Discard}
	_, err := w.analysisRun(context.Background(), apiclient.New(srv.URL, "t"), "r1", "run1", time.Second)
	if cliErr, ok := err.(*apiclient.CLIError); !ok || cliErr.Code != "REQUEST_AI_UNAVAILABLE" {
		t.Fatalf("err = %v", err)
	}
}

func TestRenderPlanTree_IndentsByParentAndNamesWhatAPhaseWaitsFor(t *testing.T) {
	doc := apiclient.Document{
		"tasks": []any{
			map[string]any{"id": "pl", "parent_id": "", "task_type": "plan", "status": "open", "title": "Plan"},
			map[string]any{"id": "ph2", "parent_id": "pl", "task_type": "phase", "status": "open", "title": "B phase"},
			map[string]any{"id": "ph1", "parent_id": "pl", "task_type": "phase", "status": "open", "title": "A phase"},
			map[string]any{"id": "t1", "parent_id": "ph1", "task_type": "task", "status": "blocked", "title": "work"},
		},
		"depends_on_edges": []any{map[string]any{"from_task_id": "ph2", "to_task_id": "ph1"}},
	}
	lines := strings.Split(renderPlanTree(doc), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[1], "  phase") || !strings.Contains(lines[1], "A phase") ||
		!strings.HasPrefix(lines[2], "    task") || !strings.Contains(lines[3], "(after ph1)") {
		t.Errorf("tree =\n%s", strings.Join(lines, "\n"))
	}
}

func TestLogin_SavesTheTokenAndNeverPrintsIt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/cli-token" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"jwt":"secret-jwt","expires_at":{"seconds":1790000000}}`))
	}))
	defer srv.Close()
	var saved config.Credentials
	exit := ExecuteLogin(context.Background(), srv.URL, "a@b.c", "pw", func(c config.Credentials) error { saved = c; return nil }, true)
	if exit != 0 || saved.JWT != "secret-jwt" || saved.APIURL != srv.URL || saved.ExpiresAt.IsZero() {
		t.Fatalf("exit=%d saved=%+v", exit, saved)
	}
	if exit := ExecuteLogin(context.Background(), srv.URL, "", "pw", func(config.Credentials) error { return nil }, true); exit != 2 {
		t.Errorf("missing email: exit=%d", exit)
	}
}

func TestApprove_ExecutionModeIsSentOnlyWhenGiven(t *testing.T) {
	for name, tc := range map[string]struct {
		args []string
		want any
	}{
		"continuous":       {[]string{"a1", "--version", "3", "--digest", "dg", "--execution-mode", "continuous"}, "continuous"},
		"normalised":       {[]string{"a1", "--version", "3", "--digest", "dg", "--execution-mode", " Continuous "}, "continuous"},
		"no flag":          {[]string{"a1", "--version", "3", "--digest", "dg"}, nil},
		"empty flag value": {[]string{"a1", "--version", "3", "--digest", "dg", "--execution-mode", ""}, nil},
	} {
		g := &gateway{t: t, reply: map[string]string{
			"POST /v1/approvals/a1/approve": `{"approval":{"id":"a1","status":"approved"},"requestStatus":"executing","executionMode":"continuous"}`,
		}}
		srv, factory := g.server()
		var exit int
		cmd := newApprovalDecideCmd(flowEnv{factory: factory, json: new(bool), exit: &exit}, "approve")
		cmd.SetArgs(tc.args)
		err := cmd.ExecuteContext(context.Background())
		srv.Close()
		if err != nil || exit != 0 || len(g.bodies) != 1 {
			t.Fatalf("%s: err=%v exit=%d calls=%v", name, err, exit, g.calls)
		}
		if got, has := g.bodies[0]["executionMode"]; got != tc.want || (tc.want == nil && has) {
			t.Errorf("%s: body = %v, want executionMode %v", name, g.bodies[0], tc.want)
		}
	}
}

func TestDecide_ABadExecutionModeIsAUsageErrorBeforeAnyCall(t *testing.T) {
	for name, tc := range map[string]struct {
		decision string
		args     []string
	}{
		"step_by_step is not offered": {"approve", []string{"a1", "--execution-mode", "step_by_step"}},
		"garbage":                     {"approve", []string{"a1", "--execution-mode", "turbo"}},
		"reject refuses the flag":     {"reject", []string{"a1", "--comment", "no", "--execution-mode", "continuous"}},
	} {
		g := &gateway{t: t}
		srv, factory := g.server()
		var exit int
		cmd := newApprovalDecideCmd(flowEnv{factory: factory, json: new(bool), exit: &exit}, tc.decision)
		cmd.SetArgs(tc.args)
		_ = cmd.ExecuteContext(context.Background())
		srv.Close()
		if exit != 2 || len(g.calls) != 0 {
			t.Errorf("%s: exit=%d calls=%v", name, exit, g.calls)
		}
	}
}
