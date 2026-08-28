# SOL-PRF-04: Profile-aware agent execution routing — resolve profile, build env/args/preamble, spawn with `agent.execPrompt`

**Resolves:** [BUG-PRF-04](../BUG-PRF-04-profile-aware-agent-execution-not-implemented.md)
**Service:** `workflow-service` + `task-service` (both own an agent-exec call site) + `project-service` (new `GetProjectContext` RPC) + `tenant-service` (consumer of its already-specified `GetResolvedProfile`, no change needed there beyond SOL-PRF-02)
**Affected files (proposed):**
- `backend-go/proto/orca/project/v1/project.proto` (new: `GetProjectContext` RPC + `ProjectContext` message — already sketched, unimplemented)
- `backend-go/services/project-service/internal/usecase/get_project_context.go` (new)
- `backend-go/services/project-service/internal/adapter/grpc/server.go` (edit: new handler)
- `backend-go/services/workflow-service/internal/domain/step.go` (edit: `AgentStepConfig` gains `UserID`)
- `backend-go/services/workflow-service/internal/domain/agent_environment.go` (new: pure env/args/preamble builder)
- `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go` (edit: resolve profile + project context, build env, fix method name)
- `backend-go/services/workflow-service/internal/adapter/infrafleetclient/profile_resolver.go`, `project_context_resolver.go` (new)
- `backend-go/services/workflow-service/internal/usecase/ports.go` (extend: `ProfileResolver`, `ProjectContextResolver`)
- `backend-go/services/task-service/internal/domain/agent_environment.go` (new: same pure builder, task-service's own copy)
- `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go` (edit: resolve profile + project context, build env)
- `backend-go/services/task-service/internal/adapter/grpcclient/profile_resolver.go`, `project_context_resolver.go` (new)
- `backend-go/services/task-service/internal/usecase/ports.go` (extend: `ProfileResolver`, `ProjectContextResolver`)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

This is the highest-leverage gap in the profile domain: every other bug in
this batch (BUG-PRF-01/02/03) exists to produce a correct
`ResolvedProfile`, but per BUG-PRF-04's own finding, **nothing downstream
ever asks for one** — `grep -rln "GetResolvedProfile\|tenantv1\."
backend-go/` outside `tenant-service`/`api-gateway` shows zero real
callers. Fixing PRF-01/02/03 alone changes nothing a user can observe
without this fix too.

### Where the eight spec steps land, per-service

`docs/logic/profile/BL-PRF-04-profile-aware-agent-execution.md`'s 8 steps
map onto TDD-assigned service boundaries with no ambiguity:

| Step | Owner (per TDD) | Status |
|---|---|---|
| 1. Load Project | `project-service.GetProjectContext` | **Sketched, not implemented** — `project-service.md` §2's "Boundary decision" names this exact RPC: "`project-service` exposes a read-only `GetProjectContext` (project + membership + dev-server pointer — TS's `ProjectContext` shape)... Callers do a two-step saga: resolve context here, then call the execution-owning service." Confirmed absent from `project.proto` today. |
| 2. Check Server Availability | `infra-fleet-service.GetFleetHealth` | Exists (per SOL-PRF-03's citation), unused by this path |
| 3. Resolve Effective Profile | `tenant-service.GetResolvedProfile` | Exists (`tenant-service.md` §3/§7 — "`task-service`/`workflow-service` call `GetResolvedProfile` before dispatching an agent-spawn step, building the same shell/MCP/editor/agent environment `ProfileAwareAgentSpawner` builds today"), **zero real callers** |
| 4. Build Worktree | `git-gateway-service.CreateWorktree` | Exists, out of this bug's scope (BUG-PRF-04's own "What backend-go has" note) |
| 5-7. Build env/args, spawn | `workflow-service`/`task-service`'s own agent-exec adapters, relaying to the Dev Server Agent via `infra-fleet-service.Relay` | **This solution's core scope** |
| 8. Stream PTY | Existing relay/WS infrastructure | Out of scope, unaffected |

`tenant-service.md` §7 is the load-bearing citation that this dependency
edge (`workflow-service`/`task-service` → `tenant-service`) is not a new
architectural decision this solution invents — it's already asserted as
fact in `tenant-service`'s own TDD, just not built. **Flagged
inconsistency**: `02-microservices-decomposition.md`'s dependency-graph
diagram (`:110-166`) shows `task --> tenant` but omits `wf --> tenant` —
this solution's `workflow-service` change needs that edge added to the
diagram as a documentation fix, not a design decision requiring new
justification (the prose in `tenant-service.md` §7 already justifies it).

### The relay method is `agent.execPrompt`, not `agent.exec` — a pre-existing, already-diagnosed bug this solution fixes as a prerequisite

BUG-PRF-04's own "What's missing" section flags: "the relay method name
itself is flagged unverified... even the bare passthrough this bug credits
as 'existing' isn't proven to work end-to-end yet," citing
`agent_step_executor.go:14-25`'s own doc comment. That doc comment is
stale — `task-service`'s `SimpleExecutor`
(`grpcclient/simple_executor.go:15-98`) already did the reconciliation work
this bug flags as outstanding, and its own extensive doc comment names the
answer precisely: `"agent.exec"` is a real but *different* RPC (generic
`{binary,args,cwd,stdin,env,timeoutMs}` process-exec, no prompt/model/
trustPreset concept), and the real prompt-driven handler is
`"agent.execPrompt"` (`agent-print-mode-exec.ts`), accepting exactly the
fields BL-PRF-04's `pty.spawn` call needs: `prompt`, `worktreePath`,
`stepId`, `trustPreset`, `model`, `accountId`, **`env`**, `timeoutMs`. This
solution's env-injection design is only possible *because* `env` is
already a real, agent-side-supported field of the correct RPC — switching
`workflow-service`'s `AgentExecutor` from `"agent.exec"` to
`"agent.execPrompt"` (matching `task-service`'s already-correct choice) is
a **prerequisite fix**, not an optional cleanup: sending `env`/`model` to
`"agent.exec"` would hit a handler with no concept of either field.

### One env/args/preamble builder, per-service, not a new shared service

`03-clean-architecture-guidelines.md`'s cross-service shared-code policy
(cited by `infra-fleet-service.md` §6 re: its own wire-protocol client) is
"a wire-protocol client is not a business-logic-free cross-cutting
concern" — the same reasoning applies here in reverse: `resolveAgentBinary`/
`buildAgentArgs`/`buildProjectContext`/`mergeProfileEnv` (BL-PRF-04's own
functions) *are* pure, small, stable, TS-ported domain logic with zero I/O
— exactly the shape `domain/` packages hold per
`03-clean-architecture-guidelines.md`. `workflow-service` and
`task-service` already each maintain their own parallel relay-dispatch
adapter for the identical conceptual operation (`agent_step_executor.go`
vs. `simple_executor.go` — see `simple_executor.go:1-98`'s own doc comment
acknowledging this parallelism explicitly), so a second small duplication
(one `agent_environment.go` file per service, ~80 lines, no I/O) follows
the same precedent rather than inventing a third pattern (a new shared
service, or promoting business logic into `common/`, which
`infra-fleet-service.md` §6 explicitly reserves for cross-cutting,
non-business-logic code only). If this duplication becomes a real
maintenance burden, promoting just the pure `domain/agent_environment.go`
logic (not the adapter/relay-call layers) to a shared internal module is
the documented escape hatch — mirroring `infra-fleet-service.md` §6's own
"promote `wire/`... if duplication becomes a real burden" precedent.

---

## Design — `project-service`: `GetProjectContext` RPC

```protobuf
// project.proto (new)
message GetProjectContextRequest {
  string project_id = 1;
}
message ProjectContext {
  string project_id = 1;
  string project_name = 2;
  string description = 3;
  string repo_url = 4;          // from the project's primary Repo (position 0), if any
  string dev_server_id = 5;
  string dev_server_hostname = 6; // resolved via infra-fleet-service, best-effort — empty if unresolvable
}
rpc GetProjectContext(GetProjectContextRequest) returns (ProjectContext);
```

```go
// internal/usecase/get_project_context.go
type GetProjectContext struct {
	projects ProjectRepository
	repos    RepoRepository
	hosts    DevServerHostnameResolver // new, thin port -> infra-fleet-service.ListDevServers, resolves id -> host; empty on any failure (best-effort, never blocks the read)
}

func (uc *GetProjectContext) Execute(ctx context.Context, projectID string) (domain.ProjectContext, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil { /* ... */ }
	project, err := uc.projects.Get(ctx, tenantID, projectID)
	if err != nil { /* ErrProjectNotFound -> KindNotFound */ }

	repos, _ := uc.repos.ListRepos(ctx, projectID) // best-effort; a project with no repos yet has an empty RepoURL
	var repoURL string
	if len(repos) > 0 {
		repoURL = repos[0].URL
	}
	hostname, _ := uc.hosts.Hostname(ctx, tenantID, project.DevServerID) // "" on any failure — never fails the whole read over a display-only field

	return domain.ProjectContext{
		ProjectID: project.ID, ProjectName: project.Name, Description: project.Description,
		RepoURL: repoURL, DevServerID: project.DevServerID, DevServerHostname: hostname,
	}, nil
}
```

Access control: read-only, membership-gated the same as `GetProject`
(`projectActionAnyMember`, `authorization.go:26-27`) — an execution-dispatch
caller (workflow-service/task-service, acting on behalf of an
already-authenticated end user) must present that user's membership, not a
service-identity bypass; no new authorization branch needed, reuses the
existing `requireProjectAccess(ctx, ..., projectActionAnyMember)`.

---

## Design — domain: `agent_environment.go` (identical shape in both services)

```go
package domain

// AgentBinaryMap mirrors BL-PRF-04's resolveAgentBinary AGENT_MAP verbatim.
var AgentBinaryMap = map[string]string{
	"claude-opus-4-5":   "claude",
	"claude-sonnet-4-5": "claude",
	"codex":              "codex",
	"gemini":             "gemini",
	"opencode":           "opencode",
}

// ResolveAgentBinary implements BL-PRF-04's resolveAgentBinary — unknown/
// empty model falls back to "claude", matching the spec's `?? 'claude'`.
func ResolveAgentBinary(model string) string {
	if bin, ok := AgentBinaryMap[model]; ok {
		return bin
	}
	return "claude"
}

// TrustPresetArgs mirrors BL-PRF-04's buildAgentArgs TRUST_ARGS verbatim.
var TrustPresetArgs = map[string][]string{
	"minimal":    {"--trust", "minimal"},
	"standard":   {"--trust", "standard"},
	"permissive": {"--trust", "full", "--dangerously-skip-permissions"},
}

// BuildAgentArgs implements BL-PRF-04's buildAgentArgs — unknown/empty
// preset falls back to "standard".
func BuildAgentArgs(trustPreset string) []string {
	if args, ok := TrustPresetArgs[trustPreset]; ok {
		return args
	}
	return TrustPresetArgs["standard"]
}

// AgentEnv is the profile-derived environment BuildAgentEnv produces —
// serialized into agent.execPrompt's `env` field (a map[string]string on
// the wire, per agent-print-mode-exec.ts's handled params).
type AgentEnv map[string]string

// BuildAgentEnv implements BL-PRF-04 step 5's agentEnv construction.
// resolved is tenant-service's ResolvedProfile.Settings (already-decoded
// generic JSON map, same shape ResolveProfile itself operates on —
// profile_resolution.go's domain.Settings) — this function reads
// shell.envVars/shell.pathAdditions/agent.preferredModel out of it the
// same way ResolveProfile's own merge helpers do, not a separate decode.
func BuildAgentEnv(resolved map[string]any, userID, projectID, projectName, existingPath string) AgentEnv {
	env := AgentEnv{}

	if shell, ok := resolved["shell"].(map[string]any); ok {
		if vars, ok := shell["envVars"].(map[string]any); ok {
			for k, v := range vars {
				if s, ok := v.(string); ok {
					env[k] = s
				}
			}
		}
		if adds, ok := shell["pathAdditions"].([]any); ok && len(adds) > 0 {
			parts := make([]string, 0, len(adds)+1)
			for _, a := range adds {
				if s, ok := a.(string); ok {
					parts = append(parts, s)
				}
			}
			parts = append(parts, existingPath)
			env["PATH"] = strings.Join(parts, ":") // ":" only — Windows dev-server-agent hosts are out of scope for THIS join (the agent host's own shell owns PATH separator conventions; see Cross-Platform note below)
		}
	}

	// Per-user credential isolation — BL-PRF-04 step 5's GH_CONFIG_DIR/
	// GLAB_CONFIG_DIR, keyed by userID so two users' agent sessions on the
	// same dev server never share gh/glab auth state.
	env["GH_CONFIG_DIR"] = path.Join("/home/dev/.config/gh", userID)
	env["GLAB_CONFIG_DIR"] = path.Join("/home/dev/.config/glab-cli", userID)

	if agent, ok := resolved["agent"].(map[string]any); ok {
		if model, ok := agent["preferredModel"].(string); ok && model != "" {
			env["ANTHROPIC_MODEL"] = model
		}
	}
	env["ORCA_PROJECT_ID"] = projectID
	env["ORCA_PROJECT_NAME"] = projectName
	return env
}

// ProjectContext mirrors BL-PRF-04's buildProjectContext inputs — a subset
// of project-service's own ProjectContext plus worktree/user fields it
// doesn't have (worktree.path/branch come from git-gateway-service's
// CreateWorktree result, user.name/email/departmentName from context the
// caller already carries or a lightweight lookup — see this file's Test
// plan for what a real implementation still needs to source these from).
type PreambleInput struct {
	ProjectName, Description, RepoURL, DevServerHostname string
	WorktreePath, Branch                                 string
	UserName, UserEmail, DepartmentName                  string
}

// BuildProjectContext implements BL-PRF-04's buildProjectContext verbatim,
// including its exact field order and blank trailing line.
func BuildProjectContext(in PreambleInput) string {
	team := in.DepartmentName
	if team == "" {
		team = "No team"
	}
	lines := []string{
		"# Orca Project Context",
		"Project: " + in.ProjectName,
		"Description: " + in.Description,
		"Repository: " + in.RepoURL,
		"Working directory: " + in.WorktreePath,
		"Branch: " + in.Branch,
		"Dev Server: " + in.DevServerHostname,
		fmt.Sprintf("Developer: %s (%s)", in.UserName, in.UserEmail),
		"Team: " + team,
		"",
	}
	return strings.Join(lines, "\n")
}
```

**Cross-Platform note** (per this repo's `AGENTS.md`): `BuildAgentEnv`'s
`PATH` join uses `:` unconditionally because the target is the **Dev
Server Agent's host shell environment**, not this Go service's own OS —
Orca's cross-platform rule ("never assume `/` or `\`... use path.join or
Electron/Node path utilities") governs paths *this process* touches
locally; the PATH string here is opaque data handed to a remote shell
whose separator convention this service cannot introspect at env-build
time. Flagged explicitly as a known limitation if Windows dev-server-agent
hosts are ever in scope (today, `agent/`'s Dev Server Agent targets
Linux/macOS hosts only, per `02-microservices-decomposition.md`'s "Dev
Server Agent" framing) — not silently assumed away.

**`_modelFallbackReason` interaction**: if
[SOL-PRF-02](./SOL-PRF-02-approvedmodels-servertags-merge.md) is
implemented, `resolved["agent"]["preferredModel"]` may already be the
company-forced fallback model, not the user's raw preference — `BuildAgentEnv`
reads whatever `ResolveProfile` put there, so the fallback is transparently
respected with zero additional logic here; `ANTHROPIC_MODEL` always reflects
the *enforced* model, never a bypassed one.

---

## Design — `workflow-service`: `AgentExecutor` (edit)

```go
// internal/domain/step.go (edit)
type AgentStepConfig struct {
	ConnectionID string `json:"connectionId"`
	Prompt       string `json:"prompt"`
	WorktreePath string `json:"worktreePath,omitempty"`
	TrustPreset  string `json:"trustPreset,omitempty"`
	UserID       string `json:"userId,omitempty"` // NEW — whose profile to resolve; empty = legacy behavior, see below
	ProjectID    string `json:"projectId,omitempty"` // NEW — for GetProjectContext + ORCA_PROJECT_*
}
```

```go
// internal/adapter/infrafleetclient/agent_step_executor.go (edit)

// agentExecPromptMethod replaces the old, wrong agentExecMethod = "agent.exec"
// — see this file's Design rationale section for the full citation trail;
// this fixes BUG-PRF-04's own flagged "even the bare passthrough isn't
// proven to work" concern as a byproduct of adding env injection.
const agentExecPromptMethod = "agent.execPrompt"

type agentExecPromptParams struct {
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath"`
	TrustPreset  string            `json:"trustPreset,omitempty"`
	Model        string            `json:"model,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	InitFile     string            `json:"initFile,omitempty"` // project-context preamble, per BL-PRF-04 step 6's "Pass system preamble via stdin/file"
}

type AgentExecutor struct {
	client   infrafleetv1.InfraFleetServiceClient
	profiles usecase.ProfileResolver        // new
	projects usecase.ProjectContextResolver // new
}

func (e *AgentExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg domain.AgentStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil { /* unchanged */ }

	params := agentExecPromptParams{Prompt: cfg.Prompt, WorktreePath: cfg.WorktreePath, TrustPreset: cfg.TrustPreset}

	// Profile-aware path only when UserID is present — a step authored
	// before this migration (no UserID in its config JSON) degrades to
	// today's bare passthrough rather than failing outright. New steps
	// (created via a workflow-template edit) always carry UserID; this is
	// an expand/contract-style compatibility shim, not a permanent branch.
	if cfg.UserID != "" {
		resolved, err := e.profiles.GetResolvedProfile(ctx, cfg.UserID)
		if err != nil {
			return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: resolve profile: %w", err)
		}
		model, _ := resolved["agent"].(map[string]any)["preferredModel"].(string)
		params.Model = domain.ResolveAgentBinary(model) // NOTE: ResolveAgentBinary maps model->binary; agent.execPrompt's `model` param wants the raw model NAME (agent-print-mode-exec.ts's own model field, not a binary), so this line uses the raw `model` string directly — ResolveAgentBinary is NOT called here; see the corrected line below.
		params.Model = model // corrected: agent.execPrompt's model field is the model name; ResolveAgentBinary would only matter if this RPC took a binary path, which it doesn't (agent-print-mode-exec.ts resolves the binary server-side from the model name itself)
		params.TrustPreset = cfg.TrustPreset // already a raw string agent.execPrompt interprets itself (only "full" is special-cased server-side, per simple_executor.go's doc comment) — BuildAgentArgs/TrustPresetArgs is NOT sent over this RPC (it has no `args` field), it exists for a future pty.spawn-shaped RPC BL-PRF-04 sketches but agent.execPrompt doesn't expose; flagged as an intentional gap, not an oversight — see Design rationale.

		existingPath := "" // this service has no visibility into the target host's existing PATH — env.PATH is additive-only from pathAdditions with an empty existing suffix, degrading gracefully (still correct: pathAdditions still gets prepended into the agent's env, just without the host's real PATH appended — the agent binary's own shell init still runs and can supply its own PATH transitively, see Test plan's flagged assumption to verify against a live agent)
		env := domain.BuildAgentEnv(resolved, cfg.UserID, cfg.ProjectID, "", existingPath)
		params.Env = env

		if cfg.ProjectID != "" {
			pctx, err := e.projects.GetProjectContext(ctx, cfg.ProjectID)
			if err == nil { // best-effort — a preamble-build failure must never block the agent spawn itself
				env["ORCA_PROJECT_NAME"] = pctx.ProjectName
				params.InitFile = domain.BuildProjectContext(domain.PreambleInput{
					ProjectName: pctx.ProjectName, Description: pctx.Description, RepoURL: pctx.RepoURL,
					WorktreePath: cfg.WorktreePath, DevServerHostname: pctx.DevServerHostname,
				})
			}
		}
	}

	var result execResult
	if err := relay(ctx, e.client, cfg.ConnectionID, agentExecPromptMethod, params, &result); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: %w", err)
	}
	return toStepResult(result)
}
```

The `params.Model = domain.ResolveAgentBinary(model)` / `params.Model =
model` double-assignment above is deliberately left **visible, not
cleaned up**, in this sketch: it documents a real subtlety a careless
implementation would get wrong — `agent.execPrompt`'s `model` param (per
`simple_executor.go:44-45`'s citation of `agent-print-mode-exec.ts:45`,
"defaults to `'claude'` when absent... only `'claude'`-prefixed models are
supported for one-shot exec today") wants the **model name**, and the
agent-side handler resolves *its own* binary from that name — `ResolveAgentBinary`/
`AgentBinaryMap` in this solution's `domain/agent_environment.go` exist for
completeness against BL-PRF-04's spec text but are **not** actually wired
into any RPC call this solution makes, because `agent.execPrompt` has no
`cmd`/binary-path field to put one in (unlike the spec's own `pty.spawn`
sketch, which does: `cmd: resolveAgentBinary(...)`). The real
implementation should delete the first assignment and its comment; kept
here only so this citation trail survives into the code review.

`TrustPresetArgs`/`BuildAgentArgs` are similarly built for spec fidelity
but **not sent** — `agent.execPrompt` takes a bare `trustPreset` string and
interprets it server-side (only `"full"` does anything, per
`simple_executor.go`'s own doc comment quoting `agent-print-mode-exec.ts:97-99`),
not a CLI-args array. Both functions are still worth keeping in
`domain/agent_environment.go`: they're exercised directly by this
solution's Test plan (spec-fidelity unit tests) and become live the day
`infra-fleet-service`/the Dev Server Agent exposes a richer spawn RPC that
does take an args array — documented, inert-for-now code, not dead code to
delete.

`workflow-service.md`'s `ProfileResolver`/`ProjectContextResolver` ports
(new, `internal/usecase/ports.go`):

```go
type ProfileResolver interface {
	GetResolvedProfile(ctx context.Context, userID string) (map[string]any, error)
}
type ProjectContextResolver interface {
	GetProjectContext(ctx context.Context, projectID string) (ProjectContext, error)
}
```

implemented in `internal/adapter/infrafleetclient/profile_resolver.go`
(dials `tenantv1.TenantServiceClient`, a new client `cmd/server/main.go`
must construct) and `project_context_resolver.go` (dials
`projectv1.ProjectServiceClient`, also new).

---

## Design — `task-service`: `SimpleExecutor` (edit)

Same shape, applied to the already-correct-method-name call site.
`agentExecPromptParams` (`simple_executor.go:116-120`) gains `TrustPreset`,
`Model`, `Env` fields (currently only `Prompt`/`WorktreePath`/`StepID`);
`SimpleExecutor.Execute` resolves the task's assignee profile
(`task.AssigneeID`, if `task-service`'s domain carries one — flagged as an
assumption to confirm against `task-service`'s actual `domain.Task` shape;
if tasks have no per-task assignee, this falls back to `task-service`'s
existing `TeamScopeResolver`-adjacent identity, i.e. whichever user
triggered task execution via the request context) and project context
(`task.ProjectID`, already available) the identical way, calling the same
new `ProfileResolver`/`ProjectContextResolver` ports (task-service's own
copies, same interface shape, separate implementations dialing the same
two service clients — `task-service.md`'s dependency table already lists
`task --> tenant`, so only the `project-service` client dial is new here).

---

## Design — server-unavailability / degraded handling (spec steps 2, "Server Unavailability")

Out of full scope for this solution (BUG-PRF-04's acceptance criteria list
this, but it's a UI-modal/retry concern owned by `api-gateway`/frontend
reacting to a `FAILED_PRECONDITION`/`UNAVAILABLE` status this solution's
callers should surface, not swallow) — **but** this solution's env-build
path must not itself mask a genuinely unreachable dev server as a
different failure. `AgentExecutor`/`SimpleExecutor` should call
`infra-fleet-service.GetFleetHealth` (SOL-PRF-03's already-designed
`DevServerHealthChecker` port, reused here rather than a third
implementation) before the relay call and return a distinguishable
`apperrors.KindUnavailable`/`KindFailedPrecondition` (degraded = proceed
with a warning logged, per spec's "degraded... Proceeding," unreachable =
fail) so `api-gateway`'s existing error-surfacing path can drive the
modal/toast BL-PRF-04 describes — the modal/toast UI itself is explicitly
out of this backend-go solution's scope.

---

## Test plan

- `internal/domain/agent_environment_test.go` (both services, near-
  identical test files): `ResolveAgentBinary`/`BuildAgentArgs` spec-fidelity
  table tests (every `AGENT_MAP`/`TRUST_ARGS` entry, plus the unknown-key
  fallback case) — kept even though not wired into the live RPC call today
  (see Design's "inert-for-now" note), so a future richer spawn RPC can
  reuse them with confidence they're correct. `BuildAgentEnv`: envVars
  merge correctly, pathAdditions joined with the existing PATH,
  `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` keyed by the right userID (two
  different userIDs produce two different paths — regression guard for
  per-user isolation), `ANTHROPIC_MODEL` set only when
  `agent.preferredModel` is non-empty. `BuildProjectContext`: exact string
  match against BL-PRF-04's example output (including the trailing blank
  line), `DepartmentName` empty → "No team".
- `agent_step_executor_test.go`: fake `ProfileResolver`/
  `ProjectContextResolver` — `cfg.UserID` empty → legacy passthrough
  (`agent.execPrompt` called with no `env`/`model`, exactly today's
  params modulo the method-name fix) — **not** the old broken
  `"agent.exec"` name; assert the relay call's method string is
  `"agent.execPrompt"` unconditionally, closing BUG-PRF-04's flagged
  unverified-method-name gap as a hard regression test, not just a fix.
  `cfg.UserID` set → `ProfileResolver` called with the right userID, `env`
  populated, `params.Model` equals the raw resolved model string (not a
  binary name — regression guard for the double-assignment subtlety flagged
  in the Design section). `ProjectContextResolver` failure → spawn still
  proceeds (best-effort `InitFile`), asserted via a fake that errors and
  checking the relay call still happens with `InitFile == ""`.
- `simple_executor_test.go`: same coverage, task-service's call site.
- `project-service`'s `get_project_context_test.go`: membership-gated
  (non-member denied via existing `projectActionAnyMember` OPA path),
  `RepoURL` empty when the project has zero repos (not an error),
  `DevServerHostname` empty when the hostname resolver errors (best-effort,
  never fails the whole RPC).
- Integration/contract gap flagged explicitly, not silently assumed:
  `agent.execPrompt`'s real, current handler
  (`agent-print-mode-exec.ts`) must be re-read in full against this
  solution's exact `agentExecPromptParams` field set (`env` in particular)
  before shipping — `simple_executor.go`'s own doc comment already did this
  verification for `prompt`/`worktreePath`/`stepId`/`trustPreset`/`model`/
  `accountId`/`env`/`timeoutMs`, so `env` support is asserted, not
  guessed, but this solution's specific env *key names*
  (`GH_CONFIG_DIR`/`ANTHROPIC_MODEL`/`ORCA_PROJECT_ID`/`ORCA_PROJECT_NAME`)
  reaching the spawned process as literal environment variables (versus,
  say, the agent CLI needing them under different names) is an assumption
  this solution inherits from BL-PRF-04's spec text, not independently
  re-verified against `agent/`'s actual env-passthrough code — flag as an
  open verification item for whoever implements this, same posture
  `git-gateway-service.md`'s "reference implementation" framing takes
  toward its own Known gaps.

## References

- `specs/backend-go/tdd/services/project-service.md:26-52` (`GetProjectContext`
  already sketched in the "Boundary decision" section, confirmed
  unimplemented), `:117-119` (RPC surface listing)
- `specs/backend-go/tdd/services/tenant-service.md:230-244` (§7 — "profile
  injection into agent-spawn environment... `task-service`/`workflow-service`
  call `GetResolvedProfile` before dispatching an agent-spawn step" — the
  load-bearing citation that this dependency edge is already TDD-asserted)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:110-166`
  (dependency graph — flagged missing `wf --> tenant` edge)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md`
  (domain/ purity rule `agent_environment.go` follows; cross-service
  shared-code policy grounding the per-service-duplication decision)
- `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go:1-67`
  (existing, wrong-method-name implementation this solution replaces)
- `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go:1-193`
  (the already-correct `agent.execPrompt` reconciliation this solution's
  `workflow-service` fix mirrors, and whose doc comment is the primary
  source for `agent.execPrompt`'s real param/result shape)
- `backend-go/services/workflow-service/internal/domain/step.go:54-87`
  (`AgentStepConfig`/`ShellStepConfig` — note `ShellStepConfig` already has
  an `Env map[string]string` field, the precedent this solution's
  `AgentStepConfig.Env`-equivalent, delivered via `agentExecPromptParams`
  rather than the step config itself, follows)
- `backend-go/services/task-service/internal/adapter/grpcclient/project_execution_resolver.go:1-51`
  (existing connectionId/worktreePath resolution this solution does NOT
  duplicate — `GetProjectContext` is additive, for name/description/repoUrl
  only, not a second connection resolver)
- `backend-go/services/infra-fleet-service.md` §6 (cross-service
  shared-code policy citation, `wire/`-promotion precedent)
- `docs/logic/profile/BL-PRF-04-profile-aware-agent-execution.md:1-168`
  (full 8-step flow, `resolveAgentBinary`/`buildAgentArgs`/
  `buildProjectContext` reference implementations this solution ports
  near-verbatim into `domain/agent_environment.go`)
- [SOL-PRF-02](./SOL-PRF-02-approvedmodels-servertags-merge.md) (the
  `_modelFallbackReason` interaction this solution's `ANTHROPIC_MODEL`
  assignment depends on being correct upstream)
- [SOL-PRF-03](./SOL-PRF-03-project-devserver-assignment.md) (the
  `DevServerHealthChecker` port this solution reuses for the
  server-unavailability gate rather than building a third implementation)

## `agent/` (Dev Server Agent) changes needed

**None required for this solution's core `env`/`model`/`trustPreset`
injection** — `agent.execPrompt`'s handler
(`agent-print-mode-exec.ts`, per `simple_executor.go`'s already-verified
citation) already accepts `env`/`model`/`trustPreset`/`stepId` and
presumably an `initFile`-equivalent field for the preamble (this
solution's `InitFile` param name is BL-PRF-04's own spec terminology —
**explicitly flagged as unverified against the real handler's actual field
name for the preamble/system-prompt input**, unlike `env`/`model`/
`trustPreset` which `simple_executor.go`'s doc comment already confirmed
field-by-field). Whoever implements this solution should re-read
`agent-print-mode-exec.ts` once more specifically for how (or whether) it
accepts a system-prompt/preamble string today — if it doesn't, that IS a
real `agent/` gap this solution would need to either close there or work
around (e.g. prepending the preamble into `prompt` itself instead of a
separate field) before step 6 of the spec flow can work end-to-end.
