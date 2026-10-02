import { describe, expect, it } from 'vitest'
import {
  hasPromptErrors,
  parseServerPromptError,
  validatePromptDraft,
  type PromptDraft
} from './mcp-prompt-validation'

const draft = (over: Partial<PromptDraft> = {}): PromptDraft => ({
  name: 'my_prompt',
  description: 'desc',
  arguments: [{ name: 'topic', description: '', required: true }],
  template: 'Explain {{topic}}',
  ...over
})
const none = new Set<string>()
const codes = (issues?: { code: string }[]): string[] => (issues ?? []).map((i) => i.code)

describe('validatePromptDraft', () => {
  it('accepts a valid draft', () => {
    expect(hasPromptErrors(validatePromptDraft(draft(), none))).toBe(false)
  })

  it('validates names: format, built-in collision, duplicates', () => {
    expect(codes(validatePromptDraft(draft({ name: 'Bad Name' }), none).name)).toEqual([
      'name_invalid'
    ])
    expect(codes(validatePromptDraft(draft({ name: 'ab' }), none).name)).toEqual(['name_invalid'])
    expect(codes(validatePromptDraft(draft({ name: 'plan_task' }), none).name)).toEqual([
      'name_builtin'
    ])
    expect(codes(validatePromptDraft(draft(), new Set(['my_prompt'])).name)).toEqual(['name_taken'])
  })

  it('limits description, template and argument count', () => {
    expect(
      codes(validatePromptDraft(draft({ description: 'x'.repeat(501) }), none).description)
    ).toEqual(['description_too_long'])
    expect(codes(validatePromptDraft(draft({ template: '' }), none).template)).toContain(
      'template_empty'
    )
    expect(
      codes(
        validatePromptDraft(draft({ template: 'x'.repeat(8193), arguments: [] }), none).template
      )
    ).toEqual(['template_too_long'])
    const many = Array.from({ length: 11 }, (_, i) => ({
      name: `a${i}`,
      description: '',
      required: false
    }))
    expect(
      codes(validatePromptDraft(draft({ arguments: many, template: 'x' }), none).form)
    ).toEqual(['too_many_args'])
  })

  it('validates argument names and duplicates', () => {
    const e = validatePromptDraft(
      draft({
        arguments: [
          { name: 'Bad', description: '', required: false },
          { name: 'ok', description: '', required: false },
          { name: 'ok', description: '', required: false }
        ],
        template: 'x'
      }),
      none
    )
    expect(e.arguments?.[0]?.name?.code).toBe('arg_name_invalid')
    expect(e.arguments?.[1]).toBeUndefined()
    expect(e.arguments?.[2]?.name?.code).toBe('arg_name_duplicate')
  })

  it('flags undeclared variables, unused required arguments and unsupported syntax', () => {
    const e = validatePromptDraft(draft({ template: 'Use {{other}} and {{.x}}' }), none)
    expect(codes(e.template).sort()).toEqual([
      'arg_unused_required',
      'unsupported_syntax',
      'var_undeclared'
    ])
    expect(e.template?.find((i) => i.code === 'var_undeclared')?.params).toEqual({ name: 'other' })
  })

  it('does not require optional arguments to be used', () => {
    const d = draft({
      arguments: [{ name: 'topic', description: '', required: false }],
      template: 'static'
    })
    expect(hasPromptErrors(validatePromptDraft(d, none))).toBe(false)
  })
})

describe('parseServerPromptError', () => {
  it('maps MCP_PROMPT_INVALID fields', () => {
    expect(
      parseServerPromptError('MCP_PROMPT_INVALID', 'template: too long').template?.[0].params
    ).toEqual({
      message: 'too long'
    })
    expect(
      parseServerPromptError('MCP_PROMPT_INVALID', 'MCP_PROMPT_INVALID: name: taken').name
    ).toHaveLength(1)
    expect(
      parseServerPromptError('MCP_PROMPT_INVALID', 'arguments[2].name: bad').arguments?.[2]?.name
    ).toBeTruthy()
    expect(parseServerPromptError('MCP_PROMPT_INVALID', 'weird: thing').form).toHaveLength(1)
    expect(parseServerPromptError('MCP_PROMPT_INVALID', 'no field').form?.[0].params).toEqual({
      message: 'no field'
    })
  })

  it('maps the other prompt codes and unknown codes', () => {
    expect(parseServerPromptError('MCP_PROMPT_NAME_CONFLICT', 'dup').name).toHaveLength(1)
    expect(parseServerPromptError('MCP_PROMPT_VERSION_CONFLICT', 'x').kind).toBe('version_conflict')
    expect(parseServerPromptError('MCP_PROMPT_BUILTIN_READONLY', 'x').kind).toBe('builtin_readonly')
    expect(parseServerPromptError('MCP_NOT_ADMIN', 'x').kind).toBe('not_admin')
    expect(parseServerPromptError(null, 'server says no').form?.[0].params).toEqual({
      message: 'server says no'
    })
  })
})
