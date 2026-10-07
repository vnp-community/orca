// src/relay/agent-protocol-features.ts
// Static protocol version and feature declarations for the CR-REQ-033 handshake.
// Only add a feature name AFTER the corresponding implementation exists.
//
// ai.complete.usage promises usage/provider/latencyMs, maxTokens AND error.data
// (one name for three changes per CR spec). agent.execPrompt.readonly promises
// the applied echo field (task 04). agent.execPrompt.resultBlock promises parsed.
// agent.execPrompt.changes promises the changes report.

export const AGENT_PROTOCOL_VERSION = 2

export const AGENT_FEATURES = [
  'agent.execPrompt',
  'agent.execPrompt.readonly',
  'agent.execPrompt.workspaceKind',
  'agent.execPrompt.changes',
  'agent.execPrompt.resultBlock',
  'agent.capabilities',
  'ai.complete',
  'ai.complete.usage'
] as const

export type AgentFeature = (typeof AGENT_FEATURES)[number]
