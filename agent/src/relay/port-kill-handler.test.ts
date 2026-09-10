import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { MethodHandler, RequestContext } from './dispatcher'
import { PortKillHandler, handlePortsKill } from './port-kill-handler'
import * as portScanHandler from './port-scan-handler'
import type * as PortScanHandlerModule from './port-scan-handler'

vi.mock('./port-scan-handler', async (importOriginal) => {
  const actual = await importOriginal<typeof PortScanHandlerModule>()
  return { ...actual, detectListeningPorts: vi.fn() }
})
const detectListeningPortsMock = vi.mocked(portScanHandler.detectListeningPorts)

function createHandlers(): Map<string, MethodHandler> {
  const handlers = new Map<string, MethodHandler>()
  new PortKillHandler({
    onRequest: (method: string, handler: MethodHandler): void => {
      handlers.set(method, handler)
    }
  } as never)
  return handlers
}

function requestContext(): RequestContext {
  return { clientId: 1, isStale: () => false }
}

describe('PortKillHandler', () => {
  let killSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    detectListeningPortsMock.mockReset()
    killSpy?.mockRestore()
    killSpy = vi.spyOn(process, 'kill').mockImplementation(() => true)
  })

  it('kills a pid that IS listening on the given port', async () => {
    detectListeningPortsMock.mockResolvedValue([
      { port: 3000, host: '127.0.0.1', pid: 4242, processName: 'node' }
    ])
    const handlers = createHandlers()

    const result = await handlers.get('ports.kill')!({ pid: 4242, port: 3000 }, requestContext())

    expect(result).toEqual({ ok: true })
    expect(killSpy).toHaveBeenCalledWith(4242, 'SIGTERM')
  })

  it('returns {ok: false} on a pid/port mismatch without calling process.kill', async () => {
    detectListeningPortsMock.mockResolvedValue([
      { port: 3000, host: '127.0.0.1', pid: 9999, processName: 'python' } // different pid
    ])
    const handlers = createHandlers()

    const result = await handlers.get('ports.kill')!({ pid: 4242, port: 3000 }, requestContext())

    expect(result).toEqual({ ok: false, reason: 'pid 4242 is not listening on port 3000' })
    expect(killSpy).not.toHaveBeenCalled()
  })

  it('returns {ok: false} when the port is not detected at all', async () => {
    detectListeningPortsMock.mockResolvedValue([])
    const handlers = createHandlers()

    const result = await handlers.get('ports.kill')!({ pid: 4242, port: 3000 }, requestContext())

    expect(result).toEqual({ ok: false, reason: 'pid 4242 is not listening on port 3000' })
    expect(killSpy).not.toHaveBeenCalled()
  })

  it('rejects an invalid pid without touching the process table', async () => {
    const handlers = createHandlers()

    const result = await handlers.get('ports.kill')!({ pid: -1, port: 3000 }, requestContext())

    expect(result).toEqual({ ok: false, reason: 'invalid pid' })
    expect(detectListeningPortsMock).not.toHaveBeenCalled()
    expect(killSpy).not.toHaveBeenCalled()
  })

  it('rejects a missing pid', async () => {
    const handlers = createHandlers()
    const result = await handlers.get('ports.kill')!({ port: 3000 }, requestContext())
    expect(result).toEqual({ ok: false, reason: 'invalid pid' })
  })

  it('rejects an invalid port without touching the process table', async () => {
    const handlers = createHandlers()

    const result = await handlers.get('ports.kill')!({ pid: 4242, port: 0 }, requestContext())

    expect(result).toEqual({ ok: false, reason: 'invalid port' })
    expect(detectListeningPortsMock).not.toHaveBeenCalled()
    expect(killSpy).not.toHaveBeenCalled()
  })

  it('surfaces a process.kill failure as {ok: false, reason}', async () => {
    detectListeningPortsMock.mockResolvedValue([
      { port: 3000, host: '127.0.0.1', pid: 4242, processName: 'node' }
    ])
    killSpy.mockImplementation(() => {
      throw new Error('ESRCH: no such process')
    })
    const handlers = createHandlers()

    const result = await handlers.get('ports.kill')!({ pid: 4242, port: 3000 }, requestContext())

    expect(result).toEqual({ ok: false, reason: 'ESRCH: no such process' })
  })
})

// handlePortsKill is ports.kill's entry point on the LIVE switch-based
// dispatcher (agent-rpc-dispatch.ts) — see that file's 'ports.kill' case.
// Covered separately from the class-based PortKillHandler above since both
// wire the same validation independently (see handlePortsKill's doc comment).
describe('handlePortsKill', () => {
  let killSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    detectListeningPortsMock.mockReset()
    killSpy = vi.spyOn(process, 'kill').mockImplementation(() => true)
  })

  it('returns a JSON-RPC success envelope with {ok: true} for a valid kill', async () => {
    detectListeningPortsMock.mockResolvedValue([
      { port: 3000, host: '127.0.0.1', pid: 4242, processName: 'node' }
    ])

    const response = await handlePortsKill('req-1', { pid: 4242, port: 3000 })

    expect(response).toEqual({ jsonrpc: '2.0', id: 'req-1', result: { ok: true } })
    expect(killSpy).toHaveBeenCalledWith(4242, 'SIGTERM')
  })

  it('returns a JSON-RPC success envelope with {ok: false} for an invalid pid', async () => {
    const response = await handlePortsKill('req-2', { pid: -1, port: 3000 })

    expect(response).toEqual({ jsonrpc: '2.0', id: 'req-2', result: { ok: false, reason: 'invalid pid' } })
    expect(detectListeningPortsMock).not.toHaveBeenCalled()
  })
})
