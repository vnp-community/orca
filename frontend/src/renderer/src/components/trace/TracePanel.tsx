// ─── TracePanel ───────────────────────────────────────────────────────────────
// Floating DevTools panel that shows live trace events collected by the
// shared/trace system. Similar in spirit to React Query DevTools.
//
// Toggle:  Ctrl+Shift+T  (or via DevServer settings)
// Enable:  localStorage.setItem('ORCA_TRACE', '1') + reload
//
// Architecture:
//   shared/trace/index.ts → emits TraceEvent
//   shared/trace/browser.ts → registerTraceSink → store.addTraceEvent
//   store/slices/trace.ts → traceEvents[] reactive store
//   TracePanel.tsx (this file) → reads store, renders events

import React, { useState, useCallback, useMemo, useRef, useEffect } from 'react'
import { useAppStore } from '../../store'
import type { TraceEvent, TraceLevel } from '../../../../shared/trace'
import { enableBrowserTrace, disableBrowserTrace, isBrowserTraceEnabled, isBackendStreamConnected } from '../../../../shared/trace/browser'

// ─── Level colors ─────────────────────────────────────────────────────────────
const LEVEL_COLOR: Record<TraceLevel, string> = {
  start: '#60a5fa',   // blue-400
  step:  '#a78bfa',   // violet-400
  ok:    '#34d399',   // emerald-400
  fail:  '#f87171',   // red-400
}
const LEVEL_BG: Record<TraceLevel, string> = {
  start: 'rgba(96, 165, 250, 0.08)',
  step:  'rgba(167, 139, 250, 0.08)',
  ok:    'rgba(52, 211, 153, 0.10)',
  fail:  'rgba(248, 113, 113, 0.12)',
}

// ─── TraceEventRow ────────────────────────────────────────────────────────────
function TraceEventRow({ event, baseTs }: { event: TraceEvent; baseTs: number }) {
  const [expanded, setExpanded] = useState(false)
  const color = LEVEL_COLOR[event.level]
  const bg = LEVEL_BG[event.level]
  const delta = ((event.ts - baseTs) / 1000).toFixed(3)
  const elapsed = event.elapsedMs !== undefined ? ` +${event.elapsedMs}ms` : ''
  // Heuristic: backend events have flow names with ':' and come from SSE
  // Frontend events typically originate from browser-side tracer calls
  const isBackend = event.flow.includes(':') && !event.flow.startsWith('ui:')

  const fieldEntries = Object.entries(event.fields).filter(([, v]) => v !== undefined)

  return (
    <div
      onClick={() => fieldEntries.length > 0 && setExpanded(e => !e)}
      style={{
        padding: '3px 8px',
        borderBottom: '1px solid rgba(255,255,255,0.05)',
        cursor: fieldEntries.length > 0 ? 'pointer' : 'default',
        background: expanded ? bg : 'transparent',
        fontFamily: 'monospace',
        fontSize: '11px',
        lineHeight: '1.5',
      }}
    >
      {/* Main row */}
      <div style={{ display: 'flex', gap: 8, alignItems: 'baseline' }}>
        <span style={{ color: 'rgba(255,255,255,0.3)', width: 52, flexShrink: 0 }}>
          {delta}s
        </span>
        <span style={{
          color,
          fontWeight: 600,
          width: 36,
          flexShrink: 0,
          textTransform: 'uppercase',
          fontSize: '10px',
        }}>
          {event.level === 'start' ? 'new' : event.level}
        </span>
        {/* Source badge: ▲ backend, ▼ frontend */}
        <span style={{
          fontSize: '9px',
          padding: '0 4px',
          borderRadius: 3,
          background: isBackend ? 'rgba(251,191,36,0.15)' : 'rgba(96,165,250,0.12)',
          color: isBackend ? '#fbbf24' : '#93c5fd',
          flexShrink: 0,
        }}>
          {isBackend ? '▲ srv' : '▼ cli'}
        </span>
        <span style={{ color: 'rgba(255,255,255,0.7)', flexShrink: 0 }}>
          {event.flow}
        </span>
        <span style={{ color: 'rgba(255,255,255,0.35)', fontSize: '10px' }}>
          id={event.id}
        </span>
        {event.label && (
          <span style={{ color: '#e2e8f0', marginLeft: 4 }}>› {event.label}</span>
        )}
        {elapsed && (
          <span style={{ color: 'rgba(255,255,255,0.3)', marginLeft: 'auto', fontSize: '10px' }}>
            {elapsed}
          </span>
        )}
      </div>

      {/* Expanded fields */}
      {expanded && fieldEntries.length > 0 && (
        <div style={{ marginTop: 4, paddingLeft: 96 }}>
          {fieldEntries.map(([k, v]) => (
            <div key={k} style={{ color: 'rgba(255,255,255,0.55)', fontSize: '10px' }}>
              <span style={{ color: '#93c5fd' }}>{k}</span>
              <span style={{ color: 'rgba(255,255,255,0.3)' }}>: </span>
              <span style={{ color: event.level === 'fail' && k === 'err' ? '#fca5a5' : '#d1fae5' }}>
                {String(v)}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── TracePanel ───────────────────────────────────────────────────────────────
export function TracePanel() {
  const traceEvents    = useAppStore(s => s.traceEvents)
  const tracePanelOpen = useAppStore(s => s.tracePanelOpen)
  const clearTraceEvents   = useAppStore(s => s.clearTraceEvents)
  const setTracePanelOpen  = useAppStore(s => s.setTracePanelOpen)

  const [filter, setFilter] = useState('')
  const [traceOn, setTraceOn] = useState(() => isBrowserTraceEnabled())
  const [sseConnected, setSseConnected] = useState(false)
  const scrollRef = useRef<HTMLDivElement>(null)

  // Poll SSE connection status every 2s
  useEffect(() => {
    const interval = setInterval(() => setSseConnected(isBackendStreamConnected()), 2000)
    setSseConnected(isBackendStreamConnected())
    return () => clearInterval(interval)
  }, [])

  // Auto-scroll to bottom when new events arrive
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [traceEvents.length])

  const toggleTrace = useCallback(() => {
    if (traceOn) {
      disableBrowserTrace()
      setTraceOn(false)
    } else {
      enableBrowserTrace()
      setTraceOn(true)
    }
  }, [traceOn])

  const filtered = useMemo(() => {
    if (!filter.trim()) {return traceEvents}
    const q = filter.toLowerCase()
    return traceEvents.filter(e =>
      e.flow.toLowerCase().includes(q) ||
      e.id.includes(q) ||
      e.label?.toLowerCase().includes(q) ||
      Object.values(e.fields).some(v => String(v ?? '').toLowerCase().includes(q))
    )
  }, [traceEvents, filter])

  const baseTs = traceEvents[0]?.ts ?? Date.now()

  const copyJson = useCallback(() => {
    navigator.clipboard.writeText(JSON.stringify(filtered, null, 2))
      .catch(() => undefined)
  }, [filtered])

  if (!tracePanelOpen) {return null}

  return (
    <div
      id="orca-trace-panel"
      style={{
        position: 'fixed',
        bottom: 16,
        right: 16,
        width: 620,
        height: 380,
        zIndex: 9999,
        borderRadius: 10,
        background: '#0f172a',
        border: '1px solid rgba(255,255,255,0.12)',
        boxShadow: '0 24px 64px rgba(0,0,0,0.7)',
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
        fontFamily: 'system-ui, sans-serif',
      }}
    >
      {/* Header */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        padding: '8px 12px',
        borderBottom: '1px solid rgba(255,255,255,0.08)',
        background: 'rgba(255,255,255,0.03)',
        flexShrink: 0,
      }}>
        <span style={{ fontSize: 12, fontWeight: 700, color: '#60a5fa', letterSpacing: '0.05em' }}>
          ◈ ORCA TRACE
        </span>
        <span style={{
          fontSize: 10,
          padding: '1px 6px',
          borderRadius: 4,
          background: traceOn ? 'rgba(52,211,153,0.2)' : 'rgba(255,255,255,0.07)',
          color: traceOn ? '#34d399' : 'rgba(255,255,255,0.4)',
          cursor: 'pointer',
          border: `1px solid ${traceOn ? 'rgba(52,211,153,0.4)' : 'rgba(255,255,255,0.1)'}`,
        }} onClick={toggleTrace}>
          {traceOn ? '● LIVE' : '○ OFF'}
        </span>
        <span style={{ color: 'rgba(255,255,255,0.25)', fontSize: 10 }}>
          {filtered.length}/{traceEvents.length} events
        </span>
        {/* SSE connection indicator */}
        <span style={{
          fontSize: '10px',
          padding: '1px 6px',
          borderRadius: 4,
          background: sseConnected ? 'rgba(52,211,153,0.15)' : 'rgba(255,255,255,0.05)',
          color: sseConnected ? '#34d399' : 'rgba(255,255,255,0.3)',
          border: `1px solid ${sseConnected ? 'rgba(52,211,153,0.3)' : 'rgba(255,255,255,0.08)'}`,
          title: sseConnected ? 'Backend stream connected' : 'Backend stream disconnected',
        }}>
          {sseConnected ? '● srv' : '○ srv'}
        </span>

        {/* Filter */}
        <input
          type="text"
          placeholder="filter flow / id / field..."
          value={filter}
          onChange={e => setFilter(e.target.value)}
          style={{
            flex: 1,
            background: 'rgba(255,255,255,0.05)',
            border: '1px solid rgba(255,255,255,0.1)',
            borderRadius: 5,
            padding: '2px 8px',
            fontSize: 11,
            color: '#e2e8f0',
            outline: 'none',
          }}
        />

        {/* Actions */}
        <button
          onClick={copyJson}
          title="Copy filtered events as JSON"
          style={btnStyle}
        >
          Copy JSON
        </button>
        <button onClick={clearTraceEvents} style={btnStyle}>Clear</button>
        <button onClick={() => setTracePanelOpen(false)} style={{ ...btnStyle, color: '#f87171' }}>
          ✕
        </button>
      </div>

      {/* Event list */}
      <div
        ref={scrollRef}
        style={{ flex: 1, overflowY: 'auto', overflowX: 'hidden' }}
      >
        {filtered.length === 0 ? (
          <div style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            color: 'rgba(255,255,255,0.2)',
            fontSize: 12,
            gap: 8,
          }}>
            <span style={{ fontSize: 28 }}>◈</span>
            {traceOn
              ? 'Waiting for trace events...'
              : 'Trace is OFF. Click ○ OFF to enable.'
            }
            {!traceOn && (
              <span style={{ fontSize: 10, opacity: 0.6 }}>
                Or run: localStorage.setItem('ORCA_TRACE','1') in console
              </span>
            )}
          </div>
        ) : (
          filtered.map((e, i) => (
            <TraceEventRow key={`${e.id}-${e.level}-${i}`} event={e} baseTs={baseTs} />
          ))
        )}
      </div>

      {/* Footer hint */}
      <div style={{
        padding: '4px 12px',
        borderTop: '1px solid rgba(255,255,255,0.06)',
        fontSize: 10,
        color: 'rgba(255,255,255,0.2)',
        flexShrink: 0,
      }}>
        Click row to expand fields · Ctrl+Shift+T to toggle · ORCA_TRACE=1 enables backend logs
      </div>
    </div>
  )
}

const btnStyle: React.CSSProperties = {
  background: 'rgba(255,255,255,0.06)',
  border: '1px solid rgba(255,255,255,0.1)',
  borderRadius: 5,
  padding: '2px 8px',
  fontSize: 11,
  color: 'rgba(255,255,255,0.6)',
  cursor: 'pointer',
}
