// src/relay/agent-protocol-features.ts
// Static protocol version and feature declarations for the CR-REQ-033 handshake.
// Only add a feature name AFTER the corresponding implementation exists.
//
// ai.complete.usage promises usage/provider/latencyMs, maxTokens AND error.data
// (one name for three changes per CR spec). agent.execPrompt.readonly promises
// the applied echo field (task 04). agent.execPrompt.resultBlock promises parsed.
// agent.execPrompt.changes promises the changes report. agent.execPrompt.knowledgeHub promises
// knowledgeRoot/proposeChangesets and the skills list of agent.capabilities.
// agent.execPrompt.permissionProfile promises permissionProfile (a named write profile that never needs full trust).
// mcp.callTool promises the bridge of agent-rpc-dispatch-mcp.ts (allowlisted MCP servers over stdio).

export const AGENT_PROTOCOL_VERSION = 2

export const AGENT_FEATURES = [
  'agent.execPrompt',
  'agent.execPrompt.readonly',
  'agent.execPrompt.workspaceKind',
  'agent.execPrompt.changes',
  'agent.execPrompt.resultBlock',
  'agent.execPrompt.knowledgeHub',
  'agent.execPrompt.permissionProfile',
  'agent.capabilities',
  'ai.complete',
  'ai.complete.usage',
  'mcp.callTool'
] as const

export type AgentFeature = (typeof AGENT_FEATURES)[number]
