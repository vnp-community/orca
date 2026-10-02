import { useState } from 'react'
import { PowerOffIcon } from 'lucide-react'
import { toast } from 'sonner'
import { DropdownMenuItem, DropdownMenuSeparator } from '@/components/ui/dropdown-menu'
import { useConfirmationDialog } from '@/components/confirmation-dialog'
import { useAppStore } from '@/store'
import { selectMcpOriginForTab } from '@/store/slices/mcp-terminal-origin'
import { useStopMcpTerminal } from '@/hooks/useStopMcpTerminal'
import { ptyIdToOriginKey, displayMcpClientName } from '@/lib/mcp-terminal-origin'
import { translate } from '@/i18n/i18n'
import type { McpOrigin } from '../../../../shared/mcp-types'

function StopItem({ tabId, origin }: { tabId: string; origin: McpOrigin }): React.JSX.Element {
  // The confirmation hook is only called here so tabs without origin work without a provider.
  const confirm = useConfirmationDialog()
  const stop = useStopMcpTerminal()
  const [busy, setBusy] = useState(false)
  const handle = useAppStore((s) => {
    const ptyIds = s.ptyIdsByTabId?.[tabId] ?? []
    return ptyIds.map(ptyIdToOriginKey).find((h) => s.mcpOriginByHandle?.[h] === origin) ?? null
  })
  const onStop = async (): Promise<void> => {
    if (!handle) {
      return
    }
    const name = displayMcpClientName(origin.clientName)
    const ok = await confirm({
      title: translate('auto.mcp.origin.stopConfirmTitle', 'Stop agent-created process?'),
      description: translate(
        'auto.mcp.origin.stopConfirmBody',
        'This process was started by {{name}} through MCP. Stop it?',
        { name }
      ),
      confirmLabel: translate('auto.mcp.origin.stop', 'Stop')
    })
    if (!ok) {
      return
    }
    setBusy(true)
    try {
      await stop(handle)
      toast.success(translate('auto.mcp.origin.stopped', 'Process stopped'))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <>
      <DropdownMenuSeparator />
      <DropdownMenuItem disabled={busy || !handle} onSelect={() => void onStop()}>
        <PowerOffIcon className="size-3.5" />
        {translate('auto.mcp.origin.stopMenu', 'Stop agent-created process')}
      </DropdownMenuItem>
    </>
  )
}

export function McpOriginStopMenuItem({ tabId }: { tabId: string }): React.JSX.Element | null {
  const origin = useAppStore((s) => selectMcpOriginForTab(s, tabId))
  return origin ? <StopItem tabId={tabId} origin={origin} /> : null
}
