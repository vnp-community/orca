export type CodeIntelNotificationSink = (method: string, params: any) => void

let currentSink: CodeIntelNotificationSink | null = null
let currentWs: object | null = null

export function setCodeIntelNotifier(sink: CodeIntelNotificationSink, ws: object | null = null) {
  currentSink = sink
  currentWs = ws
}

export function clearNotifierIfWs(ws: object) {
  if (currentWs === ws) {
    currentSink = null
    currentWs = null
  }
}

import { readRuntimeSwitches } from './codeintel/runtime-switches'

export function emitCodeIntelNotification(method: string, params: any) {
  if (readRuntimeSwitches().codeintelDisabled) return
  if (!currentSink) return
  if (!params || typeof params.workspaceRoot !== 'string') return
  
  try {
    currentSink(method, params)
  } catch {
    // Ignore errors sending notification
  }
}
