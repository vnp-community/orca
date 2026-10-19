// Package agentrelay is the one place Go services build, send and read the two dev-server agent RPCs that run the AI
// CLI: agent.execPrompt (an agent run in a workspace) and ai.complete (a text completion). Both travel through
// infra-fleet-service's Relay or RelayByDevServer RPC; this package picks between them from a Target.
//
// Wire contract: JSON field names and omitempty rules match the agent (agent/src/relay/agent-print-mode-exec.ts,
// agent-exec-prompt-options.ts, ai-complete-handler.ts). Request-scoped data (tenant, user) is attached by the
// WithMetadata option, so this package imports nothing from a service's internal/ tree.
//
// Live output: the line-by-line agent.execPrompt.output notifications are forwarded by infra-fleet-service on its own
// stream and are neither consumed nor changed here. Callers that want live output subscribe with the same StepID.
//
// ai.complete never carries a credential: the agent resolves the account's key itself, so the params type has no field
// for one and a test keeps it that way.
//
// Prompt and result text never reach logs or error messages from this package (C9); errors carry only the RPC method
// and the cause reported by the relay.
package agentrelay
