import { useState } from 'react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { useAppStore } from '@/store'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { parseMcpError } from '@/runtime/runtime-mcp-error'
import { McpInlineAlert } from './McpListStates'

/** Slot content for MCP_ADMIN_ENABLE_CARD: lets an admin switch MCP on when it is off. */
export function McpAdminEnableCard(): React.JSX.Element {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const enable = async (): Promise<void> => {
    setBusy(true)
    setError(null)
    try {
      await mcpClient.call('mcp.admin.settings.set', { enabled: true })
      await useAppStore.getState().refreshMcpServerInfo()
    } catch (e) {
      setError(parseMcpError(e).detail)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-3 rounded-md border border-border p-4">
      <p className="text-sm text-muted-foreground">
        {translate(
          'auto.mcp.enable.body',
          'Turn MCP on to let people connect AI agents such as Claude Code. You can restrict what agents may do afterwards.'
        )}
      </p>
      {error ? <McpInlineAlert message={error} /> : null}
      <Button size="sm" disabled={busy} aria-busy={busy} onClick={() => void enable()}>
        {translate('auto.mcp.enable.action', 'Turn on MCP')}
      </Button>
    </div>
  )
}

export default McpAdminEnableCard
