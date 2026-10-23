package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// TestScriptStatus is where the test-script lane of a Request is. The lane starts when execution may start (the Request is executing
// and its test specs completed), runs the Test Script Agent on the dev server next to the coding agent and never changes the
// status of the Request (docs/integrate/TDD-REQ-TESTSCRIPT-001, ORC-TSC-ADR-01, ORC-TSC-ADR-11).
type TestScriptStatus string

const (
	TestScriptPending   TestScriptStatus = "pending"
	TestScriptRunning   TestScriptStatus = "running"
	TestScriptCompleted TestScriptStatus = "completed"
	// TestScriptNeedsReview: the scripts are committed but some files did not pass the spec check after the fix rounds.
	TestScriptNeedsReview TestScriptStatus = "needs_review"
	// TestScriptBlocked is a configuration problem a person fixes before running again; TestScriptFailed is a run that did not finish.
	TestScriptBlocked   TestScriptStatus = "blocked"
	TestScriptFailed    TestScriptStatus = "failed"
	TestScriptSkipped   TestScriptStatus = "skipped"
	TestScriptCancelled TestScriptStatus = "cancelled"
)

// Active is true while the run still needs the reconcile pass.
func (s TestScriptStatus) Active() bool { return s == TestScriptPending || s == TestScriptRunning }

// Finished is true when nothing more will happen to the row unless a person runs it again.
func (s TestScriptStatus) Finished() bool { return !s.Active() }

// Reason codes of a test-script run that did not (or could not) run.
const (
	TestScriptNoTestSpecs          = "REQUEST_TESTSCRIPT_NO_TESTSPECS"
	TestScriptNoDevServer          = "REQUEST_TESTSCRIPT_NO_DEV_SERVER"
	TestScriptToolMissing          = "REQUEST_TESTSCRIPT_TOOL_MISSING"
	TestScriptFrameworkUnsupported = "REQUEST_TESTSCRIPT_FRAMEWORK_UNSUPPORTED"
	TestScriptGenerator            = "REQUEST_TESTSCRIPT_GENERATOR"
	TestScriptTCIRSource           = "REQUEST_TESTSCRIPT_TCIR_SOURCE"
	TestScriptSpecVersionChanged   = "REQUEST_TESTSCRIPT_SPEC_VERSION_CHANGED"
	TestScriptWorktree             = "REQUEST_TESTSCRIPT_WORKTREE"
	TestScriptUnreachable          = "REQUEST_TESTSCRIPT_UNREACHABLE"
	TestScriptDeadline             = "REQUEST_TESTSCRIPT_DEADLINE"
	TestScriptJobLost              = "REQUEST_TESTSCRIPT_JOB_LOST"
	TestScriptFailedCode           = "REQUEST_TESTSCRIPT_FAILED"
	TestScriptActorNotAllowed      = "REQUEST_EXECUTION_ACTOR_NOT_ALLOWED"
)

// TestScriptStepID is the step of the external id sent to the Test Script Agent: with the Request id it makes the start idempotent.
const TestScriptStepID = "testscript"

// TestScriptRun is one attempt to generate the test scripts of a Request.
type TestScriptRun struct {
	ID        string
	TenantID  string
	RequestID string
	Attempt   int
	State     TestScriptStatus
	// ReasonCode and ReasonMessage say why a run is needs_review, blocked, failed, skipped or cancelled.
	ReasonCode    string
	ReasonMessage string
	// TestSpecRunID is the test-spec attempt whose TCIR was read; Feature and SpecVersion name that TCIR.
	TestSpecRunID string
	Feature       string
	SpecVersion   int
	// TSAJobID is the job of the Test Script Agent on the dev server.
	TSAJobID string
	// Branch, BaseRef, WorktreePath and CommitSHA say where the scripts are (a worktree of its own, never the Plan's).
	Branch       string
	BaseRef      string
	WorktreePath string
	CommitSHA    string
	// Summary is the job as tsa_get reported it when it ended: counts, files, report and manifest paths, cost; a small JSON object.
	Summary      json.RawMessage
	StartedBy    string
	StartedAt    time.Time
	FinishedAt   *time.Time
	UpdatedAt    time.Time
	SupersededAt *time.Time
}

// TestScriptCounts are the test cases by outcome.
type TestScriptCounts struct {
	Files       int `json:"files"`
	Generated   int `json:"generated"`
	NeedsReview int `json:"needs-review"`
	Failed      int `json:"failed"`
	Skipped     int `json:"skipped"`
}

// TestScriptFile is one generated file.
type TestScriptFile struct {
	Path     string   `json:"path"`
	Status   string   `json:"status"`
	Attempts int      `json:"attempts"`
	Cases    []string `json:"cases"`
	Errors   int      `json:"errors"`
}

// TestScriptSummary is the part of Summary the screen reads.
type TestScriptSummary struct {
	Counts    TestScriptCounts `json:"counts"`
	Files     []TestScriptFile `json:"files"`
	Report    string           `json:"report"`
	Manifest  string           `json:"manifest"`
	Generator string           `json:"generator"`
	CostUSD   float64          `json:"cost_usd"`
	Seconds   float64          `json:"seconds"`
}

var branchUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// TestScriptBranch is the branch the agent writes to: one per Request, feature and spec version, so two Requests on the same
// feature never share a branch and new test specs never rewrite the scripts of the old ones (ORC-TSC-ADR-07). The short Request id
// keeps it unique when numbers start again after a database reset, so a new Request never continues an old Request's branch.
// specVersion 0 (the test specs did not report one) leaves the suffix out.
func TestScriptBranch(requestNumber int64, requestID, feature string, specVersion int) string {
	b := fmt.Sprintf("tsa/REQ-%d-%s-%s", requestNumber, RequestIDShort(requestID), branchUnsafe.ReplaceAllString(feature, "-"))
	if specVersion > 0 {
		b += fmt.Sprintf("-v%d", specVersion)
	}
	return b
}

func ErrTestScriptNotConfigured() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_TESTSCRIPT_NOT_CONFIGURED", "test scripts are not enabled in this deployment", nil)
}

func ErrTestScriptNotFound(requestID string) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_TESTSCRIPT_NOT_FOUND", "the Request has no generated test scripts: "+requestID, nil)
}

func ErrTestScriptFileInvalid(path string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_TESTSCRIPT_FILE_INVALID", "not an output of the test-script job: "+path, nil)
}

func ErrTestScriptWrongStatus(status RequestStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_TESTSCRIPT_WRONG_STATUS",
		"test scripts are generated while a Request is executing or after it completed; this one is "+string(status), nil)
}

// TestScriptLaneStatus is true for the statuses in which the lane may start: execution has begun (or is over, for a Re-run).
func TestScriptLaneStatus(s RequestStatus) bool { return s == RequestStatusExecuting }
