package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestScanSetupScriptImports_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing repo id is rejected", func(t *testing.T) {
		uc := NewScanSetupScriptImports(&fakeDevServerReachability{}, &fakeProjectClient{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), ScanSetupScriptImportsInput{})
		if err == nil {
			t.Fatal("expected error for missing repo_id")
		}
	})

	t.Run("reachable dev server dispatches to relay executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", DevServerID: "ds1", URL: "/srv/repo"}}
		reachability := &fakeDevServerReachability{reachable: true}
		relay := &fakeGitExecutor{setupScriptImportPaths: []string{"source ./lib.sh"}}
		local := &fakeGitExecutor{}
		uc := NewScanSetupScriptImports(reachability, projects, local, relay)

		got, err := uc.Execute(context.Background(), ScanSetupScriptImportsInput{RepoID: "repo-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("unexpected result: %+v", got)
		}
		if !relay.calledScanSetupScriptImports || local.calledScanSetupScriptImports {
			t.Error("expected relay executor to be called, not local")
		}
	})

	t.Run("no dev server bound dispatches to local executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/local/repo"}}
		reachability := &fakeDevServerReachability{}
		local := &fakeGitExecutor{}
		relay := &fakeGitExecutor{}
		uc := NewScanSetupScriptImports(reachability, projects, local, relay)

		_, err := uc.Execute(context.Background(), ScanSetupScriptImportsInput{RepoID: "repo-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledScanSetupScriptImports || relay.calledScanSetupScriptImports {
			t.Error("expected local executor to be called, not relay")
		}
	})

	t.Run("repo not found is reported", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoErr: context.DeadlineExceeded}
		uc := NewScanSetupScriptImports(&fakeDevServerReachability{}, projects, &fakeGitExecutor{}, &fakeGitExecutor{})

		_, err := uc.Execute(context.Background(), ScanSetupScriptImportsInput{RepoID: "repo-1"})
		if err == nil {
			t.Fatal("expected error when GetRepo fails")
		}
	})
}
