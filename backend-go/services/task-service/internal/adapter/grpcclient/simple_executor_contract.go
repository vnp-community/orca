package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/agentrelay"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

const (
	// stdoutScanLimit bounds what the parser sees; the block is printed last so only the tail matters.
	stdoutScanLimit = 2 << 20
	// contractRequestPrefix marks a run started by request-service; only those may run with full trust.
	contractRequestPrefix = "req:"
)

type contractRunOptions struct {
	nonce                                string
	useKnowledgeHub, requirePassingCheck bool
}

// execParams builds the agent.execPrompt params: a nil receiver is a plain run, otherwise a contract run (result block,
// change report, and the long timeout a task that writes code and runs its checks needs; the agent's five minute
// default cut those off twice in a row and sent the whole Request back to the backlog).
func (o *contractRunOptions) execParams(run agentrelay.Run, repoRoot string) agentrelay.ExecParams {
	if o == nil {
		return agentrelay.Plain(run)
	}
	run.ResultNonce = o.nonce
	// The repository root, not the task's worktree: the graph is built for the project's checkout, and the root comes
	// from the project's connection, never from the caller.
	if o.useKnowledgeHub && repoRoot != "" {
		run.KnowledgeRoot = repoRoot
	}
	return agentrelay.Contract(run)
}

// relayError marks failures of the relay call itself, as opposed to preparation errors, so the
// contract path can classify them without parsing messages. Its text is the wrapped error's.
type relayError struct{ err error }

func (e *relayError) Error() string { return e.err.Error() }
func (e *relayError) Unwrap() error { return e.err }

// Transient reports peer-unavailable or deadline failures, which are worth retrying.
func (e *relayError) Transient() bool {
	switch status.Code(e.err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	}
	return errors.Is(e.err, context.DeadlineExceeded)
}

// execPromptError marks what agentrelay reports as a relay failure. A result the agent sent but we cannot read, and a
// parameter combination agentrelay refuses before sending, are not relay failures: the first is a protocol fault and the
// second a bug here, and neither is worth a retry.
func execPromptError(err error) error {
	switch {
	case errors.Is(err, agentrelay.ErrMalformedResult):
		return fmt.Errorf("simple_executor: decode agent.execPrompt result: %w", err)
	case errors.Is(err, agentrelay.ErrConflictingAccess), errors.Is(err, agentrelay.ErrProfileRequired), errors.Is(err, agentrelay.ErrProposeWithoutRoot):
		return fmt.Errorf("simple_executor: build agent.execPrompt params: %w", err)
	}
	return &relayError{fmt.Errorf("simple_executor: relay agent.execPrompt: %w", err)}
}

// WithExecutionRecords enables persisting contract-run records; without it ExecuteWithContract refuses to run.
func (s *SimpleExecutor) WithExecutionRecords(r usecase.TaskExecutionRecordRepository) *SimpleExecutor {
	s.records = r
	return s
}

// ExecuteWithContract runs the caller's prompt with a result block nonce, parses the structured
// result and stores exactly one record per run. Any outcome other than a clean `done` is returned
// as *usecase.ExecutionFailure carrying the failure class and record id.
func (s *SimpleExecutor) ExecuteWithContract(ctx context.Context, in usecase.ContractExecuteInput) (usecase.ContractExecuteOutput, error) {
	if s.records == nil {
		return usecase.ContractExecuteOutput{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_CONTRACT_UNAVAILABLE", "execution records are not configured", nil)
	}
	// Full trust is acceptable only because request-service gates readiness and verifies afterwards.
	if !strings.HasPrefix(in.RequestID, contractRequestPrefix) {
		return usecase.ContractExecuteOutput{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_CONTRACT_REQUIRES_REQUEST", "contract runs must come from a request execution", nil)
	}
	result, runErr := s.runAgentPrompt(ctx, in.TenantID, in.TaskID, in.RequestID, in.WorktreePath, in.Prompt, &contractRunOptions{nonce: in.ResultNonce, useKnowledgeHub: in.UseKnowledgeHub, requirePassingCheck: in.RequirePassingCheck})
	var relayErr *relayError
	if runErr != nil && !errors.As(runErr, &relayErr) {
		return usecase.ContractExecuteOutput{}, runErr
	}

	stdout := result.Stdout
	if len(stdout) > stdoutScanLimit {
		stdout = stdout[len(stdout)-stdoutScanLimit:]
	}
	var agentParsed *domain.AgentParsed
	if result.Parsed != nil {
		agentParsed = &domain.AgentParsed{OK: result.Parsed.OK, Value: result.Parsed.Value, Code: result.Parsed.Code, Detail: result.Parsed.Detail}
	}
	parsed := domain.ParseExecutionResult(stdout, in.ResultNonce, agentParsed)
	if relayErr != nil {
		parsed = domain.ParsedExecution{Status: domain.ParseStatusMissing, Code: domain.ResultCodeBlockMissing}
	}
	class, code := domain.ClassifyRunFailureChecked(result.TimedOut, result.ExitCode, parsed, runErr, in.RequirePassingCheck)

	rec, insertErr := s.records.InsertExecutionRecord(ctx, domain.ExecutionRecord{
		TenantID: in.TenantID, TaskID: in.TaskID, ExecutionLinkID: in.ExecutionLinkID, Attempt: max(in.Attempt, 1),
		SpecDigest: in.SpecDigest, PacketDigest: in.PacketDigest, TemplateVersion: in.TemplateVersion,
		ParseStatus: parsed.Status, FailureClass: class,
		Result: domain.StorableJSON(parsed.Raw), Changes: domain.StorableJSON(changesJSON(ctx, in.TaskID, result.Changes)),
		StdoutTail: domain.TailUTF8(stdout, domain.MaxStdoutTailBytes),
	})
	if insertErr != nil {
		// The run's own verdict must not be masked by a bookkeeping failure.
		slog.WarnContext(ctx, "task: execution record not stored", slog.String("task_id", in.TaskID), slog.Any("error", insertErr))
		rec = domain.ExecutionRecord{}
	}
	if class != "" {
		return usecase.ContractExecuteOutput{Record: rec}, &usecase.ExecutionFailure{Class: class, Code: code, RecordID: rec.ID, Cause: runErr}
	}
	return usecase.ContractExecuteOutput{ExecutionRef: fmt.Sprintf("task-exec:%s:%s", in.TaskID, in.RequestID), Record: rec}, nil
}

// changesJSON stores the agent's before/after report as the typed value agentrelay decoded. Marshalling that struct
// cannot fail; the log line keeps it from ever being a silent gap in the record if it did.
func changesJSON(ctx context.Context, taskID string, c *agentrelay.Changes) []byte {
	if c == nil {
		return nil
	}
	raw, err := json.Marshal(c)
	if err != nil {
		slog.WarnContext(ctx, "task: agent change report not encoded", slog.String("task_id", taskID), slog.Any("error", err))
		return nil
	}
	return raw
}

var _ usecase.ContractAgentExecutor = (*SimpleExecutor)(nil)
