// src/relay/agent-capability-params.ts
// Allowlists and parameter validation for agent.capabilities. Split out of agent-capability-report.ts to
// keep that file within the line budget.

export const CAPABILITY_TOOL_ALLOWLIST: readonly { id: string; args: readonly string[] }[] = [
  { id: 'go', args: ['version'] },
  { id: 'node', args: ['--version'] },
  { id: 'pnpm', args: ['--version'] },
  { id: 'npm', args: ['--version'] },
  { id: 'git', args: ['--version'] },
  { id: 'openspec', args: ['--version'] },
  { id: 'tdd-agent', args: ['--version'] },
  { id: 'claude', args: ['--version'] },
  { id: 'codegraph', args: ['--version'] },
  { id: 'gitnexus', args: ['--version'] },
  { id: 'rg', args: ['--version'] },
  { id: 'make', args: ['--version'] },
  { id: 'semgrep', args: ['--version'] }
]

export const DEFAULT_ENV_NAMES = ['ANTHROPIC_API_KEY', 'OPENAI_API_KEY', 'GOOGLE_API_KEY'] as const

export const ENV_NAME_PATTERN = /^[A-Z][A-Z0-9_]{0,63}$/

export type CapabilityReportParams = {
  tools?: string[]
  envNames?: string[]
  refresh?: boolean
}

export function validateCapabilityParams(raw: Record<string, unknown>):
  | {
      ok: true
      value: Required<CapabilityReportParams> & {
        unknownTools: string[]
        rejectedEnvNames: string[]
      }
    }
  | { ok: false; code: 'INVALID_CAPABILITY_PARAMS' | 'TOO_MANY_ENV_NAMES'; message: string } {
  if (raw['tools'] !== undefined && !Array.isArray(raw['tools'])) {
    return {
      ok: false,
      code: 'INVALID_CAPABILITY_PARAMS',
      message: 'tools must be an array of strings'
    }
  }
  if (raw['tools'] && (raw['tools'] as unknown[]).some((t) => typeof t !== 'string')) {
    return {
      ok: false,
      code: 'INVALID_CAPABILITY_PARAMS',
      message: 'tools must be an array of strings'
    }
  }
  if (raw['envNames'] !== undefined && !Array.isArray(raw['envNames'])) {
    return {
      ok: false,
      code: 'INVALID_CAPABILITY_PARAMS',
      message: 'envNames must be an array of strings'
    }
  }
  if (raw['envNames'] && (raw['envNames'] as unknown[]).some((n) => typeof n !== 'string')) {
    return {
      ok: false,
      code: 'INVALID_CAPABILITY_PARAMS',
      message: 'envNames must be an array of strings'
    }
  }
  if (raw['refresh'] !== undefined && typeof raw['refresh'] !== 'boolean') {
    return { ok: false, code: 'INVALID_CAPABILITY_PARAMS', message: 'refresh must be a boolean' }
  }

  const envNames: string[] = (raw['envNames'] as string[] | undefined) ?? [...DEFAULT_ENV_NAMES]
  if (envNames.length > 64) {
    return {
      ok: false,
      code: 'TOO_MANY_ENV_NAMES',
      message: `envNames must have at most 64 items, got ${envNames.length}`
    }
  }

  const requestedTools: string[] =
    (raw['tools'] as string[] | undefined) ?? CAPABILITY_TOOL_ALLOWLIST.map((t) => t.id)
  const allowedIds = new Set(CAPABILITY_TOOL_ALLOWLIST.map((t) => t.id))
  const validTools = requestedTools.filter((t) => allowedIds.has(t))
  const unknownTools = requestedTools.filter((t) => !allowedIds.has(t))

  const validEnvNames = envNames.filter((n) => ENV_NAME_PATTERN.test(n))
  const rejectedEnvNames = envNames.filter((n) => !ENV_NAME_PATTERN.test(n))

  return {
    ok: true,
    value: {
      tools: validTools,
      envNames: validEnvNames,
      refresh: (raw['refresh'] as boolean | undefined) ?? false,
      unknownTools,
      rejectedEnvNames
    }
  }
}
