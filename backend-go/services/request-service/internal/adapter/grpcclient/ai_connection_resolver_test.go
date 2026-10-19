package grpcclient

import (
	"testing"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

func reachableDevServer(id string) *fakeInfra {
	return &fakeInfra{health: []*infrafleetv1.DevServerHealth{{DevServerId: id, Reachable: true}}}
}

func TestResolveForProject_FallbackTakesRepoPathFromTheDefaultRepoURL(t *testing.T) {
	cases := []struct{ name, url, want string }{
		{"posix folder", "/home/dev/orca", "/home/dev/orca"},
		{"windows folder", `C:\work\orca`, `C:\work\orca`},
		{"windows folder with slashes", "D:/work/orca", "D:/work/orca"},
		{"unc share", `\\nas\code\orca`, `\\nas\code\orca`},
		{"https remote", "https://github.com/acme/orca.git", ""},
		{"scp-like remote", "git@github.com:acme/orca.git", ""},
		{"file url", "file:///home/dev/orca", ""},
		{"relative path", "orca/backend", ""},
		{"parent segment", "/home/dev/../other", ""},
		{"empty url", "", ""},
		{"control character", "/home/dev/orca\n--flag", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projects := &fakeProjects{repos: []*projectv1.Repo{{Id: "r1", Url: tc.url, DevServerId: "ds-1"}}}
			got, err := NewAIConnectionResolver(reachableDevServer("ds-1"), projects).ResolveForProject(ctxWithIdentity(), "proj")
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got.DevServerID != "ds-1" || got.RepoPath != tc.want || got.WorktreeID != "" {
				t.Fatalf("got %+v, want repo path %q", got, tc.want)
			}
		})
	}
}

func TestResolveForProject_FallbackUsesTheFirstRepoOnly(t *testing.T) {
	// The second repo lives elsewhere: the path must come from the repo whose dev server was checked.
	projects := &fakeProjects{repos: []*projectv1.Repo{
		{Id: "r1", Url: "/srv/primary", DevServerId: "ds-1"},
		{Id: "r2", Url: "/srv/secondary", DevServerId: "ds-2"},
	}}
	got, err := NewAIConnectionResolver(reachableDevServer("ds-1"), projects).ResolveForProject(ctxWithIdentity(), "proj")
	if err != nil || got.DevServerID != "ds-1" || got.RepoPath != "/srv/primary" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestResolveForProject_ConnectedKeepsTheInfraRepoPath(t *testing.T) {
	infra := &fakeInfra{connected: true, repoPath: "/srv/from-connection", worktreeID: "wt-1"}
	projects := &fakeProjects{repos: []*projectv1.Repo{{Url: "/srv/other", DevServerId: "ds-1"}}}
	got, err := NewAIConnectionResolver(infra, projects).ResolveForProject(ctxWithIdentity(), "proj")
	if err != nil || got.RepoPath != "/srv/from-connection" || got.WorktreeID != "wt-1" || got.DevServerID != "" {
		t.Fatalf("got %+v err %v", got, err)
	}
}
