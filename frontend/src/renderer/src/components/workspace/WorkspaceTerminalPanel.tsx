// WorkspaceTerminalPanel.tsx — bridges the Workspace terminal panel (F38) to
// the app's real terminal-pane/PTY infra, instead of a separate PTY stack.
// See docs/guides/project-workspace-f38-doc-vs-code.md §4 step 5.
import { useEffect } from 'react'
import { Loader2 } from 'lucide-react'
import { useShallow } from 'zustand/react/shallow'
import { useAppStore } from '../../store'
import TerminalPane from '../terminal-pane/TerminalPane'
import { createNewTerminalTab, closeTerminalTab } from '../terminal/terminal-tab-actions'

type WorkspaceTerminalPanelProps = {
  worktreeId: string
}

export function WorkspaceTerminalPanel({ worktreeId }: WorkspaceTerminalPanelProps) {
  const tabIds = useAppStore(
    useShallow((s) => (s.tabsByWorktree[worktreeId] ?? []).map((tab) => tab.id))
  )
  const tabId = tabIds[0]

  // Why: a worktree opened only through Workspace (never activated in the
  // main sidebar) has no terminal tab yet — reuse the same tab-creation path
  // the sidebar uses (createNewTerminalTab), not a new PTY spawn path.
  useEffect(() => {
    if (tabIds.length === 0) {
      createNewTerminalTab(worktreeId)
    }
  }, [worktreeId, tabIds.length])

  if (!tabId) {
    return (
      <div className="flex h-full items-center justify-center gap-2 text-xs text-muted-foreground">
        <Loader2 size={12} className="animate-spin" />
        Starting terminal…
      </div>
    )
  }

  return (
    <TerminalPane
      key={tabId}
      tabId={tabId}
      worktreeId={worktreeId}
      isActive
      onPtyExit={() => closeTerminalTab(tabId)}
      onCloseTab={() => closeTerminalTab(tabId)}
    />
  )
}
