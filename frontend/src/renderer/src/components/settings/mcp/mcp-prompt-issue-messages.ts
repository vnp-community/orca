import { translate } from '@/i18n/i18n'
import type { PromptIssue } from './mcp-prompt-validation'

/** Translation lives here (called at render time) so the validation module stays pure. */
export function promptIssueMessage(issue: PromptIssue): string {
  const p = issue.params ?? {}
  switch (issue.code) {
    case 'name_invalid':
      return translate(
        'auto.mcp.prompts.err.nameInvalid',
        'Use 3-48 characters: lowercase letters, digits and underscores, starting with a letter.'
      )
    case 'name_builtin':
      return translate(
        'auto.mcp.prompts.err.nameBuiltin',
        'This name is used by a built-in prompt.'
      )
    case 'name_taken':
      return translate('auto.mcp.prompts.err.nameTaken', 'A prompt with this name already exists.')
    case 'description_too_long':
      return translate(
        'auto.mcp.prompts.err.descriptionLong',
        'Keep the description under {{max}} characters.',
        p
      )
    case 'template_empty':
      return translate('auto.mcp.prompts.err.templateEmpty', 'The template cannot be empty.')
    case 'template_too_long':
      return translate(
        'auto.mcp.prompts.err.templateLong',
        'The template is limited to {{max}} characters.',
        p
      )
    case 'too_many_args':
      return translate(
        'auto.mcp.prompts.err.tooManyArgs',
        'A prompt can have at most {{max}} arguments.',
        p
      )
    case 'arg_name_invalid':
      return translate(
        'auto.mcp.prompts.err.argNameInvalid',
        'Use lowercase letters, digits and underscores, starting with a letter (max 32).'
      )
    case 'arg_name_duplicate':
      return translate('auto.mcp.prompts.err.argNameDuplicate', 'Argument names must be unique.')
    case 'var_undeclared':
      return translate(
        'auto.mcp.prompts.err.varUndeclared',
        '{{variable}} is used but not declared as an argument.',
        { variable: `{{${String(p.name)}}}` }
      )
    case 'arg_unused_required':
      return translate(
        'auto.mcp.prompts.err.argUnused',
        'Required argument "{{name}}" is not used in the template.',
        p
      )
    case 'unsupported_syntax':
      return translate(
        'auto.mcp.prompts.err.unsupported',
        'Unsupported syntax: {{found}}. Only simple variables such as {{example}} are allowed.',
        { found: p.found, example: '{{name}}' }
      )
    default:
      // Server text is shown verbatim (plain text).
      return String(p.message ?? '')
  }
}
