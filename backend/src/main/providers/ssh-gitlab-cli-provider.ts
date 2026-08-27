/**
 * SshGitlabCliProvider — IHostedCliProvider backed by an SSH relay
 * connection's JSON-RPC channel. Mirrors SshGithubCliProvider.ts — see its
 * header comment for the relay-ssh/desktop caveat.
 */
import type { SshChannelMultiplexer } from '../ssh/ssh-channel-multiplexer'
import { requestGitStreamable } from '../ssh/ssh-git-response-stream-reader'
import type { IHostedCliProvider } from './types'

type ExecResult = { stdout: string; stderr: string; exitCode: number }

export class SshGitlabCliProvider implements IHostedCliProvider {
  constructor(
    private readonly connectionId: string,
    private readonly mux: SshChannelMultiplexer
  ) {}

  getConnectionId(): string {
    return this.connectionId
  }

  async exec(
    args: string[],
    cwd: string | undefined,
    userId: string | undefined,
    options?: { timeoutMs?: number; idempotent?: boolean; env?: Record<string, string> }
  ): Promise<{ stdout: string; stderr: string }> {
    const result = (await requestGitStreamable(
      this.mux,
      'gitlab.exec',
      { args, cwd, userId, idempotent: options?.idempotent, env: options?.env },
      { timeoutMs: options?.timeoutMs }
    )) as ExecResult
    if (result.exitCode !== 0) {
      throw new Error(result.stderr.trim() || `glab ${args.join(' ')} exited with code ${result.exitCode}`)
    }
    return { stdout: result.stdout, stderr: result.stderr }
  }
}
