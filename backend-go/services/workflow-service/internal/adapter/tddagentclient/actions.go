package tddagentclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// Action names registered on the "action" step type.
const (
	ActionStart        = "tdd_agent.start"
	ActionAdvance      = "tdd_agent.advance"
	ActionWaitChild    = "tdd_agent.wait_child"
	ActionApproval     = "tdd_agent.approval"
	ActionWaitEvidence = "tdd_agent.wait_evidence"
)

const maxRedispatch = 3

// toolCaller is the Client surface the handlers use (faked in tests).
type toolCaller interface {
	CallTool(ctx context.Context, target, projectDir, tool string, args map[string]any, out any) error
	ResolveProjectDir(ctx context.Context, projectID, requested string) (string, error)
}

// ExecutionReader lets a waiting step notice that its execution was cancelled
// (CancelExecution only flips the row; it does not interrupt running steps).
type ExecutionReader interface {
	GetExecution(ctx context.Context, tenantID, id string) (domain.WorkflowExecution, error)
}

// Handlers implements the tdd_agent.* actions.
type Handlers struct {
	tools      toolCaller
	executions ExecutionReader
	approvals  ApprovalStore // nil until the step_approvals store is wired: tdd_agent.approval is then not registered
	clock      clock
	metrics    *metrics
}

func NewHandlers(tools toolCaller, executions ExecutionReader, approvals ApprovalStore) *Handlers {
	return &Handlers{tools: tools, executions: executions, approvals: approvals, clock: realClock{}, metrics: defaultMetrics}
}

// params is the union of every action's params (step config "params").
type params struct {
	Target          string          `json:"target"`
	ProjectDir      string          `json:"projectDir"`
	PollIntervalSec int             `json:"pollIntervalSec"`
	DeadlineMin     int             `json:"deadlineMin"`
	Flow            string          `json:"flow"`
	Request         json.RawMessage `json:"request"` // object, or a JSON string (templates pass objects through {{input}} tokens)
	Sources         json.RawMessage `json:"sources"` // array, or a JSON string
	RunID           string          `json:"runId"`
	StopAt          []string        `json:"stopAt"`
	ExpectGate      string          `json:"expectGate"`
	ChildFlow       string          `json:"childFlow"`
	Gate            string          `json:"gate"`
	Role            string          `json:"role"`
	// Start modes: "run" (run_start of Flow), "generate" (the project's TDD from its documents, like `tdd-agent generate`),
	// "change" (a change request, like `tdd-agent change start`).
	Mode          string   `json:"mode"`
	Force         bool     `json:"force"`
	ChangeName    string   `json:"changeName"`
	CRPath        string   `json:"crPath"`
	SRSAfterPath  string   `json:"srsAfterPath"`
	CodeTargets   []string `json:"codeTargets"`
	Verifications []string `json:"verifications"`
	AllowNoRun    bool     `json:"allowNoRun"` // a generate start found the TDD up to date: later steps succeed without a run
}

// stepEnv is what every handler resolves first.
type stepEnv struct {
	p          params
	target     string
	projectDir string
	ident      usecase.StepIdentity
	hasIdent   bool
	tenantID   string
	userID     string
}

func (h *Handlers) env(ctx context.Context, action, raw string, defaultDeadline time.Duration) (stepEnv, pollSpec, error) {
	var e stepEnv
	if raw == "" {
		raw = "{}"
	}
	if err := json.Unmarshal([]byte(raw), &e.p); err != nil {
		return e, pollSpec{}, fail(CodeInvalidParams, "params: "+err.Error())
	}
	// an optional input the execution did not supply is left as its literal {{token}}
	for _, f := range []*string{&e.p.Target, &e.p.ProjectDir, &e.p.RunID, &e.p.CRPath, &e.p.SRSAfterPath, &e.p.ChangeName} {
		if unresolved(*f) {
			*f = ""
		}
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return e, pollSpec{}, fail(CodeInvalidParams, err.Error())
	}
	e.tenantID = tenantID
	e.userID, _ = tenant.UserID(ctx)
	projectID, _ := tenant.ProjectID(ctx)
	e.target = e.p.Target
	if e.target == "" {
		if projectID == "" {
			return e, pollSpec{}, fail(CodeInvalidParams, "no target and the execution has no project")
		}
		e.target = "project:" + projectID
	}
	e.projectDir, err = h.tools.ResolveProjectDir(ctx, projectID, e.p.ProjectDir)
	if err != nil {
		return e, pollSpec{}, err
	}
	e.ident, e.hasIdent = usecase.StepIdentityFrom(ctx)
	if action != ActionStart && e.p.RunID == "" && !e.p.AllowNoRun {
		return e, pollSpec{}, fail(CodeInvalidParams, action+" needs runId (e.g. {{outputs.start.runId}})")
	}
	spec := pollSpec{Interval: 10 * time.Second, Deadline: defaultDeadline}
	if e.p.PollIntervalSec > 0 {
		spec.Interval = time.Duration(e.p.PollIntervalSec) * time.Second
	}
	if e.p.DeadlineMin > 0 {
		spec.Deadline = time.Duration(e.p.DeadlineMin) * time.Minute
	}
	return e, spec, nil
}

// cancelled reports whether the step's execution was cancelled (always false outside a wave dispatch).
func (h *Handlers) cancelled(ctx context.Context, e stepEnv) func() bool {
	return func() bool {
		if !e.hasIdent || h.executions == nil {
			return false
		}
		exec, err := h.executions.GetExecution(ctx, e.tenantID, e.ident.ExecutionID)
		return err == nil && exec.Status == domain.StatusCancelled
	}
}

func (h *Handlers) call(ctx context.Context, e stepEnv, tool string, args map[string]any, out any) error {
	err := h.tools.CallTool(ctx, e.target, e.projectDir, tool, args, out)
	h.metrics.call(tool, err)
	return err
}

func (h *Handlers) runGet(ctx context.Context, e stepEnv, runID string) (runView, error) {
	var v runView
	err := h.call(ctx, e, "run_get", map[string]any{"run_id": runID}, &v)
	return v, err
}

// finish turns a handler outcome into the step result: output always carries what is known of the run.
func finish(action string, out map[string]any, err error) (domain.StepResult, error) {
	if out == nil {
		out = map[string]any{"action": action}
	}
	status := domain.ResultStatusCompleted
	if err != nil {
		status = domain.ResultStatusFailed
		var sf *stepFailure
		var te *ToolError
		var be *BridgeError
		switch {
		case errors.As(err, &sf):
			out["error"], out["message"] = sf.Code, sf.Message
		case errors.As(err, &te):
			out["error"], out["message"] = te.Code, te.Message
		case errors.As(err, &be):
			out["error"], out["message"] = be.Code, be.Message
		case errors.Is(err, ErrTransport):
			out["error"], out["message"] = CodeUnreachable, err.Error()
		default:
			out["error"], out["message"] = "TDD_AGENT_ERROR", err.Error()
		}
	}
	raw, merr := json.Marshal(out)
	if merr != nil {
		return domain.StepResult{}, fmt.Errorf("tddagentclient: marshal output: %w", merr)
	}
	return domain.StepResult{Status: status, OutputJSON: string(raw)}, nil
}

// cancelRun stops the tdd-agent run when the user cancelled the execution (TDD §7.6).
func (h *Handlers) cancelRun(ctx context.Context, e stepEnv, runID string, err error) {
	var sf *stepFailure
	if !errors.As(err, &sf) || sf.Code != CodeCancelled || !e.hasIdent {
		return
	}
	reason := fmt.Sprintf("orca execution %s cancelled", e.ident.ExecutionID)
	if cerr := h.call(context.WithoutCancel(ctx), e, "run_cancel", map[string]any{"run_id": runID, "reason": reason}, nil); cerr != nil {
		slog.WarnContext(ctx, "tdd_agent: run_cancel failed", slog.String("run_id", runID), slog.Any("error", cerr))
	}
}

// Start creates the run, or finds the one this step created before a restart.
func (h *Handlers) Start(ctx context.Context, raw string) (domain.StepResult, error) {
	e, _, err := h.env(ctx, ActionStart, raw, time.Minute)
	if err != nil {
		return finish(ActionStart, nil, err)
	}
	mode := e.p.Mode
	if mode == "" {
		mode = "run"
	}
	switch {
	case mode == "run" && e.p.Flow == "":
		return finish(ActionStart, nil, fail(CodeInvalidParams, "flow is required"))
	case mode == "change" && (e.p.ChangeName == "" || e.p.CRPath == ""):
		return finish(ActionStart, nil, fail(CodeInvalidParams, "changeName and crPath are required"))
	case mode != "run" && mode != "generate" && mode != "change":
		return finish(ActionStart, nil, fail(CodeInvalidParams, "mode must be run, generate or change"))
	}
	out := map[string]any{"action": ActionStart, "mode": mode, "flow": e.p.Flow, "projectDir": e.projectDir}
	if e.hasIdent {
		var found struct {
			Runs []struct {
				RunID  string `json:"run_id"`
				Status string `json:"status"`
			} `json:"runs"`
		}
		args := map[string]any{"external_execution_id": e.ident.ExecutionID, "external_step_id": e.ident.StepID, "limit": 1}
		if err := h.call(ctx, e, "run_list", args, &found); err != nil {
			return finish(ActionStart, out, err)
		}
		if len(found.Runs) > 0 {
			out["runId"], out["status"], out["reused"] = found.Runs[0].RunID, found.Runs[0].Status, true
			return finish(ActionStart, out, nil)
		}
	}
	var external map[string]string
	if e.hasIdent {
		external = map[string]string{"system": "orca", "executionId": e.ident.ExecutionID, "stepId": e.ident.StepID, "userId": e.userID}
	}
	switch mode {
	case "generate":
		return h.startGenerate(ctx, e, external, out)
	case "change":
		return h.startChange(ctx, e, external, out)
	}
	request := map[string]any{}
	if err := decodeLoose(e.p.Request, &request); err != nil {
		return finish(ActionStart, out, fail(CodeInvalidParams, "request: "+err.Error()))
	}
	var sources []map[string]any
	if err := decodeLoose(e.p.Sources, &sources); err != nil {
		return finish(ActionStart, out, fail(CodeInvalidParams, "sources: "+err.Error()))
	}
	args := map[string]any{"flow": e.p.Flow, "request": request}
	if len(sources) > 0 {
		args["sources"] = sources
	}
	if external != nil {
		args["external"] = external
	}
	var started struct {
		RunID string `json:"run_id"`
		Run   struct {
			Status string `json:"status"`
		} `json:"run"`
	}
	if err := h.call(ctx, e, "run_start", args, &started); err != nil {
		return finish(ActionStart, out, err)
	}
	out["runId"], out["status"], out["reused"] = started.RunID, started.Run.Status, false
	return finish(ActionStart, out, nil)
}

func (h *Handlers) startGenerate(ctx context.Context, e stepEnv, external map[string]string, out map[string]any) (domain.StepResult, error) {
	args := map[string]any{"force": e.p.Force}
	if external != nil {
		args["external"] = external
	}
	var res struct {
		Status   string `json:"status"`
		RunID    string `json:"run_id"`
		Document string `json:"document"`
	}
	if err := h.call(ctx, e, "generate_start", args, &res); err != nil {
		return finish(ActionStart, out, err)
	}
	out["flow"], out["runId"], out["status"], out["reused"] = "tdd-generation", res.RunID, res.Status, false
	if res.Status == "up_to_date" {
		out["document"] = res.Document
	}
	return finish(ActionStart, out, nil)
}

func (h *Handlers) startChange(ctx context.Context, e stepEnv, external map[string]string, out map[string]any) (domain.StepResult, error) {
	args := map[string]any{"name": e.p.ChangeName, "cr_path": e.p.CRPath}
	if e.p.SRSAfterPath != "" {
		args["srs_after_path"] = e.p.SRSAfterPath
	}
	if len(e.p.CodeTargets) > 0 {
		args["code_targets"] = e.p.CodeTargets
	}
	if len(e.p.Verifications) > 0 {
		args["verifications"] = e.p.Verifications
	}
	if external != nil {
		args["external"] = external
	}
	var res struct {
		RunID  string `json:"run_id"`
		Status string `json:"status"`
	}
	if err := h.call(ctx, e, "change_start", args, &res); err != nil {
		return finish(ActionStart, out, err)
	}
	out["flow"], out["runId"], out["status"], out["reused"] = "change-request", res.RunID, res.Status, false
	return finish(ActionStart, out, nil)
}

// noRun ends a step that follows a generate start which found the TDD up to date.
func noRun(action string, e stepEnv) (domain.StepResult, bool, error) {
	if e.p.RunID != "" {
		return domain.StepResult{}, false, nil
	}
	res, err := finish(action, map[string]any{"action": action, "skipped": "no run: the TDD is up to date", "projectDir": e.projectDir}, nil)
	return res, true, err
}

// stopReached says whether v is where the step waits for, or a failure.
func stopReached(v runView, stopAt []string, expectGate string) (bool, error) {
	switch v.status() {
	case RunBlocked:
		return true, fail(CodeBlocked, fmt.Sprintf("gate %s blocked: %v", v.Run.Result.BlockedGate, v.Run.Result.Blockers))
	case RunFailed:
		return true, fail(CodeRunFailed, fmt.Sprintf("run failed: %v", v.Run.Result.Blockers))
	}
	for _, s := range stopAt {
		if v.status() != s {
			continue
		}
		gate := v.Run.Result.PendingGate
		if s == RunAwaitingEvidence {
			gate = v.Run.Result.BlockedGate
		}
		if expectGate != "" && gate != expectGate {
			return true, fail(CodeUnexpectedGate, fmt.Sprintf("run stopped at %s %q, the template expects %q", s, gate, expectGate))
		}
		return true, nil
	}
	return false, nil
}

// Advance moves the run to its next stop (approval, evidence, completion) via a background advance on the dev server.
func (h *Handlers) Advance(ctx context.Context, raw string) (domain.StepResult, error) {
	e, spec, err := h.env(ctx, ActionAdvance, raw, 60*time.Minute)
	if err != nil {
		return finish(ActionAdvance, nil, err)
	}
	if res, skipped, rerr := noRun(ActionAdvance, e); skipped {
		return res, rerr
	}
	stopAt := e.p.StopAt
	if len(stopAt) == 0 {
		stopAt = []string{RunAwaitingApproval, RunAwaitingEvidence, RunCompleted}
	}
	var last runView
	dispatches := 0
	first := true
	started := time.Now()
	err = poll(ctx, h.clock, spec, h.cancelled(ctx, e), func() (bool, error) {
		v, err := h.runGet(ctx, e, e.p.RunID)
		if err != nil {
			return false, err
		}
		h.metrics.observeGates(ctx, e.p.RunID, last, v)
		last = v
		// A blocked run met at the first look is a retry of this step: run the blocked gate again.
		if !(first && v.status() == RunBlocked) {
			if done, serr := stopReached(v, stopAt, e.p.ExpectGate); done {
				return true, serr
			}
		}
		first = false
		if v.heartbeatStale(h.clock.Now()) {
			slog.WarnContext(ctx, "tdd_agent: background advance has not beaten for three intervals", slog.String("run_id", e.p.RunID), slog.Int("pid", v.Process.PID))
		}
		if v.processAlive() {
			return false, nil
		}
		if dispatches >= maxRedispatch {
			return true, fail(CodeAdvanceCrashed, fmt.Sprintf("the background advance stopped %d times without reaching a stop", dispatches))
		}
		var d struct {
			Dispatched bool   `json:"dispatched"`
			Reason     string `json:"reason"`
		}
		if err := h.call(ctx, e, "run_dispatch", map[string]any{"run_id": e.p.RunID}, &d); err != nil {
			return false, err
		}
		if d.Dispatched {
			dispatches++
			if dispatches > 1 {
				slog.WarnContext(ctx, "tdd_agent: re-dispatched a run whose background advance had stopped", slog.String("run_id", e.p.RunID), slog.Int("dispatches", dispatches))
			}
		}
		return false, nil
	})
	h.cancelRun(ctx, e, e.p.RunID, err)
	h.metrics.waited(ActionAdvance, started, err)
	out := last.output(ActionAdvance, e.projectDir)
	out["runId"] = e.p.RunID
	out["dispatches"] = dispatches
	return finish(ActionAdvance, out, err)
}

// WaitChild waits for the newest child run of childFlow (default testspec) to finish.
func (h *Handlers) WaitChild(ctx context.Context, raw string) (domain.StepResult, error) {
	e, spec, err := h.env(ctx, ActionWaitChild, raw, 60*time.Minute)
	if err != nil {
		return finish(ActionWaitChild, nil, err)
	}
	if res, skipped, rerr := noRun(ActionWaitChild, e); skipped {
		return res, rerr
	}
	flow := e.p.ChildFlow
	if flow == "" {
		flow = "testspec"
	}
	out := map[string]any{"action": ActionWaitChild, "runId": e.p.RunID, "childFlow": flow, "projectDir": e.projectDir}
	var child runView
	childID := ""
	started := time.Now()
	err = poll(ctx, h.clock, spec, h.cancelled(ctx, e), func() (bool, error) {
		if childID == "" {
			parent, err := h.runGet(ctx, e, e.p.RunID)
			if err != nil {
				return false, err
			}
			c, ok := parent.newestChild(flow)
			if !ok {
				if parent.terminal() {
					return true, fail(CodeChildMissing, "the run finished without a "+flow+" child run")
				}
				return false, nil
			}
			childID = c.RunID
		}
		v, err := h.runGet(ctx, e, childID)
		if err != nil {
			return false, err
		}
		child = v
		switch v.status() {
		case RunCompleted:
			return true, nil
		case RunBlocked, RunFailed:
			return true, fail(CodeBlocked, fmt.Sprintf("child run %s %s: %v", childID, v.status(), v.Run.Result.Blockers))
		}
		return false, nil
	})
	h.cancelRun(ctx, e, e.p.RunID, err)
	h.metrics.waited(ActionWaitChild, started, err)
	if childID != "" {
		out["childRunId"] = childID
		out["status"] = child.status()
		out["blockers"] = child.Run.Result.Blockers
		out["gates"] = child.gateStatuses()
		outputs := map[string]any{}
		for name, t := range child.Tasks {
			if t.Output != nil {
				outputs[name] = t.Output
			}
		}
		out["outputs"] = outputs
	}
	return finish(ActionWaitChild, out, err)
}

// WaitEvidence waits until the run leaves awaiting_evidence at gate (default G6): test receipts are recorded elsewhere.
func (h *Handlers) WaitEvidence(ctx context.Context, raw string) (domain.StepResult, error) {
	e, spec, err := h.env(ctx, ActionWaitEvidence, raw, 7*24*time.Hour)
	if err != nil {
		return finish(ActionWaitEvidence, nil, err)
	}
	gate := e.p.Gate
	if gate == "" {
		gate = "G6"
	}
	var last runView
	started := time.Now()
	err = poll(ctx, h.clock, spec, h.cancelled(ctx, e), func() (bool, error) {
		v, err := h.runGet(ctx, e, e.p.RunID)
		if err != nil {
			return false, err
		}
		last = v
		if v.status() == RunFailed {
			return true, fail(CodeRunFailed, fmt.Sprintf("run failed: %v", v.Run.Result.Blockers))
		}
		if v.status() != RunAwaitingEvidence || v.Run.Result.BlockedGate != gate {
			return true, nil // the gate passed; the next advance step takes the run further
		}
		// Receipts are recorded outside the run; only an advance re-evaluates the evidence gate (it needs no LLM).
		if !v.processAlive() {
			if err := h.call(ctx, e, "run_dispatch", map[string]any{"run_id": e.p.RunID}, nil); err != nil {
				return false, err
			}
		}
		return false, nil
	})
	h.cancelRun(ctx, e, e.p.RunID, err)
	h.metrics.waited(ActionWaitEvidence, started, err)
	out := last.output(ActionWaitEvidence, e.projectDir)
	out["runId"], out["gate"] = e.p.RunID, gate
	return finish(ActionWaitEvidence, out, err)
}

// Register wires the actions through register (stepexecutors.ActionExecutor.Register); tdd_agent.approval only
// when an approval store exists.
func (h *Handlers) Register(register func(actionName string, handler func(ctx context.Context, paramsJSON string) (domain.StepResult, error))) {
	register(ActionStart, h.Start)
	register(ActionAdvance, h.Advance)
	register(ActionWaitChild, h.WaitChild)
	register(ActionWaitEvidence, h.WaitEvidence)
	if h.approvals != nil {
		register(ActionApproval, h.Approval)
	}
}

func unresolved(s string) bool { return strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") }

// decodeLoose decodes raw into out whether it is the value itself or a JSON string holding it; absent or an
// unresolved {{token}} leaves out unchanged.
func decodeLoose(raw json.RawMessage, out any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		if str == "" || unresolved(str) {
			return nil
		}
		raw = json.RawMessage(str)
	}
	return json.Unmarshal(raw, out)
}
