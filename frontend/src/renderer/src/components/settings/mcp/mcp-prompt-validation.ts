import {
  BUILTIN_PROMPT_NAMES,
  PROMPT_ARG_NAME_RE,
  PROMPT_LIMITS,
  PROMPT_NAME_RE,
  extractTemplateVariables,
  findUnsupportedSyntax
} from './mcp-prompt-template'

export type PromptDraft = {
  id?: string
  version?: number
  name: string
  description: string
  arguments: { name: string; description: string; required: boolean }[]
  template: string
}

export type PromptIssueCode =
  | 'name_invalid'
  | 'name_builtin'
  | 'name_taken'
  | 'description_too_long'
  | 'template_empty'
  | 'template_too_long'
  | 'too_many_args'
  | 'arg_name_invalid'
  | 'arg_name_duplicate'
  | 'var_undeclared'
  | 'arg_unused_required'
  | 'unsupported_syntax'
  | 'server'

/** Codes plus params so the component (not this pure module) owns translation. */
export type PromptIssue = { code: PromptIssueCode; params?: Record<string, string | number> }

export type PromptFieldErrors = {
  name?: PromptIssue[]
  description?: PromptIssue[]
  template?: PromptIssue[]
  arguments?: Record<number, { name?: PromptIssue }>
  form?: PromptIssue[]
  /** Set from server error codes that need dedicated UI. */
  kind?: 'version_conflict' | 'builtin_readonly' | 'not_admin'
}

export function hasPromptErrors(e: PromptFieldErrors): boolean {
  return Boolean(
    e.kind ||
    e.name?.length ||
    e.description?.length ||
    e.template?.length ||
    e.form?.length ||
    (e.arguments && Object.keys(e.arguments).length > 0)
  )
}

function validateArguments(d: PromptDraft): PromptFieldErrors['arguments'] {
  const out: NonNullable<PromptFieldErrors['arguments']> = {}
  const seen = new Set<string>()
  d.arguments.forEach((a, i) => {
    if (!PROMPT_ARG_NAME_RE.test(a.name)) {
      out[i] = { name: { code: 'arg_name_invalid' } }
    } else if (seen.has(a.name)) {
      out[i] = { name: { code: 'arg_name_duplicate' } }
    }
    seen.add(a.name)
  })
  return Object.keys(out).length ? out : undefined
}

function validateTemplate(d: PromptDraft): PromptIssue[] {
  const issues: PromptIssue[] = []
  if (d.template.trim() === '') {
    issues.push({ code: 'template_empty' })
  }
  if (d.template.length > PROMPT_LIMITS.template) {
    issues.push({ code: 'template_too_long', params: { max: PROMPT_LIMITS.template } })
  }
  const unsupported = findUnsupportedSyntax(d.template)
  if (unsupported.length > 0) {
    issues.push({ code: 'unsupported_syntax', params: { found: unsupported.join(', ') } })
  }
  const declared = new Set(d.arguments.map((a) => a.name))
  const used = extractTemplateVariables(d.template)
  for (const v of used) {
    if (!declared.has(v)) {
      issues.push({ code: 'var_undeclared', params: { name: v } })
    }
  }
  for (const a of d.arguments) {
    if (a.required && a.name && !used.includes(a.name)) {
      issues.push({ code: 'arg_unused_required', params: { name: a.name } })
    }
  }
  return issues
}

/** Mirrors backend rules for early feedback; the server stays the source of truth. */
export function validatePromptDraft(
  d: PromptDraft,
  existingNames: ReadonlySet<string>
): PromptFieldErrors {
  const out: PromptFieldErrors = {}
  if (!PROMPT_NAME_RE.test(d.name)) {
    out.name = [{ code: 'name_invalid' }]
  } else if ((BUILTIN_PROMPT_NAMES as readonly string[]).includes(d.name)) {
    out.name = [{ code: 'name_builtin' }]
  } else if (existingNames.has(d.name)) {
    out.name = [{ code: 'name_taken' }]
  }
  if (d.description.length > PROMPT_LIMITS.description) {
    out.description = [{ code: 'description_too_long', params: { max: PROMPT_LIMITS.description } }]
  }
  if (d.arguments.length > PROMPT_LIMITS.args) {
    out.form = [{ code: 'too_many_args', params: { max: PROMPT_LIMITS.args } }]
  }
  const args = validateArguments(d)
  if (args) {
    out.arguments = args
  }
  const template = validateTemplate(d)
  if (template.length) {
    out.template = template
  }
  return out
}

const SERVER_PREFIX = /^MCP_PROMPT_INVALID:\s*/
const ARG_FIELD = /^arguments(?:\[(\d+)\]|\.(\d+))(?:\.name)?$/

/** `MCP_PROMPT_INVALID: <field>: <reason>` -> error on that field; unknown fields go to the form banner. */
export function parseServerPromptError(code: string | null, message: string): PromptFieldErrors {
  const issue = (m: string): PromptIssue => ({ code: 'server', params: { message: m } })
  switch (code) {
    case 'MCP_PROMPT_VERSION_CONFLICT':
      return { kind: 'version_conflict', form: [issue(message)] }
    case 'MCP_PROMPT_BUILTIN_READONLY':
      return { kind: 'builtin_readonly', form: [issue(message)] }
    case 'MCP_NOT_ADMIN':
      return { kind: 'not_admin', form: [issue(message)] }
    case 'MCP_PROMPT_NAME_CONFLICT':
      return { name: [issue(message)] }
    case 'MCP_PROMPT_INVALID': {
      const body = message.replace(SERVER_PREFIX, '')
      const sep = body.indexOf(': ')
      const field = sep > 0 ? body.slice(0, sep).trim() : ''
      const reason = sep > 0 ? body.slice(sep + 2) : body
      if (field === 'name' || field === 'description' || field === 'template') {
        return { [field]: [issue(reason)] }
      }
      const arg = ARG_FIELD.exec(field)
      if (arg) {
        return { arguments: { [Number(arg[1] ?? arg[2])]: { name: issue(reason) } } }
      }
      return { form: [issue(body)] }
    }
    default:
      return { form: [issue(message)] }
  }
}
