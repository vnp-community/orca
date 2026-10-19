// Types cho MCP Proxy peer-invoke connectivity (CR-ORCA-005)
// Mapping từ ws.Client.Call() error cases trong SOL-ORCA-005

export type MCPProxyStatus =
  | 'connected' // HTTP 200 từ mcp-proxy
  | 'unreachable' // network error hoặc HTTP non-200 non-auth
  | 'auth-error' // HTTP 403/401: "no grant for..."
  | 'unconfigured' // "peer-invoke not configured: set ORCA_MCP_PROXY_URL"
  | 'checking'

export interface MCPProxyStatusInfo {
  status: MCPProxyStatus
  url?: string // ORCA_MCP_PROXY_URL value khi biết
  lastChecked?: number // epoch ms
  error?: string // error message từ backend Call()
  latencyMs?: number // HTTP round-trip time (backend timeout: 30s)
}
