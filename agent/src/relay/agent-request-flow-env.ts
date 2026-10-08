// Environment for agent runs started by the Request flow. Why: provider keys live on the dev server; Orca
// tokens and credential references must never ride along in `env` (CR-REQ-035).
export const REQUEST_FLOW_ENV_ALLOWLIST = ['ORCA_REQUEST_ID', 'ORCA_PROJECT_ID'] as const

export function buildRequestFlowEnv(requestId: string, projectId: string): Record<string, string> {
  return { ORCA_REQUEST_ID: requestId, ORCA_PROJECT_ID: projectId }
}

export function isWithinRequestFlowEnvAllowlist(env: Record<string, string>): boolean {
  return Object.keys(env).every((k) =>
    (REQUEST_FLOW_ENV_ALLOWLIST as readonly string[]).includes(k)
  )
}
