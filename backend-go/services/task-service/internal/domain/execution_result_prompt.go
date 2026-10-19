package domain

import "github.com/stablyai/orca-go/prompts/task_execution"

// ContractPromptSuffix tells the agent how to end a contract run: the result block, fenced by the run's nonce, that
// ParseExecutionResult reads. The wording is a template in backend-go/prompts; this keeps the call next to the parser.
func ContractPromptSuffix(nonce string, requirePassingCheck, knowledgeHub bool) (string, error) {
	return task_execution.RenderResultBlock(task_execution.ResultBlockInput{Nonce: nonce, RequirePassingCheck: requirePassingCheck, KnowledgeHub: knowledgeHub})
}
