/**
 * What the backend broadcasts about a running task: `task.activity.subscribe` (api-gateway channels_task_activity.go), the
 * only channel that carries an agent's output while `task.execute` runs. The spec listens on its own gateway socket so it can
 * tell "the backend sent it and the screen did not show it" (a frontend gap) from "the backend never sent it" (a backend or
 * agent gap).
 */
import type { GatewayRpcClient } from './gateway-rpc-client'

export type ActivityFrame = {
  taskId: string
  receivedAt: number
  engine: string
  eventType: string
  /** Cumulative output of the run so far (agent_output_partial carries the whole buffer each time). */
  stdout: string
  stderr: string
}

type RawFrame = {
  taskId?: string
  engine?: string
  eventType?: string
  payload?: { stdout?: string; stderr?: string }
}

export class TaskActivityTap {
  readonly frames: ActivityFrame[] = []
  private readonly closers: (() => void)[] = []

  /** Subscribes to every task id up front, before the Plan is approved, so no early frame is missed. */
  static async open(rpc: GatewayRpcClient, taskIds: string[]): Promise<TaskActivityTap> {
    const tap = new TaskActivityTap()
    for (const taskId of taskIds) {
      const sub = await rpc.subscribe('task.activity.subscribe', { taskId }, (frame) => {
        const raw = frame.result as RawFrame | null
        if (!raw || typeof raw !== 'object') {
          return
        }
        tap.frames.push({
          taskId: raw.taskId ?? taskId,
          receivedAt: frame.receivedAt,
          engine: raw.engine ?? '',
          eventType: raw.eventType ?? '',
          stdout: raw.payload?.stdout ?? '',
          stderr: raw.payload?.stderr ?? ''
        })
      })
      tap.closers.push(sub.close)
    }
    return tap
  }

  close(): void {
    this.closers.forEach((c) => c())
  }

  outputFramesOf(taskId: string): ActivityFrame[] {
    return this.frames.filter((f) => f.taskId === taskId && f.eventType === 'agent_output_partial')
  }

  /** Everything the backend said about the task, for the leak scan. */
  allTextOf(taskId: string): string {
    return this.frames
      .filter((f) => f.taskId === taskId)
      .map((f) => `${f.stdout}\n${f.stderr}`)
      .join('\n')
  }
}

/** Markdown marks and runs of whitespace removed, so a sentence matches whether the screen renders it as markdown or text. */
export function normalizeForMatch(text: string): string {
  return text
    .replace(/[`*_#>|]/g, '')
    .replace(/\s+/g, ' ')
    .trim()
    .toLowerCase()
}

const GLYPHS = '·»▸↳■'

/** One activity line without its leading glyph (`▸ Read src/calc.py` -> `read src/calc.py`), as the task panel renders it. */
export function activityLineText(line: string): string {
  const t = line.trim()
  return normalizeForMatch(GLYPHS.includes(t.charAt(0)) ? t.slice(1) : t)
}

/**
 * A short phrase the agent really did, taken from the last output frame: a tool line if there is one (the most telling
 * line: "Edit src/calc.py"), else the longest line; at most 28 characters. Empty when the backend never sent any output.
 */
export function distinctivePhrase(frames: ActivityFrame[]): string {
  const last = frames.at(-1)
  if (!last) {
    return ''
  }
  const raw = last.stdout.split('\n').filter((l) => l.trim().length > 0)
  const tools = raw.filter((l) => l.trim().startsWith('▸')).map(activityLineText)
  const pool = tools.filter((l) => l.length >= 10)
  const pick = (pool.length > 0 ? pool : raw.map(activityLineText))
    .filter((l) => l.length >= 8)
    .sort((a, b) => b.length - a.length)[0]
  return (pick ?? '').slice(0, 28)
}
