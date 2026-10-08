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

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

const (
	contractMaxOutputBytes = 1 << 20
	// stdoutScanLimit bounds what the parser sees; the block is printed last so only the tail matters.
	stdoutScanLimit = 2 << 20
	// contractRequestPrefix marks a run started by request-service; only those may run with full trust.
	contractRequestPrefix = "req:"
)

type resultBlockParam struct {
	Nonce string `json:"nonce"`
}

type agentParsedResult struct {
	OK     bool            `json:"ok"`
	Value  json.RawMessage `json:"value,omitempty"`
	Code   string          `json:"code,omitempty"`
	Detail string          `json:"detail,omitempty"`
}

type contractRunOptions struct{ nonce string }

func (o *contractRunOptions) apply(p *agentExecPromptParams) {
	p.ResultBlock = &resultBlockParam{Nonce: o.nonce}
	p.ReportChanges = true
	p.MaxOutputBytes = contractMaxOutputBytes
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
	if strings.TrimSpace(in.Prompt) == "" {
		return usecase.ContractExecuteOutput{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_EXECUTE_PROMPT_REQUIRED", "a contract run needs the prompt built from the task spec", nil)
	}
	// Full trust is acceptable only because request-service gates readiness and verifies afterwards.
	if !strings.HasPrefix(in.RequestID, contractRequestPrefix) {
		return usecase.ContractExecuteOutput{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_CONTRACT_REQUIRES_REQUEST", "contract runs must come from a request execution", nil)
	}
	result, runErr := s.runAgentPrompt(ctx, in.TenantID, in.TaskID, in.RequestID, in.WorktreePath, in.Prompt, &contractRunOptions{nonce: in.ResultNonce})
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
	class, code := domain.ClassifyRunFailure(result.TimedOut, result.ExitCode, parsed, runErr)

	rec, insertErr := s.records.InsertExecutionRecord(ctx, domain.ExecutionRecord{
		TenantID: in.TenantID, TaskID: in.TaskID, ExecutionLinkID: in.ExecutionLinkID, Attempt: max(in.Attempt, 1),
		SpecDigest: in.SpecDigest, PacketDigest: in.PacketDigest, TemplateVersion: in.TemplateVersion,
		ParseStatus: parsed.Status, FailureClass: class,
		Result: domain.StorableJSON(parsed.Raw), Changes: domain.StorableJSON(result.Changes),
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

var _ usecase.ContractAgentExecutor = (*SimpleExecutor)(nil)
