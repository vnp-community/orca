# SOL-WF-02: Server/provider resolution, variable interpolation, `action`/`parallel` step types, and live execution streaming for `workflow-service`

**Resolves:** [BUG-WF-02](../BUG-WF-02-workflow-execution-partial.md)
**Service:** `workflow-service` (primary) + `infra-fleet-service` (schema/proto extension for fleet-tag resolution) + `api-gateway` (streaming wiring)
**Affected files (proposed):**
- `backend-go/proto/orca/workflow/v1/workflow.proto`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (DevServer.tags + tag-filtered list)
- `backend-go/services/infra-fleet-service/migrations/000N_dev_server_tags.{up,down}.sql`
- `backend-go/services/workflow-service/internal/domain/step.go`, `dag.go`, `interpolation.go` (new)
- `backend-go/services/workflow-service/internal/usecase/ports.go` (new `ServerResolver`, `ProviderResolver`, `EventPublisher` ports)
- `backend-go/services/workflow-service/internal/usecase/execute.go`, `wave_dispatcher.go`
- `backend-go/services/workflow-service/internal/adapter/serverresolver/` (new)
- `backend-go/services/workflow-service/internal/adapter/providerresolver/` (new)
- `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`, `shell_step_executor.go`, `notification_step_executor.go`, `action_step_executor.go` (new), `parallel_step_executor.go` (new)
- `backend-go/services/workflow-service/internal/adapter/eventstream/` (new, in-process pub/sub)
- `backend-go/services/workflow-service/internal/adapter/grpc/server.go` (`StreamExecutionEvents`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (new `registerWorkflowChannels`, `workflow.execution.subscribe`)
- Corresponding `_test.go` files
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

- `workflow-service.md` §2 draws the bounded-context line precisely where
  this bug's gaps sit: DAG build/dispatch is owned here, but
  "host/connection resolution is `infra-fleet-service`'s job" and "AI
  provider selection is `ai-provider-service`'s job (§7)." Both are
  currently **not called at all** (BUG-WF-02's finding) — this solution
  adds the calling code inside `workflow-service`, it does not move
  resolution logic into `workflow-service` itself, matching §2's boundary.
- §3.2 states outright: **"Every `agent`-type workflow step fails today"**
  because of TS Gap 4's param-building bug, and instructs that the Go
  `AgentStepExecutor` be "built from scratch against whatever
  `infra-fleet-service`'s real relay contract... actually is" — never a
  port of the buggy TS caller. This solution's step-executor changes
  (adding `Target`/`Provider` resolution before the relay call) sit
  directly on top of that existing executor design without altering its
  already-correct instruction (confirmed still true by reading
  `agent_step_executor.go:12-26`'s own doc comment, which flags the
  `agent.exec`-vs-`agent.execPrompt` reconciliation as a still-open,
  separately-tracked item — this solution does not attempt to resolve
  that method-name question, only the target/provider resolution steps
  that must happen before any relay call regardless of which method name
  is correct).
- §4's `StepExecutor` list ("Five implementations") is the exact
  `StepType` enum this bug finds incomplete (5 of 6: no `action`, no
  `parallel`) — §4 does not describe `action` or `parallel` in detail
  (their names appear nowhere in workflow-service.md itself), so their
  concrete shape below is a genuine extension, not a gap-fill against an
  already-specified design — flagged explicitly.
- §7's dependency diagram (`wf --> aiprov`, `wf --> infra`) already draws
  both edges this solution's `ServerResolver`/`ProviderResolver` calls
  use — no new edge in `02-microservices-decomposition.md`'s dependency
  graph is required for server/provider resolution. **A new edge IS
  required** for `fleet:tag:<tag>` resolution to work: `infra-fleet-service`
  needs a `tags` concept on `DevServer` that does not exist in its current
  proto (`infrafleet.proto:117-124` — `DevServer` has `id/tenant_id/host/
  mode/ssh_target_id` only, confirmed by direct read). That proto/schema
  extension is this solution's one genuine cross-service addition — flagged
  below, same discipline SOL-009 used for its `FilesystemExecutor` port
  addition to `git-gateway-service`.
- §7's `wf --> aiprov` priority note ("explicit
  `step.config.provider.accountId` pin... beats `ai-provider-service`'s
  priority-chain resolution (user > project > server)") is implemented
  almost verbatim below — `aiprovider.proto:69-79`'s `ResolveProvider`
  RPC already implements the user→project cascade
  (`ResolveProviderRequest{tenant_id, user_id, project_id}` — confirmed by
  direct read), so this solution's `ProviderResolver` adapter is a thin
  caller, not new cascade logic. The `> server` tier `workflow-service.md`
  §7 names is `ai-provider-service`'s own cascade to extend, not something
  `workflow-service` can add on `aiprovider-service`'s behalf — flagged as
  an out-of-scope dependency, not silently assumed away.
- §8's "Deadlines" NFR (explicit `context.WithTimeout` on every outbound
  call to `infra-fleet-service`/`ai-provider-service`, distinct from a
  step's own 30-minute deadline) governs every new outbound call this
  solution adds (`ServerResolver.Resolve`, `ProviderResolver.Resolve`) —
  designed with the same 5s intra-cluster default per
  `08-inter-service-communication.md`'s "Deadlines are mandatory... no
  unbounded gRPC call exists anywhere" rule.
- `08-inter-service-communication.md`'s three-channel table puts
  `StreamExecutionEvents`-shaped "live push to a UI" in a gap the document
  doesn't directly name (its WS section (`api-gateway` responsibilities,
  item 5) is about `infra-fleet-service`'s terminal streams and
  `notification-service`'s push events specifically) — this solution's
  live-streaming design (below) reasons from that same pattern (accept a
  WS connection, open a corresponding gRPC server-streaming call, pipe
  frames) rather than inventing a new transport, and flags the
  single-instance in-process pub/sub caveat explicitly as an open scaling
  question, mirroring how `08-inter-service-communication.md` itself
  leaves the agent-relay-protocol choice (Option A/B) as "a genuine open
  question flagged here rather than papered over."

---

## Design — server resolution (`ServerResolver`)

### Domain: `Target` replaces raw `ConnectionID` passthrough

```go
// internal/domain/step.go (extended)
//
// Target is a dispatch-target string in one of four shapes — the
// orchestrator (this service) resolves it to a concrete connectionId
// before relaying, closing BUG-WF-02's "no server resolution logic
// anywhere" finding:
//   "connection:<id>"   — direct passthrough, today's ConnectionID shape (back-compat)
//   "project:<id>"      — resolve via project-service.GetProject().dev_server_id, then infra-fleet-service.ResolveConnection(dev_server_id=...)
//   "server:<id>"       — resolve via infra-fleet-service.ResolveConnection(dev_server_id=<id>) directly
//   "fleet:tag:<tag>"   — load-balance across infra-fleet-service's healthy dev servers carrying <tag>
// ConnectionID is kept as a deprecated alias: when Target is empty and
// ConnectionID is set, it's treated as "connection:<ConnectionID>" —
// every step config persisted before this change keeps working unchanged.
type AgentStepConfig struct {
    Target       string `json:"target,omitempty"`
    ConnectionID string `json:"connectionId,omitempty"` // deprecated, see Target's doc comment
    Prompt       string `json:"prompt"`
    WorktreePath string `json:"worktreePath,omitempty"`
    TrustPreset  string `json:"trustPreset,omitempty"`
    // Provider pins a specific ai-provider-service account, bypassing the
    // priority cascade — workflow-service.md §7: "explicit
    // step.config.provider.accountId pin (validated active) beats
    // ai-provider-service's priority-chain resolution."
    Provider *ProviderPin `json:"provider,omitempty"`
    Model    string       `json:"model,omitempty"` // ai-provider-service resolves the ACCOUNT, not a model — aiprovider.proto:47-52's ProviderAccount has no model field, confirmed by direct read; Model stays a step-level pass-through param
}

type ProviderPin struct {
    AccountID string `json:"accountId"`
}
```

`ShellStepConfig`/`NotificationStepConfig` gain the identical
`Target`/deprecated-`ConnectionID` pair (no `Provider`/`Model` — those are
agent-specific).

### Port (`usecase/ports.go`)

```go
// ServerResolver turns a step's Target string into a connectionId ready
// for infra-fleet-service.Relay — see domain.AgentStepConfig.Target's doc
// comment for the four accepted shapes. An empty connectionId result
// (ResolveConnectionResponse.Connected==false's existing convention,
// infrafleet.proto:159) means "execute locally," unchanged from today.
type ServerResolver interface {
    Resolve(ctx context.Context, tenantID, target string) (connectionID string, err error)
}

// ProviderResolver resolves which ai-provider-service account an agent
// step should use — workflow-service.md §7's priority note.
type ProviderResolver interface {
    Resolve(ctx context.Context, tenantID, userID, projectID string, pin *domain.ProviderPin) (accountID string, err error)
}
```

### Adapter (`adapter/serverresolver/resolver.go`)

```go
type resolver struct {
    projects projectv1.ProjectServiceClient
    infra    infrafleetv1.InfraFleetServiceClient
    // fleetTagCounters round-robins fleet:tag:<tag> targets — an
    // in-memory, per-instance atomic counter keyed by tag. Deliberately
    // NOT globally coordinated: per workflow-service.md §8's concurrency
    // NFR framing (bounded worker pools are about THIS process's outbound
    // budget, not cluster-wide coordination), a per-replica counter that
    // self-corrects over many calls is sufficient — a perfectly even
    // global distribution is not a correctness requirement here, matching
    // how 08-inter-service-communication.md defers to the service mesh
    // for cross-service load-balancing rather than reimplementing it
    // per-service.
    fleetTagCounters sync.Map // map[string]*atomic.Uint64
}

func (r *resolver) Resolve(ctx context.Context, tenantID, target string) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second) // §8's intra-cluster default
    defer cancel()

    switch {
    case target == "":
        return "", nil // execute locally — unchanged default
    case strings.HasPrefix(target, "connection:"):
        return strings.TrimPrefix(target, "connection:"), nil
    case strings.HasPrefix(target, "server:"):
        devServerID := strings.TrimPrefix(target, "server:")
        resp, err := r.infra.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: devServerID})
        return resp.GetConnectionId(), err
    case strings.HasPrefix(target, "project:"):
        projectID := strings.TrimPrefix(target, "project:")
        proj, err := r.projects.GetProject(ctx, &projectv1.GetProjectRequest{Id: projectID})
        if err != nil { return "", fmt.Errorf("serverresolver: resolve project %s: %w", projectID, err) }
        resp, err := r.infra.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: proj.GetProject().GetDevServerId()})
        return resp.GetConnectionId(), err
    case strings.HasPrefix(target, "fleet:tag:"):
        return r.resolveFleetTag(ctx, tenantID, strings.TrimPrefix(target, "fleet:tag:"))
    default:
        // Legacy bare connectionId (pre-Target step configs written
        // directly as domain.AgentStepConfig.ConnectionID) — see
        // AgentExecutor's call site below for how Target/ConnectionID
        // are combined before Resolve is ever called.
        return target, nil
    }
}

func (r *resolver) resolveFleetTag(ctx context.Context, tenantID, tag string) (string, error) {
    resp, err := r.infra.ListDevServersByTag(ctx, &infrafleetv1.ListDevServersByTagRequest{Tag: tag, HealthyOnly: true})
    if err != nil { return "", err }
    servers := resp.GetDevServers()
    if len(servers) == 0 {
        return "", fmt.Errorf("serverresolver: no healthy dev server tagged %q", tag)
    }
    counter, _ := r.fleetTagCounters.LoadOrStore(tag, new(atomic.Uint64))
    chosen := servers[counter.(*atomic.Uint64).Add(1)%uint64(len(servers))]
    connResp, err := r.infra.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: chosen.GetId()})
    return connResp.GetConnectionId(), err
}
```

Every `StepExecutor` (`AgentExecutor.Execute`, etc.,
`infrafleetclient/agent_step_executor.go:51-67`) is extended to call
`serverResolver.Resolve(ctx, tenantID, cfg.effectiveTarget())` — where
`effectiveTarget()` is `cfg.Target` if set, else `"connection:"+cfg.ConnectionID`
if that's set, else `""` — **before** building `relay(...)`'s params, and
uses the resolved `connectionId` in place of today's raw `cfg.ConnectionID`.

### infra-fleet-service extension: `DevServer.tags` + `ListDevServersByTag`

Not in `infrafleet.proto` today (`DevServer` message,
`infrafleet.proto:117-124`, confirmed by direct read: `id/tenant_id/host/
mode/ssh_target_id` only). This solution proposes:

```protobuf
message DevServer {
  // ... existing fields unchanged ...
  repeated string tags = 6; // free-form, tenant-scoped; e.g. "gpu", "region:us-east"
}

rpc ListDevServersByTag(ListDevServersByTagRequest) returns (ListDevServersByTagResponse);
message ListDevServersByTagRequest {
  string tag = 1;
  bool healthy_only = 2; // filters against GetFleetHealth's own reachability check
}
message ListDevServersByTagResponse {
  repeated DevServer dev_servers = 1;
}
```

Schema: `infra-fleet-service/migrations/000N_dev_server_tags.up.sql` adds
`tags TEXT[] NOT NULL DEFAULT '{}'` to `dev_servers` with a GIN index —
same shape as SOL-WF-01's `templates.tags` addition, for consistency.
`ListDevServersByTag`'s usecase joins against the existing health-check
mechanism `GetFleetHealth` already uses (`infrafleet.proto`'s
`DevServerHealth.reachable` field) to implement `healthy_only` — no new
health-tracking mechanism, reuses what §"GetFleetHealth" already
maintains. **This is the one piece of this solution that touches a
service other than `workflow-service` in its data model — call this out
explicitly to whoever implements infra-fleet-service's side, since it's
not in that service's own TDD doc either** (not read as part of this
pass; flag for cross-checking against `infra-fleet-service.md` before
implementation).

---

## Design — AI provider resolution (`ProviderResolver`)

```go
// internal/adapter/providerresolver/resolver.go
func (r *resolver) Resolve(ctx context.Context, tenantID, userID, projectID string, pin *domain.ProviderPin) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    if pin != nil && pin.AccountID != "" {
        // Explicit pin — validate active per workflow-service.md §7's
        // "(validated active)" parenthetical. ListAccounts (aiprovider.proto:21)
        // is the only read that returns ProviderAccount.status; a single
        // scan-by-id is acceptable at this call volume (one call per agent
        // step dispatch, already inside a 5s-budgeted call).
        accounts, err := r.aiprovider.ListAccounts(ctx, &aiproviderv1.ListAccountsRequest{TenantId: tenantID})
        if err != nil { return "", err }
        for _, a := range accounts.GetAccounts() {
            if a.GetId() == pin.AccountID {
                if a.GetStatus() != "active" {
                    return "", fmt.Errorf("providerresolver: pinned account %s is not active (status=%s)", pin.AccountID, a.GetStatus())
                }
                return a.GetId(), nil
            }
        }
        return "", fmt.Errorf("providerresolver: pinned account %s not found", pin.AccountID)
    }

    // No pin — delegate to ai-provider-service's own priority cascade.
    resp, err := r.aiprovider.ResolveProvider(ctx, &aiproviderv1.ResolveProviderRequest{
        TenantId: tenantID, UserId: userID, ProjectId: projectID,
    })
    if err != nil { return "", err }
    return resp.GetAccount().GetId(), nil
}
```

`AgentExecutor.Execute` calls this before the relay call and threads the
resolved `accountId` (plus `cfg.Model`, passed through untouched — see
`domain.AgentStepConfig.Model`'s doc comment above) into
`agentExecParams`, extending that struct with `AccountID`/`Model` fields.
`userID`/`projectID` come from `ExecutionContext` (new — see interpolation
section below, which needs the same context threaded through for the same
reason: both provider resolution and interpolation need to know who
triggered the run and which project it's scoped to).

---

## Design — variable interpolation

### Proto: `ExecuteRequest.inputs_json`

```protobuf
message ExecuteRequest {
  string template_id = 1;
  string project_id = 2;
  string root_trace_id = 3;
  string request_id = 4;
  string inputs_json = 5; // caller-supplied {{...}} values, e.g. {"feature_description": "..."}
}
```

### Domain: `Interpolate`

```go
// internal/domain/interpolation.go (new)
//
// ExecutionContext carries everything a step's Config's {{...}} tokens can
// reference — built once per Execute call, threaded through every wave.
type ExecutionContext struct {
    Inputs    map[string]any            // from ExecuteRequest.inputs_json
    Outputs   map[string]map[string]any // stepId -> parsed OutputJSON, accumulated wave-by-wave
    ProjectID string
    UserID    string // TriggeredBy
}

var interpolationToken = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

// Interpolate replaces every {{...}} token in configJSON with its resolved
// value (JSON-string-escaped), leaving unresolvable tokens untouched (see
// note below) rather than failing the whole step — matching
// ConditionStepExecutor's existing "fail-safe" posture (workflow-service.md
// §4: "fail-safe-false on unparseable input") applied to interpolation:
// a step author's typo in a token name should surface as a visibly wrong
// output value during testing, not silently abort an entire wave.
func Interpolate(configJSON string, ctx ExecutionContext) (string, error) {
    var interpErr error
    result := interpolationToken.ReplaceAllStringFunc(configJSON, func(match string) string {
        expr := strings.TrimSpace(interpolationToken.FindStringSubmatch(match)[1])
        val, ok := resolveToken(expr, ctx)
        if !ok {
            return match // left as literal {{...}} text — visible, not silently dropped
        }
        return jsonEscapeAndQuoteIfNeeded(val)
    })
    return result, interpErr
}

func resolveToken(expr string, ctx ExecutionContext) (any, bool) {
    switch {
    case expr == "now()":
        return time.Now().UTC().Format(time.RFC3339), true
    case expr == "project.id":
        return ctx.ProjectID, true
    case expr == "user.id":
        return ctx.UserID, true
    case strings.HasPrefix(expr, "outputs."):
        // outputs.<stepId>.<field> — dot-path into that step's parsed output
        parts := strings.SplitN(strings.TrimPrefix(expr, "outputs."), ".", 2)
        if len(parts) != 2 { return nil, false }
        stepOut, ok := ctx.Outputs[parts[0]]
        if !ok { return nil, false }
        return digPath(stepOut, parts[1])
    default:
        val, ok := ctx.Inputs[expr]
        return val, ok
    }
}
```

`project.name`/other project metadata fields are deliberately **not**
resolved in this first pass (would require an extra `project-service` call
per step dispatch, on top of the `ServerResolver`/`ProviderResolver` calls
already added) — flagged as a documented follow-up, not silently dropped;
`project.id` (already known locally, no extra call) is provided now.

### Wiring into wave dispatch

`waveDispatcher` (`wave_dispatcher.go`) gains an `execCtx *domain.ExecutionContext`
field, built once in `Execute.Execute` from `ExecuteRequest.inputs_json` +
`exec.ProjectID` + `exec.TriggeredBy` and passed into
`newWaveDispatcher`/`dispatchWaves`. `dispatchStep`
(`wave_dispatcher.go:172-191`) interpolates before calling the executor:

```go
func (d *waveDispatcher) dispatchStep(ctx context.Context, step domain.Step, se domain.StepExecution) bool {
    interpolated, err := domain.Interpolate(string(step.Config), *d.execCtx)
    // ... error handling, same shape as today's runStep error path ...
    result, err := d.runStep(ctx, step /* with Config = interpolated */, &se)
    // On success, thread the output into execCtx for LATER waves:
    if err == nil && result.Status == domain.ResultStatusCompleted {
        var parsed map[string]any
        json.Unmarshal([]byte(result.OutputJSON), &parsed) // best-effort; non-object outputs simply aren't dot-path-able later
        d.execCtxMu.Lock()
        d.execCtx.Outputs[step.ID] = parsed
        d.execCtxMu.Unlock()
    }
    // ...
}
```

A `sync.Mutex` guards `execCtx.Outputs` because steps within one wave
dispatch concurrently (`wave_dispatcher.go:149-160`'s bounded worker pool)
— writes only ever happen after a step's own terminal result, and reads
(by a later wave's steps) only happen after `dispatchWave` has returned
(the wave gate, `wave_dispatcher.go:95-98`'s doc comment), so there is no
read-during-same-wave-write race for a *later* wave's interpolation, only
concurrent *writes* within the same wave need the mutex.

---

## Design — `action` and `parallel` step types

Neither is described in detail by `workflow-service.md` §4 (flagged above
as a genuine extension). Minimal, extensible shapes:

```go
// internal/domain/step.go (extended)
const (
    StepTypeAction   StepType = "action"
    StepTypeParallel StepType = "parallel"
)

// ActionStepConfig dispatches to a named, in-process action handler — the
// generic "do one named thing" step type BUG-WF-02 finds entirely absent.
// No handlers are registered by this solution itself (no concrete action
// catalog is specified anywhere in the read TDD docs); this wires the
// TYPE SYSTEM so `action` steps are at least recognized and fail with a
// clear, typed error rather than domain.StepTypeUnspecified's silent
// no-op — mirrors usecase.ErrStepExecutorNotRegistered's existing pattern
// (ports.go) for "the shape exists, the catalog doesn't yet."
type ActionStepConfig struct {
    ActionName string          `json:"actionName"`
    Params     json.RawMessage `json:"params,omitempty"`
}

// ParallelStepConfig fans SubSteps out concurrently and aggregates their
// results — Promise.allSettled + allowPartialFailure semantics per
// BUG-WF-02's spec summary. SubSteps are full domain.Step values (their
// own Type dispatches through the SAME StepExecutorRegistry a top-level
// wave step would use), but their DependsOn is ignored — sub-steps always
// run together, in one fan-out, not wave-computed among themselves (a
// nested DAG-within-a-step is out of scope; sequencing sub-steps is
// achieved by nesting a second parallel/single step referencing the
// first's output via {{outputs...}}, not by nested dependsOn).
type ParallelStepConfig struct {
    SubSteps            []Step `json:"subSteps"`
    AllowPartialFailure  bool   `json:"allowPartialFailure,omitempty"`
}
```

```go
// internal/adapter/stepexecutors/parallel.go (new)
//
// ParallelExecutor needs the SAME StepExecutorRegistry the wave dispatcher
// uses, to recursively resolve each sub-step's own executor — injected at
// construction (cmd/server/main.go wires it after the registry itself is
// built, a two-phase init since ParallelExecutor IS one of the registry's
// own entries).
type ParallelExecutor struct {
    registry usecase.StepExecutorRegistry
}

func (e *ParallelExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
    var cfg domain.ParallelStepConfig
    json.Unmarshal([]byte(stepConfigJSON), &cfg)

    results := make([]domain.StepResult, len(cfg.SubSteps))
    errs := make([]error, len(cfg.SubSteps))
    var wg sync.WaitGroup
    for i, sub := range cfg.SubSteps {
        wg.Add(1)
        go func(i int, sub domain.Step) {
            defer wg.Done()
            executor, err := e.registry.Resolve(sub.Type)
            if err != nil { errs[i] = err; return }
            results[i], errs[i] = executor.Execute(ctx, string(sub.Config)) // allSettled: every sub-step runs regardless of siblings' outcome
        }(i, sub)
    }
    wg.Wait()

    anyFailed := false
    agg := make(map[string]any, len(cfg.SubSteps))
    for i, sub := range cfg.SubSteps {
        if errs[i] != nil || results[i].Status == domain.ResultStatusFailed {
            anyFailed = true
        }
        agg[sub.ID] = results[i] // per-sub-step outcome, keyed by sub-step id — later {{outputs.parallelStepId...}} access is a documented non-goal (sub-step outputs are visible via agg, not individually addressable through the outer step id)
    }
    if anyFailed && !cfg.AllowPartialFailure {
        return domain.StepResult{Status: domain.ResultStatusFailed}, fmt.Errorf("stepexecutors: parallel: one or more sub-steps failed")
    }
    outputJSON, _ := json.Marshal(agg)
    return domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: string(outputJSON)}, nil
}
```

`ActionExecutor` (new, `adapter/stepexecutors/action.go`) holds a
`map[string]ActionHandler` (empty at construction — see doc comment above)
and returns a clear `usecase.ErrNoActionHandlerRegistered` error for any
`ActionName` not found, rather than a generic dispatch failure.

`domain.StepType.Valid()` (`step.go:26-33`) and the proto `StepType` enum
(`workflow.proto:53-60`) both gain `STEP_TYPE_ACTION`/`STEP_TYPE_PARALLEL`.

---

## Design — live streaming (`StreamExecutionEvents`)

### Proto

```protobuf
rpc StreamExecutionEvents(StreamExecutionEventsRequest) returns (stream ExecutionEvent);

message StreamExecutionEventsRequest {
  string execution_id = 1;
}
message ExecutionEvent {
  string execution_id = 1;
  string step_id = 2;       // empty for execution-level events
  string type = 3;          // step.output | step.completed | execution.completed
  string payload_json = 4;
  int64 occurred_at_unix_ms = 5;
}
```

### Port + in-process adapter

```go
// usecase/ports.go
type EventPublisher interface {
    Publish(ctx context.Context, event domain.ExecutionEvent) error
}
```

```go
// internal/adapter/eventstream/broker.go (new)
//
// In-process pub/sub, keyed by execution_id — a subscriber (the gRPC
// StreamExecutionEvents handler) registers a channel for one execution_id;
// Publish fans out to every registered channel for that id. Explicitly
// single-instance: if workflow-service runs more than one replica, a
// StreamExecutionEvents call must land on the SAME replica whose
// dispatch goroutine is driving that execution — flagged as an open
// scaling question below, not silently assumed away (matching
// 08-inter-service-communication.md's own "genuine open question flagged
// here rather than papered over" framing for the agent-relay protocol
// choice).
type Broker struct {
    mu   sync.Mutex
    subs map[string][]chan domain.ExecutionEvent // keyed by execution_id
}
```

`waveDispatcher.dispatchStep` publishes a `step.output`/`step.completed`
event after each step's terminal result (in addition to the existing
`stepExecutions.UpdateStepExecution` persistence call — the stream is a
best-effort live push, the Postgres row remains the source of truth for
anything that must survive a missed stream frame or a restart, matching
§8's resumability posture). `Execute.runToCompletion`
(`execute.go:132-142`) publishes `execution.completed` after persisting
the final status.

**Scaling caveat, flagged explicitly:** this in-process design only works
correctly if `api-gateway`'s WS bridge routes a given `execution_id`'s
`workflow.execution.subscribe` call to the `workflow-service` replica
actually running that execution's dispatch goroutine. Two ways to close
this, neither decided here (same "flag, don't paper over" posture
`08-inter-service-communication.md` uses for its own open questions):
(a) make `workflow-service` a single-writer-per-execution via consistent
hashing at the k8s Service/mesh layer, or (b) replace the in-process
`Broker` with a NATS JetStream subject per execution
(`orca.workflow.execution.<id>.events`, ephemeral, per
`08-inter-service-communication.md`'s event-conventions naming scheme) at
the cost of NATS round-trip latency on every `step.output` frame. Default
recommendation for an initial pass: (a), since `workflow-service` is
already effectively single-writer-per-execution in-process (the dispatch
goroutine started by `Execute`, `execute.go:123`, lives entirely on one
replica for that execution's lifetime) — (b) is the right call only once
horizontal scaling of `workflow-service` itself is load-tested and found
to need it.

### wscompat wiring

```go
// api-gateway/internal/adapter/wscompat/channels.go — new
//
// workflow.execution.subscribe opens a gRPC server-streaming call and
// pipes frames to the WS client as push events — same "accept WS, open
// corresponding gRPC stream, pipe frames both directions" shape
// 08-inter-service-communication.md's API Gateway responsibilities
// section (item 5) already describes for infra-fleet-service's terminal
// streams, applied here to workflow-service's new RPC.
func registerWorkflowChannels(r *Registry, client workflowv1.WorkflowServiceClient) {
    r.RegisterStream("workflow.execution.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage, push func(any)) error {
        in, err := decodeArg[struct{ ExecutionID string `json:"executionId"` }](args, 0)
        if err != nil { return err }
        stream, err := client.StreamExecutionEvents(ctx, &workflowv1.StreamExecutionEventsRequest{ExecutionId: in.ExecutionID})
        if err != nil { return err }
        for {
            event, err := stream.Recv()
            if err == io.EOF { return nil }
            if err != nil { return err }
            push(event)
        }
    })
}
```

(`RegisterStream` is illustrative — the exact server-streaming
registration primitive should match whatever `wscompat` already uses for
`terminal.*`'s `AttachPty`-backed channels, not a newly invented shape;
confirm against that existing pattern before implementing.)

---

## Test plan

- `adapter/serverresolver/resolver_test.go` — one test per `Target` prefix
  (`connection:`, `server:`, `project:`, `fleet:tag:`, bare-legacy, empty)
  against fake `ProjectServiceClient`/`InfraFleetServiceClient`;
  `fleet:tag:` round-robins across ≥2 fake healthy servers over repeated
  calls (assert both get selected, not just the first); zero healthy
  servers for a tag returns a clear error, never an empty-string
  connectionId (which would silently mean "execute locally" — a
  correctness bug, not just a UX one).
- `adapter/providerresolver/resolver_test.go` — pinned active account
  wins over cascade; pinned inactive/unknown account errors without
  falling back to the cascade silently; no pin delegates to
  `ResolveProvider` with the right tenant/user/project.
- `domain/interpolation_test.go` — every token kind (`{{feature_description}}`
  from Inputs, `{{outputs.stepA.field}}` from Outputs, `{{project.id}}`,
  `{{user.id}}`, `{{now()}}`); unresolvable token left as literal text
  (not stripped, not an error); a token embedded inside a larger JSON
  string value round-trips correctly (JSON-escaping test).
- `usecase/wave_dispatcher_test.go` — a 2-wave DAG where wave 1's step
  output is referenced by a `{{outputs...}}` token in wave 2's config;
  assert wave 2's executor receives the interpolated value, not the raw
  token; concurrent writes to `execCtx.Outputs` within one wave don't race
  (run with `-race`).
- `adapter/stepexecutors/parallel_test.go` — all sub-steps succeed →
  aggregate completed; one sub-step fails + `AllowPartialFailure=false` →
  aggregate failed, but EVERY sub-step still ran (assert via a fake
  executor's call count, allSettled semantics); one sub-step fails +
  `AllowPartialFailure=true` → aggregate completed, failed sub-step's
  outcome still present in the aggregated output.
- `adapter/stepexecutors/action_test.go` — unregistered `ActionName`
  returns `ErrNoActionHandlerRegistered`, never a panic or a silent no-op.
- `adapter/eventstream/broker_test.go` — a subscriber registered before
  `Publish` receives the event; a subscriber that unsubscribes mid-stream
  doesn't leak a goroutine (test with a bounded run + `-race`).
- `adapter/grpc/server_test.go` — `StreamExecutionEvents` for an unknown
  `execution_id` returns immediately (empty stream, not a hang).
- Integration-style: `Execute` end-to-end with a 2-step DAG (`agent` step
  targeting `"project:<id>"`, `condition` step reading the agent step's
  `{{outputs...}}`) against fake `project`/`infra-fleet`/`ai-provider`
  clients — asserts the full resolve → dispatch → interpolate chain, not
  just each piece in isolation.

**Needs `agent/` (Dev Server Agent) changes:** No new capability is
required from `agent/` for server/provider resolution, interpolation, or
`parallel`/`action` step types — all of this resolves/computes
server-side before the existing `Relay` call. The one adjacent,
NOT-in-scope-here item: `agent_step_executor.go:12-26`'s own doc comment
already flags that the `agent.exec` method name it currently sends may not
match the real agent handler contract (`agent.execPrompt` per TS Gap 4's
resolution) — reconciling that method name is a separate, already-tracked
verification step, not a new `agent/` feature this solution requires.
`fleet:tag:` resolution and live streaming both stay entirely within
`workflow-service`/`infra-fleet-service`/`api-gateway` — the execution
plane's `agent/` code is untouched.

## References

- `specs/backend-go/tdd/services/workflow-service.md:38-46` (§2 bounded context, resolution ownership split), `:88-107` (§3.2, Gap 4 correction discipline), `:109-142` (§4 domain model, StepExecutor list), `:244-286` (§7 dependencies, `wf --> aiprov`/`wf --> infra` edges and priority-cascade note), `:287-317` (§8 NFRs, deadlines/concurrency)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:110-166` (dependency graph, no new edge needed for server/provider resolution; `fleet:tag` schema addition flagged as the one genuine cross-service extension)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:26-28` (mandatory deadlines), `:47-67` (API Gateway WS-bridge pattern this solution's `workflow.execution.subscribe` follows), `:84-108` (agent-relay "flag, don't paper over" precedent this solution's streaming-scaling caveat follows)
- `backend-go/services/workflow-service/internal/domain/step.go:60-106` — current `AgentStepConfig`/`ShellStepConfig`/`NotificationStepConfig`/`StepExecutor`
- `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go:12-67` — current executor, method-name caveat
- `backend-go/services/workflow-service/internal/usecase/execute.go:16-126`, `wave_dispatcher.go:1-214` — current dispatch flow this solution extends
- `backend-go/proto/orca/workflow/v1/workflow.proto:53-60,84-90` — `StepType` enum, `ExecuteRequest`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:117-124,150-170` — `DevServer` (no `tags`), `ResolveConnectionRequest`/`Response`
- `backend-go/proto/orca/aiprovider/v1/aiprovider.proto:47-52,69-79` — `ProviderAccount` (no `model` field), `ResolveProvider`
- `backend-go/proto/orca/project/v1/project.proto:83-87` — `Project.dev_server_id`, the `project:<id>` resolution target
- `specs/backend-go/bugs/logic-v1/BUG-WF-02-workflow-execution-partial.md` — problem statement
- `specs/backend-go/bugs/missing-v1/BUG-034-task-channels-not-implemented.md` — adjacent finding on `task-service`'s `SimpleExecutor`/`ComplexExecutor` stubs; not this bug's scope, cited only because both trace back to the same "execution plane contract not yet reconciled" theme
