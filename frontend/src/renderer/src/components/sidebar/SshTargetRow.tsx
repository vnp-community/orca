/**
 * Row used in the "Open project on SSH host" step to pick an SSH target.
 *
 * Why extracted: keeps AddRepoSteps.tsx under the 400-line oxlint limit
 * while isolating the inline-connect interaction logic.
 */
import React, { useCallback, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import type { SshTarget, SshConnectionState } from '../../../../shared/ssh-types'
import { translate } from '@/i18n/i18n'
import { SshUserIndicator } from '../ssh/SshUserIndicator'
import { useSshUserAccount } from '../../hooks/useSshUserAccount'
import { useSshProvisioning } from '../../hooks/useSshProvisioning'
import { useAuthUser } from '../../hooks/useAuthSession'
import { useAppStore } from '@/store'


type Props = {
  target: SshTarget & { state?: SshConnectionState }
  isSelected: boolean
  onSelect: (id: string) => void
  onConnect: (id: string) => Promise<void>
}

export function SshTargetRow({
  target,
  isSelected,
  onSelect,
  onConnect
}: Props): React.JSX.Element {
  const [connecting, setConnecting] = useState(false)

  const authUser = useAuthUser()
  const { linuxUsername, provisioned, previewUsername } = useSshUserAccount(
    target.id,
    { previewFromEmail: authUser?.email }
  )
  const provisioningStatus = useAppStore(
    s => s.sshUserAccounts.get(target.id)?.provisioningStatus ?? { phase: 'idle' as const }
  )
  useSshProvisioning(target.id)

  const mountedRef = useRef(true)
  const status = target.state?.status ?? 'disconnected'
  const isConnected = status === 'connected'
  const isBusy =
    connecting ||
    status === 'connecting' ||
    status === 'deploying-relay' ||
    status === 'reconnecting'
  const dotColor = isConnected
    ? 'bg-green-500'
    : isBusy
      ? 'bg-yellow-500'
      : 'bg-muted-foreground/30'

  const handleRowClick = (): void => {
    if (isConnected) {
      onSelect(target.id)
    }
  }

  const handleConnectClick = (e: React.MouseEvent): void => {
    // Why: prevent the row's onClick from also firing and treating the click
    // as a selection when the target is disconnected.
    e.stopPropagation()
    if (isBusy) {
      return
    }
    setConnecting(true)
    void onConnect(target.id).finally(() => {
      if (mountedRef.current) {
        setConnecting(false)
      }
    })
  }

  const handleRowRootRef = useCallback((node: HTMLDivElement | null): void => {
    // Why: SSH connects can resolve after this row is removed; the row ref
    // gives the async completion the same guard without a mount-only Effect.
    mountedRef.current = node !== null
  }, [])

  return (
    <div
      ref={handleRowRootRef}
      role={isConnected ? 'button' : undefined}
      tabIndex={isConnected ? 0 : undefined}
      className={`w-full flex flex-col px-3 py-2 rounded-md border text-xs transition-colors ${
        isSelected ? 'border-foreground/30 bg-accent' : 'border-border hover:bg-accent/50'
      } ${isConnected ? 'cursor-pointer' : ''}`}
      onClick={handleRowClick}
      onKeyDown={(e) => {
        if (isConnected && (e.key === 'Enter' || e.key === ' ')) {
          e.preventDefault()
          onSelect(target.id)
        }
      }}
    >
      <div className="flex items-center gap-2 w-full">
        <span className={`size-2 rounded-full shrink-0 ${dotColor}`} />
      <span className={`font-medium truncate ${!isConnected ? 'text-muted-foreground' : ''}`}>
        {target.label || `${target.username}@${target.host}`}
      </span>
      {!isConnected && (
        // Why: inline Connect avoids forcing the user out to Settings just to
        // bring up a configured target.
        <button
          type="button"
          className="ml-auto shrink-0 rounded px-1.5 py-0.5 text-[11px] font-medium text-foreground hover:bg-accent/70 disabled:opacity-50 disabled:cursor-default flex items-center gap-1"
          onClick={handleConnectClick}
          disabled={isBusy}
        >
          {isBusy ? (
            <>
              <Loader2 className="size-3 animate-spin" />
              {translate('auto.components.sidebar.SshTargetRow.4677394048', 'Connecting…')}
            </>
          ) : (
            translate('auto.components.sidebar.SshTargetRow.75ad429b5d', 'Connect')
          )}
        </button>
      )}
      </div>
      {import.meta.env.ORCA_PLATFORM === 'web' && (
        <div className="w-full border-t border-border/50 pt-1 mt-1">
          <SshUserIndicator
            serverId={target.id}
            linuxUsername={linuxUsername ?? previewUsername ?? 'orca-?'}
            provisioned={provisioned}
            provisioningStatus={provisioningStatus}
          />
        </div>
      )}
    </div>
  )
}
