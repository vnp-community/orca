package domain

import (
	"reflect"
	"testing"
)

func TestResolveProfile_CompanyOnly_NilDepartmentEmptyTeamsEmptyUser(t *testing.T) {
	company := Settings{
		"agent": Settings{"model": "sonnet"},
	}

	got, err := ResolveProfile(company, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := Settings{"agent": Settings{"model": "sonnet"}}
	assertSettingsEqual(t, got.Settings, want)

	if got.Sources["agent.model"] != SourceCompany {
		t.Errorf("expected agent.model source=company, got %q", got.Sources["agent.model"])
	}
}

func TestResolveProfile_DepartmentOverridesSomeCompanyFieldsButNotAll(t *testing.T) {
	company := Settings{
		"agent": Settings{"model": "sonnet", "maxTokens": float64(4000)},
	}
	department := Settings{
		"agent": Settings{"model": "opus"},
	}

	got, err := ResolveProfile(company, department, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent, ok := got.Settings["agent"].(Settings)
	if !ok {
		t.Fatalf("expected agent section to be a Settings map, got %T", got.Settings["agent"])
	}
	if agent["model"] != "opus" {
		t.Errorf("expected department to override agent.model, got %v", agent["model"])
	}
	if agent["maxTokens"] != float64(4000) {
		t.Errorf("expected company's agent.maxTokens to survive (sibling field not redefined by department), got %v", agent["maxTokens"])
	}
	if got.Sources["agent.model"] != SourceDepartment {
		t.Errorf("expected agent.model source=department, got %q", got.Sources["agent.model"])
	}
	if got.Sources["agent.maxTokens"] != SourceCompany {
		t.Errorf("expected agent.maxTokens source=company, got %q", got.Sources["agent.maxTokens"])
	}
}

func TestResolveProfile_UserOverridesEverythingBelowIt(t *testing.T) {
	company := Settings{"editor": Settings{"theme": "dark"}}
	department := Settings{"editor": Settings{"theme": "light"}}
	teams := []TeamSettingsLayer{
		{TeamID: "t1", Priority: 0, Settings: Settings{"editor": Settings{"theme": "solarized"}}},
	}
	user := Settings{"editor": Settings{"theme": "high-contrast"}}

	got, err := ResolveProfile(company, department, teams, user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	editor := got.Settings["editor"].(Settings)
	if editor["theme"] != "high-contrast" {
		t.Errorf("expected user layer to win, got %v", editor["theme"])
	}
	if got.Sources["editor.theme"] != SourceUser {
		t.Errorf("expected editor.theme source=user, got %q", got.Sources["editor.theme"])
	}
}

func TestResolveProfile_MultipleTeams_HigherPriorityWinsConflict(t *testing.T) {
	company := Settings{}
	teams := []TeamSettingsLayer{
		{TeamID: "low", Priority: 1, Settings: Settings{"agent": Settings{"model": "low-priority-model"}}},
		{TeamID: "high", Priority: 10, Settings: Settings{"agent": Settings{"model": "high-priority-model"}}},
	}

	got, err := ResolveProfile(company, nil, teams, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent := got.Settings["agent"].(Settings)
	if agent["model"] != "high-priority-model" {
		t.Errorf("expected higher-priority team to win, got %v", agent["model"])
	}
	if got.Sources["agent.model"] != TeamSource("high") {
		t.Errorf("expected source=team:high, got %q", got.Sources["agent.model"])
	}
}

func TestResolveProfile_TeamPriorityTie_BrokenByTeamID(t *testing.T) {
	company := Settings{}
	teams := []TeamSettingsLayer{
		{TeamID: "team-b", Priority: 5, Settings: Settings{"agent": Settings{"model": "from-b"}}},
		{TeamID: "team-a", Priority: 5, Settings: Settings{"agent": Settings{"model": "from-a"}}},
	}

	got, err := ResolveProfile(company, nil, teams, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// team-a < team-b lexically, so team-a is treated as lower priority on
	// a tie and team-b (applied later) wins — deterministic regardless of
	// input slice order.
	agent := got.Settings["agent"].(Settings)
	if agent["model"] != "from-b" {
		t.Errorf("expected team-b (later lexically) to win the tie, got %v", agent["model"])
	}
}

func TestResolveProfile_SecurityIsCompanyLockedAgainstEveryOtherLayer(t *testing.T) {
	company := Settings{"security": Settings{"allowedMCPServers": []any{"filesystem"}}}
	department := Settings{"security": Settings{"allowedMCPServers": []any{"filesystem", "shell"}}}
	teams := []TeamSettingsLayer{
		{TeamID: "t1", Priority: 100, Settings: Settings{"security": Settings{"forcedAgentPolicy": "unrestricted"}}},
	}
	user := Settings{"security": Settings{"allowedMCPServers": []any{"anything"}}}

	got, err := ResolveProfile(company, department, teams, user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	security := got.Settings["security"].(Settings)
	if !reflect.DeepEqual(security["allowedMCPServers"], []any{"filesystem"}) {
		t.Errorf("expected security.allowedMCPServers to remain company's value only, got %v", security["allowedMCPServers"])
	}
	if _, leaked := security["forcedAgentPolicy"]; leaked {
		t.Error("expected team's security.forcedAgentPolicy to never appear in the resolved profile")
	}
	if got.Sources["security.allowedMCPServers"] != SourceCompany {
		t.Errorf("expected security.allowedMCPServers source=company, got %q", got.Sources["security.allowedMCPServers"])
	}
}

func TestResolveProfile_SecurityAbsentFromCompany_ResolvesToNoSecuritySection(t *testing.T) {
	department := Settings{"security": Settings{"allowedMCPServers": []any{"filesystem"}}}

	got, err := ResolveProfile(Settings{}, department, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := got.Settings["security"]; ok {
		t.Errorf("expected no security section when company doesn't define one, got %v", got.Settings["security"])
	}
}

func TestResolveProfile_ShellPathAdditionsConcatenateCompanyFirstUserLast(t *testing.T) {
	company := Settings{"shell": Settings{"pathAdditions": []any{"/usr/local/company-bin"}}}
	department := Settings{"shell": Settings{"pathAdditions": []any{"/opt/dept-bin"}}}
	teams := []TeamSettingsLayer{
		{TeamID: "t1", Priority: 1, Settings: Settings{"shell": Settings{"pathAdditions": []any{"/opt/team-bin"}}}},
	}
	user := Settings{"shell": Settings{"pathAdditions": []any{"/home/user/bin"}}}

	got, err := ResolveProfile(company, department, teams, user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	shell := got.Settings["shell"].(Settings)
	want := []any{"/usr/local/company-bin", "/opt/dept-bin", "/opt/team-bin", "/home/user/bin"}
	if !reflect.DeepEqual(shell["pathAdditions"], want) {
		t.Errorf("expected concatenated pathAdditions %v, got %v", want, shell["pathAdditions"])
	}
}

func TestResolveProfile_ShellDefaultShellUsesHighestPriorityLayer(t *testing.T) {
	company := Settings{"shell": Settings{"defaultShell": "/bin/bash", "pathAdditions": []any{"/company-bin"}}}
	user := Settings{"shell": Settings{"defaultShell": "/bin/zsh"}}

	got, err := ResolveProfile(company, nil, nil, user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	shell := got.Settings["shell"].(Settings)
	if shell["defaultShell"] != "/bin/zsh" {
		t.Errorf("expected user's defaultShell to win, got %v", shell["defaultShell"])
	}
	// pathAdditions is additive, unaffected by defaultShell's per-key override.
	if !reflect.DeepEqual(shell["pathAdditions"], []any{"/company-bin"}) {
		t.Errorf("expected company's pathAdditions to survive, got %v", shell["pathAdditions"])
	}
}

func TestResolveProfile_MCPServersDedupedByName_HighestPriorityWins(t *testing.T) {
	company := Settings{"mcp": Settings{"servers": []any{
		Settings{"name": "filesystem", "config": "company-fs-config"},
		Settings{"name": "shared-only-in-company", "config": "keep-me"},
	}}}
	user := Settings{"mcp": Settings{"servers": []any{
		Settings{"name": "filesystem", "config": "user-fs-config"},
	}}}

	got, err := ResolveProfile(company, nil, nil, user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mcp := got.Settings["mcp"].(Settings)
	servers, ok := mcp["servers"].([]any)
	if !ok {
		t.Fatalf("expected mcp.servers to be []any, got %T", mcp["servers"])
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 deduplicated servers, got %d: %v", len(servers), servers)
	}
	// First-seen position (company's) is preserved for "filesystem", but its
	// config comes from the higher-priority user layer.
	first := servers[0].(map[string]any)
	if first["name"] != "filesystem" || first["config"] != "user-fs-config" {
		t.Errorf("expected filesystem entry at position 0 with user's config, got %v", first)
	}
	second := servers[1].(map[string]any)
	if second["name"] != "shared-only-in-company" || second["config"] != "keep-me" {
		t.Errorf("expected company-only entry preserved at position 1, got %v", second)
	}

	if got.Sources["mcp.servers.filesystem"] != SourceUser {
		t.Errorf("expected mcp.servers.filesystem source=user, got %q", got.Sources["mcp.servers.filesystem"])
	}
	if got.Sources["mcp.servers.shared-only-in-company"] != SourceCompany {
		t.Errorf("expected mcp.servers.shared-only-in-company source=company, got %q", got.Sources["mcp.servers.shared-only-in-company"])
	}
}

func TestResolveProfile_EmptyUserOverrides_DoesNotPanicOrChangeResult(t *testing.T) {
	company := Settings{"agent": Settings{"model": "sonnet"}}

	withNilUser, err := ResolveProfile(company, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	withEmptyUser, err := ResolveProfile(company, nil, nil, Settings{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertSettingsEqual(t, withNilUser.Settings, withEmptyUser.Settings)
}

func TestResolveProfile_NilDepartmentAndNoTeams_FallsThroughToCompanyAndUser(t *testing.T) {
	company := Settings{"agent": Settings{"model": "sonnet"}, "editor": Settings{"theme": "dark"}}
	user := Settings{"editor": Settings{"theme": "light"}}

	got, err := ResolveProfile(company, nil, nil, user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent := got.Settings["agent"].(Settings)
	if agent["model"] != "sonnet" {
		t.Errorf("expected company's agent.model to survive with no department/team layers, got %v", agent["model"])
	}
	editor := got.Settings["editor"].(Settings)
	if editor["theme"] != "light" {
		t.Errorf("expected user's editor.theme to win, got %v", editor["theme"])
	}
}

func assertSettingsEqual(t *testing.T, got, want Settings) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("settings mismatch:\n got:  %#v\n want: %#v", got, want)
	}
}
