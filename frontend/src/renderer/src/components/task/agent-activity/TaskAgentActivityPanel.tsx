/**
 * TaskAgentActivityPanel — live view of what the agent behind a task is doing (files it reads,
 * tools it calls, what it says), fed by `task.activity.subscribe` (useTaskAgentOutput).
 *
 * @module components/task/agent-activity/TaskAgentActivityPanel
 */

import React, { useCallback, useEffect, useRef, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useTaskAgentOutput } from '../../../hooks/useTaskAgentOutput'
import { useInferredExecutionEngine } from '../ExecutionEngineBadge'
import type { AgentActivityKind, AgentActivityLine } from './agent-activity-lines'
import type { OrcaTask } from '../../../../../shared/task-types'

const T = 'auto.components.task.agentActivity.'
// Within this distance of the bottom the view keeps following new lines.
const FOLLOW_THRESHOLD_PX = 24

const GLYPH: Record<AgentActivityKind, string> = {
  info: '·',
  text: '»',
  tool: '▸',
  result: '↳',
  end: '■',
  stderr: '!',
  raw: '»'
}

function lineClass(line: AgentActivityLine): string {
  if (line.tone === 'error' || line.kind === 'stderr') {
    return 'text-destructive'
  }
  if (line.tone === 'ok') {
    return 'text-status-success'
  }
  return line.kind === 'tool' ? 'text-foreground' : 'text-muted-foreground'
}

type Props = {
  task: OrcaTask
  isRunning: boolean
  /** Set only when the task's Result tab has saved output; opens that tab. */
  onOpenSavedResult?: () => void
}

export function TaskAgentActivityPanel({
  task,
  isRunning,
  onOpenSavedResult
}: Props): React.JSX.Element | null {
  const engine = useInferredExecutionEngine(task)
  const output = useTaskAgentOutput(engine.kind === 'direct_agent' ? task.id : null)
  const [collapsed, setCollapsed] = useState(false)
  const [following, setFollowing] = useState(true)
  const scroller = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const el = scroller.current
    if (following && el) {
      el.scrollTop = el.scrollHeight
    }
  }, [output.lines, following, collapsed])

  const onScroll = useCallback(() => {
    const el = scroller.current
    if (el) {
      setFollowing(el.scrollHeight - el.scrollTop - el.clientHeight <= FOLLOW_THRESHOLD_PX)
    }
  }, [])

  const jumpToLatest = (): void => {
    setFollowing(true)
    const el = scroller.current
    if (el) {
      el.scrollTop = el.scrollHeight
    }
  }

  // Orchestration / workflow tasks stream other events; this view is the direct-agent run.
  if (engine.kind !== 'direct_agent') {
    return null
  }

  const hasLines = output.lines.length > 0
  const state = isRunning ? 'running' : hasLines ? 'finished' : 'idle'
  const showSavedLink = !isRunning && !hasLines && Boolean(onOpenSavedResult)

  return (
    <section
      className="mt-2 rounded-md border border-border"
      data-testid="task-agent-activity"
      data-state={state}
      aria-label={translate(`${T}title`, 'Agent activity')}
    >
      <header className="flex items-center gap-2 px-2 py-1">
        <Button
          variant="ghost"
          size="xs"
          className="h-auto gap-1 px-1 py-0.5"
          aria-expanded={!collapsed}
          onClick={() => setCollapsed((c) => !c)}
          data-testid="task-agent-activity-toggle"
        >
          {collapsed ? <ChevronRight className="size-3" /> : <ChevronDown className="size-3" />}
          <span className="text-xs font-medium">{translate(`${T}title`, 'Agent activity')}</span>
        </Button>
        {isRunning ? (
          <Badge variant="dot" data-testid="task-agent-activity-live" data-state="running">
            <span className="size-1.5 animate-pulse rounded-full bg-status-success" aria-hidden />
            {translate(`${T}live`, 'Live')}
          </Badge>
        ) : (
          <Badge variant="outline" data-testid="task-agent-activity-status" data-state={state}>
            {hasLines ? translate(`${T}finished`, 'Finished') : translate(`${T}idle`, 'Idle')}
          </Badge>
        )}
        {output.status === 'reconnecting' && (
          <span
            className="text-xs text-muted-foreground"
            data-testid="task-agent-activity-reconnecting"
          >
            {translate(`${T}reconnecting`, 'Reconnecting…')}
          </span>
        )}
      </header>
      {!collapsed && (
        <div className="border-t border-border">
          {output.olderHidden && (
            <p
              className="px-2 pt-1 text-[11px] text-muted-foreground"
              data-testid="task-agent-activity-older"
            >
              {translate(`${T}olderHidden`, 'Older activity is not shown.')}
            </p>
          )}
          <div
            ref={scroller}
            onScroll={onScroll}
            role="log"
            aria-live="polite"
            aria-label={translate(`${T}log`, 'Agent activity log')}
            className="scrollbar-sleek max-h-56 overflow-y-auto px-2 py-1 font-mono text-[11px] leading-relaxed"
            data-testid="task-agent-activity-lines"
          >
            {hasLines ? (
              output.lines.map((line) => (
                <div
                  key={line.id}
                  className={cn('flex gap-1.5 break-words whitespace-pre-wrap', lineClass(line))}
                  data-testid="task-agent-activity-line"
                  data-kind={line.kind}
                  data-tone={line.tone}
                >
                  <span aria-hidden className="shrink-0 select-none">
                    {GLYPH[line.kind]}
                  </span>
                  <span className="min-w-0">{line.text}</span>
                </div>
              ))
            ) : (
              <p
                className="font-sans text-xs text-muted-foreground"
                data-testid="task-agent-activity-empty"
              >
                {output.status === 'unavailable'
                  ? translate(
                      `${T}unavailable`,
                      'Live agent activity needs a connected runtime environment.'
                    )
                  : isRunning
                    ? translate(`${T}waiting`, 'Waiting for the agent to report what it is doing…')
                    : onOpenSavedResult
                      ? translate(
                          `${T}savedElsewhere`,
                          'Live activity is not kept after you reload the page. See the saved output in the Result tab.'
                        )
                      : translate(
                          `${T}empty`,
                          'Nothing to show yet. Activity appears here while the agent runs; it is not kept after you reload the page.'
                        )}
              </p>
            )}
            {showSavedLink && (
              <Button
                variant="link"
                size="xs"
                className="mt-1 h-auto px-0 font-sans underline"
                onClick={onOpenSavedResult}
                data-testid="task-agent-activity-open-result"
              >
                {translate(`${T}openResult`, 'Open the Result tab')}
              </Button>
            )}
          </div>
          {!following && hasLines && (
            <div className="flex justify-end px-2 pb-1">
              <Button
                variant="outline"
                size="xs"
                onClick={jumpToLatest}
                data-testid="task-agent-activity-jump"
              >
                {translate(`${T}jumpToLatest`, 'Jump to latest')}
              </Button>
            </div>
          )}
        </div>
      )}
    </section>
  )
}
