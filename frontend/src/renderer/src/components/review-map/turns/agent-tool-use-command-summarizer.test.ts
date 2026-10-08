import { describe, expect, it } from 'vitest'
import {
  createAgentTurnToolCollector,
  normalizeShellCommand,
  normalizeToolInput
} from './agent-tool-use-command-summarizer'

describe('normalizeShellCommand', () => {
  it.each([
    ['pnpm test --filter x', { name: 'pnpm', sub: 'test', category: 'test' }],
    ['pnpm run lint', { name: 'pnpm', sub: 'lint', category: 'lint' }],
    ['npm install left-pad', { name: 'npm', sub: 'install', category: 'install' }],
    ['go test ./...', { name: 'go', sub: 'test', category: 'test' }],
    ['git commit -m "wip"', { name: 'git', sub: 'commit', category: 'git' }],
    ['/usr/bin/tsc --noEmit', { name: 'tsc', category: 'typecheck' }],
    ['FOO=bar vitest run', { name: 'vitest', category: 'test' }]
  ])('%s', (raw, expected) => {
    expect(normalizeShellCommand(raw)).toEqual(expected)
  })

  it('reports only the first command of a chain, dropping cd', () => {
    expect(normalizeShellCommand('cd /tmp && rm -rf build')).toBeNull()
    expect(normalizeShellCommand('rm -rf /tmp/x && echo done')).toEqual({ name: 'rm', category: 'other' })
  })

  it('does not leak arguments, secrets, URLs or paths as sub-commands', () => {
    const results = [
      normalizeShellCommand('echo hunter2'),
      normalizeShellCommand('curl -H "Authorization: Bearer abc" https://example.com/x'),
      normalizeShellCommand('cat /etc/passwd'),
      normalizeShellCommand('git hunter2')
    ]
    const text = JSON.stringify(results)
    expect(text).not.toMatch(/hunter2|Bearer|example\.com|passwd/)
  })

  it('rejects URLs, empty input and redirect-only input', () => {
    expect(normalizeShellCommand('https://example.com')).toBeNull()
    expect(normalizeShellCommand('   ')).toBeNull()
    expect(normalizeShellCommand('> out.txt')).toBeNull()
  })

  it('never throws on arbitrary input', () => {
    const noise = ['\u0000', '\ud800', '"\'`$()', 'a'.repeat(5000), 'é'.repeat(300), '&&;;||']
    for (let i = 0; i < 200; i++) {
      noise.push(Array.from({ length: 20 }, () => String.fromCharCode(Math.floor(Math.random() * 0xffff))).join(''))
    }
    for (const raw of noise) {
      expect(() => normalizeShellCommand(raw)).not.toThrow()
    }
  })
})

describe('normalizeToolInput', () => {
  it('only treats shell-like tools as commands', () => {
    expect(normalizeToolInput('Edit', 'src/a.ts')).toBeNull()
    expect(normalizeToolInput('Bash', 'pnpm test')).toMatchObject({ name: 'pnpm', category: 'test' })
    expect(normalizeToolInput('Bash', { command: 'git status' })).toMatchObject({ name: 'git', sub: 'status' })
  })
})

describe('createAgentTurnToolCollector', () => {
  const working = (updatedAt: number, toolName: string, toolInput: string) => ({
    state: 'working',
    updatedAt,
    toolName,
    toolInput
  })

  it('counts tools, aggregates repeated commands and dedupes repeated pings', () => {
    const c = createAgentTurnToolCollector()
    c.observe('p', working(1, 'Bash', 'pnpm test'))
    c.observe('p', working(2, 'Bash', 'pnpm test')) // same ping, new updatedAt
    c.observe('p', working(3, 'Edit', 'a.ts'))
    c.observe('p', working(4, 'Bash', 'pnpm test'))
    const summary = c.take('p')!
    expect(summary).toMatchObject({ v: 1, totalToolUses: 3, truncated: false })
    expect(summary.toolCounts).toEqual({ Bash: 2, Edit: 1 })
    expect(summary.commands).toEqual([{ name: 'pnpm', sub: 'test', category: 'test', count: 2 }])
  })

  it('ignores non-working states and clears pane state on take', () => {
    const c = createAgentTurnToolCollector()
    c.observe('p', { state: 'done', updatedAt: 1, toolName: 'Bash', toolInput: 'ls' })
    expect(c.take('p')).toBeNull()
    c.observe('p', working(1, 'Bash', 'ls'))
    expect(c.take('p')).not.toBeNull()
    expect(c.take('p')).toBeNull()
  })

  it('keeps panes independent', () => {
    const c = createAgentTurnToolCollector()
    c.observe('a', working(1, 'Bash', 'git status'))
    c.observe('b', working(1, 'Bash', 'pnpm test'))
    expect(c.take('a')!.commands[0].name).toBe('git')
    expect(c.take('b')!.commands[0].name).toBe('pnpm')
  })

  it('caps commands at 20 and tool keys at 32, flagging truncation', () => {
    const c = createAgentTurnToolCollector()
    for (let i = 0; i < 40; i++) {
      c.observe('p', working(i, `Tool${i}`, `prog${i}`))
      c.observe('p', working(i, 'Bash', `prog${String.fromCharCode(97 + (i % 26))}${i} arg`))
    }
    const summary = c.take('p')!
    expect(summary.commands.length).toBeLessThanOrEqual(20)
    expect(Object.keys(summary.toolCounts).length).toBeLessThanOrEqual(32)
    expect(summary.truncated).toBe(true)
  })
})
