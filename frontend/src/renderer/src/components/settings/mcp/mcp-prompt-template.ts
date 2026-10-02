// Custom prompts only support `{{name}}` substitution (BE decides; this mirrors it for early feedback).
export const PROMPT_NAME_RE = /^[a-z][a-z0-9_]{2,47}$/
export const PROMPT_ARG_NAME_RE = /^[a-z][a-z0-9_]{0,31}$/
export const PROMPT_LIMITS = {
  template: 8192,
  description: 500,
  args: 10,
  customPerTenant: 50
} as const
export const BUILTIN_PROMPT_NAMES = [
  'review_pull_request',
  'triage_issue',
  'plan_task',
  'summarize_worktree',
  'handoff_to_agent'
] as const

const VAR_SOURCE = String.raw`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`
const BLOCK_RE = /\{\{[\s\S]*?\}\}/g

/** Variable names in order of first appearance, de-duplicated. */
export function extractTemplateVariables(template: string): string[] {
  const seen = new Set<string>()
  for (const m of template.matchAll(new RegExp(VAR_SOURCE, 'g'))) {
    seen.add(m[1])
  }
  return [...seen]
}

/** Short descriptions of constructs a custom prompt cannot use ({{.x}}, {{#if}}, pipes, stray braces). */
export function findUnsupportedSyntax(template: string): string[] {
  const rest = template.replace(new RegExp(VAR_SOURCE, 'g'), '')
  const found = new Set<string>()
  for (const m of rest.matchAll(BLOCK_RE)) {
    found.add(m[0].length > 30 ? `${m[0].slice(0, 29)}…` : m[0])
  }
  const leftover = rest.replace(BLOCK_RE, '')
  if (leftover.includes('{{') || leftover.includes('}}')) {
    found.add('{{ }}')
  }
  return [...found]
}

/** Display-only substitution; unknown samples show as ⟨name⟩. Output is plain text. */
export function renderTemplatePreview(template: string, samples: Record<string, string>): string {
  return template.replace(new RegExp(VAR_SOURCE, 'g'), (_m, name: string) => {
    const sample = Object.prototype.hasOwnProperty.call(samples, name) ? samples[name] : undefined
    return sample ?? `⟨${name}⟩`
  })
}
