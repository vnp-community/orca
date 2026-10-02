// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from 'vitest'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { create } from 'zustand'
import type { McpApproval } from '../../../../shared/mcp-types'
import { createMcpApprovalSlice, type McpApprovalSlice } from './mcp-approval-slice'

const make = () =>
  create<McpApprovalSlice>()((...a: unknown[]) =>
    (createMcpApprovalSlice as (...x: unknown[]) => McpApprovalSlice)(...a)
  )
const ap = (id: string, over: Partial<McpApproval> = {}): McpApproval => ({
  id,
  createdAt: '2026-10-01T10:00:00Z',
  expiresAt: '2026-10-01T10:02:00Z',
  status: 'pending',
  tool: { name: 't', title: 'T', risk: 'exec' },
  clientName: 'c',
  sessionId: 's',
  argsPreview: { text: 'SECRET-ARGS', redacted: false },
  paramsHash: 'h',
  ...over
})

let store: ReturnType<typeof make>
beforeEach(() => {
  store = make()
  localStorage.clear()
  sessionStorage.clear()
})

describe('mcp-approval-slice', () => {
  it('enqueues pending only, dedupes, orders oldest first and opens the prompt', () => {
    const s = store.getState()
    s.enqueueMcpApproval(ap('b', { createdAt: '2026-10-01T10:01:00Z' }), 0)
    s.enqueueMcpApproval(ap('a'), 0)
    s.enqueueMcpApproval(ap('a'), 0)
    s.enqueueMcpApproval(ap('x', { status: 'approved' }), 0)
    expect(store.getState().mcpApprovalQueue.map((q) => q.id)).toEqual(['a', 'b'])
    expect(store.getState().mcpApprovalPromptOpen).toBe(true)
  })

  it('live deadline uses the server span anchored at receipt (clock-skew proof)', () => {
    store.getState().enqueueMcpApproval(ap('a'), 1_000_000)
    expect(store.getState().mcpApprovalLocalDeadline.a).toBe(1_000_000 + 120_000 - 2000)
  })

  it('replace keeps the server as source of truth and closes the prompt when empty', () => {
    const s = store.getState()
    s.enqueueMcpApproval(ap('a'), 0)
    s.replaceMcpApprovals([ap('a', { status: 'expired' })], 0)
    expect(store.getState().mcpApprovalQueue).toEqual([])
    expect(store.getState().mcpApprovalPromptOpen).toBe(false)
  })

  it('decide later stays closed until a new approval arrives', () => {
    const s = store.getState()
    s.enqueueMcpApproval(ap('a'), 0)
    s.setMcpApprovalPromptOpen(false)
    s.replaceMcpApprovals([ap('a')], 0)
    expect(store.getState().mcpApprovalPromptOpen).toBe(false)
    s.replaceMcpApprovals([ap('a'), ap('b', { createdAt: '2026-10-01T10:05:00Z' })], 0)
    expect(store.getState().mcpApprovalPromptOpen).toBe(true)
  })

  it('focus moves an approval to the front; resolve removes it', () => {
    const s = store.getState()
    s.enqueueMcpApproval(ap('a'), 0)
    s.enqueueMcpApproval(ap('b', { createdAt: '2026-10-01T10:05:00Z' }), 0)
    s.focusMcpApproval('b')
    expect(store.getState().mcpApprovalQueue[0].id).toBe('b')
    s.resolveMcpApproval('b')
    expect(store.getState().mcpApprovalQueue.map((q) => q.id)).toEqual(['a'])
  })

  it('never writes argsPreview to web storage and clear wipes everything', () => {
    store.getState().enqueueMcpApproval(ap('a'), 0)
    expect(JSON.stringify({ ...localStorage })).not.toContain('SECRET-ARGS')
    expect(JSON.stringify({ ...sessionStorage })).not.toContain('SECRET-ARGS')
    store.getState().clearMcpApprovals()
    expect(store.getState().mcpApprovalQueue).toEqual([])
  })

  it('is referenced by no persistence code under store/', () => {
    const dir = join(__dirname, '..')
    const offenders: string[] = []
    const walk = (d: string): void => {
      for (const e of readdirSync(d, { withFileTypes: true })) {
        const p = join(d, e.name)
        if (e.isDirectory()) {
          walk(p)
        } else if (
          e.name.endsWith('.ts') &&
          !/test|mcp-approval-slice|types\.ts|index\.ts/.test(e.name)
        ) {
          if (/mcpApproval/.test(readFileSync(p, 'utf8'))) {
            offenders.push(p)
          }
        }
      }
    }
    walk(dir)
    expect(offenders).toEqual([])
  })
})
