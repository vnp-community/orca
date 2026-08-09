// ─── Trace Slice ──────────────────────────────────────────────────────────────
// Zustand slice that stores live trace events from the shared trace system.
// Events flow from: shared/trace/index.ts → browser.ts sink → this store.
//
// The UI reads from this store to render TracePanel.
// Rolling buffer: keeps last MAX_EVENTS entries (FIFO) to avoid unbounded growth.

import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { TraceEvent } from '../../../../shared/trace'

const MAX_TRACE_EVENTS = 500

export interface TraceSlice {
  /** Whether trace event collection is active */
  traceEnabled: boolean
  /** Rolling buffer of collected trace events (newest last) */
  traceEvents: TraceEvent[]
  /** Whether the TracePanel UI is visible */
  tracePanelOpen: boolean

  addTraceEvent(event: TraceEvent): void
  clearTraceEvents(): void
  setTraceEnabled(enabled: boolean): void
  setTracePanelOpen(open: boolean): void
}

export const createTraceSlice: StateCreator<AppState, [], [], TraceSlice> = (set) => ({
  traceEnabled: false,
  traceEvents: [],
  tracePanelOpen: false,

  addTraceEvent(event: TraceEvent) {
    set((state) => {
      const events = [...state.traceEvents, event]
      // FIFO trim: drop oldest if over limit
      return {
        traceEvents: events.length > MAX_TRACE_EVENTS
          ? events.slice(events.length - MAX_TRACE_EVENTS)
          : events
      }
    })
  },

  clearTraceEvents() {
    set({ traceEvents: [] })
  },

  setTraceEnabled(enabled: boolean) {
    set({ traceEnabled: enabled })
  },

  setTracePanelOpen(open: boolean) {
    set({ tracePanelOpen: open })
  }
})
