/**
 * Run configuration for the live-stack UI flow. Mirrors orca_stack_config.py: the admin of the local stack comes from
 * deploy/dev/.env (or ORCA_E2E_* overrides) and is never printed or put in a trace.
 */
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const STACK_ENV_FILE = resolve(__dirname, '../../../../../deploy/dev/.env')

function readEnvFile(path: string): Record<string, string> {
  const values: Record<string, string> = {}
  let text = ''
  try {
    text = readFileSync(path, 'utf8')
  } catch {
    return values
  }
  for (const raw of text.split('\n')) {
    const line = raw.trim()
    const eq = line.indexOf('=')
    if (!line || line.startsWith('#') || eq < 0) {
      continue
    }
    values[line.slice(0, eq).trim()] = line
      .slice(eq + 1)
      .trim()
      .replace(/^['"]|['"]$/g, '')
  }
  return values
}

export type LiveStackConfig = {
  gatewayUrl: string
  frontendUrl: string
  adminEmail: string
  readonly adminPassword: string
  devServerName: string
  /** A git repo on the dev server; empty means "make a scratch repo on this machine". */
  repoPath: string
  runPrefix: string
  /** Approving a Plan runs an AI agent on the dev server; off unless the operator opts in. */
  executeAgent: boolean
  /** `stub` when run_ui_flow.py swapped the agent's claude for the deterministic fake: shortens the AI waits. */
  aiMode: 'stub' | 'real'
  /** `fail` (default) turns every known product gap into a failed soft assertion; `annotate` only records it. */
  gapMode: 'fail' | 'annotate'
}

export function isLiveRun(): boolean {
  return process.env.ORCA_E2E_LIVE === '1'
}

export function loadLiveStackConfig(): LiveStackConfig {
  const stack = readEnvFile(STACK_ENV_FILE)
  const pick = (name: string, stackKey = '', fallback = ''): string =>
    process.env[name] || (stackKey ? stack[stackKey] : '') || fallback
  const stamp = new Date().toISOString().replace(/[-:T]/g, '').slice(4, 14) // MMDDhhmmss, same shape as the Python runner's prefix
  const cfg: LiveStackConfig = {
    gatewayUrl: pick(
      'ORCA_E2E_GATEWAY_URL',
      '',
      `http://localhost:${stack.API_GATEWAY_PUBLIC_PORT ?? '8081'}`
    ).replace(/\/$/, ''),
    frontendUrl: pick(
      'ORCA_E2E_FRONTEND_URL',
      '',
      `http://localhost:${stack.FRONTEND_HTTP_PORT ?? '8080'}`
    ).replace(/\/$/, ''),
    adminEmail: pick('ORCA_E2E_ADMIN_EMAIL', 'BOOTSTRAP_ADMIN_EMAIL'),
    adminPassword: pick('ORCA_E2E_ADMIN_PASSWORD', 'BOOTSTRAP_ADMIN_PASSWORD'),
    devServerName: pick('ORCA_E2E_DEV_SERVER_NAME', '', 'dev-local-mac'),
    repoPath: pick('ORCA_E2E_REPO_PATH'),
    runPrefix: `e2e-ui-${process.env.ORCA_E2E_RUN_ID || stamp}`,
    executeAgent: ['1', 'true', 'yes'].includes(
      (process.env.ORCA_E2E_EXECUTE_AGENT ?? '').toLowerCase()
    ),
    aiMode: process.env.ORCA_E2E_AI_MODE === 'stub' ? 'stub' : 'real',
    gapMode: process.env.ORCA_E2E_LIVE_GAPS === 'annotate' ? 'annotate' : 'fail'
  }
  // Why: a getter-less copy would be logged by `console.log(cfg)`; keep the secret out of enumerable keys.
  Object.defineProperty(cfg, 'adminPassword', { enumerable: false })
  return cfg
}

/** Only the front-end URL is needed at config-load time (Playwright baseURL), without touching secrets. */
export function liveFrontendUrl(): string {
  return loadLiveStackConfig().frontendUrl
}

/** Seconds the spec waits for an AI-backed step: short for the fake claude, generous for a real model. */
export function aiBudgetMs(cfg: LiveStackConfig, stubSeconds: number, realSeconds: number): number {
  return (cfg.aiMode === 'stub' ? stubSeconds : realSeconds) * 1000
}
