import { describe, it, expect, vi } from 'vitest'
import { setCodeIntelNotifier, clearNotifierIfWs, emitCodeIntelNotification } from './codeintel-notification-sink'
import WebSocket from 'ws'

describe('codeintel-notification-sink', () => {
  it('emits if workspaceRoot is present', () => {
    const sink = vi.fn()
    setCodeIntelNotifier(sink)
    
    emitCodeIntelNotification('test', { workspaceRoot: '/foo' })
    expect(sink).toHaveBeenCalledWith('test', { workspaceRoot: '/foo' })
  })

  it('drops if workspaceRoot is missing', () => {
    const sink = vi.fn()
    setCodeIntelNotifier(sink)
    
    emitCodeIntelNotification('test', { foo: 'bar' })
    emitCodeIntelNotification('test', null)
    expect(sink).not.toHaveBeenCalled()
  })

  it('drops if no sink is set', () => {
    setCodeIntelNotifier(null as any) // clear
    // Should not throw
    emitCodeIntelNotification('test', { workspaceRoot: '/foo' })
  })

  it('clears sink if matching ws', () => {
    const sink = vi.fn()
    const ws = {} as WebSocket
    setCodeIntelNotifier(sink, ws)
    
    clearNotifierIfWs({} as WebSocket) // different ws
    emitCodeIntelNotification('test', { workspaceRoot: '/foo' })
    expect(sink).toHaveBeenCalledTimes(1)
    
    clearNotifierIfWs(ws) // same ws
    emitCodeIntelNotification('test', { workspaceRoot: '/bar' })
    expect(sink).toHaveBeenCalledTimes(1) // not called again
  })
})
