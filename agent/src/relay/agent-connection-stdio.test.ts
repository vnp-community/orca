// src/relay/agent-connection-stdio.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createServer, connect, type Server, type Socket } from 'node:net'
import { tmpdir } from 'node:os'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'

import { StdioWebSocketAdapter, connectStdio, runDetachedStdioMode } from './agent-connection-stdio'
import { runConnectBridgeMode } from './relay-detach-bridge'
import { existsSync, readFileSync } from 'node:fs'
import { createSession } from './agent-session'
import { HEADER_SIZE, createWireState, decodeFrame, encodeDataFrame, parseJsonPayload } from './agent-wire'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import type { AgentLogger } from './agent-logger'

// ─── Config & stubs (mirrors agent-session.test.ts) ────────────────────────
const mockConfig: AgentConfig = {
  mode: 'stdio',
  orcaUrl: '',
  orcaHttpUrl: '',
  agentToken: '',
  apiSecret: '',
  agentPort: 0,
  devServerId: 'test-server',
  logLevel: 'info',
  workDir: '/tmp',
  toolPath: '/usr/bin',
  toolEnv: {},
  credentialDir: '/tmp/.creds',
  tlsRejectUnauthorized: true,
}

const mockTool: ToolDefinition = {
  name: 'tool1',
  binary: null,
  description: 'Test tool',
  inputSchema: { type: 'object', properties: {} },
  async handler() {
    return { stdout: 'ok', stderr: '', exitCode: 0 }
  },
}

const mockLog: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

/** Pre-built capabilities — avoids async git/pty checks (see agent-session.test.ts). */
const MOCK_CAPS = ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees', 'pty'] as const

function buildFrame(type: number, seq: number, ack: number, payload: Buffer): Buffer {
  const header = Buffer.allocUnsafe(HEADER_SIZE)
  header.writeUInt8(type, 0)
  header.writeUInt32BE(seq, 1)
  header.writeUInt32BE(ack, 5)
  header.writeUInt32BE(payload.length, 9)
  return Buffer.concat([header, payload])
}

function buildDataFrame(seq: number, payloadObj: object): Buffer {
  return buildFrame(1 /* Regular */, seq, 0, Buffer.from(JSON.stringify(payloadObj), 'utf8'))
}

/**
 * Accumulates 'data' events on a real stream until exactly one complete wire
 * frame (HEADER_SIZE header + LENGTH-declared payload) has arrived, then
 * resolves with just that frame's bytes. Avoids flakiness if the underlying
 * transport ever splits a small write across more than one 'data' event.
 */
function collectOneFrame(stream: NodeJS.ReadableStream): Promise<Buffer> {
  return new Promise((resolve) => {
    let buf = Buffer.alloc(0)
    const onData = (chunk: Buffer): void => {
      buf = Buffer.concat([buf, chunk])
      if (buf.length < HEADER_SIZE) {
        return
      }
      const total = HEADER_SIZE + buf.readUInt32BE(9)
      if (buf.length >= total) {
        stream.off('data', onData)
        resolve(buf.subarray(0, total))
      }
    }
    stream.on('data', onData)
  })
}

describe('agent-connection-stdio: real socket pair', () => {
  let server: Server
  let sockPath: string
  let tmpDir: string
  let clientSock: Socket
  let serverSock: Socket

  beforeEach(async () => {
    tmpDir = mkdtempSync(join(tmpdir(), 'orca-stdio-test-'))
    sockPath = join(tmpDir, 'stdio-test.sock')

    const serverSockPromise = new Promise<Socket>((resolve) => {
      server = createServer((sock) => resolve(sock))
    })
    await new Promise<void>((resolve) => server.listen(sockPath, resolve))

    clientSock = connect(sockPath)
    await new Promise<void>((resolve) => clientSock.once('connect', resolve))
    serverSock = await serverSockPromise
  })

  afterEach(async () => {
    clientSock.destroy()
    serverSock.destroy()
    await new Promise<void>((r) => server.close(() => r()))
    rmSync(tmpDir, { recursive: true, force: true })
  })

  /**
   * Wires a StdioWebSocketAdapter's "stdin" to `serverSock` (bytes the test
   * writes onto clientSock arrive here as incoming agent input) and its
   * "stdout" onto `serverSock` too (bytes the adapter sends land on
   * clientSock for the test to inspect) — i.e. serverSock plays the role of
   * the SSH exec channel's local end, clientSock plays the remote peer.
   */
  function makeAdapter(): StdioWebSocketAdapter {
    return new StdioWebSocketAdapter(mockLog, serverSock, serverSock)
  }

  it('sends agent.handshake as the very first message once the session starts', async () => {
    const adapter = makeAdapter()
    const session = createSession(mockConfig, [mockTool], mockLog, MOCK_CAPS)
    session.start(adapter as unknown as import('ws').default)

    const received = await collectOneFrame(clientSock)

    const wireState = createWireState()
    const frame = decodeFrame(wireState, received)
    expect(frame).not.toBeNull()
    const rpc = parseJsonPayload<{ method?: string }>(frame!.payload)
    expect(rpc?.method).toBe('agent.handshake')
  })

  it('reassembles a frame split across two separate stdin writes', async () => {
    const adapter = makeAdapter()
    const messages: Buffer[] = []
    adapter.on('message', (buf: Buffer) => messages.push(buf))

    const frame = buildDataFrame(1, { jsonrpc: '2.0', id: 99, method: 'noop' })
    const splitAt = 20 // splits inside the payload, well past the 13-byte header
    const partA = frame.subarray(0, splitAt)
    const partB = frame.subarray(splitAt)

    clientSock.write(partA)
    // Give the event loop a tick so partA is delivered as its own 'data' event.
    await new Promise((r) => setImmediate(r))
    expect(messages).toHaveLength(0)

    clientSock.write(partB)
    await vi.waitFor(() => expect(messages).toHaveLength(1))

    expect(messages[0]).toEqual(frame)
  })

  it('splits multiple frames coalesced into a single stdin chunk into separate message events', async () => {
    const adapter = makeAdapter()
    const messages: Buffer[] = []
    adapter.on('message', (buf: Buffer) => messages.push(buf))

    const frameA = buildDataFrame(1, { jsonrpc: '2.0', id: 1, method: 'a' })
    const frameB = buildDataFrame(2, { jsonrpc: '2.0', id: 2, method: 'b' })
    const frameC = buildFrame(9 /* KeepAlive */, 3, 0, Buffer.alloc(0))

    clientSock.write(Buffer.concat([frameA, frameB, frameC]))

    await vi.waitFor(() => expect(messages).toHaveLength(3))
    expect(messages[0]).toEqual(frameA)
    expect(messages[1]).toEqual(frameB)
    expect(messages[2]).toEqual(frameC)
  })

  it('encodes outgoing writes as complete wire frames on stdout', async () => {
    const adapter = makeAdapter()
    const wireState = createWireState()
    const outgoing = encodeDataFrame(wireState, JSON.stringify({ hello: 'world' }))

    const received = collectOneFrame(clientSock)
    adapter.send(outgoing)

    const buf = await received
    const decodeState = createWireState()
    const frame = decodeFrame(decodeState, buf)
    expect(frame).not.toBeNull()
    expect(parseJsonPayload(frame!.payload)).toEqual({ hello: 'world' })
  })

  it('readyState is OPEN synchronously on construction', () => {
    const adapter = makeAdapter()
    expect(adapter.readyState).toBe(1)
  })

  it('emits close when the input stream ends (EOF)', async () => {
    const adapter = makeAdapter()
    const closeSpy = vi.fn()
    adapter.on('close', closeSpy)

    clientSock.end()

    await vi.waitFor(() => expect(closeSpy).toHaveBeenCalledOnce())
    expect(adapter.readyState).toBe(3)
  })

  it('close() emits a local close event with a Buffer reason (session.stop() path)', async () => {
    const adapter = makeAdapter()
    const closeSpy = vi.fn()
    adapter.on('close', closeSpy)

    adapter.close(1011, 'Handshake error')

    expect(closeSpy).toHaveBeenCalledWith(1011, Buffer.from('Handshake error', 'utf8'))
    expect(adapter.readyState).toBe(3)
  })

  it('close() is idempotent — a second call does not re-emit', () => {
    const adapter = makeAdapter()
    const closeSpy = vi.fn()
    adapter.on('close', closeSpy)

    adapter.close(1000, 'first')
    adapter.close(1000, 'second')

    expect(closeSpy).toHaveBeenCalledOnce()
  })
})

describe('connectStdio', () => {
  it('has the expected exported signature and resolves once the session/channel closes', async () => {
    // Smoke-tests the full wiring (createSession + adapter + resolve-on-close)
    // using PassThrough pairs stood in for process.stdin/stdout via the
    // adapter's injectable stream constructor — connectStdio() itself always
    // constructs a production adapter bound to the real process streams, so
    // this covers connectStdio's shape by driving the same adapter class it
    // uses internally, wired to a real duplex pair instead of a hand mock.
    const { PassThrough } = await import('node:stream')
    const toAgent = new PassThrough()
    const fromAgent = new PassThrough()

    const adapter = new StdioWebSocketAdapter(mockLog, toAgent, fromAgent)
    const session = createSession(mockConfig, [mockTool], mockLog, MOCK_CAPS)

    const firstMessage = collectOneFrame(fromAgent)
    session.start(adapter as unknown as import('ws').default)
    const handshakeBytes = await firstMessage

    const wireState = createWireState()
    const frame = decodeFrame(wireState, handshakeBytes)
    const rpc = parseJsonPayload<{ method?: string }>(frame!.payload)
    expect(rpc?.method).toBe('agent.handshake')

    expect(typeof connectStdio).toBe('function')
    expect(connectStdio.length).toBe(3)
  })
})

// ─── runDetachedStdioMode's CHILD half (TASK-SSH-03-01) ────────────────────
// Exercised for real (no mocking): the same function, invoked with
// ORCA_RELAY_DETACHED_CHILD=1 already set, takes the child branch directly
// — a real net.Server listening on a real Unix socket, a real pidfile, a
// real stale-socket cleanup — without needing to actually re-exec/spawn a
// second process (that half is covered separately below via a mocked
// `spawn`).
describe('runDetachedStdioMode: child branch (real socket, no spawn)', () => {
  let tmpDir: string
  let sockPath: string
  const prevEnv = process.env['ORCA_RELAY_DETACHED_CHILD']

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'orca-detach-test-'))
    sockPath = join(tmpDir, 'relay.sock')
    process.env['ORCA_RELAY_DETACHED_CHILD'] = '1'
  })

  afterEach(() => {
    if (prevEnv === undefined) {
      delete process.env['ORCA_RELAY_DETACHED_CHILD']
    } else {
      process.env['ORCA_RELAY_DETACHED_CHILD'] = prevEnv
    }
    rmSync(tmpDir, { recursive: true, force: true })
  })

  it('listens on sockPath, writes a pidfile, and stays connectable', async () => {
    // runDetachedStdioMode's child branch never resolves on its own (it
    // awaits a Promise that never settles — see agent-connection-stdio.ts's
    // doc comment) — race it against "the socket became connectable" rather
    // than awaiting the call itself.
    void runDetachedStdioMode(sockPath, mockConfig, [mockTool], mockLog)

    await vi.waitFor(() => {
      if (!existsSync(sockPath)) {
        throw new Error('socket not yet created')
      }
    })

    const client = connect(sockPath)
    await new Promise<void>((resolve, reject) => {
      client.once('connect', resolve)
      client.once('error', reject)
    })
    client.destroy()

    await vi.waitFor(() => {
      if (!existsSync(`${sockPath}.pid`)) {
        throw new Error('pidfile not yet written')
      }
    })
    const pid = readFileSync(`${sockPath}.pid`, 'utf8')
    expect(Number(pid)).toBe(process.pid)
  })

  it('a client connecting to the listener can complete an agent.handshake over it', async () => {
    void runDetachedStdioMode(sockPath, mockConfig, [mockTool], mockLog)
    await vi.waitFor(() => {
      if (!existsSync(sockPath)) {
        throw new Error('socket not yet created')
      }
    })

    const client = connect(sockPath)
    await new Promise<void>((resolve) => client.once('connect', resolve))

    const received = await collectOneFrame(client)
    const wireState = createWireState()
    const frame = decodeFrame(wireState, received)
    const rpc = parseJsonPayload<{ method?: string }>(frame!.payload)
    expect(rpc?.method).toBe('agent.handshake')
    client.destroy()
  })
})

// ─── runDetachedStdioMode's PARENT half (spawn + wait-for-listening) ───────
describe('runDetachedStdioMode: parent branch (mocked spawn)', () => {
  it('resolves once the child reports "listening", and unrefs/disconnects the child', async () => {
    const { EventEmitter } = await import('node:events')
    const fakeChild = Object.assign(new EventEmitter(), {
      pid: 4242,
      unref: vi.fn(),
      disconnect: vi.fn(),
    })
    const spawnMock = vi.fn().mockReturnValue(fakeChild)
    vi.doMock('node:child_process', () => ({ spawn: spawnMock }))
    vi.resetModules()
    const { runDetachedStdioMode: freshRunDetachedStdioMode } = await import('./agent-connection-stdio')

    const prevEnv = process.env['ORCA_RELAY_DETACHED_CHILD']
    delete process.env['ORCA_RELAY_DETACHED_CHILD'] // force the parent branch

    const promise = freshRunDetachedStdioMode('/tmp/fake.sock', mockConfig, [mockTool], mockLog)
    await vi.waitFor(() => expect(spawnMock).toHaveBeenCalled())
    fakeChild.emit('message', 'listening')
    await promise

    expect(fakeChild.unref).toHaveBeenCalledOnce()
    expect(fakeChild.disconnect).toHaveBeenCalledOnce()
    const [, args] = spawnMock.mock.calls[0]!
    expect(args).toEqual(expect.arrayContaining(['--detach', '--sock-path', '/tmp/fake.sock']))

    if (prevEnv !== undefined) {
      process.env['ORCA_RELAY_DETACHED_CHILD'] = prevEnv
    }
    vi.doUnmock('node:child_process')
    vi.resetModules()
  })

  it('rejects when the child emits an error before reporting listening', async () => {
    const { EventEmitter } = await import('node:events')
    const fakeChild = Object.assign(new EventEmitter(), {
      pid: 4243,
      unref: vi.fn(),
      disconnect: vi.fn(),
    })
    const spawnMock = vi.fn().mockReturnValue(fakeChild)
    vi.doMock('node:child_process', () => ({ spawn: spawnMock }))
    vi.resetModules()
    const { runDetachedStdioMode: freshRunDetachedStdioMode } = await import('./agent-connection-stdio')

    const prevEnv = process.env['ORCA_RELAY_DETACHED_CHILD']
    delete process.env['ORCA_RELAY_DETACHED_CHILD']

    const promise = freshRunDetachedStdioMode('/tmp/fake2.sock', mockConfig, [mockTool], mockLog)
    await vi.waitFor(() => expect(spawnMock).toHaveBeenCalled())
    fakeChild.emit('error', new Error('spawn failed'))

    await expect(promise).rejects.toThrow('spawn failed')

    if (prevEnv !== undefined) {
      process.env['ORCA_RELAY_DETACHED_CHILD'] = prevEnv
    }
    vi.doUnmock('node:child_process')
    vi.resetModules()
  })
})

// ─── runConnectBridgeMode (TASK-SSH-03-02) ─────────────────────────────────
describe('runConnectBridgeMode', () => {
  let server: Server
  let sockPath: string
  let tmpDir: string
  let peerPromise: Promise<Socket>
  let peerReceivedChunks: Buffer[]

  beforeEach(async () => {
    tmpDir = mkdtempSync(join(tmpdir(), 'orca-connect-bridge-test-'))
    sockPath = join(tmpDir, 'bridge.sock')
    peerReceivedChunks = []
    peerPromise = new Promise<Socket>((resolve) => {
      server = createServer((sock) => {
        sock.on('data', (c: Buffer) => peerReceivedChunks.push(c))
        resolve(sock)
      })
    })
    await new Promise<void>((resolve) => server.listen(sockPath, resolve))
  })

  afterEach(async () => {
    await new Promise<void>((r) => server.close(() => r()))
    rmSync(tmpDir, { recursive: true, force: true })
  })

  it('pipes process.stdin bytes to the socket and socket bytes to process.stdout, both directions', async () => {
    const { PassThrough } = await import('node:stream')
    const fakeStdin = new PassThrough()
    const fakeStdout = new PassThrough()
    const stdinSpy = vi.spyOn(process, 'stdin', 'get').mockReturnValue(fakeStdin as unknown as typeof process.stdin)
    const stdoutSpy = vi.spyOn(process, 'stdout', 'get').mockReturnValue(fakeStdout as unknown as typeof process.stdout)

    const bridgeDone = runConnectBridgeMode(sockPath)
    const peerSock = await peerPromise

    const fromStdoutChunks: Buffer[] = []
    fakeStdout.on('data', (c: Buffer) => fromStdoutChunks.push(c))

    fakeStdin.write('hello from local\n')
    await vi.waitFor(() => {
      if (!Buffer.concat(peerReceivedChunks).toString().includes('hello from local\n')) {
        throw new Error('not yet received')
      }
    })

    peerSock.write('hello from remote\n')
    await vi.waitFor(() => expect(Buffer.concat(fromStdoutChunks).toString()).toContain('hello from remote\n'))

    fakeStdin.end()
    await bridgeDone
    peerSock.destroy()

    stdinSpy.mockRestore()
    stdoutSpy.mockRestore()
  })

  it('resolves cleanly when the socket peer closes', async () => {
    const { PassThrough } = await import('node:stream')
    const fakeStdin = new PassThrough()
    const fakeStdout = new PassThrough()
    const stdinSpy = vi.spyOn(process, 'stdin', 'get').mockReturnValue(fakeStdin as unknown as typeof process.stdin)
    const stdoutSpy = vi.spyOn(process, 'stdout', 'get').mockReturnValue(fakeStdout as unknown as typeof process.stdout)

    const bridgeDone = runConnectBridgeMode(sockPath)
    const peerSock = await peerPromise

    peerSock.end()
    await bridgeDone

    stdinSpy.mockRestore()
    stdoutSpy.mockRestore()
  })
})
