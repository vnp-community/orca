// src/renderer/src/components/workspace/AgentPanel.tsx
// BUG-FE-ORCH-001: Remote agent start/stop/resume control panel
// Shows agent status badge, agent type selector, and action buttons

import { useState, useEffect, useCallback } from 'react'
import { Play, Square, RotateCcw, Loader2, Bot } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { toast } from 'sonner'
import { useAppStore } from '../../store'
import { useShallow } from 'zustand/react/shallow'
import type {
  RemoteAgentSession,
  RemoteAgentStatus
} from '../../store/slices/remote-agent-sessions'
import { Tracers } from '../../../../shared/trace/tracers'
import {
  registerOpenAgentOrchSpan,
  takeOpenAgentOrchSpan
} from '@/lib/agent-orchestration-active-spans'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useAuthUser } from '../../hooks/useAuthSession'
import { getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { getConnectionId } from '../../lib/connection-context'
import { findWorktreeById } from '../../store/slices/worktree-helpers'
import {
  resolveRuntimeAgentProvider,
  startRuntimeAgentSession,
  resumeRuntimeAgentSession,
  stopRuntimeAgentSession,
  subscribeRuntimeAgentStatus,
  type RuntimeAgentSessionStatus
} from '../../runtime/runtime-agent-orchestration-client'

// Backend AgentSession.status carries more detail (infrafleet.proto's
// documented spawning|idle|running|waiting|completed|error|stopped) than
// this panel's 4-value badge vocabulary — collapse it here rather than
// growing AgentStatusBadge's config for states the UI doesn't act on
// differently yet.
function errorText(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

function toRemoteAgentStatus(status: RuntimeAgentSessionStatus): RemoteAgentStatus {
  switch (status) {
    case 'spawning':
      return 'starting'
    case 'idle':
    case 'running':
    case 'waiting':
      return 'running'
    case 'error':
      return 'error'
    case 'completed':
    case 'stopped':
    default:
      return 'stopped'
  }
}

type AgentPanelProps = {
  worktreeId: string
}

type AgentType = 'claude' | 'codex' | 'custom'
type TrustPreset = 'standard' | 'permissive' | 'strict'

// ─── Status Badge ─────────────────────────────────────────────────────────────

function AgentStatusBadge({ status }: { status: RemoteAgentSession['status'] | undefined }) {
  if (!status || status === 'stopped') {
    return (
      <Badge variant="secondary" className="text-[10px]">
        Idle
      </Badge>
    )
  }
  const configs: Record<string, { label: string; className: string }> = {
    starting: {
      label: 'Starting…',
      className: 'bg-yellow-500/20 text-yellow-600 border-yellow-500/30'
    },
    running: {
      label: 'Running',
      className: 'bg-green-500/20  text-green-600  border-green-500/30'
    },
    stopped: { label: 'Idle', className: '' },
    error: { label: 'Error', className: 'bg-red-500/20    text-red-600    border-red-500/30' }
  }
  const cfg = configs[status] ?? configs.stopped
  return (
    <Badge variant="outline" className={`text-[10px] ${cfg.className}`}>
      {status === 'starting' && <Loader2 size={10} className="animate-spin mr-1" />}
      {cfg.label}
    </Badge>
  )
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function AgentPanel({ worktreeId }: AgentPanelProps) {
  const [agentType, setAgentType] = useState<AgentType>('claude')
  const [trustPreset, setTrustPreset] = useState<TrustPreset>('standard')
  const [isActing, setIsActing] = useState(false)

  const { project } = useWorkspace()
  const currentUser = useAuthUser()

  const { session, setRemoteAgentSession, updateAgentStatus } = useAppStore(
    useShallow((s) => ({
      session: s.remoteAgentSessions[worktreeId] as RemoteAgentSession | undefined,
      setRemoteAgentSession: s.setRemoteAgentSession,
      updateAgentStatus: s.updateAgentStatus
    }))
  )

  // Subscribe to status change events. agent.subscribeStatus is tenant-wide
  // (no worktreeId filter server-side, see channels_agent.go), so this reads
  // the latest session id from the store on every push rather than closing
  // over a stale value from mount time.
  useEffect(() => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    let cancelled = false
    let unsubscribe: (() => void) | null = null
    subscribeRuntimeAgentStatus(target, (event) => {
      const current = useAppStore.getState().remoteAgentSessions[worktreeId]
      if (current?.sessionId && current.sessionId === event.session_id) {
        updateAgentStatus({
          worktreeId,
          sessionId: event.session_id,
          status: toRemoteAgentStatus(event.status)
        })
      }
    })
      .then((handle) => {
        if (cancelled) {
          handle.unsubscribe()
          return
        }
        unsubscribe = handle.unsubscribe
      })
      .catch(() => {
        // Why no toast here: a status-subscribe failure (e.g. no runtime
        // environment connected yet) shouldn't itself surface as an error —
        // startAgent/resumeAgent's own error handling covers the user-facing
        // signal when the same target is unusable.
      })
    return () => {
      cancelled = true
      unsubscribe?.()
    }
  }, [worktreeId, updateAgentStatus])

  const startAgent = useCallback(async () => {
    if (!currentUser) {
      toast.error('Sign in before starting an agent')
      return
    }
    const worktree = findWorktreeById(useAppStore.getState().worktreesByRepo, worktreeId)
    if (!worktree) {
      toast.error('Worktree not found')
      return
    }
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const connectionId = getConnectionId(worktreeId) ?? ''
    setIsActing(true)
    // Optimistic update
    updateAgentStatus({ worktreeId, status: 'starting' })
    // Why: span stays open (no ok() here) — it waits in the registry for the
    // statusChanged push event to confirm 'running' (BL-AG-05, TASK-FE-002.3).
    const span = Tracers.uiAgentOrchSpawnFlow.start({ worktreeId, agentType, trustPreset })
    registerOpenAgentOrchSpan(worktreeId, span)
    try {
      // TASK-AG-01-07: StartAgentSession never resolves its own account/model
      // — the caller must call aiProvider.resolve first.
      const { accountId, modelId } = await resolveRuntimeAgentProvider(target, {
        userId: currentUser.id,
        projectId: project?.id ?? '',
        devServerId: project?.devServerId ?? '',
        agentType
      })
      const { ack: startedSession } = await startRuntimeAgentSession(
        target,
        {
          connectionId,
          worktreeId,
          userId: currentUser.id,
          cwd: worktree.path,
          modelId,
          accountId,
          trustPreset
        },
        () => {
          // Pty output isn't surfaced by this panel yet — see SOL-FE-PW-004
          // "Not done in this spec".
        }
      )
      span.step('rpc-resolved', { sessionId: startedSession.id, status: startedSession.status })
      setRemoteAgentSession(worktreeId, {
        sessionId: startedSession.id,
        worktreeId,
        agentType,
        trustPreset,
        status: toRemoteAgentStatus(startedSession.status),
        startedAt: Date.now()
      })
      // Span stays open, waiting for agent.subscribeStatus's 'running'|'error'
      // to close it (TASK-FE-002.3) — resolveRuntimeAgentProvider/
      // startRuntimeAgentSession's ack alone doesn't confirm the agent
      // actually came up.
    } catch (err: unknown) {
      takeOpenAgentOrchSpan(worktreeId)
      span.fail(err, { worktreeId, agentType })
      updateAgentStatus({ worktreeId, status: 'error', errorMessage: errorText(err) })
      toast.error(`Failed to start agent: ${errorText(err)}`)
    } finally {
      setIsActing(false)
    }
  }, [
    worktreeId,
    agentType,
    trustPreset,
    updateAgentStatus,
    setRemoteAgentSession,
    currentUser,
    project
  ])

  const stopAgent = useCallback(async () => {
    if (!session?.sessionId) {
      return
    }
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    setIsActing(true)
    // Why: stop() is a simple request/response — unlike spawn/resume, 'starting'
    // is never an intermediate status here, so the span closes immediately.
    const span = Tracers.uiAgentOrchStopFlow.start({ worktreeId, sessionId: session.sessionId })
    try {
      await stopRuntimeAgentSession(target, session.sessionId)
      updateAgentStatus({ worktreeId, status: 'stopped' })
      span.ok({ worktreeId, sessionId: session.sessionId })
    } catch (err: unknown) {
      span.fail(err, { worktreeId, sessionId: session.sessionId })
      toast.error(`Failed to stop agent: ${errorText(err)}`)
    } finally {
      setIsActing(false)
    }
  }, [session, worktreeId, updateAgentStatus])

  const resumeAgent = useCallback(async () => {
    if (!session?.sessionId || !currentUser) {
      return
    }
    const worktree = findWorktreeById(useAppStore.getState().worktreesByRepo, worktreeId)
    if (!worktree) {
      toast.error('Worktree not found')
      return
    }
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const connectionId = getConnectionId(worktreeId) ?? ''
    setIsActing(true)
    updateAgentStatus({ worktreeId, status: 'starting' })
    const span = Tracers.uiAgentOrchResumeFlow.start({ worktreeId, sessionId: session.sessionId })
    registerOpenAgentOrchSpan(worktreeId, span)
    try {
      // ResumeAgentSessionRequest resumes by worktree, not by the old
      // sessionId (infra-fleet finds the worktree's most recent session) —
      // see SOL-FE-PW-004's contract-mismatch #2.
      const { ack: resumed } = await resumeRuntimeAgentSession(
        target,
        {
          connectionId,
          worktreeId,
          userId: currentUser.id,
          cwd: worktree.path
        },
        () => {
          // Pty output isn't surfaced by this panel yet — see SOL-FE-PW-004
          // "Not done in this spec".
        }
      )
      setRemoteAgentSession(worktreeId, {
        sessionId: resumed.id,
        worktreeId,
        agentType,
        trustPreset,
        status: toRemoteAgentStatus(resumed.status),
        startedAt: session.startedAt
      })
      span.step('rpc-resolved', { sessionId: resumed.id })
      // Span stays open — agent.subscribeStatus's 'running' will close it
      // (TASK-FE-002.3).
    } catch (err: unknown) {
      takeOpenAgentOrchSpan(worktreeId)
      span.fail(err, { worktreeId, sessionId: session.sessionId })
      updateAgentStatus({ worktreeId, status: 'error', errorMessage: errorText(err) })
      toast.error(`Failed to resume agent: ${errorText(err)}`)
    } finally {
      setIsActing(false)
    }
  }, [
    session,
    worktreeId,
    updateAgentStatus,
    setRemoteAgentSession,
    currentUser,
    agentType,
    trustPreset
  ])

  const status = session?.status
  const isRunning = status === 'running'
  const isStopped = !status || status === 'stopped'
  const isStarting = status === 'starting'
  const canResume = status === 'stopped' && !!session?.sessionId
  const isDisabled = isActing || isStarting

  return (
    <div className="agent-panel flex flex-col gap-3 p-3 border rounded-lg bg-card">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Bot size={14} className="text-muted-foreground" />
          <span className="text-sm font-medium">Remote Agent</span>
        </div>
        <AgentStatusBadge status={status} />
      </div>

      {/* Error message */}
      {status === 'error' && session?.errorMessage && (
        <p className="text-xs text-destructive bg-destructive/10 rounded px-2 py-1">
          {session.errorMessage}
        </p>
      )}

      {/* Config (only shown when idle) */}
      {isStopped && (
        <div className="grid grid-cols-2 gap-2">
          <div className="space-y-1">
            <label className="text-xs text-muted-foreground">Agent type</label>
            <Select value={agentType} onValueChange={(v) => setAgentType(v as AgentType)}>
              <SelectTrigger className="h-7 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="claude">Claude</SelectItem>
                <SelectItem value="codex">Codex</SelectItem>
                <SelectItem value="custom">Custom</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1">
            <label className="text-xs text-muted-foreground">Trust level</label>
            <Select value={trustPreset} onValueChange={(v) => setTrustPreset(v as TrustPreset)}>
              <SelectTrigger className="h-7 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="standard">Standard</SelectItem>
                <SelectItem value="permissive">Permissive</SelectItem>
                <SelectItem value="strict">Strict</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      )}

      {/* Actions */}
      <div className="flex gap-2">
        {isStopped && !canResume && (
          <Button
            size="sm"
            className="flex-1 h-7 text-xs gap-1"
            onClick={startAgent}
            disabled={isDisabled}
          >
            <Play size={12} />
            Start Agent
          </Button>
        )}

        {canResume && (
          <>
            <Button
              size="sm"
              variant="outline"
              className="flex-1 h-7 text-xs gap-1"
              onClick={resumeAgent}
              disabled={isDisabled}
            >
              <RotateCcw size={12} />
              Resume
            </Button>
            <Button
              size="sm"
              className="flex-1 h-7 text-xs gap-1"
              onClick={startAgent}
              disabled={isDisabled}
            >
              <Play size={12} />
              New Session
            </Button>
          </>
        )}

        {(isRunning || isStarting) && (
          <Button
            size="sm"
            variant="destructive"
            className="flex-1 h-7 text-xs gap-1"
            onClick={stopAgent}
            disabled={isDisabled}
          >
            {isActing ? <Loader2 size={12} className="animate-spin" /> : <Square size={12} />}
            Stop
          </Button>
        )}
      </div>
    </div>
  )
}
