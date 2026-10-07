import fs from 'fs'
import path from 'path'
import { runGit } from './codeintel-git-exec'

export type SymbolSourceResult = {
  text?: string
  truncated?: boolean
  sourceOmitted?: 'gitignored' | 'binary' | 'sensitive'
}

function isSensitiveFile(filePath: string): boolean {
  const base = path.basename(filePath)
  if (base.startsWith('.env')) return true
  if (base.endsWith('.pem') || base.endsWith('.key')) return true
  if (base.startsWith('id_rsa')) return true
  return false
}

export async function readSymbolSource(
  workspaceRoot: string,
  filePath: string,
  startLine: number,
  endLine: number
): Promise<SymbolSourceResult> {
  if (isSensitiveFile(filePath)) {
    return { sourceOmitted: 'sensitive' }
  }

  const absPath = path.join(workspaceRoot, filePath)
  
  let realPath = ''
  try {
    realPath = fs.realpathSync.native(absPath)
  } catch {
    return { text: '' } // file doesn't exist
  }

  const realRoot = fs.realpathSync.native(workspaceRoot)
  if (realPath !== realRoot && !realPath.startsWith(realRoot + path.sep)) {
    return { text: '' } // outside workspace
  }

  try {
    const res = await runGit(['check-ignore', '-q', '--', realPath], workspaceRoot, { throwOnError: false })
    if (res.exitCode === 0) {
      return { sourceOmitted: 'gitignored' }
    }
  } catch {}

  const fd = fs.openSync(realPath, 'r')
  try {
    const buf = Buffer.alloc(8192, 1)
    const bytesRead = fs.readSync(fd, buf, 0, 8192, 0)
    if (bytesRead > 0 && buf.subarray(0, bytesRead).includes(0)) {
      return { sourceOmitted: 'binary' }
    }
  } finally {
    fs.closeSync(fd)
  }

  const content = fs.readFileSync(realPath, 'utf8')
  const lines = content.split('\n')
  
  // startLine and endLine are 1-based
  // if endLine is missing or large, it bounds to array length
  const s = Math.max(0, startLine - 1)
  const e = Math.min(lines.length, endLine) // endLine is inclusive in typical AST, so slice(s, endLine) gives lines up to endLine
  
  const snippet = lines.slice(s, e).join('\n')
  
  const MAX_BYTES = 200 * 1024
  if (Buffer.byteLength(snippet, 'utf8') > MAX_BYTES) {
    let truncatedStr = snippet
    while (Buffer.byteLength(truncatedStr, 'utf8') > MAX_BYTES) {
      const dropCount = Math.floor(truncatedStr.split('\n').length / 2)
      truncatedStr = truncatedStr.split('\n').slice(0, -dropCount).join('\n')
    }
    return { text: truncatedStr, truncated: true }
  }

  return { text: snippet }
}
