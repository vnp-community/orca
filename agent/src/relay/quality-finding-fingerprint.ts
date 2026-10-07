import crypto from 'crypto'
import fs from 'fs/promises'
import path from 'path'
import os from 'os'
import { RawQualityFinding } from './quality-parser-types'

export function normalizeAnchor(line: string | null): string {
  if (line === null || line === undefined) return ''
  let normalized = line.normalize('NFC').trim()
  normalized = normalized.replace(/\s+/g, ' ')
  if (normalized.length > 512) {
    normalized = normalized.slice(0, 512)
  }
  return normalized
}

export function normalizeMessage(
  msg: string,
  ctx: { repoRoot: string; home?: string; tmp?: string }
): string {
  let res = msg.normalize('NFC')
  
  // Replace paths
  if (ctx.repoRoot) {
    res = res.split(ctx.repoRoot).join('<repo>')
  }
  const home = ctx.home || os.homedir()
  if (home) {
    res = res.split(home).join('~')
  }
  const tmp = ctx.tmp || os.tmpdir()
  if (tmp) {
    res = res.split(tmp).join('<tmp>')
  }

  // Replace times
  res = res.replace(/\b\d+(\.\d+)?(ms|s|m|h)\b/g, '<time>')
  // Replace ISO dates (simplified)
  res = res.replace(/\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?/g, '<date>')

  // Replace line/col references like file.ts:10:5 or (10, 5) or line 10
  res = res.replace(/:\d+(:\d+)?/g, ':<loc>')
  res = res.replace(/\(\d+,\s*\d+\)/g, '<loc>')
  res = res.replace(/\bline \d+\b/gi, 'line <loc>')

  res = res.trim().replace(/\s+/g, ' ')
  
  if (res.length > 1024) {
    res = res.slice(0, 1024)
  }
  
  return res
}

export type FingerprintItem = Omit<RawQualityFinding, 'evidence'> & {
  tool: string
  anchor: string
  normMessage: string
}

export function assignOccurrences(items: FingerprintItem[]): Array<FingerprintItem & { occurrence: number }> {
  // Sort items first by line, then column, to assign deterministic occurrence
  const sorted = [...items].sort((a, b) => {
    if (a.line !== b.line) return a.line - b.line
    return a.column - b.column
  })

  const groups = new Map<string, number>()
  return sorted.map(item => {
    const key = `${item.tool}\0${item.ruleId}\0${item.file}\0${item.anchor}\0${item.normMessage}`
    const count = (groups.get(key) || 0) + 1
    groups.set(key, count)
    return { ...item, occurrence: count }
  })
}

export function computeFingerprint(item: FingerprintItem, occurrence: number): { fingerprint: string; fpVersion: 1 } {
  const parts = [
    'v1',
    item.tool,
    item.ruleId,
    item.file,
    item.anchor,
    item.normMessage,
    String(occurrence)
  ]
  const key = parts.join('\0')
  const hash = crypto.createHash('sha256').update(key).digest('hex')
  return { fingerprint: hash.slice(0, 32), fpVersion: 1 }
}

export function createSourceLineReader(repoRoot: string) {
  const cache = new Map<string, string[] | null>()
  const MAX_CACHE_FILES = 64
  const MAX_FILE_SIZE = 2 * 1024 * 1024 // 2 MiB

  const readLines = async (filePath: string): Promise<string[] | null> => {
    if (cache.has(filePath)) {
      // LRU bump
      const val = cache.get(filePath)!
      cache.delete(filePath)
      cache.set(filePath, val)
      return val
    }

    let lines: string[] | null = null
    try {
      const fullPath = path.resolve(repoRoot, filePath)
      if (!fullPath.startsWith(repoRoot)) {
        throw new Error('Path outside repoRoot')
      }

      const stat = await fs.stat(fullPath)
      if (stat.size > MAX_FILE_SIZE) {
        throw new Error('File too large')
      }

      const fd = await fs.open(fullPath, 'r')
      const buffer = Buffer.alloc(Math.min(stat.size, 8192))
      const { bytesRead } = await fd.read(buffer, 0, buffer.length, 0)
      
      // check NUL
      if (buffer.subarray(0, bytesRead).includes(0)) {
        await fd.close()
        throw new Error('Binary file')
      }
      
      const content = await fs.readFile(fullPath, 'utf8')
      await fd.close()

      lines = content.split(/\r?\n/)
    } catch (e) {
      lines = null
    }

    if (cache.size >= MAX_CACHE_FILES) {
      const firstKey = cache.keys().next().value
      if (firstKey) cache.delete(firstKey)
    }
    
    cache.set(filePath, lines)
    return lines
  }

  return async (file: string, line: number): Promise<string | null> => {
    if (!file || line < 1) return null
    const lines = await readLines(file)
    if (!lines) return null
    if (line > lines.length) return null
    return lines[line - 1]
  }
}
