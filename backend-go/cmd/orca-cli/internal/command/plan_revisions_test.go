package command

import (
	"context"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

const revisionsJSON = `{"revisions":[
 {"revision":3,"runId":"run-3","status":"current","engine":"openspec","revisionFeedback":"split the migration\nand keep the schema","generatedAt":"2026-10-20T09:05:00Z","filesChanged":2,"fileCount":5,"textKept":true},
 {"revision":2,"runId":"run-2","status":"superseded","engine":"openspec","revisionFeedback":"","generatedAt":"2026-10-20T08:00:00Z","filesChanged":0,"fileCount":4,"textKept":false},
 {"revision":1,"runId":"run-1","status":"superseded","engine":"native","generatedAt":"2026-10-19T08:00:00Z","textKept":true}],
 "nextPageToken":"1"}`

func TestRenderPlanRevisions_OneRowPerVersionWithTheAskOnOneLine(t *testing.T) {
	out := renderPlanRevisions(parseDoc(t, revisionsJSON))
	for _, want := range []string{
		"VERSION", "3        current     2026-10-20 09:05  2      split the migration and keep the schema",
		"2        superseded  2026-10-20 08:00  -      -", "1        superseded  2026-10-19 08:00  -      -", "more: --page-token 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestRenderPlanRevisions_AnAskIsCutAtTheWidthOnACharacterBoundary(t *testing.T) {
	ask := strings.Repeat("ế", planAskWidth+5)
	out := renderPlanRevisions(parseDoc(t, `{"revisions":[{"revision":1,"status":"current","engine":"openspec","textKept":true,"revisionFeedback":"`+ask+`"}]}`))
	if !strings.Contains(out, strings.Repeat("ế", planAskWidth)+"…") || strings.Contains(out, strings.Repeat("ế", planAskWidth+1)) {
		t.Fatalf("%s", out)
	}
}

func TestRenderPlanRevisions_NoVersions(t *testing.T) {
	if out := renderPlanRevisions(parseDoc(t, `{"revisions":[]}`)); !strings.Contains(out, "no plan version") {
		t.Fatal(out)
	}
}

func TestRenderPlanBundle_MarksWhatChangedSinceTheVersionBefore(t *testing.T) {
	out := renderPlanBundle(parseDoc(t, `{"engine":"openspec","bundle":{"changeId":"c","branch":"b","commit":"x","files":[
 {"path":"tasks.md","size":10,"change":"changed"},{"path":"design.md","size":5,"change":"removed"},{"path":"proposal.md","size":3,"change":"unchanged"}]}}`))
	if !strings.Contains(out, "tasks.md") || !strings.Contains(out, "[changed]") || !strings.Contains(out, "[removed]") || strings.Contains(out, "[unchanged]") {
		t.Fatalf("%s", out)
	}
}

func TestPlanRevisions_CommandsCallTheRoutesWithTheirArguments(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{
		"GET /v1/requests/r1/plan/revisions":   revisionsJSON,
		"GET /v1/requests/r1/plan":             `{"runId":"run-2","status":"succeeded","engine":"openspec","bundle":{"changeId":"c","branch":"b","commit":"x","files":[]}}`,
		"GET /v1/requests/r1/plan/bundle/file": `{"path":"tasks.md","content":"# old\n"}`,
	}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	env := flowEnv{factory: factory, json: new(bool), exit: &exit}

	for _, args := range [][]string{{"revisions", "r1", "--page-size", "5", "--page-token", "3"}, {"bundle", "r1", "--revision", "2"}, {"bundle", "r1", "--file", "tasks.md", "--revision", "1"}} {
		cmd := newPlanCmd(env)
		cmd.SetArgs(args)
		if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 {
			t.Fatalf("%v: err=%v exit=%d", args, err, exit)
		}
	}
	if len(g.calls) != 3 {
		t.Fatalf("calls = %v", g.calls)
	}
	// The scripted gateway keys on the path, so the query is checked through the client directly.
	cli := apiclient.New(srv.URL, "tok")
	if _, err := cli.GetPlanProposalRevision(context.Background(), "r1", "", 2); err != nil {
		t.Fatal(err)
	}
}

func TestPlanBundle_ANegativeRevisionIsAUsageErrorWithoutAnyCall(t *testing.T) {
	g := &gateway{t: t}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newPlanBundleCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"r1", "--revision", "-1"})
	_ = cmd.ExecuteContext(context.Background())
	if exit != 2 || len(g.calls) != 0 {
		t.Fatalf("exit=%d calls=%v", exit, g.calls)
	}
}
