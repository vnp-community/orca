// src/renderer/src/lib/agent-ws-trace-status.ts
// Đọc lại (KHÔNG tạo) trace event agentWs:*/agentToken:* đã có trong store trace
// (nạp qua SSE /api/trace-stream) để hiển thị read-only trên DevServerCard.
// Không gọi RPC, không tạo span.
import type { TraceEvent } from '../../../shared/trace'

const AGENT_WS_FLOW_PREFIXES = ['agentWs:', 'agentToken:', 'agent:tokenManager'] as const

export type AgentWsCardStatus = {
  flow: string
  level: TraceEvent['level']
  ts: number
  reason?: string
}

/**
 * Trả về event agentWs: hoặc agentToken: gần nhất khớp devServerId, nếu có.
 * traceEvents nên là mảng đã giới hạn kích thước (ring buffer) — không quét
 * toàn bộ lịch sử mỗi render.
 */
export function latestAgentWsStatusForDevServer(
  traceEvents: readonly TraceEvent[],
  devServerId: string
): AgentWsCardStatus | null {
  for (let i = traceEvents.length - 1; i >= 0; i -= 1) {
    const event = traceEvents[i]
    // Defensive: typeof guard, not a bare `event.flow.startsWith` assumption.
    // TraceEvent.flow is typed as always-present, but a malformed event from
    // the SSE stream (any backend-go service can publish onto
    // orca.*.trace.span, forwarded byte-for-byte with no shape validation —
    // see browser.ts's own guard on this exact boundary) has crashed this
    // exact line in production (incident 2026-09-13, DevServerCard/Settings
    // > Dev Servers). Both layers guard independently on purpose.
    if (
      typeof event.flow !== 'string' ||
      !AGENT_WS_FLOW_PREFIXES.some((prefix) => event.flow.startsWith(prefix))
    ) {
      continue
    }
    if (event.fields.devServerId !== devServerId) {
      continue
    }
    return {
      flow: event.flow,
      level: event.level,
      ts: event.ts,
      reason: typeof event.fields.reason === 'string' ? event.fields.reason : undefined
    }
  }
  return null
}
