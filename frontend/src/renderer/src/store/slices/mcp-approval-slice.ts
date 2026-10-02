import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { McpApproval } from '../../../../shared/mcp-types'

// Why: approvals carry argsPreview/paramsHash. This slice is memory-only; it must never be
// added to any persisted UI snapshot (see mcp-approval-slice.test.ts).
export type McpApprovalSlice = {
  /** Pending approvals only, oldest first. */
  mcpApprovalQueue: McpApproval[]
  /** id -> client-clock deadline (ms), immune to server clock skew. */
  mcpApprovalLocalDeadline: Record<string, number>
  /** false after "Decide later" until a NEW approval arrives or the user reopens. */
  mcpApprovalPromptOpen: boolean
  /** Approval the Approvals tab should scroll to (deep link). */
  mcpApprovalFocusId: string | null
  enqueueMcpApproval: (a: McpApproval, receivedAt: number) => void
  resolveMcpApproval: (id: string) => void
  /** Server is the source of truth: replaces the whole pending set. */
  replaceMcpApprovals: (list: McpApproval[], fetchedAt: number) => void
  focusMcpApproval: (id: string) => void
  setMcpApprovalPromptOpen: (open: boolean) => void
  setMcpApprovalFocusId: (id: string | null) => void
  clearMcpApprovals: () => void
}

// Live events: createdAt->expiresAt span from the server, anchored at receipt, minus a margin.
const LIVE_MARGIN_MS = 2000

export function mcpApprovalDeadline(a: McpApproval, receivedAt: number, live: boolean): number {
  const expires = Date.parse(a.expiresAt)
  if (!live) {
    return expires
  }
  const span = expires - Date.parse(a.createdAt)
  return Number.isFinite(span) ? receivedAt + span - LIVE_MARGIN_MS : expires
}

const byCreated = (x: McpApproval, y: McpApproval): number =>
  Date.parse(x.createdAt) - Date.parse(y.createdAt)

export const createMcpApprovalSlice: StateCreator<AppState, [], [], McpApprovalSlice> = (set) => ({
  mcpApprovalQueue: [],
  mcpApprovalLocalDeadline: {},
  mcpApprovalPromptOpen: false,
  mcpApprovalFocusId: null,

  enqueueMcpApproval: (a, receivedAt) =>
    set((s) => {
      if (a.status !== 'pending' || s.mcpApprovalQueue.some((q) => q.id === a.id)) {
        return {}
      }
      return {
        mcpApprovalQueue: [...s.mcpApprovalQueue, a].sort(byCreated),
        mcpApprovalLocalDeadline: {
          ...s.mcpApprovalLocalDeadline,
          [a.id]: mcpApprovalDeadline(a, receivedAt, true)
        },
        mcpApprovalPromptOpen: true
      }
    }),

  resolveMcpApproval: (id) =>
    set((s) => {
      const { [id]: _drop, ...rest } = s.mcpApprovalLocalDeadline
      const queue = s.mcpApprovalQueue.filter((q) => q.id !== id)
      return {
        mcpApprovalQueue: queue,
        mcpApprovalLocalDeadline: rest,
        ...(queue.length === 0 ? { mcpApprovalPromptOpen: false } : {})
      }
    }),

  replaceMcpApprovals: (list, fetchedAt) =>
    set((s) => {
      const pending = list.filter((a) => a.status === 'pending').sort(byCreated)
      const known = new Set(s.mcpApprovalQueue.map((q) => q.id))
      const deadlines: Record<string, number> = {}
      for (const a of pending) {
        // Keep the live-event deadline when we already had this one.
        deadlines[a.id] =
          s.mcpApprovalLocalDeadline[a.id] ?? mcpApprovalDeadline(a, fetchedAt, false)
      }
      const hasNew = pending.some((a) => !known.has(a.id))
      return {
        mcpApprovalQueue: pending,
        mcpApprovalLocalDeadline: deadlines,
        mcpApprovalPromptOpen:
          pending.length === 0 ? false : hasNew ? true : s.mcpApprovalPromptOpen
      }
    }),

  focusMcpApproval: (id) =>
    set((s) => {
      const target = s.mcpApprovalQueue.find((q) => q.id === id)
      if (!target) {
        return {}
      }
      return {
        mcpApprovalQueue: [target, ...s.mcpApprovalQueue.filter((q) => q.id !== id)],
        mcpApprovalPromptOpen: true
      }
    }),

  setMcpApprovalPromptOpen: (open) => set({ mcpApprovalPromptOpen: open }),
  setMcpApprovalFocusId: (id) => set({ mcpApprovalFocusId: id }),
  clearMcpApprovals: () =>
    set({
      mcpApprovalQueue: [],
      mcpApprovalLocalDeadline: {},
      mcpApprovalPromptOpen: false,
      mcpApprovalFocusId: null
    })
})
