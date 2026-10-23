package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// TestScriptRef names a test-script row a pass of the reconcile job picked.
type TestScriptRef struct {
	TenantID  string
	RequestID string
	RunID     string
}

// TestScriptRunRepository stores the attempts to generate the test scripts of a Request. Its own table: neither the code lane nor
// the test-spec lane reads or writes it (ORC-TSC-ADR-11). All methods take the tenant from ctx except the two scans.
type TestScriptRunRepository interface {
	Current(ctx context.Context, requestID string) (domain.TestScriptRun, bool, error)
	// List is every attempt, newest first.
	List(ctx context.Context, requestID string) ([]domain.TestScriptRun, error)
	// Create stores run as the current attempt unless one exists; created is false for the loser of a race.
	Create(ctx context.Context, run domain.TestScriptRun) (created bool, err error)
	// Replace supersedes the current attempt (if any) and stores run in its place.
	Replace(ctx context.Context, run domain.TestScriptRun) error
	Update(ctx context.Context, run domain.TestScriptRun) error
	Claim(ctx context.Context, id, owner string, lease time.Duration) (bool, error)
	Release(ctx context.Context, id, owner string) error
	// ListDue picks, across tenants, current rows that still need work and have been quiet for a while.
	ListDue(ctx context.Context, quiet time.Duration, limit int) ([]TestScriptRef, error)
	// ListReadyWithoutRun picks, across tenants, executing Requests whose current test specs completed and that never got a
	// test-script row (both start events lost).
	ListReadyWithoutRun(ctx context.Context, quiet time.Duration, limit int) ([]TestScriptRef, error)
}

// TestScriptStartInput is what tsa_start receives. TCIR is read by the agent itself from tdd-agent on the dev server
// (ORC-TSC-ADR-06), so nothing of the Request's content is sent.
type TestScriptStartInput struct {
	Feature     string
	SpecVersion int
	Branch      string
	BaseRef     string
	TestsPaths  []string
	TestRepo    string
	Generator   string
	Model       string
	ExecutionID string
	StepID      string
	// Attempt keys the TSA job together with ExecutionID: a re-run must regenerate, while a duplicate start of one attempt reuses its job.
	Attempt int
}

// TestScriptStarted is the answer of tsa_start.
type TestScriptStarted struct {
	JobID  string
	Reused bool
	State  string
}

// TestScriptJob is what tsa_get reports about a job.
type TestScriptJob struct {
	State     string
	ErrorCode string
	ErrorText string
	Branch    string
	Worktree  string
	Commit    string
	// Raw is the whole tsa_get answer; stored as the row's summary when the job ends.
	Raw []byte
	// AllGenerated is true when every file passed the spec check.
	AllGenerated bool
}

// TSA job states (tsa_get).
const (
	TSAQueued    = "queued"
	TSARunning   = "running"
	TSADone      = "done"
	TSAError     = "error"
	TSACancelled = "cancelled"
)

// TestScriptText is one page of tsa-report.md.
type TestScriptText struct {
	Path  string
	Total int
	Text  string
	Next  *int
}

// TestScriptAgent drives the Test Script Agent on the dev server of a project (tsa_start, tsa_get, tsa_report, tsa_cancel). Errors
// are the ones of common/tddagent: *ToolError, *BridgeError, or one wrapping ErrTransport.
type TestScriptAgent interface {
	Start(ctx context.Context, conn AnalysisConnection, in TestScriptStartInput) (TestScriptStarted, error)
	Job(ctx context.Context, conn AnalysisConnection, jobID string) (TestScriptJob, error)
	Report(ctx context.Context, conn AnalysisConnection, jobID string, offset int) (TestScriptText, error)
	// File reads one output of the job (a generated script, the manifest, the report); the agent refuses any other path.
	File(ctx context.Context, conn AnalysisConnection, jobID, path string, offset int) (TestScriptText, error)
	Cancel(ctx context.Context, conn AnalysisConnection, jobID, reason string) error
}

// TestScriptOptions are the deployment's choices sent with every start; empty fields leave the dev server's .tsa/config.toml in charge.
type TestScriptOptions struct {
	TestsPaths []string
	TestRepo   string
	Generator  string
	Model      string
}

// TestScriptSettings says whether Requests of a project generate test scripts (the deployment switch today; TestSpecSettingsFunc fits).
type TestScriptSettings interface {
	Enabled(ctx context.Context, projectID string) bool
}
