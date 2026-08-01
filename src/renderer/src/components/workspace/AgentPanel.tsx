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
  SelectValue,
} from '@/components/ui/select'
import { toast } from 'sonner'
import { useAppStore } from '../../store'
import { useShallow } from 'zustand/react/shallow'
import type { RemoteAgentSession } from '../../store/slices/remote-agent-sessions'

interface AgentPanelProps {
  worktreeId: string
}

type AgentType = 'claude' | 'codex' | 'custom'
type TrustPreset = 'standard' | 'permissive' | 'strict'

// ─── Status Badge ─────────────────────────────────────────────────────────────

function AgentStatusBadge({ status }: { status: RemoteAgentSession['status'] | undefined }) {
  if (!status || status === 'stopped') {
    return <Badge variant="secondary" className="text-[10px]">Idle</Badge>
  }
  const configs: Record<string, { label: string; className: string }> = {
    starting:  { label: 'Starting…', className: 'bg-yellow-500/20 text-yellow-600 border-yellow-500/30' },
    running:   { label: 'Running',   className: 'bg-green-500/20  text-green-600  border-green-500/30'  },
    stopped:   { label: 'Idle',      className: ''                                                        },
    error:     { label: 'Error',     className: 'bg-red-500/20    text-red-600    border-red-500/30'     },
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

  const { session, setRemoteAgentSession, updateAgentStatus } = useAppStore(
    useShallow(s => ({
      session: s.remoteAgentSessions[worktreeId] as RemoteAgentSession | undefined,
      setRemoteAgentSession: s.setRemoteAgentSession,
      updateAgentStatus:     s.updateAgentStatus,
    }))
  )

  // Subscribe to status change events
  useEffect(() => {
    const unsubscribe = window.api.agentOrchestration.onStatusChanged(event => {
      if (event.worktreeId === worktreeId) {
        updateAgentStatus(event)
      }
    })
    return unsubscribe
  }, [worktreeId, updateAgentStatus])

  const startAgent = useCallback(async () => {
    setIsActing(true)
    // Optimistic update
    updateAgentStatus({ worktreeId, status: 'starting' })
    try {
      const result = await window.api.agentOrchestration.start({
        worktreeId,
        agentType,
        trustPreset,
      })
      setRemoteAgentSession(worktreeId, {
        sessionId: result.sessionId,
        worktreeId,
        agentType,
        trustPreset,
        status: result.status === 'already-running' ? 'running' : 'starting',
        startedAt: Date.now(),
      })
      if (result.status === 'already-running') {
        toast.info('Agent is already running')
      }
    } catch (err: any) {
      updateAgentStatus({ worktreeId, status: 'error', errorMessage: err.message })
      toast.error(`Failed to start agent: ${err.message}`)
    } finally {
      setIsActing(false)
    }
  }, [worktreeId, agentType, trustPreset, updateAgentStatus, setRemoteAgentSession])

  const stopAgent = useCallback(async () => {
    if (!session?.sessionId) return
    setIsActing(true)
    try {
      await window.api.agentOrchestration.stop({ sessionId: session.sessionId })
      updateAgentStatus({ worktreeId, status: 'stopped' })
    } catch (err: any) {
      toast.error(`Failed to stop agent: ${err.message}`)
    } finally {
      setIsActing(false)
    }
  }, [session, worktreeId, updateAgentStatus])

  const resumeAgent = useCallback(async () => {
    if (!session?.sessionId) return
    setIsActing(true)
    updateAgentStatus({ worktreeId, status: 'starting' })
    try {
      const result = await window.api.agentOrchestration.resume({ sessionId: session.sessionId })
      if (!result.resumed) {
        toast.error('Could not resume agent session')
        updateAgentStatus({ worktreeId, status: 'stopped' })
      }
    } catch (err: any) {
      updateAgentStatus({ worktreeId, status: 'error', errorMessage: err.message })
      toast.error(`Failed to resume agent: ${err.message}`)
    } finally {
      setIsActing(false)
    }
  }, [session, worktreeId, updateAgentStatus])

  const status = session?.status
  const isRunning   = status === 'running'
  const isStopped   = !status || status === 'stopped'
  const isStarting  = status === 'starting'
  const canResume   = status === 'stopped' && !!session?.sessionId
  const isDisabled  = isActing || isStarting

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
            <Select value={agentType} onValueChange={v => setAgentType(v as AgentType)}>
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
            <Select value={trustPreset} onValueChange={v => setTrustPreset(v as TrustPreset)}>
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
            {isActing
              ? <Loader2 size={12} className="animate-spin" />
              : <Square size={12} />
            }
            Stop
          </Button>
        )}
      </div>
    </div>
  )
}
