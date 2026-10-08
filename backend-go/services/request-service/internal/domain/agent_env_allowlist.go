package domain

// AgentEnvAllowlist is the only environment an agent run started by the Request flow receives. Provider
// keys live on the dev server; no Orca token or credential reference may ride along.
var AgentEnvAllowlist = []string{"ORCA_REQUEST_ID", "ORCA_PROJECT_ID"}

// BuildAgentEnv is the one constructor of that environment; callers cannot pass extra keys.
func BuildAgentEnv(requestID, projectID string) map[string]string {
	return map[string]string{"ORCA_REQUEST_ID": requestID, "ORCA_PROJECT_ID": projectID}
}

// EnvWithinAllowlist reports whether env holds only allowed keys (guard for relay call sites and tests).
func EnvWithinAllowlist(env map[string]string) bool {
	for k := range env {
		ok := false
		for _, a := range AgentEnvAllowlist {
			ok = ok || a == k
		}
		if !ok {
			return false
		}
	}
	return true
}
