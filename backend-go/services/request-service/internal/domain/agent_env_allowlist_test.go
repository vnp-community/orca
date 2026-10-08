package domain

import "testing"

func TestBuildAgentEnv_OnlyTheAllowedKeys(t *testing.T) {
	env := BuildAgentEnv("r1", "p1")
	if len(env) != 2 || env["ORCA_REQUEST_ID"] != "r1" || env["ORCA_PROJECT_ID"] != "p1" || !EnvWithinAllowlist(env) {
		t.Fatalf("%v", env)
	}
	for _, bad := range []string{"ANTHROPIC_API_KEY", "ORCA_TOKEN", "credential_ref", "resolvedApiKey"} {
		if EnvWithinAllowlist(map[string]string{bad: "x"}) {
			t.Errorf("%s must be refused", bad)
		}
	}
}
