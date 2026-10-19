package domain

import (
	"fmt"
	"strings"
)

// ContractPromptSuffix tells the agent how to end a contract run: the result block, fenced by the run's nonce, that
// ParseExecutionResult reads. It lives next to the parser so the instructions and the schema cannot drift apart.
func ContractPromptSuffix(nonce string, requirePassingCheck, knowledgeHub bool) string {
	var b strings.Builder
	b.WriteString("\n\nWhen you are done, end your reply with exactly one result block and print the two marker lines only there, each alone on its line:\n")
	fmt.Fprintf(&b, "ORCA_RESULT_BEGIN %s\n{\"schema_version\":1,\"status\":\"done|blocked|failed|needs_info\",\"summary\":\"\",\"files_changed\":[],\"checks_run\":[{\"id\":\"the command you ran\",\"exit\":0}],\"questions\":[],\"notes\":\"\"", nonce)
	if knowledgeHub {
		b.WriteString(",\"knowledge\":{\"available\":true,\"snapshot\":\"\",\"read\":[],\"gaps\":[],\"conflicts\":[]},\"writeback\":{\"documents\":[],\"proposal_id\":\"\",\"proposal_ok\":false,\"touched\":[]}")
	}
	fmt.Fprintf(&b, "}\nORCA_RESULT_END %s\n", nonce)
	b.WriteString("status is done only when the work is finished; use needs_info with at least one question when you cannot continue without an answer, blocked when something outside the task stops you, failed when you could not do it. No other keys.\n")
	if requirePassingCheck {
		b.WriteString("This task has a Verify step: run it for real and list it in checks_run with its real exit code. A done without a check that exited 0 is not accepted.\n")
	}
	if knowledgeHub {
		b.WriteString("The MCP server tdd-agent is available: use it as the skills tdd-orca-task and tdd-knowledge describe, and fill knowledge and writeback truthfully; leave a list empty rather than guess.\n")
	}
	return b.String()
}
