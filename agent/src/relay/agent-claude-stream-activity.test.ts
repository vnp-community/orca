import { describe, expect, it } from 'vitest'
import { BoundedOutputBuffer } from './agent-bounded-output-buffer'
import {
  ACTIVITY_GLYPH,
  ClaudeStreamActivityParser,
  type ClaudeStreamActivityOptions
} from './agent-claude-stream-activity'
import { resolveFinalStdout } from './agent-claude-final-stdout'

const ID_READ = 'toolu_read'
const ID_BASH = 'toolu_bash'

function line(event: unknown): string {
  return `${JSON.stringify(event)}\n`
}

const init = {
  type: 'system',
  subtype: 'init',
  model: 'claude-test',
  session_id: 's1',
  tools: ['Read']
}
const assistantText = (text: string): unknown => ({
  type: 'assistant',
  message: { content: [{ type: 'text', text }] }
})
const toolUse = (id: string, name: string, input: unknown): unknown => ({
  type: 'assistant',
  message: { content: [{ type: 'tool_use', id, name, input }] }
})
const toolResult = (id: string, content: unknown, isError = false): unknown => ({
  type: 'user',
  message: {
    content: [
      { type: 'tool_result', tool_use_id: id, content, ...(isError ? { is_error: true } : {}) }
    ]
  }
})
const result = (text: string, extra: Record<string, unknown> = {}): unknown => ({
  type: 'result',
  subtype: 'success',
  is_error: false,
  num_turns: 2,
  duration_ms: 5732,
  result: text,
  ...extra
})

function makeParser(overrides: Partial<ClaudeStreamActivityOptions> = {}): {
  parser: ClaudeStreamActivityParser
  emitted: string[]
  lines: () => string[]
} {
  const emitted: string[] = []
  const parser = new ClaudeStreamActivityParser({
    emit: (t) => emitted.push(t),
    redact: (t) => t,
    cwdPrefixes: ['/work/repo'],
    ...overrides
  })
  return { parser, emitted, lines: () => emitted.join('').split('\n').filter(Boolean) }
}

describe('ClaudeStreamActivityParser tool calls and usage', () => {
  const kg = 'mcp__tdd-agent__kg_get_element'

  it('counts calls per tool name, never their input or output', () => {
    const { parser } = makeParser()
    parser.push(
      Buffer.from(
        line(init) +
          line(toolUse('t1', kg, { id: 'FR-SYS-07' })) +
          line(toolResult('t1', 'SECRET DOCUMENT TEXT')) +
          line(toolUse('t2', kg, { id: 'ADR-12' })) +
          line(toolUse('t3', 'Read', { file_path: '/work/repo/a.go' })) +
          line(result('done'))
      )
    )
    const out = parser.finish()
    expect(out.toolCalls).toEqual({ [kg]: 2, Read: 1 })
    expect(JSON.stringify(out.toolCalls)).not.toContain('FR-SYS-07')
    expect(JSON.stringify(out.toolCalls)).not.toContain('SECRET')
  })

  it('reads the usage figures claude reports and leaves out the ones it does not', () => {
    const { parser } = makeParser()
    parser.push(
      Buffer.from(
        line(init) +
          line(
            result('ok', {
              total_cost_usd: 0.8123,
              usage: { input_tokens: 1200, output_tokens: 340, cache_read_input_tokens: 9000 }
            })
          )
      )
    )
    expect(parser.finish().usage).toEqual({
      turns: 2,
      durationMs: 5732,
      costUsd: 0.8123,
      inputTokens: 1200,
      outputTokens: 340,
      cacheReadTokens: 9000
    })
  })

  it('reports no cost rather than guessing one', () => {
    const { parser } = makeParser()
    parser.push(Buffer.from(line(init) + line(result('ok'))))
    const usage = parser.finish().usage
    expect(usage).toEqual({ turns: 2, durationMs: 5732 })
    expect(usage).not.toHaveProperty('costUsd')
  })

  it('ignores nonsense figures and gives no usage when the result event has none', () => {
    const { parser } = makeParser()
    parser.push(
      Buffer.from(
        line(init) +
          line({
            type: 'result',
            subtype: 'success',
            is_error: false,
            result: 'x',
            num_turns: -3,
            total_cost_usd: 'free',
            usage: { input_tokens: Number.NaN }
          })
      )
    )
    expect(parser.finish().usage).toBeNull()
  })

  it('stops tracking new tool names after a ceiling and counts the rest as other', () => {
    const { parser } = makeParser()
    let input = line(init)
    for (let i = 0; i < 70; i++) {
      input += line(toolUse(`id${i}`, `tool_${i}`, {}))
    }
    input += line(toolUse('last', 'tool_0', {}))
    parser.push(Buffer.from(input + line(result('ok'))))
    const calls = parser.finish().toolCalls
    expect(Object.keys(calls)).toHaveLength(65)
    expect(calls['(other)']).toBe(6)
    expect(calls['tool_0']).toBe(2)
  })
})

describe('ClaudeStreamActivityParser', () => {
  it('turns a full tool session into readable activity lines', () => {
    const { parser, lines } = makeParser()
    parser.push(
      Buffer.from(
        line(init) +
          line(assistantText("I'll read the file.")) +
          line(toolUse(ID_READ, 'Read', { file_path: '/work/repo/src/calc.py' })) +
          line(toolResult(ID_READ, '1\timport os\n2\tx = 1\n3\t')) +
          line(
            toolUse(ID_BASH, 'Bash', { command: 'python3 -m unittest', description: 'run tests' })
          ) +
          line(toolResult(ID_BASH, 'Ran 3 tests in 0.001s\n\nOK')) +
          line(result('All good.'))
      )
    )
    const outcome = parser.finish()
    expect(lines()).toEqual([
      `${ACTIVITY_GLYPH.info} Session started (claude-test)`,
      `${ACTIVITY_GLYPH.text} I'll read the file.`,
      `${ACTIVITY_GLYPH.tool} Read src/calc.py`,
      `${ACTIVITY_GLYPH.result} ok · 3 lines`,
      `${ACTIVITY_GLYPH.tool} Bash: python3 -m unittest`,
      `${ACTIVITY_GLYPH.result} ok · OK`,
      `${ACTIVITY_GLYPH.end} Done · 2 turns · 5.7s`
    ])
    expect(outcome).toMatchObject({ structured: true, resultText: 'All good.', isError: false })
  })

  it('never shows file content, edit bodies or written content', () => {
    const { parser, emitted } = makeParser()
    parser.push(
      Buffer.from(
        line(toolUse('w', 'Write', { file_path: '/work/repo/a.py', content: 'TOP-SECRET-BODY' })) +
          line(
            toolUse('e', 'Edit', {
              file_path: '/work/repo/a.py',
              old_string: 'OLD-BODY',
              new_string: 'NEW-BODY'
            })
          ) +
          line(toolResult('w', 'FILE-CONTENT-LINE-1\nFILE-CONTENT-LINE-2')) +
          line(toolUse(ID_READ, 'Read', { file_path: '/work/repo/b.py' })) +
          line(toolResult(ID_READ, 'FILE-CONTENT-LINE-1\nFILE-CONTENT-LINE-2'))
      )
    )
    parser.finish()
    const all = emitted.join('')
    for (const body of ['TOP-SECRET-BODY', 'OLD-BODY', 'NEW-BODY', 'FILE-CONTENT-LINE']) {
      expect(all).not.toContain(body)
    }
    expect(all).toContain(`${ACTIVITY_GLYPH.tool} Edit a.py`)
    expect(all).toContain(`${ACTIVITY_GLYPH.result} ok · 2 lines`)
  })

  it('summarises Glob/Grep, WebFetch (no query string) and unknown tools', () => {
    const { parser, lines } = makeParser()
    parser.push(
      Buffer.from(
        line(toolUse('g', 'Glob', { pattern: '**/*.py', path: '/work/repo/src' })) +
          line(toolResult('g', 'a.py\nb.py')) +
          line(toolUse('r', 'Grep', { pattern: 'def add', path: '/work/repo' })) +
          line(toolUse('f', 'WebFetch', { url: 'https://example.com/a?token=abc#frag' })) +
          line(toolUse('m', 'mcp__docs__search', { query: 'calc' })) +
          line(toolUse('n', 'Mystery', { weird: 1 }))
      )
    )
    parser.finish()
    expect(lines()).toEqual([
      `${ACTIVITY_GLYPH.tool} Glob **/*.py in src`,
      `${ACTIVITY_GLYPH.result} ok · 2 results`,
      `${ACTIVITY_GLYPH.tool} Grep: def add in .`,
      `${ACTIVITY_GLYPH.tool} WebFetch https://example.com/a`,
      `${ACTIVITY_GLYPH.tool} mcp__docs__search: calc`,
      `${ACTIVITY_GLYPH.tool} Mystery`
    ])
  })

  it('shows tool errors briefly and keeps Bash exit code with the last output line', () => {
    const { parser, lines } = makeParser()
    parser.push(
      Buffer.from(
        line(toolUse(ID_BASH, 'Bash', { command: 'python3 -m unittest' })) +
          line(toolResult(ID_BASH, 'Exit code 1\nF\nFAILED (failures=1)', true)) +
          line(toolUse('w', 'Write', { file_path: '/x' })) +
          line(
            toolResult(
              'w',
              '<tool_use_error>Error: No such tool available: Write</tool_use_error>',
              true
            )
          )
      )
    )
    parser.finish()
    expect(lines()).toEqual([
      `${ACTIVITY_GLYPH.tool} Bash: python3 -m unittest`,
      `${ACTIVITY_GLYPH.result} error: Exit code 1 · FAILED (failures=1)`,
      `${ACTIVITY_GLYPH.tool} Write /x`,
      `${ACTIVITY_GLYPH.result} error: Error: No such tool available: Write`
    ])
  })

  it('reports a failed run and keeps the error text as the result', () => {
    const { parser, lines } = makeParser()
    parser.push(Buffer.from(line(result('Not logged in · Please run /login', { is_error: true }))))
    const outcome = parser.finish()
    expect(lines()).toEqual([`${ACTIVITY_GLYPH.end} Failed: Not logged in · Please run /login`])
    expect(outcome).toMatchObject({
      isError: true,
      resultText: 'Not logged in · Please run /login'
    })
  })

  it('notes a result event that has no answer text', () => {
    const { parser } = makeParser()
    parser.push(Buffer.from(line({ type: 'result', subtype: 'error_max_turns', is_error: true })))
    expect(parser.finish()).toMatchObject({
      resultText: null,
      errorNote: 'claude ended without an answer (error_max_turns)'
    })
  })

  it('ignores thinking blocks, hook and rate-limit events and unknown event types of a known stream', () => {
    const { parser, lines } = makeParser()
    parser.push(
      Buffer.from(
        line({ type: 'system', subtype: 'hook_started', hook_name: 'x', session_id: 's' }) +
          line({
            type: 'assistant',
            message: { content: [{ type: 'thinking', thinking: '', signature: 'SIGNATURE-XYZ' }] }
          }) +
          line({ type: 'rate_limit_event', rate_limit_info: {} }) +
          line({ type: 'brand_new_event', session_id: 's', payload: 1 })
      )
    )
    parser.finish()
    expect(lines()).toEqual([])
  })

  it('shows lines that are not JSON as plain text and does not call the stream structured', () => {
    const { parser, lines } = makeParser()
    parser.push(Buffer.from('plain answer line one\nsecond line\n{"kind":"diagnosis"}\n'))
    const outcome = parser.finish()
    expect(lines()).toEqual([
      `${ACTIVITY_GLYPH.text} plain answer line one`,
      `${ACTIVITY_GLYPH.text} second line`,
      `${ACTIVITY_GLYPH.text} {"kind":"diagnosis"}`
    ])
    expect(outcome.structured).toBe(false)
  })

  it('handles a line split across chunks, including inside a multi-byte UTF-8 character', () => {
    const { parser, lines } = makeParser()
    const bytes = Buffer.from(line(assistantText('Đã đọc xong 日本語 ✓')) + line(result('Xong')))
    const cut = bytes.indexOf(Buffer.from('Đ')) + 1 // between the two bytes of "Đ"
    parser.push(bytes.subarray(0, cut))
    parser.push(bytes.subarray(cut, cut + 7))
    parser.push(bytes.subarray(cut + 7))
    const outcome = parser.finish()
    expect(lines()[0]).toBe(`${ACTIVITY_GLYPH.text} Đã đọc xong 日本語 ✓`)
    expect(outcome.resultText).toBe('Xong')
  })

  it('processes a final line that has no trailing newline and tolerates CRLF', () => {
    const { parser } = makeParser()
    parser.push(
      Buffer.from(`${JSON.stringify(assistantText('hi'))}\r\n${JSON.stringify(result('bye'))}`)
    )
    expect(parser.finish().resultText).toBe('bye')
  })

  it('skips a line longer than the limit and still reads the events after it', () => {
    const { parser, lines } = makeParser({ maxLineChars: 1000 })
    const huge = 'x'.repeat(5000)
    parser.push(Buffer.from(`${JSON.stringify(toolResult('a', huge))}`.slice(0, 2500)))
    parser.push(Buffer.from(`${'y'.repeat(2500)}\n${line(result('after the big line'))}`))
    const outcome = parser.finish()
    expect(lines()).toContain(`${ACTIVITY_GLYPH.info} skipped an oversized agent event`)
    expect(outcome.resultText).toBe('after the big line')
  })

  it('truncates long text blocks, long tool arguments and many-line answers', () => {
    const { parser, lines } = makeParser()
    const many = Array.from({ length: 10 }, (_, i) => `line ${i}`).join('\n')
    parser.push(
      Buffer.from(
        line(assistantText(many)) +
          line(assistantText('w'.repeat(1000))) +
          line(toolUse('b', 'Bash', { command: `echo ${'z'.repeat(1000)}` }))
      )
    )
    parser.finish()
    const out = lines()
    expect(out.filter((l) => l.startsWith(`${ACTIVITY_GLYPH.text} line `))).toHaveLength(6)
    expect(out).toContain(`${ACTIVITY_GLYPH.text} …`)
    expect(out.every((l) => l.length <= 260)).toBe(true)
    expect(out.some((l) => l.endsWith('…'))).toBe(true)
  })

  it('redacts before shortening, so a cut secret cannot survive as a fragment', () => {
    const secret = 'sk-ant-abcdefghijklmnopqrstuvwxyz0123456789'
    const { parser, emitted } = makeParser({ redact: (t) => t.split(secret).join('***') })
    parser.push(
      Buffer.from(
        line(toolUse('b', 'Bash', { command: `curl -H "x: ${'p'.repeat(130)} ${secret}"` })) +
          line(assistantText(`key is ${secret}`)) +
          line(toolResult('b', `leaked ${secret}`, true))
      )
    )
    parser.finish()
    const all = emitted.join('')
    expect(all).not.toContain('sk-ant')
    expect(all).not.toContain('abcdefghij')
    expect(all).toContain('***')
  })

  it('stops emitting at the activity limit but still reports the final status', () => {
    const { parser, lines } = makeParser({ maxActivityBytes: 300 })
    const events = Array.from({ length: 50 }, (_, i) =>
      line(toolUse(`t${i}`, 'Read', { file_path: `/work/repo/file-${i}.py` }))
    )
    parser.push(Buffer.from(events.join('') + line(result('done'))))
    parser.finish()
    const out = lines()
    expect(out.filter((l) => l.includes('activity limit reached'))).toHaveLength(1)
    expect(out.at(-1)).toMatch(/^■ Done/)
    expect(out.length).toBeLessThan(25)
  })

  it('emits one batch per pushed chunk and nothing after finish()', () => {
    const { parser, emitted } = makeParser()
    parser.push(Buffer.from(line(assistantText('a')) + line(assistantText('b'))))
    expect(emitted).toEqual([`${ACTIVITY_GLYPH.text} a\n${ACTIVITY_GLYPH.text} b\n`])
    parser.finish()
    parser.push(Buffer.from(line(assistantText('late'))))
    expect(emitted).toHaveLength(1)
  })

  it('shortens absolute paths with a Windows-style prefix', () => {
    const { parser, lines } = makeParser({ cwdPrefixes: ['C:\\work\\repo'] })
    parser.push(
      Buffer.from(line(toolUse('r', 'Read', { file_path: 'C:\\work\\repo\\src\\calc.py' })))
    )
    parser.finish()
    expect(lines()).toEqual([`${ACTIVITY_GLYPH.tool} Read src/calc.py`])
  })
})

describe('resolveFinalStdout', () => {
  const rawOf = (text: string): BoundedOutputBuffer => {
    const buf = new BoundedOutputBuffer(1024)
    buf.append(Buffer.from(text))
    return buf
  }
  const base = {
    structured: true,
    resultText: null,
    isError: false,
    assistantText: '',
    errorNote: null,
    toolCalls: {},
    usage: null
  }

  it('returns the result text with the trailing newline text mode used to print', () => {
    expect(resolveFinalStdout({ ...base, resultText: 'answer' }, rawOf('{json}'), 1024)).toEqual({
      stdout: 'answer\n',
      truncated: false
    })
    expect(
      resolveFinalStdout({ ...base, resultText: 'answer\n' }, rawOf('{json}'), 1024).stdout
    ).toBe('answer\n')
  })

  it('returns empty stdout for an empty result and falls back to the assistant text without a result event', () => {
    expect(resolveFinalStdout({ ...base, resultText: '' }, rawOf('x'), 1024).stdout).toBe('')
    expect(
      resolveFinalStdout({ ...base, assistantText: 'partial\n' }, rawOf('x'), 1024).stdout
    ).toBe('partial\n')
  })

  it('returns the raw bytes when the stream was not stream-json', () => {
    expect(
      resolveFinalStdout({ ...base, structured: false }, rawOf('plain text\n'), 1024).stdout
    ).toBe('plain text\n')
  })

  it('keeps the tail of an answer that exceeds the budget and flags truncation', () => {
    const out = resolveFinalStdout({ ...base, resultText: `${'a'.repeat(50)}TAIL` }, rawOf(''), 10)
    expect(out.truncated).toBe(true)
    expect(out.stdout.endsWith('TAIL\n')).toBe(true)
    expect(out.stdout.length).toBeLessThanOrEqual(10)
  })
})
