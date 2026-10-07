import WebSocket from 'ws'

export type CodeIntelNotificationSink = (method: string, params: any) => void

let currentSink: CodeIntelNotificationSink | null = null
let currentWs: WebSocket | null = null

export function setCodeIntelNotifier(sink: CodeIntelNotificationSink, ws: WebSocket | null = null) {
  currentSink = sink
  currentWs = ws
}

export function clearNotifierIfWs(ws: WebSocket) {
  if (currentWs === ws) {
    currentSink = null
    currentWs = null
  }
}

export function emitCodeIntelNotification(method: string, params: any) {
  if (!currentSink) return
  if (!params || typeof params.workspaceRoot !== 'string') return
  
  try {
    currentSink(method, params)
  } catch {
    // Ignore errors sending notification
  }
}
