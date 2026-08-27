/**
 * DevServerGithubCliProvider — IHostedCliProvider backed by a Dev Server's
 * existing agent-WebSocket connection (see dev-server-provider-lifecycle.ts).
 *
 * Calls the agent's `github.exec` RPC (agent-github-cli-handler.ts), which
 * validates the argv against an endpoint allowlist and runs `gh` with
 * per-user GH_CONFIG_DIR isolation. See ADR-018 /
 * specs/agent/api/gaps-and-findings.md.
 */
import type { DevServerRelayConnection } from './dev-server-relay-connection'
import type { IHostedCliProvider } from './types'

type ExecResult = { stdout: string; stderr: string; exitCode: number }

export class DevServerGithubCliProvider implements IHostedCliProvider {
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
      'github.exec',
      { args, cwd, userId, idempotent: options?.idempotent, env: options?.env },
      options?.timeoutMs
    )
    if (result.exitCode !== 0) {
      throw new Error(result.stderr.trim() || `gh ${args.join(' ')} exited with code ${result.exitCode}`)
    }
    return { stdout: result.stdout, stderr: result.stderr }
  }
}
