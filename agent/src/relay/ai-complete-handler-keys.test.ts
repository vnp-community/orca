import { createServer, type IncomingHttpHeaders, type Server } from 'node:http'
import type { AddressInfo } from 'node:net'
import type * as CliSessionModule from './ai-complete-cli-session'
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('./agent-credential-store', () => ({ readDecryptedKey: vi.fn(async () => 'encrypted-blob') }))
vi.mock('./ai-complete-cli-session', async importOriginal => {
  const real = await importOriginal<typeof CliSessionModule>()
  return { ...real, cliSessionSupported: vi.fn(async () => false) }
})

const ENV_KEY = 'sk-ant-from-env-0000000000'
const FORWARDED_KEY = 'sk-ant-forwarded-1111111111'

const logged: string[] = []
const log = {
  info: (m: string) => logged.push(m),
  warn: (m: string) => logged.push(m),
  error: (m: string) => logged.push(m),
  debug: (m: string) => logged.push(m)
} as never

let server: Server
let seen: IncomingHttpHeaders[] = []

beforeAll(async () => {
  server = createServer((req, res) => {
    seen.push(req.headers)
    req.resume()
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({ content: [{ type: 'text', text: 'ok' }], usage: { input_tokens: 1, output_tokens: 1 } }))
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
})
afterAll(() => new Promise<void>(resolve => server.close(() => resolve())))

describe('ai.complete key resolution', () => {
  const saved = { ...process.env }
  beforeEach(() => {
    seen = []
    logged.length = 0
    for (const k of ['ANTHROPIC_API_KEY', 'OPENAI_API_KEY', 'GOOGLE_API_KEY', 'ORCA_AI_MODEL_ID']) {
      delete process.env[k]
    }
    process.env['ORCA_AI_ANTHROPIC_BASE_URL'] = `http://127.0.0.1:${(server.address() as AddressInfo).port}`
  })
  afterEach(() => {
    process.env = { ...saved }
  })

  async function run(params: Record<string, unknown>) {
    const { handleAIComplete } = await import('./ai-complete-handler')
    return handleAIComplete({ prompt: 'p', model: 'claude-sonnet-4', ...params } as never, {} as never, log)
  }

  it('prefers the key in the agent environment over a forwarded one', async () => {
    process.env['ANTHROPIC_API_KEY'] = ENV_KEY
    await run({ resolvedApiKey: FORWARDED_KEY })
    expect(seen[0]?.['x-api-key']).toBe(ENV_KEY)
  })

  it('uses the forwarded key when the environment has none', async () => {
    await run({ resolvedApiKey: FORWARDED_KEY })
    expect(seen[0]?.['x-api-key']).toBe(FORWARDED_KEY)
  })

  it('explains a server wiring gap when only an accountId is given and a credential exists', async () => {
    await expect(run({ accountId: 'acct-1' })).rejects.toThrow(/no plaintext resolvedApiKey/)
    expect(seen).toHaveLength(0)
  })

  it('never writes a key into the log', async () => {
    process.env['ANTHROPIC_API_KEY'] = ENV_KEY
    await run({ resolvedApiKey: FORWARDED_KEY })
    await run({ model: 'claude-sonnet-4' })
    const everything = logged.join('\n')
    expect(everything).not.toContain(ENV_KEY)
    expect(everything).not.toContain(FORWARDED_KEY)
  })

  it('goes to the official API when the test base URL is unset', async () => {
    delete process.env['ORCA_AI_ANTHROPIC_BASE_URL']
    const calls: string[] = []
    const real = globalThis.fetch
    globalThis.fetch = (async (url: unknown) => {
      calls.push(String(url))
      return new Response(JSON.stringify({ content: [{ type: 'text', text: 'ok' }] }), { status: 200 })
    }) as typeof fetch
    try {
      process.env['ANTHROPIC_API_KEY'] = ENV_KEY
      await run({})
    } finally {
      globalThis.fetch = real
    }
    expect(calls).toEqual(['https://api.anthropic.com/v1/messages'])
  })
})
