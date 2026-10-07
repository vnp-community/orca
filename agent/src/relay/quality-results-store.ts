import fs from 'fs'
import { QualityFinding, QualityStepResult } from './quality-run-types'

export interface ResultsStoreDeps {
  now: () => number
  ttlMs: number
  maxRunsPerWorktree: number
  redact: (text: string) => string
}

export interface StoreEntry {
  root: string
  runId: string
  steps: QualityStepResult[]
  findings: QualityFinding[]
  logPaths: Record<string, { stdout: string; stderr: string }>
  timestamp: number
  outsideScopeCount: number
}

export function createResultsStore(deps: ResultsStoreDeps) {
  const store = new Map<string, StoreEntry>()

  function evict() {
    const n = deps.now()
    const runsByRoot = new Map<string, string[]>()

    for (const [runId, entry] of store.entries()) {
      if (n - entry.timestamp > deps.ttlMs) {
        store.delete(runId)
        continue
      }
      if (!runsByRoot.has(entry.root)) runsByRoot.set(entry.root, [])
      runsByRoot.get(entry.root)!.push(runId)
    }

    for (const [root, runs] of runsByRoot.entries()) {
      if (runs.length > deps.maxRunsPerWorktree) {
        // Sort oldest first
        runs.sort((a, b) => store.get(a)!.timestamp - store.get(b)!.timestamp)
        const toRemove = runs.length - deps.maxRunsPerWorktree
        for (let i = 0; i < toRemove; i++) {
          store.delete(runs[i])
        }
      }
    }
  }

  function getEntry(root: string, runId: string) {
    evict()
    const entry = store.get(runId)
    if (!entry) return null
    if (entry.root !== root) {
      const err = new Error('Run not found for worktree')
      ;(err as any).code = 'RUN_NOT_FOUND'
      throw err
    }
    return entry
  }

  function paginateArray<T>(items: T[], offset: number, limit: number, maxBytes = 1024 * 1024) {
    if (offset < 0 || offset >= items.length) {
      return { items: [], nextOffset: null }
    }
    limit = Math.min(limit, 500)
    let end = offset
    let bytes = 0
    const chunk: T[] = []

    while (end < items.length && chunk.length < limit) {
      const item = items[end]
      const sz = Buffer.byteLength(JSON.stringify(item), 'utf8')
      if (bytes + sz > maxBytes && chunk.length > 0) {
        break
      }
      chunk.push(item)
      bytes += sz
      end++
    }
    return { items: chunk, nextOffset: end < items.length ? end : null }
  }

  return {
    put(root: string, runId: string, data: { steps: QualityStepResult[]; findings: QualityFinding[]; logPaths: Record<string, { stdout: string; stderr: string }> }) {
      evict()
      store.set(runId, {
        root,
        runId,
        steps: data.steps,
        findings: data.findings.filter(f => !f.outsideScope),
        logPaths: data.logPaths,
        timestamp: deps.now(),
        outsideScopeCount: data.findings.filter(f => f.outsideScope).length
      })
      evict()
    },

    getFindings(root: string, runId: string, opts: { offset: number; limit: number }) {
      const entry = getEntry(root, runId)
      if (!entry) {
        return { view: 'findings', totalCount: 0, truncated: false, outsideScopeCount: 0, nextOffset: null, items: [] }
      }
      const p = paginateArray(entry.findings, opts.offset, opts.limit)
      return {
        view: 'findings',
        totalCount: entry.findings.length,
        truncated: false,
        outsideScopeCount: entry.outsideScopeCount,
        nextOffset: p.nextOffset,
        items: p.items
      }
    },

    getSteps(root: string, runId: string, opts: { offset: number; limit: number }) {
      const entry = getEntry(root, runId)
      if (!entry) {
        return { view: 'steps', totalCount: 0, nextOffset: null, items: [] }
      }
      const p = paginateArray(entry.steps, opts.offset, opts.limit)
      return {
        view: 'steps',
        totalCount: entry.steps.length,
        nextOffset: p.nextOffset,
        items: p.items
      }
    },

    getLog(root: string, runId: string, stepId: string, offset: number) {
      const entry = getEntry(root, runId)
      if (!entry) return { view: 'log', nextOffset: null, text: '' }

      const paths = entry.logPaths[stepId]
      if (!paths) {
        const err = new Error('Missing stepId')
        ;(err as any).code = 'INVALID_PARAMS'
        throw err
      }

      let s1 = 0
      let s2 = 0
      try { s1 = fs.statSync(paths.stdout).size } catch {}
      try { s2 = fs.statSync(paths.stderr).size } catch {}

      const total = s1 + s2
      if (offset >= total) {
        return { view: 'log', nextOffset: null, text: '' }
      }

      const maxRead = 65536 + 512
      const readLen = Math.min(maxRead, total - offset)
      const buf = Buffer.alloc(readLen)
      let ptr = 0

      if (offset < s1) {
        try {
          const fd = fs.openSync(paths.stdout, 'r')
          const n = fs.readSync(fd, buf, 0, Math.min(s1 - offset, readLen), offset)
          ptr += n
          fs.closeSync(fd)
        } catch {}
      }

      if (ptr < readLen && offset + ptr >= s1) {
        const errOff = Math.max(0, offset + ptr - s1)
        try {
          const fd = fs.openSync(paths.stderr, 'r')
          const n = fs.readSync(fd, buf, ptr, readLen - ptr, errOff)
          ptr += n
          fs.closeSync(fd)
        } catch {}
      }

      let str = buf.toString('utf8', 0, ptr)
      
      let nextOffset: number | null = offset + ptr
      if (nextOffset >= total) {
        nextOffset = null
      } else {
        const lastNewline = str.lastIndexOf('\n')
        if (lastNewline > 0) {
          const cutBytes = Buffer.byteLength(str.substring(0, lastNewline + 1), 'utf8')
          str = str.substring(0, lastNewline + 1)
          nextOffset = offset + cutBytes
        }
      }

      return {
        view: 'log',
        nextOffset,
        text: deps.redact(str)
      }
    }
  }
}
