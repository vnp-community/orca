/**
 * review-report-theme-tokens.ts — FE-CV-TASK-090-04
 *
 * Measures the app's color tokens (light and dark) at export time so the exported
 * HTML follows the design system without any color literal in this source.
 *
 * @module components/review-map/report/review-report-theme-tokens
 */

export const REPORT_TOKEN_VARIABLES = {
  background: '--background',
  foreground: '--foreground',
  muted: '--muted',
  mutedForeground: '--muted-foreground',
  border: '--border',
  destructive: '--destructive'
} as const

export type ReportTokenName = keyof typeof REPORT_TOKEN_VARIABLES
export type ReportColorTokens = Record<ReportTokenName, string>
export type ReportThemeTokens = { light: ReportColorTokens; dark: ReportColorTokens }

// CSS system colors keep the export readable when measurement is unavailable (no DOM).
const SYSTEM_COLOR_FALLBACK: ReportColorTokens = {
  background: 'Canvas',
  foreground: 'CanvasText',
  muted: 'Canvas',
  mutedForeground: 'GrayText',
  border: 'GrayText',
  destructive: 'CanvasText'
}

function measure(isDark: boolean): ReportColorTokens {
  const probe = document.createElement('div')
  if (isDark) {probe.className = 'dark'}
  probe.style.display = 'none'
  document.body.appendChild(probe)
  try {
    const style = getComputedStyle(probe)
    const out = { ...SYSTEM_COLOR_FALLBACK }
    for (const [name, variable] of Object.entries(REPORT_TOKEN_VARIABLES) as [ReportTokenName, string][]) {
      const value = style.getPropertyValue(variable).trim()
      // Why: only plain resolved values are copied; a leftover var() would not resolve in the exported file.
      if (value && !value.includes('var(')) {out[name] = value}
    }
    return out
  } finally {
    probe.remove()
  }
}

export function readReportThemeTokens(): ReportThemeTokens {
  if (typeof document === 'undefined' || !document.body) {
    return { light: { ...SYSTEM_COLOR_FALLBACK }, dark: { ...SYSTEM_COLOR_FALLBACK } }
  }
  return { light: measure(false), dark: measure(true) }
}
