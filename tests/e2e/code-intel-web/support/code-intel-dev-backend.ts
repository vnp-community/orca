// Dev-stack mode: set CODE_INTEL_E2E_BASE_URL to run @dev-stack specs against a real stack
// (BE-CV-SOL-073). Without it those specs are skipped, never faked.
export const devStackUrl = process.env.CODE_INTEL_E2E_BASE_URL ?? null
export const DEV_STACK_SKIP_REASON = 'needs a real code-intel dev stack: set CODE_INTEL_E2E_BASE_URL'

export const devCredentials = {
  admin: {
    email: process.env.CODE_INTEL_E2E_ADMIN_EMAIL ?? '',
    password: process.env.CODE_INTEL_E2E_ADMIN_PASSWORD ?? ''
  }
}
