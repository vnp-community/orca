import type { McpAdminSettings } from '../../../../../shared/mcp-types'

export type McpSettingsValues = Omit<McpAdminSettings, 'killSwitch'>
export type McpSettingsErrors = Partial<Record<'maxTokenDays' | 'approvalTtlSeconds', string>>

export const MAX_TOKEN_DAYS_RANGE = { min: 1, max: 90 } as const
export const APPROVAL_TTL_RANGE = { min: 30, max: 900 } as const

export function pickSettingsValues(s: McpAdminSettings): McpSettingsValues {
  return {
    enabled: s.enabled,
    dcrEnabled: s.dcrEnabled,
    maxTokenDays: s.maxTokenDays,
    approvalTtlSeconds: s.approvalTtlSeconds
  }
}

/** Only changed fields are sent (no version on settings; last save wins). */
export function diffSettings(
  base: McpSettingsValues,
  next: McpSettingsValues
): Partial<McpSettingsValues> {
  const patch: Partial<McpSettingsValues> = {}
  for (const key of Object.keys(base) as (keyof McpSettingsValues)[]) {
    if (base[key] !== next[key]) {
      ;(patch as Record<string, unknown>)[key] = next[key]
    }
  }
  return patch
}

const inRange = (v: number, r: { min: number; max: number }): boolean =>
  Number.isInteger(v) && v >= r.min && v <= r.max

/** Returns error kinds, not text; the component translates them. */
export function validateSettings(v: McpSettingsValues): McpSettingsErrors {
  const errors: McpSettingsErrors = {}
  if (!inRange(v.maxTokenDays, MAX_TOKEN_DAYS_RANGE)) {
    errors.maxTokenDays = 'range'
  }
  if (!inRange(v.approvalTtlSeconds, APPROVAL_TTL_RANGE)) {
    errors.approvalTtlSeconds = 'range'
  }
  return errors
}
