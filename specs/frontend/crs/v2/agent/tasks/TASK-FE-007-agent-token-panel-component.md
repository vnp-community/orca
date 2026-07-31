# TASK-FE-007 — Tạo `AgentTokenPanel.tsx` component

**Solution:** [SOL-FE-AG-002](../solutions/SOL-FE-AG-002-direct-websocket-token-ui.md)  
**File:** `src/renderer/src/components/dev-server/AgentTokenPanel.tsx` [NEW]  
**Depends on:** TASK-FE-006 (useAddDevServer returns agentToken)  
**Status:** ✅ DONE (2026-07-26)  

---

## Mục tiêu

Tạo component mới `AgentTokenPanel` hiển thị agentToken + lệnh khởi động agent khi `direct-websocket` mode được sử dụng.

---

## Code cần tạo

### File: `src/renderer/src/components/dev-server/AgentTokenPanel.tsx`

```tsx
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
import { Button } from '@/components/ui/button'

type AgentTokenPanelProps = {
  /** One-time authentication token for the agent */
  agentToken: string
  /** Orca WebSocket URL the agent should connect to */
  orcaUrl: string
  /** True while waiting for agent to connect (spinner mode) */
  waiting?: boolean
}

export function AgentTokenPanel({ agentToken, orcaUrl, waiting }: AgentTokenPanelProps) {
  const [copied, setCopied] = useState(false)

  const command = `ORCA_URL=${orcaUrl} \\\n  AGENT_TOKEN=${agentToken} \\\n  node agent.js`

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(
        `ORCA_URL=${orcaUrl} AGENT_TOKEN=${agentToken} node agent.js`
      )
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard API may fail in some environments
    }
  }

  return (
    <div className="rounded-md border bg-muted/50 p-3 space-y-3">
      {/* Header */}
      <div className="flex items-center gap-2">
        {waiting ? (
          <>
            <Loader2 className="size-4 animate-spin text-blue-500 shrink-0" />
            <span className="text-sm font-medium">
              Waiting for agent to connect…
            </span>
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
          <Terminal className="size-3" />
          <span>Run on your dev server:</span>
        </div>
        <div className="relative rounded bg-background border p-2 pr-9">
          <pre className="text-xs font-mono whitespace-pre-wrap break-all">
            {command}
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

      {/* Timeout warning */}
      {waiting && (
        <p className="text-xs text-muted-foreground">
          Token expires in 60s. Start the agent before time runs out.
          If it times out, click <strong>Test Connection</strong> again.
        </p>
      )}
    </div>
  )
}
```

---

## Acceptance Criteria

- [x] File tạo tại `src/renderer/src/components/dev-server/AgentTokenPanel.tsx`
- [x] `agentToken` và `orcaUrl` props hiển thị trong command block
- [x] Copy button: click → clipboard write → icon chuyển thành checkmark 2s
- [x] `waiting=true` → Loader2 spinner + timeout warning text
- [x] `waiting=false` → CheckCircle2 + "Agent token ready"
- [x] Command format: `ORCA_URL=... AGENT_TOKEN=... node agent.js`
- [x] TypeScript compile không lỗi
- [x] Không import `electron`
