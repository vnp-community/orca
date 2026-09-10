package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestResolveAgentBinary_SpecFidelity(t *testing.T) {
	for model, want := range AgentBinaryMap {
		if got := ResolveAgentBinary(model); got != want {
			t.Errorf("ResolveAgentBinary(%q) = %q, want %q", model, got, want)
		}
	}
	if got := ResolveAgentBinary("unknown-model"); got != "claude" {
		t.Errorf("ResolveAgentBinary(unknown) = %q, want claude", got)
	}
	if got := ResolveAgentBinary(""); got != "claude" {
		t.Errorf("ResolveAgentBinary(empty) = %q, want claude", got)
	}
}

func TestBuildAgentArgs_SpecFidelity(t *testing.T) {
	for preset, want := range TrustPresetArgs {
		got := BuildAgentArgs(preset)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("BuildAgentArgs(%q) = %v, want %v", preset, got, want)
		}
	}
	if got := BuildAgentArgs("unknown-preset"); !reflect.DeepEqual(got, TrustPresetArgs["standard"]) {
		t.Errorf("BuildAgentArgs(unknown) = %v, want standard preset", got)
	}
	if got := BuildAgentArgs(""); !reflect.DeepEqual(got, TrustPresetArgs["standard"]) {
		t.Errorf("BuildAgentArgs(empty) = %v, want standard preset", got)
	}
}

func TestBuildAgentEnv_MergesEnvVars(t *testing.T) {
	resolved := map[string]any{
		"shell": map[string]any{
			"envVars": map[string]any{"FOO": "bar", "BAZ": "qux"},
		},
	}
	env := BuildAgentEnv(resolved, "user-1", "proj-1", "My Project", "")
	if env["FOO"] != "bar" || env["BAZ"] != "qux" {
		t.Errorf("expected envVars merged, got %+v", env)
	}
}

func TestBuildAgentEnv_PathAdditionsJoinedWithExistingPath(t *testing.T) {
	resolved := map[string]any{
		"shell": map[string]any{
			"pathAdditions": []any{"/company/bin", "/dept/bin"},
		},
	}
	env := BuildAgentEnv(resolved, "user-1", "proj-1", "My Project", "/usr/bin:/bin")
	want := "/company/bin:/dept/bin:/usr/bin:/bin"
	if env["PATH"] != want {
		t.Errorf("PATH = %q, want %q", env["PATH"], want)
	}
}

func TestBuildAgentEnv_GHConfigDirKeyedByUserID(t *testing.T) {
	env1 := BuildAgentEnv(map[string]any{}, "user-1", "proj-1", "P", "")
	env2 := BuildAgentEnv(map[string]any{}, "user-2", "proj-1", "P", "")

	if env1["GH_CONFIG_DIR"] == env2["GH_CONFIG_DIR"] {
		t.Error("expected two different userIDs to produce two different GH_CONFIG_DIR paths")
	}
	if env1["GLAB_CONFIG_DIR"] == env2["GLAB_CONFIG_DIR"] {
		t.Error("expected two different userIDs to produce two different GLAB_CONFIG_DIR paths")
	}
	wantGH := "/home/dev/.config/gh/user-1"
	if env1["GH_CONFIG_DIR"] != wantGH {
		t.Errorf("GH_CONFIG_DIR = %q, want %q", env1["GH_CONFIG_DIR"], wantGH)
	}
}

func TestBuildAgentEnv_AnthropicModelOnlyWhenPreferredModelSet(t *testing.T) {
	withModel := BuildAgentEnv(map[string]any{
		"agent": map[string]any{"preferredModel": "claude-opus-4-5"},
	}, "u1", "p1", "P", "")
	if withModel["ANTHROPIC_MODEL"] != "claude-opus-4-5" {
		t.Errorf("expected ANTHROPIC_MODEL set, got %+v", withModel)
	}

	withoutModel := BuildAgentEnv(map[string]any{}, "u1", "p1", "P", "")
	if _, present := withoutModel["ANTHROPIC_MODEL"]; present {
		t.Errorf("expected no ANTHROPIC_MODEL key when agent.preferredModel is unset, got %+v", withoutModel)
	}
}

func TestBuildAgentEnv_SetsProjectIDAndName(t *testing.T) {
	env := BuildAgentEnv(map[string]any{}, "u1", "proj-42", "My Project", "")
	if env["ORCA_PROJECT_ID"] != "proj-42" || env["ORCA_PROJECT_NAME"] != "My Project" {
		t.Errorf("unexpected ORCA_PROJECT_* fields: %+v", env)
	}
}

func TestBuildProjectContext_ExactStringMatch(t *testing.T) {
	got := BuildProjectContext(PreambleInput{
		ProjectName: "Orca", Description: "Terminal app", RepoURL: "git@example.com:orca.git",
		WorktreePath: "/srv/worktrees/orca-1", Branch: "main", DevServerHostname: "dev1.example.com",
		UserName: "Jane Doe", UserEmail: "jane@example.com", DepartmentName: "Engineering",
	})
	want := "# Orca Project Context\n" +
		"Project: Orca\n" +
		"Description: Terminal app\n" +
		"Repository: git@example.com:orca.git\n" +
		"Working directory: /srv/worktrees/orca-1\n" +
		"Branch: main\n" +
		"Dev Server: dev1.example.com\n" +
		"Developer: Jane Doe (jane@example.com)\n" +
		"Team: Engineering\n" +
		""
	if got != want {
		t.Errorf("BuildProjectContext mismatch:\n got:  %q\nwant: %q", got, want)
	}
}

func TestBuildProjectContext_EmptyDepartmentNameIsNoTeam(t *testing.T) {
	got := BuildProjectContext(PreambleInput{ProjectName: "P"})
	if !strings.Contains(got, "Team: No team") {
		t.Errorf("expected 'Team: No team' in output, got %q", got)
	}
}
