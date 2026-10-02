import type { McpRisk } from '../../../shared/mcp-types'

export const isHighRisk = (risk: McpRisk): boolean =>
  risk === 'exec' || risk === 'destructive' || risk === 'admin'
