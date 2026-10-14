package command

import (
	"context"
	"strings"
	"testing"
)

func TestRequestCleanupCmd_PostsToTheCleanupRouteAndSaysWhatStays(t *testing.T) {
	g := &gateway{t: t, reply: map[string]string{"POST /v1/requests/r1/proposal/cleanup": `{"removed":true,"abandoned":true,"branch":"request/1-proposal"}`}}
	srv, factory := g.server()
	defer srv.Close()
	var exit int
	cmd := newRequestCleanupCmd(flowEnv{factory: factory, json: new(bool), exit: &exit})
	cmd.SetArgs([]string{"r1"})
	if err := cmd.ExecuteContext(context.Background()); err != nil || exit != 0 || len(g.calls) != 1 {
		t.Fatalf("err=%v exit=%d calls=%v", err, exit, g.calls)
	}
}

func TestRenderProposalCleanup(t *testing.T) {
	cases := map[string]string{
		`{"removed":true,"abandoned":true,"branch":"request/1-proposal"}`: "marked the change abandoned; branch request/1-proposal is kept",
		`{"removed":true,"branch":"request/1-proposal"}`:                  "removed the proposal worktree; branch request/1-proposal is kept",
		`{"removed":false}`: "no proposal worktree to remove",
	}
	for in, want := range cases {
		if got := renderProposalCleanup(parseDoc(t, in)); !strings.Contains(got, want) {
			t.Errorf("%s => %q, want it to contain %q", in, got, want)
		}
	}
}
