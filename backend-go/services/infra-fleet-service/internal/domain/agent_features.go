package domain

const (
	FeatureAgentExecPrompt   = "agent.execPrompt"
	FeatureReadonly          = "agent.execPrompt.readonly"
	FeatureWorkspaceKind     = "agent.execPrompt.workspaceKind"
	FeatureChanges           = "agent.execPrompt.changes"
	FeatureResultBlock       = "agent.execPrompt.resultBlock"
	FeatureCapabilities      = "agent.capabilities"
	FeatureAIComplete        = "ai.complete"
	FeatureAICompleteUsage   = "ai.complete.usage"
)

// SanitizeAgentFeatures bỏ chuỗi rỗng, quá dài, trùng, không phải ASCII in được, giới hạn 64 phần tử.
func SanitizeAgentFeatures(in []string) []string {
	var result []string
	seen := make(map[string]struct{})
	for _, f := range in {
		if f == "" || len(f) > 64 {
			continue
		}
		// check ascii
		ascii := true
		for _, c := range f {
			if c < 32 || c > 126 {
				ascii = false
				break
			}
		}
		if !ascii {
			continue
		}
		if _, ok := seen[f]; !ok {
			seen[f] = struct{}{}
			result = append(result, f)
			if len(result) >= 64 {
				break
			}
		}
	}
	return result
}
