package domain

// ExecutionEngine names which of the three dispatch targets ExecuteTask
// selected for one Execute call — see task-service.md §3.1 and
// CR-FLOW-TASK-001.
type ExecutionEngine string

const (
	EngineDirectAgent   ExecutionEngine = "direct_agent"  // Engine 1 — SimpleExecutor, real (TASK-224)
	EngineOrchestration ExecutionEngine = "orchestration" // Engine 2 — ComplexExecutor, real (BE-SOL-002 integration addendum)
	EngineWorkflow      ExecutionEngine = "workflow"      // Engine 3 — WorkflowExecutor, real (TASK-FT-002-03)
)
