import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { MCP_ERROR_CODES, MCP_EVENT_TYPES, MCP_RPC_METHODS, MCP_STREAM_METHODS } from './mcp-types'
import { createFakeMcpBackend } from '../renderer/src/test-support/mcp-fake-backend'

// frontend/ may be split out of the monorepo ("isolated copy"); without the CONTRACT file the
// drift half is skipped, but the source-vs-shared half always runs.
const srcRoot = resolve(import.meta.dirname, '..')
const contractPath = resolve(srcRoot, '../../specs/backend-go/crs/v5/CONTRACT-mcp-ui-api.md')
const hasContract = existsSync(contractPath)

// Non-channel strings that happen to look like `mcp.<x>` (the per-repo .mcp.json config file).
const NOT_CHANNELS = new Set(['mcp.json'])

function productionSources(dir: string, out: string[] = []): string[] {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name)
    if (e.isDirectory()) {
      if (e.name !== 'test-support' && e.name !== 'node_modules') {
        productionSources(p, out)
      }
    } else if (/\.tsx?$/.test(e.name) && !/\.(test|spec)\.tsx?$/.test(e.name)) {
      out.push(p)
    }
  }
  return out
}

const CHANNEL_LITERAL = /['"`](mcp\.[A-Za-z][A-Za-z0-9]*(?:\.[A-Za-z][A-Za-z0-9]*)+)['"`]/g
const ERROR_LITERAL = /['"`](MCP_[A-Z0-9_]+)['"`]/g

function literalsByFile(re: RegExp): Map<string, string[]> {
  const found = new Map<string, string[]>()
  for (const file of productionSources(srcRoot)) {
    for (const m of readFileSync(file, 'utf8').matchAll(re)) {
      found.set(m[1], [...(found.get(m[1]) ?? []), relative(srcRoot, file)])
    }
  }
  return found
}

const section = (md: string, from: RegExp, to: RegExp): string => {
  const start = md.search(from)
  const rest = md.slice(start)
  const end = rest.slice(1).search(to)
  return end === -1 ? rest : rest.slice(0, end + 1)
}
const sorted = (xs: Iterable<string>): string[] => [...new Set(xs)].sort()

describe('frontend sources vs shared MCP constants', () => {
  const known = new Set<string>([...MCP_RPC_METHODS, ...MCP_STREAM_METHODS])

  it('every mcp.* channel literal in production code is a known contract channel', () => {
    const unknown = [...literalsByFile(CHANNEL_LITERAL)]
      .filter(([name]) => !known.has(name) && !NOT_CHANNELS.has(name))
      .map(([name, files]) => `${name} (${files[0]})`)
    expect(unknown).toEqual([])
  })

  it('every MCP_* error code literal in production code is a known contract code', () => {
    const unknown = [...literalsByFile(ERROR_LITERAL)]
      .filter(([name]) => !(MCP_ERROR_CODES as readonly string[]).includes(name))
      .map(([name, files]) => `${name} (${files[0]})`)
    expect(unknown).toEqual([])
  })

  it('the fake backend only implements real contract channels', () => {
    const fake = createFakeMcpBackend()
    expect(fake.handlerChannels.filter((c) => !known.has(c))).toEqual([])
  })

  it('has no duplicate channel or error entries', () => {
    expect(new Set(MCP_RPC_METHODS).size).toBe(MCP_RPC_METHODS.length)
    expect(new Set(MCP_ERROR_CODES).size).toBe(MCP_ERROR_CODES.length)
  })
})

describe.skipIf(!hasContract)('CONTRACT-mcp-ui-api.md vs shared MCP constants', () => {
  const md = hasContract ? readFileSync(contractPath, 'utf8') : ''

  it('channel table (section 2.1/2.2) equals MCP_RPC_METHODS + MCP_STREAM_METHODS', () => {
    const body = section(md, /^## 2\. /m, /^## 3\. /m)
    const rows = [...body.matchAll(/^\| `(mcp\.[A-Za-z.]+)` \|/gm)].map((m) => m[1])
    expect(rows.length).toBeGreaterThan(30)
    expect(sorted(rows)).toEqual(sorted([...MCP_RPC_METHODS, ...MCP_STREAM_METHODS]))
  })

  it('error code list (section 2.3) equals MCP_ERROR_CODES', () => {
    const body = section(md, /^### 2\.3 /m, /^## 3\. /m)
    const codes = [...body.matchAll(/`(MCP_[A-Z0-9_]+)`/g)].map((m) => m[1])
    expect(codes.length).toBeGreaterThan(20)
    expect(sorted(codes)).toEqual(sorted(MCP_ERROR_CODES))
  })

  it('McpEvent union (section 1) equals MCP_EVENT_TYPES', () => {
    const body = section(md, /^type McpEvent =/m, /^```$/m)
    const types = [...body.matchAll(/type: '([a-z.]+)'/g)].map((m) => m[1])
    expect(sorted(types)).toEqual(sorted(MCP_EVENT_TYPES))
  })

  it('every non-stream contract channel is used by production code (no dead contract)', () => {
    const used = new Set(literalsByFile(CHANNEL_LITERAL).keys())
    expect(MCP_RPC_METHODS.filter((m) => !used.has(m))).toEqual([])
  })
})
