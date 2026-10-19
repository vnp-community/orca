package tddagentclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

type fakeRelay struct {
	req    *infrafleetv1.RelayRequest
	tenant string
	result string
	err    error
}

func (f *fakeRelay) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	f.req = in
	if md, ok := metadata.FromOutgoingContext(ctx); ok && len(md.Get(grpcmw.MetadataTenantID)) > 0 {
		f.tenant = md.Get(grpcmw.MetadataTenantID)[0]
	}
	return &infrafleetv1.RelayResponse{ResultJson: f.result}, f.err
}

type fakeResolver struct{ err error }

func (f fakeResolver) Resolve(_ context.Context, _, target string) (string, error) {
	return "conn-" + target, f.err
}

type fakeRepos struct{ urls []string }

func (f fakeRepos) ListRepos(context.Context, *projectv1.ListReposRequest, ...grpc.CallOption) (*projectv1.ListReposResponse, error) {
	out := &projectv1.ListReposResponse{}
	for _, u := range f.urls {
		out.Repos = append(out.Repos, &projectv1.Repo{Url: u})
	}
	return out, nil
}

func tctx() context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), tenantID), "user-1")
}

func TestCallToolSendsTheBridgeRequestAndDecodesTheResult(t *testing.T) {
	relay := &fakeRelay{result: `{"ok":true,"isError":false,"result":{"run_id":"run-1"},"durationMs":5}`}
	c := NewClient(relay, fakeResolver{}, fakeRepos{})
	var out struct {
		RunID string `json:"run_id"`
	}
	if err := c.CallTool(tctx(), "project:p1", "/srv/projects/repo", "run_get", map[string]any{"run_id": "run-1"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.RunID != "run-1" || relay.req.GetMethod() != "mcp.callTool" || relay.req.GetConnectionId() != "conn-project:p1" || relay.tenant != tenantID {
		t.Fatalf("out %+v req %+v tenant %q", out, relay.req, relay.tenant)
	}
	var params map[string]any
	_ = json.Unmarshal([]byte(relay.req.GetParamsJson()), &params)
	if params["server"] != MCPServer || params["projectDir"] != "/srv/projects/repo" || params["tool"] != "run_get" {
		t.Fatalf("params %v", params)
	}
}

func TestCallToolClassifiesFailures(t *testing.T) {
	cases := []struct {
		name   string
		relay  *fakeRelay
		res    fakeResolver
		assert func(error) bool
	}{
		{"tool error", &fakeRelay{result: `{"ok":true,"isError":true,"error":{"code":"RUN_NOT_FOUND","message":"no"}}`}, fakeResolver{},
			func(err error) bool { var te *ToolError; return errors.As(err, &te) && te.Code == "RUN_NOT_FOUND" }},
		{"bridge refusal", &fakeRelay{result: `{"ok":false,"error":{"code":"MCP_TOOL_NOT_ALLOWED","message":"no"}}`}, fakeResolver{},
			func(err error) bool {
				var be *BridgeError
				return errors.As(err, &be) && be.Code == "MCP_TOOL_NOT_ALLOWED"
			}},
		{"server crash is transient", &fakeRelay{result: `{"ok":false,"error":{"code":"MCP_SERVER_FAILED","message":"exit"}}`}, fakeResolver{},
			func(err error) bool { return errors.Is(err, ErrTransport) }},
		{"relay down", &fakeRelay{err: status.Error(codes.Unavailable, "agent offline")}, fakeResolver{},
			func(err error) bool { return errors.Is(err, ErrTransport) }},
		{"no connection", &fakeRelay{}, fakeResolver{err: errors.New("dev server disconnected")},
			func(err error) bool { return errors.Is(err, ErrTransport) }},
		{"old agent", &fakeRelay{result: `not json`}, fakeResolver{},
			func(err error) bool { return err != nil && !errors.Is(err, ErrTransport) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewClient(tc.relay, tc.res, fakeRepos{}).CallTool(tctx(), "project:p1", "/d", "run_get", nil, nil)
			if !tc.assert(err) {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}
}

func TestResolveProjectDirStaysInsideTheProjectRepos(t *testing.T) {
	c := NewClient(&fakeRelay{}, fakeResolver{}, fakeRepos{urls: []string{"https://github.com/x/y", "/srv/projects/repo"}})
	if dir, err := c.ResolveProjectDir(tctx(), "p1", ""); err != nil || dir != "/srv/projects/repo" {
		t.Fatalf("default %q %v", dir, err)
	}
	if dir, err := c.ResolveProjectDir(tctx(), "p1", "/srv/projects/repo/sub/../docs"); err != nil || dir != "/srv/projects/repo/docs" {
		t.Fatalf("inside %q %v", dir, err)
	}
	for _, bad := range []string{"/srv/projects/repo-evil", "/srv/projects/repo/../other", "/etc"} {
		var be *BridgeError
		if _, err := c.ResolveProjectDir(tctx(), "p1", bad); !errors.As(err, &be) || be.Code != "TDD_AGENT_PROJECT_DIR_OUTSIDE_PROJECT" {
			t.Fatalf("%s: %v", bad, err)
		}
	}
	var be *BridgeError
	if _, err := NewClient(&fakeRelay{}, fakeResolver{}, fakeRepos{}).ResolveProjectDir(tctx(), "p1", ""); !errors.As(err, &be) || be.Code != "TDD_AGENT_NO_REPO" {
		t.Fatalf("no repo: %v", err)
	}
	if _, err := c.ResolveProjectDir(tctx(), "", ""); !errors.As(err, &be) || be.Code != "TDD_AGENT_NO_PROJECT" {
		t.Fatalf("no project: %v", err)
	}
}
