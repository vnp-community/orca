import { describe, expect, it } from 'vitest'
import {
  extractTemplateVariables,
  findUnsupportedSyntax,
  renderTemplatePreview
} from './mcp-prompt-template'

describe('extractTemplateVariables', () => {
  it.each([
    ['Hi {{a}}', ['a']],
    ['Hi {{ a }}', ['a']],
    ['{{a}} {{b}} {{a}}', ['a', 'b']],
    ['no vars', []],
    ['{{.a}} {{#if a}}', []]
  ])('%s', (template, expected) => {
    expect(extractTemplateVariables(template)).toEqual(expected)
  })
})

describe('findUnsupportedSyntax', () => {
  it.each([
    ['{{a}} and {{ b }}', 0],
    ['{{.a}}', 1],
    ['{{#if a}}x{{/if}}', 2],
    ['{{ a | upper }}', 1],
    ['{{a', 1],
    ['a}}', 1],
    ['plain', 0]
  ])('%s -> %i finding(s)', (template, count) => {
    expect(findUnsupportedSyntax(template)).toHaveLength(count)
  })

  it('truncates very long constructs', () => {
    expect(findUnsupportedSyntax(`{{${'x '.repeat(40)}}}`)[0].length).toBeLessThanOrEqual(30)
  })
})

describe('renderTemplatePreview', () => {
  it('fills samples and shows placeholders for the rest', () => {
    expect(renderTemplatePreview('Hello {{name}} {{ other }}', { name: 'Ann' })).toBe(
      'Hello Ann ⟨other⟩'
    )
  })

  it('does not interpret prototype keys as samples', () => {
    expect(renderTemplatePreview('{{constructor}}', {})).toBe('⟨constructor⟩')
  })

  it('leaves markup untouched as text', () => {
    expect(renderTemplatePreview('<img onerror=x> {{a}}', {})).toBe('<img onerror=x> ⟨a⟩')
  })
})
