import { describe, expect, it } from 'vitest'
import { guardMermaidSource, MAX_MERMAID_BYTES } from './review-report-diagram-guard'

describe('guardMermaidSource', () => {
  it('accepts ordinary diagrams, including arrows', () => {
    expect(guardMermaidSource('flowchart TD\n  A --> B\n  B <--> C')).toEqual({
      ok: true,
      src: 'flowchart TD\n  A --> B\n  B <--> C'
    })
    expect(guardMermaidSource('classDiagram\n  A <|-- B').ok).toBe(true)
  })

  it.each([
    ['fence', 'graph TD\n```\nA-->B'],
    ['fence', 'graph TD\n~~~\nA-->B'],
    ['init_directive', '%%{init: {"theme":"dark"}}%%\ngraph TD\nA-->B'],
    ['html', 'graph TD\nA["<img src=x onerror=alert(1)>"]-->B'],
    ['html', 'graph TD\nA["<script>x</script>"]'],
    ['html', 'graph TD\nclick A "javascript:alert(1)"'],
    ['control_chars', 'graph TD\u0000\nA-->B'],
    ['empty', '   ']
  ])('rejects %s', (reason, src) => {
    expect(guardMermaidSource(src)).toEqual({ ok: false, reason })
  })

  it('rejects sources over 6 KiB', () => {
    const big = `graph TD\n${'A-->B\n'.repeat(Math.ceil(MAX_MERMAID_BYTES / 6) + 5)}`
    expect(guardMermaidSource(big)).toEqual({ ok: false, reason: 'too_large' })
  })
})
