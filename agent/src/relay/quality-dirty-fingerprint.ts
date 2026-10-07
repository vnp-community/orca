import { execFile } from 'child_process'
import { promisify } from 'util'
import path from 'path'
import fs from 'fs'
import { createHash } from 'crypto'

const exec = promisify(execFile)

export async function dirtyFingerprint(root: string): Promise<string> {
  const { stdout } = await exec('git', ['-c', 'core.quotePath=false', 'status', '--porcelain=v1', '-z', '--untracked-files=normal'], { cwd: root, maxBuffer: 10 * 1024 * 1024 })
  
  const chunks = stdout.split('\0')
  const files: { path: string; status: string }[] = []
  
  let i = 0
  while (i < chunks.length && files.length < 5000) {
    if (!chunks[i]) {
      i++
      continue
    }
    const st = chunks[i].substring(0, 2)
    const p = chunks[i].substring(3)
    files.push({ path: p, status: st })
    
    if (st[0] === 'R' || st[0] === 'C') {
      i++ // skip old path
    }
    i++
  }

  const hash = createHash('sha256')
  for (const f of files) {
    let mtime = 0
    let size = 0
    try {
      if (!f.status.includes('D')) {
        const stat = await fs.promises.stat(path.join(root, f.path))
        mtime = stat.mtimeMs
        size = stat.size
      }
    } catch {
      // ignore
    }
    hash.update(`${f.status} ${f.path} ${mtime}:${size}\\n`)
  }

  return 'sha256:' + hash.digest('hex')
}
