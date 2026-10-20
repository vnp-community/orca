/**
 * Review artifact theme tokens — FE-OR-T210
 *
 * The drawing names its colours by role (`var(--rv-diff-add, fallback)`); the values are Orca's own tokens as they are now, read
 * when the drawing is shown, so changing the theme repaints a drawing without drawing it again.
 *
 * @module components/request/outcome-review/artifact/review-artifact-theme-tokens
 */

/** Role variable of the drawing → Orca token it takes its value from. */
export const ROLE_TOKENS: Record<string, string> = {
  '--rv-diff-add': '--status-success',
  '--rv-diff-mod': '--risk-medium',
  '--rv-diff-del': '--destructive',
  '--rv-accent': '--primary',
  '--rv-fg': '--foreground',
  '--rv-bg': '--background',
  '--rv-muted': '--muted-foreground',
  '--rv-border': '--border'
}

export function readThemeTokens(
  root: HTMLElement = document.documentElement
): Record<string, string> {
  const style = getComputedStyle(root)
  const out: Record<string, string> = {}
  for (const [role, token] of Object.entries(ROLE_TOKENS)) {
    const value = style.getPropertyValue(token).trim()
    if (value !== '') {
      out[role] = value
    }
  }
  return out
}

export function currentTheme(root: HTMLElement = document.documentElement): 'light' | 'dark' {
  return root.classList.contains('dark') ? 'dark' : 'light'
}
