package usecase

import (
	"context"
	"testing"
)

func TestScanSetupScriptImports_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing worktree id is rejected", func(t *testing.T) {
		uc := NewScanSetupScriptImports(&fakeConnectionResolver{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), ScanSetupScriptImportsInput{})
		if err == nil {
			t.Fatal("expected error for missing worktree_id")
		}
	})

	t.Run("connected worktree dispatches to relay executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, RepoPath: "/srv/repo"}}
		relay := &fakeGitExecutor{setupScriptImportPaths: []string{"source ./lib.sh"}}
		local := &fakeGitExecutor{}
		uc := NewScanSetupScriptImports(resolver, local, relay)

		got, err := uc.Execute(context.Background(), ScanSetupScriptImportsInput{WorktreeID: "wt1"})
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

	t.Run("not connected dispatches to local executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/local/repo"}}
		local := &fakeGitExecutor{}
		relay := &fakeGitExecutor{}
		uc := NewScanSetupScriptImports(resolver, local, relay)

		_, err := uc.Execute(context.Background(), ScanSetupScriptImportsInput{WorktreeID: "wt1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledScanSetupScriptImports || relay.calledScanSetupScriptImports {
			t.Error("expected local executor to be called, not relay")
		}
	})
}
