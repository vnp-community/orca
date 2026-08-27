/**
 * SshGithubCliProvider — IHostedCliProvider backed by an SSH relay
 * connection's JSON-RPC channel, mirroring SshGitProvider.exec()'s call
 * pattern for 'git.exec'. Calls the relay's `github.exec` method.
 *
 * Note: the relay-ssh binary that actually ships today is built from
 * desktop/src/relay/relay.ts, not agent/src/relay/relay.ts — the
 * `github.exec` RPC registered this session lives only on the agent's
 * direct-websocket/relay-websocket dispatcher (agent-rpc-dispatch.ts).
 * This provider is wired for architectural completeness (matching the
 * established 4-transport-class pattern) but won't have a live RPC
 * counterpart on relay-ssh connections until a dedicated desktop/ session
 * applies the equivalent change there — same accepted tech-debt pattern as
 * the rest of this session's agent-only-scoped fixes. See
 * specs/agent/api/compliance-audit-2026-08-15.md §1.
 */
import type { SshChannelMultiplexer } from '../ssh/ssh-channel-multiplexer'
import { requestGitStreamable } from '../ssh/ssh-git-response-stream-reader'
import type { IHostedCliProvider } from './types'

type ExecResult = { stdout: string; stderr: string; exitCode: number }

export class SshGithubCliProvider implements IHostedCliProvider {
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
      'github.exec',
      { args, cwd, userId, idempotent: options?.idempotent, env: options?.env },
      { timeoutMs: options?.timeoutMs }
    )) as ExecResult
    if (result.exitCode !== 0) {
      throw new Error(result.stderr.trim() || `gh ${args.join(' ')} exited with code ${result.exitCode}`)
    }
    return { stdout: result.stdout, stderr: result.stderr }
  }
}
