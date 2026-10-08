/**
 * source-control-quality-gate-slot.tsx — FE-CV-TASK-085-06
 *
 * Self-contained hook + notice for call sites that only know the worktree
 * (ChecksPanel). Renders nothing when the quality flag is off or the notice is hidden.
 *
 * @module components/right-sidebar/source-control-quality-gate-slot
 */

import { useAppStore } from '@/store'
import { SourceControlQualityGateNotice } from './source-control-quality-gate-notice'
import { useSourceControlQualityGate } from './use-source-control-quality-gate'

export function SourceControlQualityGateSlot({
  worktreeId,
  projectId,
  provider
}: {
  worktreeId: string | null
  projectId: string | null | undefined
  provider?: 'github' | 'gitlab' | 'other' | null
}): React.JSX.Element | null {
  const headOid = useAppStore((s) =>
    worktreeId ? (s.gitBranchCompareSummaryByWorktree[worktreeId]?.headOid ?? null) : null
  )
  const base = useAppStore((s) => {
    const summary = worktreeId ? s.gitBranchCompareSummaryByWorktree[worktreeId] : undefined
    return summary?.status === 'ready' ? summary.baseRef : null
  })
  const gate = useSourceControlQualityGate({
    worktreeId: worktreeId ?? '',
    projectId,
    headOid,
    base
  })

  if (!gate.viewModel.visible) {
    return null
  }
  return (
    <SourceControlQualityGateNotice
      viewModel={gate.viewModel}
      isLoading={gate.isLoading || gate.running}
      timedOut={gate.timedOut}
      canRunChecks={Boolean(projectId)}
      onRunChecks={gate.runChecks}
      onOpenReason={gate.openReason}
      provider={provider}
    />
  )
}
