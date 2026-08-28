# TASK-PRF-02-01: Add `agent.approvedModels` fallback to `ResolveProfile`

**From Solution:** SOL-PRF-02
**Priority:** P0
**Service:** `tenant-service`
**File:** `backend-go/services/tenant-service/internal/domain/profile_resolution.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`ResolveProfile` merges every field except two Merge-Rules-table rows:
`agent.approvedModels` (company-only, no lower layer can expand it) and
`fleet.allowedServerTags` (intersect, narrowing only — TASK-PRF-02-02). This
task adds the first: if company restricts approved models and the resolved
(post-merge) `agent.preferredModel` isn't in that list, force it back to
`approvedModels[0]` and record why.

## Changes to make

In `backend-go/services/tenant-service/internal/domain/profile_resolution.go`,
extend the special-cased-keys `const` block:

```go
const (
	securityKey      = "security"
	shellKey         = "shell"
	pathAdditionsKey = "pathAdditions"
	mcpKey           = "mcp"
	serversKey       = "servers"
	nameKey          = "name"

	agentKey          = "agent"          // NEW
	preferredModelKey = "preferredModel" // NEW
	approvedModelsKey = "approvedModels" // NEW
	modelFallbackKey  = "_modelFallbackReason" // NEW
)
```

Add the call in `ResolveProfile`, after the existing three merge-correction
calls:

```go
	lockSecurity(resolved, sources, company)
	mergePathAdditions(resolved, sources, layers)
	mergeMCPServers(resolved, sources, layers)
	applyApprovedModelsFallback(resolved, sources, company) // NEW
```

Add the function itself:

```go
// applyApprovedModelsFallback enforces BL-PRF-02 step 7 / Merge Rules
// table's "agent.approvedModels: Company only — user/dept cannot expand
// list" rule: if company defines a non-empty approvedModels list and the
// resolved (post-merge, so possibly dept/team/user-overridden)
// agent.preferredModel isn't in it, force it back to approvedModels[0] and
// record why in agent._modelFallbackReason.
func applyApprovedModelsFallback(resolved Settings, sources map[string]string, company Settings) {
	companyAgent, ok := asMap(emptySettings(company)[agentKey])
	if !ok {
		return
	}
	rawModels, ok := companyAgent[approvedModelsKey].([]any)
	if !ok || len(rawModels) == 0 {
		return // no restriction configured — nothing to enforce
	}
	approved := make([]string, 0, len(rawModels))
	allowed := map[string]bool{}
	for _, m := range rawModels {
		if name, ok := m.(string); ok && name != "" {
			approved = append(approved, name)
			allowed[name] = true
		}
	}
	if len(approved) == 0 {
		return
	}

	resolvedAgent, ok := asMap(resolved[agentKey])
	if !ok {
		return // no agent section at all in the resolved profile — nothing to fall back
	}
	preferred, _ := resolvedAgent[preferredModelKey].(string)
	if preferred == "" || allowed[preferred] {
		return // unset, or already approved — no fallback needed
	}

	resolvedAgent[preferredModelKey] = approved[0]
	resolvedAgent[modelFallbackKey] = fmt.Sprintf("%q not in approved list", preferred)
	resolved[agentKey] = Settings(resolvedAgent)
	sources[agentKey+"."+preferredModelKey] = SourceCompany
}
```

Add `"fmt"` to the file's import block (currently `"sort"`, `"strings"`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/tenant-service/...
go test ./services/tenant-service/internal/domain/... -run ResolveProfile -v
```

Add test cases to `profile_resolution_test.go` per SOL-PRF-02's Test plan:
preferred model in company's list -> unchanged, no `_modelFallbackReason`,
`_sources` unchanged; preferred model not in list -> forced to
`approvedModels[0]`, `_modelFallbackReason` set, `_sources` overwritten to
`"company"`; `company.approvedModels` absent/empty -> no fallback regardless
of `preferredModel`'s value; `resolved.agent` section entirely absent -> no
panic, no fallback.
