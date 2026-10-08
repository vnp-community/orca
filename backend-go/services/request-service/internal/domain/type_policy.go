package domain

import "context"

// Labels the type policies read from Task.Labels (written by the plan generator, read again at run time
// because UpdateTask replaces the whole list).
const (
	PolicyLabelGatePreDeploy   = "gate:pre_deploy"
	PolicyLabelRollback        = "rollback"
	PolicyLabelCheckBaseline   = "check:baseline"
	PolicyLabelCheckAfter      = "check:after"
	PolicyLabelCheckTestsPre   = "check:tests_before"
	PolicyLabelCheckTestsAfter = "check:tests_after"
)

// TaskRef is what a policy sees of a task-service Task.
type TaskRef struct {
	ID        string
	Title     string
	ParentID  string
	Status    string
	Labels    []string
	DependsOn []string
}

func (t TaskRef) HasLabel(label string) bool {
	for _, l := range t.Labels {
		if l == label {
			return true
		}
	}
	return false
}

// GateRequirement asks AdvanceExecution to open an Approval and skip the task until it is decided.
type GateRequirement struct {
	SubjectType SubjectType
	SubjectID   string
	Stage       string
}

type CheckVerdictStatus string

const (
	CheckVerdictPassed  CheckVerdictStatus = "passed"
	CheckVerdictFailed  CheckVerdictStatus = "failed"
	CheckVerdictMissing CheckVerdictStatus = "missing"
)

// CheckVerdict is a policy's judgement of one completion condition. Missing waits, failed returns the Request to the backlog.
type CheckVerdict struct {
	Kind    CheckKind
	Stage   string
	Status  CheckVerdictStatus
	Summary string
}

// FollowUp is a child Request a policy wants created once its parent completes.
type FollowUp struct {
	TypeHint        RequestType
	Title           string
	Body            string
	LinkReason      LinkReason
	ClientRequestID string
}

// TypePolicy holds everything that differs per Request type, so the shared execution use cases never switch on the type.
type TypePolicy interface {
	// PlanPreconditions runs when a Plan is proposed and again when it is committed.
	PlanPreconditions(ctx context.Context, req Request, p PlanProposal) error
	// PreExecutionGate returns a gate to open instead of dispatching task, or nil to dispatch.
	PreExecutionGate(ctx context.Context, req Request, task TaskRef) (*GateRequirement, error)
	// CompletionChecks judges the Request before execution_finished; checks are in recording order.
	CompletionChecks(req Request, checks []RequestCheck) ([]CheckVerdict, error)
	// OnCompleted runs after request.completed and may be delivered more than once.
	OnCompleted(req Request) ([]FollowUp, error)
}

// FailureHinter is an optional extension: extra words for the backlog reason when a task fails.
type FailureHinter interface {
	FailureHint(req Request, failed TaskRef, all []TaskRef) string
}

// CheckReader lets a policy read the newest recorded check while it still has a context to do I/O with.
type CheckReader interface {
	LatestCheck(ctx context.Context, requestID string, kind CheckKind) (RequestCheck, bool, error)
}

// ApprovalLookup reads the newest Approval of a subject.
type ApprovalLookup interface {
	LatestApproval(ctx context.Context, requestID string, st SubjectType, subjectID string) (*Approval, error)
}

type PolicyDeps struct {
	Checks    CheckReader
	Approvals ApprovalLookup
}
