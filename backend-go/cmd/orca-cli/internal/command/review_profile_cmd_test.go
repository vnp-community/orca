package command

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

const profileReadJSON = `{"projectLayer":{"projectId":"p1","spec":{"schema":"review-drawing-profile/1"},"specDigest":"d1","version":3,"updatedBy":"u1","updatedAt":"2026-10-20T10:00:00Z"},"effective":{"style":{"detail":"detailed"}},"sources":{"style.detail":"project","style.variant":"default"}}`

func mustDoc(t *testing.T, raw string) apiclient.Document {
	t.Helper()
	var d apiclient.Document
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestReviewProfileShow_ReadsTheLayerAndSaysWhereEachValueCameFrom(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{"GET /v1/projects/p1/review-drawing-profile": profileReadJSON, "GET /v1/review-drawing-profile": `{"effective":{},"sources":{}}`}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	env := flowEnv{factory: factory, json: new(bool), exit: &exit}

	cmd := newReviewProfileShowCmd(env)
	cmd.SetArgs([]string{"--project", "p1"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 {
		t.Fatalf("err=%v exit=%d", err, exit)
	}
	tenant := newReviewProfileShowCmd(env)
	tenant.SetArgs(nil)
	if err := tenant.ExecuteContext(context.Background()); err != nil || exit != 0 {
		t.Fatalf("tenant: err=%v exit=%d", err, exit)
	}
	if strings.Join(g.calls, ",") != "GET /v1/projects/p1/review-drawing-profile,GET /v1/review-drawing-profile" {
		t.Errorf("calls = %v", g.calls)
	}
	out := renderReviewProfile(mustDoc(t, profileReadJSON))
	for _, want := range []string{"project layer: version 3, saved by u1", "tenant layer: none", `"detail": "detailed"`, "style.detail", "project", "default"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestReviewProfileSet_SendsTheSpecWithTheVersionJustReadAndRefusesBadInputBeforeAnyCall(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{
		"GET /v1/projects/p1/review-drawing-profile": profileReadJSON,
		"PUT /v1/projects/p1/review-drawing-profile": `{"layer":{"projectId":"p1","version":4,"specDigest":"d2"}}`,
	}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	env := flowEnv{factory: factory, json: new(bool), exit: &exit}

	cmd := newReviewProfileSetCmd(env)
	cmd.SetArgs([]string{"--project", "p1", "--spec", `{"schema":"review-drawing-profile/1","style":{"detail":"detailed"}}`})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 {
		t.Fatalf("err=%v exit=%d", err, exit)
	}
	put := g.bodies[1]
	spec, _ := put["spec"].(map[string]any)
	if put["expectedVersion"] != float64(3) || spec["schema"] != "review-drawing-profile/1" {
		t.Fatalf("the PUT must carry the spec as an object and the version just read: %v", put)
	}

	for name, args := range map[string][]string{
		"no spec":      {"--project", "p1"},
		"not json":     {"--project", "p1", "--spec", "{nope"},
		"both sources": {"--project", "p1", "--spec", "{}", "--file", "x.json"},
	} {
		before := len(g.calls)
		bad := newReviewProfileSetCmd(env)
		bad.SetArgs(args)
		exit = 0
		if err := bad.ExecuteContext(context.Background()); err != nil || exit == 0 || len(g.calls) != before {
			t.Errorf("%s: err=%v exit=%d calls=%d", name, err, exit, len(g.calls)-before)
		}
	}
}

func TestReviewProfileReset_UsesTheVersionJustReadAndAnExplicitOneWins(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{
		"GET /v1/projects/p1/review-drawing-profile":        profileReadJSON,
		"POST /v1/projects/p1/review-drawing-profile/reset": `{"reset":true}`,
	}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	env := flowEnv{factory: factory, json: new(bool), exit: &exit}

	cmd := newReviewProfileResetCmd(env)
	cmd.SetArgs([]string{"--project", "p1"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 || g.bodies[1]["expectedVersion"] != float64(3) {
		t.Fatalf("err=%v exit=%d bodies=%v", err, exit, g.bodies)
	}
	explicit := newReviewProfileResetCmd(env)
	explicit.SetArgs([]string{"--project", "p1", "--expected-version", "9"})
	if err := explicit.ExecuteContext(context.Background()); err != nil || exit != 0 || g.bodies[len(g.bodies)-1]["expectedVersion"] != float64(9) || len(g.calls) != 3 {
		t.Fatalf("an explicit version skips the read: calls=%v bodies=%v", g.calls, g.bodies)
	}
}
