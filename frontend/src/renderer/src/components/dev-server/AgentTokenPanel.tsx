// ─── AgentTokenPanel ─────────────────────────────────────────────────────────
//
// Panel hiển thị agentToken + lệnh khởi động agent khi direct-websocket mode.
// Hiển thị sau khi backend emit agentTokenGenerated event.
//
// Usage:
//   <AgentTokenPanel
//     agentToken="agt-ds-123-1722033600"
//     orcaUrl="ws://b15.openledger.vn:6768/agent"
//     waiting={state === 'testing'}
//   />

import { useState } from 'react'
import { Copy, CheckCircle2, Loader2, Terminal } from 'lucide-react'
import { Button } from '../../components/ui/button'
import type { AgentKind } from '../../../../shared/dev-server-types'

type AgentTokenPanelProps = {
  /** One-time authentication token for the agent */
  agentToken: string
  /** Orca WebSocket URL the agent should connect to */
  orcaUrl: string
  /** True while waiting for agent to connect (shows spinner) */
  waiting?: boolean
  /**
   * CR-DS-009 / TASK-EMU-012c: which binary/entry point the displayed
   * command should target. Defaults to 'dev-server' — every call site
   * before this task omitted this prop entirely, so the default preserves
   * the exact command this component already showed.
   */
  agentKind?: AgentKind
}

/**
 * Displays a copyable agent startup command for direct-websocket mode.
 * Shown in AddDevServerDialog after the backend generates an agentToken.
 *
 * Not wired to a live agentTokenGenerated event anywhere today (a
 * pre-existing gap for BOTH agent kinds, not introduced by CR-DS-009) —
 * see AddDevServerDialog.tsx's direct-websocket instructions block for the
 * interim static text shown instead.
 */
export function AgentTokenPanel({
  agentToken,
  orcaUrl,
  waiting,
  agentKind = 'dev-server'
}: AgentTokenPanelProps) {
  const [copied, setCopied] = useState(false)

  const runCommand =
    agentKind === 'mobile-emulator' ? 'node out/emulator.js' : 'node agent.js'
  const envVarName = agentKind === 'mobile-emulator' ? 'ORCA_BACKEND_URL' : 'ORCA_URL'
  const runOnLabel =
    agentKind === 'mobile-emulator' ? 'the machine with Android Studio/Xcode' : 'your dev server'

  // One-liner for clipboard copy
  const commandOneLine = `${envVarName}=${orcaUrl} AGENT_TOKEN=${agentToken} ${runCommand}`
  // Multi-line for display
  const commandDisplay = `${envVarName}=${orcaUrl} \\\n  AGENT_TOKEN=${agentToken} \\\n  ${runCommand}`

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(commandOneLine)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard API may fail in some browser security contexts — silent fallback
    }
  }

  return (
    <div className="rounded-md border bg-muted/50 p-3 space-y-3">
      {/* Header */}
      <div className="flex items-center gap-2">
        {waiting ? (
          <>
            <Loader2 className="size-4 animate-spin text-blue-500 shrink-0" />
            <span className="text-sm font-medium">Waiting for agent to connect…</span>
          </>
        ) : (
          <>
            <CheckCircle2 className="size-4 text-green-500 shrink-0" />
            <span className="text-sm font-medium">Agent token ready</span>
          </>
        )}
      </div>

      {/* Command block */}
      <div className="space-y-1">
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          <Terminal className="size-3 shrink-0" />
          <span>Run on {runOnLabel}:</span>
        </div>
        <div className="relative rounded bg-background border p-2 pr-9">
          <pre className="text-xs font-mono whitespace-pre-wrap break-all leading-5">
            {commandDisplay}
          </pre>
          <Button
            variant="ghost"
            size="icon"
            className="absolute right-1 top-1 h-6 w-6"
            onClick={() => void handleCopy()}
            title={copied ? 'Copied!' : 'Copy command'}
          >
            {copied ? (
              <CheckCircle2 className="size-3 text-green-500" />
            ) : (
              <Copy className="size-3" />
            )}
          </Button>
        </div>
      </div>

      {/* Timeout warning — only while waiting */}
      {waiting && (
        <p className="text-xs text-muted-foreground">
          Token expires in 60s. Start the agent before time runs out.
          If it times out, click <strong>Test Connection</strong> again.
        </p>
      )}
    </div>
  )
}
