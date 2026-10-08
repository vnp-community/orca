import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  CODE_INTEL_METHOD_LIMITS,
  CODE_INTEL_PUSH_EVENTS,
  CODE_INTEL_RPC_METHODS,
  CODE_INTEL_STREAM_METHODS,
  getMethodMaxArgsBytes
} from './code-intel-rpc-methods'

// frontend/ may be split out of the monorepo; without the CONTRACT file the drift half is skipped,
// but the source-vs-constants half always runs.
const srcRoot = resolve(import.meta.dirname, '..')
const contractPath = resolve(srcRoot, '../../specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md')
const hasContract = existsSync(contractPath)

/** Channel names from a contract table section, e.g. `status` -> `codeIntel.status`. */
function channelsInSection(md: string, heading: string): string[] {
  const start = md.indexOf(heading)
  const next = md.indexOf('\n### ', start + heading.length)
  const section = md.slice(start, next === -1 ? undefined : next)
  const names: string[] = []
  for (const m of section.matchAll(/^\| `([A-Za-z][A-Za-z0-9.]*)`( \(stream\))? \|/gm)) {
    names.push(`codeIntel.${m[1]}`)
  }
  return names
}

/** Set difference helper; the "fake channel turns the test red" case relies on it. */
export function diffChannels(actual: readonly string[], expected: readonly string[]) {
  const a = new Set(actual)
  const e = new Set(expected)
  return {
    extra: [...a].filter((x) => !e.has(x)).sort(),
    missing: [...e].filter((x) => !a.has(x)).sort()
  }
}

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

describe('code-intel constants (always run)', () => {
  const methods = Object.values(CODE_INTEL_RPC_METHODS)

  it('has exactly 46 distinct channels: 26 codeIntel.* + 20 codeIntel.quality.*', () => {
    expect(new Set(methods).size).toBe(46)
    expect(methods.filter((m) => m.startsWith('codeIntel.quality.'))).toHaveLength(20)
    expect(methods.filter((m) => !m.startsWith('codeIntel.quality.'))).toHaveLength(26)
  })

  it('subscribe is the only stream', () => {
    expect(CODE_INTEL_STREAM_METHODS).toEqual(['codeIntel.subscribe'])
    expect(methods).toContain('codeIntel.subscribe')
  })

  it('has the 5 push events', () => {
    expect([...CODE_INTEL_PUSH_EVENTS]).toEqual([
      'changed',
      'reindexProgress',
      'qualityProgress',
      'qualityFinished',
      'gateChanged'
    ])
  })

  it('limits follow §2.4', () => {
    expect(CODE_INTEL_METHOD_LIMITS['codeIntel.reviewState.save'].maxArgsBytes).toBe(256 * 1024)
    expect(CODE_INTEL_METHOD_LIMITS['codeIntel.c4.save'].maxArgsBytes).toBe(96 * 1024)
    expect(CODE_INTEL_METHOD_LIMITS['codeIntel.quality.profile.save'].maxArgsBytes).toBe(96 * 1024)
    expect(CODE_INTEL_METHOD_LIMITS['codeIntel.quality.trace.confirm'].maxArgsBytes).toBe(8 * 1024)
    expect(CODE_INTEL_METHOD_LIMITS['codeIntel.quality.trace.link'].maxArgsBytes).toBe(8 * 1024)
    expect(getMethodMaxArgsBytes('codeIntel.status')).toBe(16 * 1024)
  })

  it('diffChannels reports a fake extra channel (the conformance check would go red)', () => {
    const d = diffChannels([...methods, 'codeIntel.fake'], methods)
    expect(d.extra).toEqual(['codeIntel.fake'])
    expect(d.missing).toEqual([])
  })

  it('production code never uses the forbidden codeIntel.send channel', () => {
    const offenders = productionSources(srcRoot).filter((f) =>
      /['"`]codeIntel\.send['"`]/.test(readFileSync(f, 'utf8'))
    )
    expect(offenders.map((f) => relative(srcRoot, f))).toEqual([])
  })
})

describe.skipIf(!hasContract)('code-intel contract drift (CONTRACT-codeintel-ui-api.md)', () => {
  const md = hasContract ? readFileSync(contractPath, 'utf8') : ''

  it('§3.1 + §3.2 channel tables match CODE_INTEL_RPC_METHODS exactly', () => {
    const fromContract = [
      ...channelsInSection(md, '### 3.1 '),
      ...channelsInSection(md, '### 3.2 ')
    ]
    expect(fromContract).toHaveLength(46)
    expect(diffChannels(Object.values(CODE_INTEL_RPC_METHODS), fromContract)).toEqual({
      extra: [],
      missing: []
    })
  })

  it('§5 lists the push event families the client normalizes', () => {
    for (const wire of ['changed', 'reindexProgress', 'quality.progress', 'quality.finished', 'quality.gateChanged']) {
      expect(md, wire).toContain(`event: ${wire === 'changed' ? "'changed'" : `'${wire}'`}`)
    }
  })

  it('§2.4 byte limits match the table text', () => {
    expect(md).toMatch(/`reviewState\.save` ≤ 256 KiB/)
    expect(md).toMatch(/`c4\.save` ≤ 96 KiB/)
  })
})
