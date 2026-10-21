package domain

import (
	"encoding/json"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// TestSpecStatus is where the test-spec lane of a Request is. The lane starts together with Plan generation, runs beside it and
// the code-execution lane, and never changes the status of the Request (docs/integrate/TDD-REQ-TESTSPEC-001, ORC-TS-ADR-11).
type TestSpecStatus string

const (
	TestSpecPending   TestSpecStatus = "pending"
	TestSpecStarting  TestSpecStatus = "starting"
	TestSpecRunning   TestSpecStatus = "running"
	TestSpecCompleted TestSpecStatus = "completed"
	// TestSpecBlocked is a configuration problem a person fixes before running again; TestSpecFailed is a run that did not finish.
	TestSpecBlocked   TestSpecStatus = "blocked"
	TestSpecFailed    TestSpecStatus = "failed"
	TestSpecSkipped   TestSpecStatus = "skipped"
	TestSpecCancelled TestSpecStatus = "cancelled"
)

// Active is true while the run still needs the reconcile pass.
func (s TestSpecStatus) Active() bool {
	return s == TestSpecPending || s == TestSpecStarting || s == TestSpecRunning
}

// Finished is true when nothing more will happen to the row unless a person runs it again.
func (s TestSpecStatus) Finished() bool { return !s.Active() }

// Reason codes of a test-spec run that did not (or could not) run.
const (
	TestSpecNotApplicable   = "REQUEST_TESTSPEC_NOT_APPLICABLE"
	TestSpecNoSolution      = "REQUEST_TESTSPEC_NO_SOLUTION"
	TestSpecNoPlanBundle    = "REQUEST_TESTSPEC_NO_PLAN_BUNDLE"
	TestSpecTooLarge        = "REQUEST_TESTSPEC_TOO_LARGE"
	TestSpecNoDevServer     = "REQUEST_TESTSPEC_NO_DEV_SERVER"
	TestSpecModeNone        = "REQUEST_TESTSPEC_MODE_NONE"
	TestSpecDisabledOnDev   = "REQUEST_TESTSPEC_DISABLED_ON_DEV_SERVER"
	TestSpecToolMissing     = "REQUEST_TESTSPEC_TOOL_MISSING"
	TestSpecWriteDisabled   = "REQUEST_TESTSPEC_WRITE_DISABLED"
	TestSpecLLMMissing      = "REQUEST_TESTSPEC_LLM_NOT_CONFIGURED"
	TestSpecUnreachable     = "REQUEST_TESTSPEC_UNREACHABLE"
	TestSpecDeadline        = "REQUEST_TESTSPEC_DEADLINE"
	TestSpecRunLost         = "REQUEST_TESTSPEC_RUN_LOST"
	TestSpecActorNotAllowed = "REQUEST_EXECUTION_ACTOR_NOT_ALLOWED"
)

// TestSpecRun is one attempt to generate the test specs of a Request.
type TestSpecRun struct {
	ID        string
	TenantID  string
	RequestID string
	Attempt   int
	State     TestSpecStatus
	// ReasonCode and ReasonMessage say why a run is blocked, failed, skipped or cancelled.
	ReasonCode    string
	ReasonMessage string
	// ContextMode is solution or plan: what tdd-agent said it read; ContextRef is REQ-<number>; ContextSHA256 names the document sent.
	ContextMode   string
	ContextRef    string
	ContextSHA256 string
	// TDDRunID is the run of tdd-agent (run-…) on the dev server; Feature is what it wrote (for example TDD-NOTES).
	TDDRunID string
	Feature  string
	// Summary is the snapshot of testspec_get(feature) taken when the run completed; a small JSON object.
	Summary    json.RawMessage
	StartedBy  string
	StartedAt  time.Time
	FinishedAt *time.Time
	UpdatedAt  time.Time
	// SupersededAt is set on a row a later attempt replaced.
	SupersededAt *time.Time
}

// TestSpecSummary is the part of Summary the screen reads.
type TestSpecSummary struct {
	Feature string `json:"feature"`
	Version int    `json:"version"`
	Status  string `json:"status"`
	Counts  struct {
		TestRequirement int `json:"TestRequirement"`
		TestCase        int `json:"TestCase"`
		TestGap         int `json:"TestGap"`
	} `json:"counts"`
	Files []string       `json:"files"`
	Gaps  []TestSpecGap  `json:"gaps"`
	Notes []string       `json:"notes"`
	Extra map[string]any `json:"context,omitempty"`
}

// TestSpecGap is a question the test specs could not answer from the sources.
type TestSpecGap struct {
	Code     string `json:"code"`
	Missing  string `json:"missing"`
	Question string `json:"question"`
	Owner    string `json:"owner"`
	Blocks   int    `json:"blocks"`
}

// Test-spec documents a person can read, by the name the screen asks for.
const (
	TestSpecDocRequirements = "requirements"
	TestSpecDocDesign       = "design"
	TestSpecDocCases        = "cases"
)

func ValidTestSpecDocument(name string) bool {
	return name == TestSpecDocRequirements || name == TestSpecDocDesign || name == TestSpecDocCases
}

func ErrTestSpecNotConfigured() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_TESTSPEC_NOT_CONFIGURED", "test specs are not enabled in this deployment", nil)
}

func ErrTestSpecNotFound(requestID string) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_TESTSPEC_NOT_FOUND", "the Request has no test specs: "+requestID, nil)
}

func ErrTestSpecDocumentInvalid(name string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_TESTSPEC_DOCUMENT_INVALID", "document must be requirements, design or cases, got "+name, nil)
}

func ErrTestSpecWrongStatus(status RequestStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_TESTSPEC_WRONG_STATUS",
		"test specs run from planning on, while a Request is planned, executing or after it completed; this one is "+string(status), nil)
}

// TestSpecLaneStatus is true for the statuses in which the lane may start: it starts with Plan generation (planning) and
// stays available while the Plan is approved and executed.
func TestSpecLaneStatus(s RequestStatus) bool {
	return s == RequestStatusPlanning || s == RequestStatusAwaitingPlanApproval || s == RequestStatusExecuting
}
