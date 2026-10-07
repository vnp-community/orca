import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createServer, connect, type Server, type Socket } from 'node:net'
import { tmpdir } from 'node:os'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'

import { StdioWebSocketAdapter } from './agent-connection-stdio'
import { createSession } from './agent-session'
import { HEADER_SIZE, createWireState, decodeFrame, encodeDataFrame, parseJsonPayload } from 'orca-dev-agent-transport'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import type { AgentLogger } from './agent-logger'

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

describe('agent-connection-stdio-codeintel (Part A)', () => {
  let server: Server
  let sockPath: string
  let tmpDir: string
  let clientSock: Socket
  let serverSock: Socket

  beforeEach(async () => {
    tmpDir = mkdtempSync(join(tmpdir(), 'orca-stdio-ci-test-'))
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

  it('can send codeintel.status via stdio (Part A)', async () => {
    const adapter = new StdioWebSocketAdapter(mockLog, serverSock, serverSock)
    const session = createSession(mockConfig, [mockTool], mockLog, MOCK_CAPS)
    session.start(adapter as unknown as import('ws').default)

    
    // Send handshake response
    const handshakeAck = buildDataFrame(0, { jsonrpc: '2.0', id: 1, result: { ok: true, sessionId: 'test', orcaVersion: 'test' } })
    clientSock.write(handshakeAck)

    // Send codeintel.status request
    const reqFrame = buildDataFrame(1, { jsonrpc: '2.0', id: 123, method: 'codeintel.status', params: { workspaceRoot: '/' } })
    clientSock.write(reqFrame)


    const responses: any[] = []
    
    // Read responses
    clientSock.on('data', (buf: Buffer) => {
      if (buf.length >= HEADER_SIZE) {
        const type = buf[0]
        if (type === 1 /* Regular */) {
          const len = buf.readUInt32BE(9)
          if (buf.length >= HEADER_SIZE + len) {
            const payload = buf.subarray(HEADER_SIZE, HEADER_SIZE + len)
            responses.push(JSON.parse(payload.toString('utf8')))
          }
        }
      }
    })

    // Wait for the response to codeintel.status
    await vi.waitFor(() => {
      expect(responses.some((r) => r.id === 123 && (r.result || r.error))).toBe(true)
    }, { timeout: 2000 })

    const handshake = responses.find((r) => r.method === 'agent.handshake')
    expect(handshake).toBeDefined()
    expect(handshake.params).toBeDefined()

    const res = responses.find((r) => r.id === 123)
    expect(res).toBeDefined()
    if (res.error) {
      expect(typeof res.error.code).toBe('number')
    } else {
      expect(res.result).toBeDefined()
      expect(res.result.sources).toBeDefined()
      expect(res.result.data).toBeDefined()
    }
  })
})
