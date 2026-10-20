package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// TestSpecRef names a test-spec row a pass of the reconcile job picked: the tenant to run it under, the Request it belongs to.
type TestSpecRef struct {
	TenantID  string
	RequestID string
	RunID     string
}

// TestSpecRunRepository stores the attempts to generate the test specs of a Request. It is its own table: the code-execution
// lane reads and writes none of it (ORC-TS-ADR-11). All methods take the tenant from ctx except the two scans.
type TestSpecRunRepository interface {
	// Current is the attempt the screen shows; false when the Request never had one.
	Current(ctx context.Context, requestID string) (domain.TestSpecRun, bool, error)
	// List is every attempt, newest first.
	List(ctx context.Context, requestID string) ([]domain.TestSpecRun, error)
	// Create stores run as the current attempt unless one exists; created is false for the loser of a race, so a duplicate
	// event or a reconcile pass cannot start a second run.
	Create(ctx context.Context, run domain.TestSpecRun) (created bool, err error)
	// Replace supersedes the current attempt (if any) and stores run in its place (a person pressed Re-run).
	Replace(ctx context.Context, run domain.TestSpecRun) error
	// Update writes the changeable columns of run by id and moves updated_at.
	Update(ctx context.Context, run domain.TestSpecRun) error
	// Claim takes a lease on the row (compare-and-set): false means another replica holds it.
	Claim(ctx context.Context, id, owner string, lease time.Duration) (bool, error)
	Release(ctx context.Context, id, owner string) error
	// ListDue picks, across tenants, current rows that still need work and have been quiet for a while. It only picks candidates;
	// Claim is what keeps replicas apart.
	ListDue(ctx context.Context, quiet time.Duration, limit int) ([]TestSpecRef, error)
	// ListExecutingWithoutRun picks, across tenants, executing Requests that never got a test-spec row (a lost event).
	ListExecutingWithoutRun(ctx context.Context, quiet time.Duration, limit int) ([]TestSpecRef, error)
}

// TestSpecConfig is what the dev server's tdd-agent says about test specs (testspec_get with no argument).
type TestSpecConfig struct {
	Enabled bool   `json:"enabled"`
	LLM     string `json:"llm"`
	// ContextMode is none, solution or plan: the dev server's own choice (INT-ADR-17 of tdd-agent), never Orca's.
	ContextMode string `json:"context_mode"`
	ContextPath string `json:"context_path"`
	SpecsDir    string `json:"specs_dir"`
}

// TestSpecStartInput is the document sent with testspec_start: Document for a Solution, Files for the files of an OpenSpec change.
type TestSpecStartInput struct {
	Kind     string
	Ref      string
	Document json.RawMessage
	Files    map[string]string
	// External lets the call be repeated: the same execution, step and content return the same run.
	ExecutionID string
	StepID      string
	UserID      string
}

// TestSpecStarted is the answer of testspec_start.
type TestSpecStarted struct {
	RunID  string
	Reused bool
	Mode   string
	Ref    string
	Path   string
	SHA256 string
	Notes  []string
}

// TestSpecRunState is the part of run_get the reconcile pass reads.
type TestSpecRunState struct {
	Status       string
	Blockers     []string
	ProcessAlive bool
}

// TestSpecText is one page of a test-spec document.
type TestSpecText struct {
	Document string
	Version  int
	Status   string
	Length   int
	Text     string
	// Next is the offset of the next page; nil on the last one.
	Next *int
}

// TestSpecAgent drives tdd-agent's test-spec tools on the dev server of a project (testspec_get, testspec_start, run_dispatch,
// run_get, run_cancel). Errors are the ones of common/tddagent: *ToolError, *BridgeError, or one wrapping ErrTransport.
type TestSpecAgent interface {
	Config(ctx context.Context, conn AnalysisConnection) (TestSpecConfig, error)
	Start(ctx context.Context, conn AnalysisConnection, in TestSpecStartInput) (TestSpecStarted, error)
	Dispatch(ctx context.Context, conn AnalysisConnection, runID string) error
	Run(ctx context.Context, conn AnalysisConnection, runID string) (TestSpecRunState, error)
	// Features lists the features that have test specs on the dev server (testspec_get with no argument).
	Features(ctx context.Context, conn AnalysisConnection) ([]string, error)
	// Summary is the snapshot of testspec_get(feature): counts, files, gaps, the context of the last run.
	Summary(ctx context.Context, conn AnalysisConnection, feature string) (json.RawMessage, error)
	Document(ctx context.Context, conn AnalysisConnection, feature, document string, offset int) (TestSpecText, error)
	Cancel(ctx context.Context, conn AnalysisConnection, runID, reason string) error
}

// TestSpecSettings says whether Requests of a project generate test specs. Today it is the deployment switch; a per-project
// setting (ORC-TS-T5) plugs in here without touching the lane.
type TestSpecSettings interface {
	Enabled(ctx context.Context, projectID string) bool
}

// TestSpecSettingsFunc adapts a function.
type TestSpecSettingsFunc func(ctx context.Context, projectID string) bool

func (f TestSpecSettingsFunc) Enabled(ctx context.Context, projectID string) bool {
	return f(ctx, projectID)
}

// TestSpecRedactor cleans text on its way to the dev server (the PromptRedactor of the AI path).
type TestSpecRedactor interface {
	Apply(ctx context.Context, text string) string
}
