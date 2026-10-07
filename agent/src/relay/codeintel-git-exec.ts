import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)

export async function runGit(
  args: string[],
  cwd: string,
  opts: { timeoutMs?: number; signal?: AbortSignal; maxBuffer?: number } = {}
): Promise<{ stdout: string; stderr: string }> {
  try {
    const { stdout, stderr } = await execFileAsync('git', args, {
      cwd,
      timeout: opts.timeoutMs ?? 10000,
      maxBuffer: opts.maxBuffer ?? 10 * 1024 * 1024,
      signal: opts.signal,
      env: {
        ...process.env,
        GIT_OPTIONAL_LOCKS: '0',
        LC_ALL: 'C'
      }
    })
    return { stdout, stderr }
  } catch (err: any) {
    throw err
  }
}
