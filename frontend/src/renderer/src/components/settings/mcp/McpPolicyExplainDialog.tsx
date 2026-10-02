import { useState } from 'react'
import type { McpOAuthClient, McpRpcResult, McpToolView } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { parseMcpError } from '@/runtime/runtime-mcp-error'
import { McpInlineAlert } from './McpListStates'
import { mcpDecisionLabel } from './McpPolicyEditorDialog'
import { reasonLabel } from './mcp-policy-reasons'

const ANY = '__any__'

export function McpPolicyExplainDialog({
  tools,
  clients,
  initialTool = '',
  onClose
}: {
  tools: McpToolView[]
  clients: McpOAuthClient[]
  initialTool?: string
  onClose: () => void
}): React.JSX.Element {
  const [tool, setTool] = useState(initialTool)
  const [userId, setUserId] = useState('')
  const [clientId, setClientId] = useState(ANY)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<McpRpcResult<'mcp.admin.policy.explain'> | null>(null)

  const run = async (): Promise<void> => {
    setBusy(true)
    setError(null)
    setResult(null)
    try {
      setResult(
        await mcpClient.call('mcp.admin.policy.explain', {
          tool: tool.trim(),
          ...(userId.trim() ? { userId: userId.trim() } : {}),
          ...(clientId !== ANY ? { clientId } : {})
        })
      )
    } catch (e) {
      const err = parseMcpError(e)
      setError(
        err.code === 'MCP_NOT_FOUND'
          ? translate('auto.mcp.policies.unknownTool', 'Unknown tool')
          : err.detail
      )
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => (!o ? onClose() : undefined)}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {translate('auto.mcp.policies.explainTitle', 'Explain a tool call')}
          </DialogTitle>
          <DialogDescription>
            {translate(
              'auto.mcp.policies.explainDesc',
              'See what would happen if this tool were called.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-1">
            <Label htmlFor="mcp-exp-tool">{translate('auto.mcp.policies.tool', 'Tool')}</Label>
            <Input
              id="mcp-exp-tool"
              list="mcp-exp-tools"
              className="font-mono"
              value={tool}
              onChange={(e) => setTool(e.target.value)}
            />
            <datalist id="mcp-exp-tools">
              {tools.map((t) => (
                <option key={t.name} value={t.name} />
              ))}
            </datalist>
          </div>
          <div className="space-y-1">
            <Label htmlFor="mcp-exp-user">{translate('auto.mcp.policies.userId', 'User ID')}</Label>
            <Input
              id="mcp-exp-user"
              value={userId}
              aria-describedby="mcp-exp-user-help"
              onChange={(e) => setUserId(e.target.value)}
            />
            <p id="mcp-exp-user-help" className="text-xs text-muted-foreground">
              {translate(
                'auto.mcp.policies.userIdHelp',
                'Leave empty to evaluate as a regular user.'
              )}
            </p>
          </div>
          <div className="space-y-1">
            <Label htmlFor="mcp-exp-client">
              {translate('auto.mcp.policies.client', 'Client')}
            </Label>
            <Select value={clientId} onValueChange={setClientId}>
              <SelectTrigger id="mcp-exp-client" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ANY}>{translate('auto.mcp.policies.any', 'Any')}</SelectItem>
                {clients.map((c) => (
                  <SelectItem key={c.clientId} value={c.clientId}>
                    {c.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {error ? <McpInlineAlert message={error} /> : null}
          {result ? (
            <div className="space-y-2" aria-live="polite">
              <Badge
                variant={
                  result.decision === 'deny'
                    ? 'destructive'
                    : result.decision === 'allow'
                      ? 'secondary'
                      : 'outline'
                }
              >
                {mcpDecisionLabel(result.decision)}
              </Badge>
              <ul className="space-y-1 text-sm">
                {result.reasons.map((r) => {
                  const l = reasonLabel(r)
                  return (
                    <li key={r}>
                      {l.text}
                      {l.text !== l.raw ? (
                        <span className="ml-2 font-mono text-xs text-muted-foreground">
                          {l.raw}
                        </span>
                      ) : null}
                    </li>
                  )
                })}
              </ul>
              <p className="text-xs text-muted-foreground">
                {translate(
                  'auto.mcp.policies.explainNote',
                  "Rate limits and a pending approval's state are not part of this result."
                )}
              </p>
            </div>
          ) : null}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            {translate('auto.mcp.common.close', 'Close')}
          </Button>
          <Button disabled={!tool.trim() || busy} aria-busy={busy} onClick={() => void run()}>
            {translate('auto.mcp.policies.explain', 'Explain')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
