package command

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

const openspecProposalJSON = `{"runId":"run-1","status":"succeeded","engine":"openspec",
 "proposal":{"title":"Nền AI","phases":[{"title":"a","tasks":[{"title":"x"},{"title":"y"}]},{"title":"b","tasks":[{"title":"z"}]}]},
 "bundle":{"changeId":"req-142-nen-ai","branch":"request/142-proposal","commit":"abc1234","files":[{"path":"tasks.md","size":1200},{"path":"specs/ai/spec.md","size":80000,"omitted":true}]},
 "checks":[{"code":"STRUCTURE_OK","message":"passed","severity":"info"},{"code":"KNOWLEDGE_CONFLICT","where":"design.md","message":"ADR-12 vs AC-5","severity":"warning"}],
 "knowledge":{"available":true,"snapshot":"snap-1","read":["FR-1","ADR-12"],"gaps":["quota"],"conflicts":["ADR-12 vs AC-5"]}}`

func TestRenderPlanProposal_OpenSpecShowsWhereTheChangeLivesAndWhatToWarnAbout(t *testing.T) {
	doc := parseDoc(t, openspecProposalJSON)
	out := renderPlanProposal(doc)
	for _, want := range []string{
		"run run-1  succeeded  engine openspec", "2 phases, 3 tasks", "change  req-142-nen-ai", "branch  request/142-proposal  commit abc1234",
		"tasks.md", "too large to keep", "warning  KNOWLEDGE_CONFLICT (design.md): ADR-12 vs AC-5", "info", "available=true snapshot=snap-1 read=2 gaps=1 conflicts=1",
		"conflict: ADR-12 vs AC-5",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestRenderPlanProposal_NativeHasNoBundleSection(t *testing.T) {
	out := renderPlanProposal(parseDoc(t, `{"runId":"r","status":"succeeded","engine":"native","proposal":{"title":"P","tasks":[{"title":"t"}]}}`))
	if strings.Contains(out, "change ") || strings.Contains(out, "branch") || strings.Contains(out, "knowledge") {
		t.Fatalf("a native plan has no authored change; nothing must read as 'found nothing':\n%s", out)
	}
	if !strings.Contains(out, "engine native") || !strings.Contains(out, "0 phases, 1 tasks") {
		t.Fatalf("%s", out)
	}
}

func TestRenderPlanProposal_ShowsAFailedRun(t *testing.T) {
	out := renderPlanProposal(parseDoc(t, `{"runId":"r","status":"failed","errorCode":"REQUEST_SPEC_RUN_OUT_OF_SCOPE","errorMessage":"touched backend/main.go"}`))
	if !strings.Contains(out, "error REQUEST_SPEC_RUN_OUT_OF_SCOPE: touched backend/main.go") {
		t.Fatalf("%s", out)
	}
}

func TestRenderPlanBundle_NativeAndFile(t *testing.T) {
	if out := renderPlanBundle(parseDoc(t, `{"engine":"native"}`)); !strings.Contains(out, "no authored OpenSpec change") || !strings.Contains(out, "native") {
		t.Fatal(out)
	}
	if out := renderPlanBundle(parseDoc(t, openspecProposalJSON)); !strings.Contains(out, "specs/ai/spec.md") || strings.Contains(out, "2 phases") {
		t.Fatalf("the bundle view lists files and findings, not the plan tree:\n%s", out)
	}
	if out := renderBundleFile(parseDoc(t, `{"path":"tasks.md","content":"# t\n\n"}`)); out != "# tasks.md\n# t" {
		t.Fatalf("%q", out)
	}
	if out := renderBundleFile(parseDoc(t, `{"path":"big.md","content":"<omitted size=9 sha256=a>","omitted":true}`)); !strings.Contains(out, "not stored: too large") {
		t.Fatal(out)
	}
}

func TestPlanBundleCmd_ListsFilesOrReadsOneFile(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{
		"GET /v1/requests/r1/plan":             openspecProposalJSON,
		"GET /v1/requests/r1/plan/bundle/file": `{"path":"tasks.md","content":"# t"}`,
	}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	env := flowEnv{factory: factory, json: new(bool), exit: &exit}

	list := newPlanBundleCmd(env)
	list.SetArgs([]string{"r1", "--run", "run-1"})
	if err := list.ExecuteContext(context.Background()); err != nil || exit != 0 {
		t.Fatalf("list: err=%v exit=%d", err, exit)
	}
	file := newPlanBundleCmd(env)
	file.SetArgs([]string{"r1", "--file", "specs/ai/spec.md"})
	if err := file.ExecuteContext(context.Background()); err != nil || exit != 0 {
		t.Fatalf("file: err=%v exit=%d", err, exit)
	}
	if len(g.calls) != 2 || g.calls[0] != "GET /v1/requests/r1/plan" || g.calls[1] != "GET /v1/requests/r1/plan/bundle/file" {
		t.Fatalf("calls = %v", g.calls)
	}
}

func TestPlanBundleCmd_AServiceRefusalKeepsItsCode(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{"GET /v1/requests/r1/plan/bundle/file": "!400 REQUEST_PLAN_BUNDLE_PATH_INVALID the path must name a file inside the change"}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newPlanBundleCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"r1", "--file", "../../etc/passwd"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit == 0 {
		t.Fatalf("err=%v exit=%d: a refusal must not look like success", err, exit)
	}
}

func parseDoc(t *testing.T, raw string) apiclient.Document {
	t.Helper()
	var d apiclient.Document
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatal(err)
	}
	return d
}
