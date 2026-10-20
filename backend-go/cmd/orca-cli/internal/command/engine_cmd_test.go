package command

import (
	"context"
	"strings"
	"testing"
)

const engineReadJSON = `{"projectId":"p1","engine":"openspec","openspecMinVersion":"1.2.0","solutionAgent":false,"version":4}`

func TestEngineSetCmd_ChangesOnlyWhatWasNamedAndSendsTheVersionJustRead(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{
		"GET /v1/projects/p1/request-engine": engineReadJSON,
		"PUT /v1/projects/p1/request-engine": `{"projectId":"p1","engine":"openspec","solutionAgent":true,"version":5,"preflight":{"ran":true,"ok":true,"openspecVersion":"1.3.0","blockers":[],"gaps":[{"code":"REQUEST_KNOWLEDGE_SKILLS_MISSING","message":"skills not installed","fixHint":"Run: npx skills add x"}],"warnings":[]}}`,
	}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newEngineSetCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"--project", "p1", "--solution-agent=true"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 {
		t.Fatalf("err=%v exit=%d", err, exit)
	}
	put := g.bodies[1]
	if put["engine"] != "openspec" || put["openspecMinVersion"] != "1.2.0" || put["solutionAgent"] != true || put["expectedVersion"] != float64(4) {
		t.Fatalf("an unnamed switch must keep its value: %v", put)
	}
}

func TestEngineSetCmd_RefusesToRunWithoutSayingWhatToChange(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newEngineSetCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"--project", "p1"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit == 0 || len(g.calls) != 0 {
		t.Fatalf("err=%v exit=%d calls=%v", err, exit, g.calls)
	}
}

func TestEngineShowCmd_AsksForTheCheckOnlyWhenToldTo(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{"GET /v1/projects/p1/request-engine": engineReadJSON}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	env := flowEnv{factory: factory, json: new(bool), exit: &exit}
	plain := newEngineShowCmd(env)
	plain.SetArgs([]string{"--project", "p1"})
	if err := plain.ExecuteContext(context.Background()); err != nil || exit != 0 {
		t.Fatalf("err=%v exit=%d", err, exit)
	}
	checked := newEngineShowCmd(env)
	checked.SetArgs([]string{"--project", "p1", "--check"})
	if err := checked.ExecuteContext(context.Background()); err != nil || exit != 0 {
		t.Fatalf("err=%v exit=%d", err, exit)
	}
	if len(g.calls) != 2 {
		t.Fatalf("calls = %v", g.calls)
	}
}

func TestRenderEngineSettings_SaysWhatIsOnAndWhyItCannotRunYet(t *testing.T) {
	out := renderEngineSettings(parseDoc(t, `{"projectId":"p1","engine":"openspec","openspecMinVersion":"1.2.0","solutionAgent":true,"version":5,
		"preflight":{"ran":true,"ok":false,"blockers":[{"code":"REQUEST_ENGINE_OPENSPEC_MISSING","message":"openspec is not installed","fixHint":"Install the OpenSpec CLI"}],
		"gaps":[{"code":"REQUEST_KNOWLEDGE_NOT_INITIALIZED","message":"no config","fixHint":"Set tdd-agent up"}],"warnings":["preflight note"]}}`))
	for _, want := range []string{"plan engine: openspec (openspec >= 1.2.0)", "solution by agent: on", "NOT ready; the old mode keeps working",
		"blocked: openspec is not installed (REQUEST_ENGINE_OPENSPEC_MISSING)", "fix: Install the OpenSpec CLI", "knowledge hub unused: no config", "warning: preflight note"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	plain := renderEngineSettings(parseDoc(t, engineReadJSON))
	if strings.Contains(plain, "readiness") || !strings.Contains(plain, "solution by agent: off") {
		t.Fatalf("a read without a check printed a verdict:\n%s", plain)
	}
}

func TestEngineSetCmd_PlanAutoGenerateIsSentOnlyWhenNamed(t *testing.T) {
	run := func(args ...string) map[string]any {
		g := &gateway{t: t, reply: map[string]string{
			"GET /v1/projects/p1/request-engine": engineReadJSON,
			"PUT /v1/projects/p1/request-engine": `{"projectId":"p1","engine":"openspec","planAutoGenerate":false,"version":5}`,
		}}
		srv, factory := g.server()
		defer srv.Close()
		var exit int
		cmd := newEngineSetCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
		cmd.SetArgs(append([]string{"--project", "p1"}, args...))
		if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 {
			t.Fatalf("err=%v exit=%d", err, exit)
		}
		return g.bodies[1]
	}
	if put := run("--solution-agent=true"); put["planAutoGenerate"] != nil {
		t.Fatalf("an unnamed switch must not be sent: %v", put)
	}
	if put := run("--plan-auto-generate=false"); put["planAutoGenerate"] != false {
		t.Fatalf("false must be sent: %v", put)
	}
	if put := run("--plan-auto-generate=true"); put["planAutoGenerate"] != true {
		t.Fatalf("true must be sent: %v", put)
	}
}

func TestRenderEngineSettings_ShowsPlanAutoGenerateOnlyWhenTheGatewaySendsIt(t *testing.T) {
	with := renderEngineSettings(parseDoc(t, `{"projectId":"p1","engine":"native","solutionAgent":false,"planAutoGenerate":false,"version":2}`))
	if !strings.Contains(with, "plan auto-generate: off") {
		t.Fatalf("output = %q", with)
	}
	if without := renderEngineSettings(parseDoc(t, engineReadJSON)); strings.Contains(without, "plan auto-generate") {
		t.Fatalf("an older gateway sends no value: %q", without)
	}
}
