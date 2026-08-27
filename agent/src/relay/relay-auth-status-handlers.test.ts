import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { RelayDispatcher } from './dispatcher'
import { encodeJsonRpcFrame, MessageType, type JsonRpcRequest } from './protocol'
import { registerAuthStatusHandlers } from './relay-auth-status-handlers'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'

vi.mock('./external-api-connector', () => ({
  handleGitHubAuthStatus: vi.fn(),
  handleGitLabAuthStatus: vi.fn()
}))

function decodeFirstFrame(buf: Buffer): { type: number; id: number; ack: number; payload: Buffer } {
  const type = buf[0]
  const id = buf.readUInt32BE(1)
  const ack = buf.readUInt32BE(5)
  const len = buf.readUInt32BE(9)
  const payload = buf.subarray(13, 13 + len)
  return { type, id, ack, payload }
}

const FAKE_CONFIG: AgentConfig = {
  mode: 'stdio',
  orcaUrl: '',
  orcaHttpUrl: '',
  agentToken: '',
  apiSecret: '',
  agentPort: 0,
  devServerId: 'dev-server-1',
  logLevel: 'info',
  workDir: '/work',
  toolPath: '/usr/bin',
  toolEnv: {},
  credentialDir: '/tmp/creds',
  tlsRejectUnauthorized: true
}

const NOOP_LOG: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

// Regression test for TASK-INT-01-02/SOL-INT-01: relay-ssh Dev Servers
// (Part B, RelayDispatcher) must be able to answer github.auth.status/
// gitlab.auth.status — previously implemented only in Part A
// (agent-rpc-dispatch.ts). Reuses handleGitHubAuthStatus/
// handleGitLabAuthStatus verbatim (mocked here so the test doesn't depend
// on a real `gh`/`glab` binary being installed).
describe('registerAuthStatusHandlers (relay-ssh / Part B)', () => {
  let dispatcher: RelayDispatcher
  let written: Buffer[]

  beforeEach(async () => {
    vi.useFakeTimers()
    written = []
    dispatcher = new RelayDispatcher((data) => {
      written.push(Buffer.from(data))
    })
    registerAuthStatusHandlers(dispatcher, FAKE_CONFIG, NOOP_LOG)

    const { handleGitHubAuthStatus, handleGitLabAuthStatus } = await import('./external-api-connector')
    vi.mocked(handleGitHubAuthStatus).mockReset()
    vi.mocked(handleGitLabAuthStatus).mockReset()
  })

  afterEach(() => {
    dispatcher.dispose()
    vi.useRealTimers()
  })

  function sendRequest(method: string, id: number): void {
    const req: JsonRpcRequest = { jsonrpc: '2.0', id, method, params: { userId: 'user-1' } }
    dispatcher.feed(encodeJsonRpcFrame(req, id, 0))
  }

  function findResponseFor(id: number): { id: number; result?: unknown; error?: { code: number; message: string } } | undefined {
    for (const buf of written) {
      const frame = decodeFirstFrame(buf)
      if (frame.type !== MessageType.Regular) {
        continue
      }
      try {
        const msg = JSON.parse(frame.payload.toString('utf-8')) as {
          id: number
          result?: unknown
          error?: { code: number; message: string }
        }
        if (msg.id === id && ('result' in msg || 'error' in msg)) {
          return msg
        }
      } catch {
        continue
      }
    }
    return undefined
  }

  it('github.auth.status resolves through RelayDispatcher with the same {ok, stdout, stderr} shape Part A returns', async () => {
    const { handleGitHubAuthStatus } = await import('./external-api-connector')
    vi.mocked(handleGitHubAuthStatus).mockResolvedValue({
      jsonrpc: '2.0',
      id: null,
      result: { ok: true, stdout: 'Logged in to github.com', stderr: '' }
    })

    sendRequest('github.auth.status', 1)
    await vi.advanceTimersByTimeAsync(0)

    expect(handleGitHubAuthStatus).toHaveBeenCalledWith(
      null,
      { userId: 'user-1' },
      FAKE_CONFIG,
      NOOP_LOG
    )
    const resp = findResponseFor(1)
    expect(resp?.result).toEqual({ ok: true, stdout: 'Logged in to github.com', stderr: '' })
  })

  it('gitlab.auth.status resolves through RelayDispatcher with the same {ok, stdout, stderr} shape Part A returns', async () => {
    const { handleGitLabAuthStatus } = await import('./external-api-connector')
    vi.mocked(handleGitLabAuthStatus).mockResolvedValue({
      jsonrpc: '2.0',
      id: null,
      result: { ok: true, stdout: 'Logged in to gitlab.com', stderr: '' }
    })

    sendRequest('gitlab.auth.status', 2)
    await vi.advanceTimersByTimeAsync(0)

    expect(handleGitLabAuthStatus).toHaveBeenCalledWith(
      null,
      { userId: 'user-1' },
      FAKE_CONFIG,
      NOOP_LOG
    )
    const resp = findResponseFor(2)
    expect(resp?.result).toEqual({ ok: true, stdout: 'Logged in to gitlab.com', stderr: '' })
  })

  it('propagates an error-shaped response as a JSON-RPC error frame', async () => {
    const { handleGitHubAuthStatus } = await import('./external-api-connector')
    vi.mocked(handleGitHubAuthStatus).mockResolvedValue({
      jsonrpc: '2.0',
      id: null,
      error: { code: -33000, message: 'github.auth.status unavailable: gh not found' }
    })

    sendRequest('github.auth.status', 3)
    await vi.advanceTimersByTimeAsync(0)

    const resp = findResponseFor(3)
    expect(resp?.error).toEqual({ code: -33000, message: 'github.auth.status unavailable: gh not found' })
  })
})
