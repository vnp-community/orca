/**
 * DevServerGitlabCliProvider — IHostedCliProvider backed by a Dev Server's
 * existing agent-WebSocket connection. Mirrors
 * DevServerGithubCliProvider.ts, calling the agent's `gitlab.exec` RPC
 * (agent-gitlab-cli-handler.ts). See ADR-018 /
 * specs/agent/api/gaps-and-findings.md.
 */
import type { DevServerRelayConnection } from './dev-server-relay-connection'
import type { IHostedCliProvider } from './types'

type ExecResult = { stdout: string; stderr: string; exitCode: number }

export class DevServerGitlabCliProvider implements IHostedCliProvider {
  constructor(
    private readonly devServerId: string,
    private readonly relay: DevServerRelayConnection
  ) {}

  getConnectionId(): string {
    return this.devServerId
  }

  async exec(
    args: string[],
    cwd: string | undefined,
    userId: string | undefined,
    options?: { timeoutMs?: number; idempotent?: boolean; env?: Record<string, string> }
  ): Promise<{ stdout: string; stderr: string }> {
    const result = await this.relay.call<ExecResult>(
      'gitlab.exec',
      { args, cwd, userId, idempotent: options?.idempotent, env: options?.env },
      options?.timeoutMs
    )
    if (result.exitCode !== 0) {
      throw new Error(result.stderr.trim() || `glab ${args.join(' ')} exited with code ${result.exitCode}`)
    }
    return { stdout: result.stdout, stderr: result.stderr }
  }
}
