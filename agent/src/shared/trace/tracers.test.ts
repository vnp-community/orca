import { describe, expect, it } from 'vitest'
import { registerTraceSink, type TraceEvent } from './index'
import { Tracers } from './tracers'

describe('Tracers registry — CR-TRACE-001 worktree entries', () => {
  it('exports Tracers.worktreeCreate with flow name worktree:create', () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    try {
      Tracers.worktreeCreate.start({ repoSelector: 'repo-1' })
    } finally {
      unregister()
    }
    expect(events[0]?.flow).toBe('worktree:create')
  })

  it('exports Tracers.worktreeDelete with flow name worktree:delete', () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    try {
      Tracers.worktreeDelete.start({ worktreeId: 'wt-1' })
    } finally {
      unregister()
    }
    expect(events[0]?.flow).toBe('worktree:delete')
  })

  it('exports worktreeFanOut/worktreeCompare/worktreeMerge as reserved tracers that do not throw when started', () => {
    expect(() => {
      const a = Tracers.worktreeFanOut.start({})
      a.ok({})
      const b = Tracers.worktreeCompare.start({})
      b.ok({})
      const c = Tracers.worktreeMerge.start({})
      c.ok({})
    }).not.toThrow()
  })

  it('reserved worktree tracers use the worktree:fanOut / worktree:compare / worktree:merge flow names', () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    try {
      Tracers.worktreeFanOut.start({})
      Tracers.worktreeCompare.start({})
      Tracers.worktreeMerge.start({})
    } finally {
      unregister()
    }
    expect(events.map((e) => e.flow)).toEqual([
      'worktree:fanOut',
      'worktree:compare',
      'worktree:merge'
    ])
  })
})

describe('Tracers registry — CR-TRACE-002 agentOrch entries', () => {
  it('exports Tracers.agentOrchSpawn/Stop/Resume/Switch/StatusPoll with correct agentOrch:* flow names', () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    try {
      Tracers.agentOrchSpawn.start({})
      Tracers.agentOrchStop.start({})
      Tracers.agentOrchResume.start({})
      Tracers.agentOrchSwitch.start({})
      Tracers.agentOrchStatusPoll.start({})
    } finally {
      unregister()
    }
    expect(events.map((e) => e.flow)).toEqual([
      'agentOrch:spawn',
      'agentOrch:stop',
      'agentOrch:resume',
      'agentOrch:switch',
      'agentOrch:statusPoll'
    ])
  })

  it('agentOrch:* flow names do not collide with the agent:rpc infra tracer', () => {
    const agentOrchFlows = [
      Tracers.agentOrchSpawn,
      Tracers.agentOrchStop,
      Tracers.agentOrchResume,
      Tracers.agentOrchSwitch,
      Tracers.agentOrchStatusPoll
    ]
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    try {
      for (const tracer of agentOrchFlows) tracer.start({})
    } finally {
      unregister()
    }
    expect(events.every((e) => e.flow !== 'agent:rpc')).toBe(true)
    expect(events.every((e) => e.flow.startsWith('agentOrch:'))).toBe(true)
  })
})

// ── CR-TRACE-003: Terminal Management (TASK-BE-003.4) ──────────────────────
// Why: terminalCreate/Resize/Destroy/Reattach were already registered by the
// concurrent agent-domain pty-agent-bridge.ts work (flow `terminal:reattach`,
// not `terminal:reconnect` as the original CR-TRACE-003 doc named it) —
// these tests assert the final registry state, regardless of which task
// added each entry.
describe('Tracers registry — CR-TRACE-003 terminal entries', () => {
  it('exports Tracers.terminalCreate/Resize/Destroy/Reattach with correct terminal:* flow names', () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    try {
      Tracers.terminalCreate.start({})
      Tracers.terminalResize.start({})
      Tracers.terminalDestroy.start({})
      Tracers.terminalReattach.start({})
    } finally {
      unregister()
    }
    expect(events.map((e) => e.flow)).toEqual([
      'terminal:create',
      'terminal:resize',
      'terminal:destroy',
      'terminal:reattach'
    ])
  })

  it('terminal:* flow names do not collide with worktree:*/agentOrch:* tracers', () => {
    const terminalFlows = [
      Tracers.terminalCreate,
      Tracers.terminalResize,
      Tracers.terminalDestroy,
      Tracers.terminalReattach
    ]
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    try {
      for (const tracer of terminalFlows) tracer.start({})
    } finally {
      unregister()
    }
    expect(events.every((e) => e.flow.startsWith('terminal:'))).toBe(true)
    expect(events.every((e) => !e.flow.startsWith('worktree:'))).toBe(true)
    expect(events.every((e) => !e.flow.startsWith('agentOrch:'))).toBe(true)
  })
})
