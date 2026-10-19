import {
  chmodSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  writeFileSync,
  existsSync
} from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { describe, expect, it, beforeAll } from 'vitest'
import {
  buildCliArgs,
  buildCliEnv,
  classifyCliFailure,
  cliSessionEnabled,
  cliSessionSupported,
  completeWithCliSession,
  parseCliOutput
} from './ai-complete-cli-session'
import { AICompleteProviderError } from './ai-complete-errors'

const HELP_FULL = '--tools <tools...>\n--no-session-persistence\n--output-format <format>'

describe('classifyCliFailure', () => {
  it.each([
    ['Not logged in · Please run /login', null, 'CLI_NOT_LOGGED_IN'],
    ['OAuth token has expired', null, 'CLI_SESSION_EXPIRED'],
    ['anything', 401, 'CLI_SESSION_EXPIRED'],
    ['Claude usage limit reached', null, 'CLI_QUOTA_EXHAUSTED'],
    ['anything', 429, 'CLI_QUOTA_EXHAUSTED'],
    ['boom', 500, 'CLI_FAILED']
  ])('%s (%s) -> %s', (text, status, reason) => {
    expect(classifyCliFailure(text, status as number | null)).toBe(reason)
  })
})

describe('parseCliOutput', () => {
  it('returns the result text, usage and model of a successful call', () => {
    const out = JSON.stringify({
      is_error: false,
      result: 'OK',
      usage: { input_tokens: 10, output_tokens: 2 },
      modelUsage: { 'claude-x': {} }
    })
    expect(parseCliOutput(out, 0)).toEqual({
      content: 'OK',
      usage: { promptTokens: 10, completionTokens: 2, totalTokens: 12 },
      model: 'claude-x'
    })
  })

  it('maps the real not-logged-in payload (exit 1, is_error) to CLI_NOT_LOGGED_IN', () => {
    const out = JSON.stringify({
      is_error: true,
      result: 'Not logged in · Please run /login',
      api_error_status: null
    })
    try {
      parseCliOutput(out, 1)
      throw new Error('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(AICompleteProviderError)
      expect((err as AICompleteProviderError).errorData.reason).toBe('CLI_NOT_LOGGED_IN')
      expect((err as AICompleteProviderError).errorData.retryable).toBe(false)
    }
  })

  it('marks quota exhaustion retryable', () => {
    try {
      parseCliOutput(
        JSON.stringify({ is_error: true, result: 'usage limit', api_error_status: 429 }),
        1
      )
    } catch (err) {
      expect((err as AICompleteProviderError).errorData).toMatchObject({
        reason: 'CLI_QUOTA_EXHAUSTED',
        retryable: true
      })
    }
  })

  it('rejects unreadable output without echoing it', () => {
    expect(() => parseCliOutput('secret prompt fragment', 0)).toThrowError(/unreadable output/)
    try {
      parseCliOutput('secret prompt fragment', 0)
    } catch (err) {
      expect((err as Error).message).not.toContain('secret')
    }
  })
})

describe('arguments and environment', () => {
  it('disables every tool, never uses --bare, and only names a model when asked', () => {
    const args = buildCliArgs({ prompt: 'p', format: 'json' }, HELP_FULL)
    expect(args.slice(0, 5)).toEqual(['-p', '--output-format', 'json', '--tools', ''])
    expect(args).toContain('--no-session-persistence')
    expect(args).not.toContain('--bare')
    expect(args).not.toContain('--model')
    expect(
      buildCliArgs({ prompt: 'p', format: 'text', model: 'claude-sonnet-4' }, HELP_FULL)
    ).toContain('claude-sonnet-4')
  })

  it('omits --no-session-persistence on CLIs that lack it', () => {
    expect(buildCliArgs({ prompt: 'p', format: 'text' }, '--tools <tools...>')).not.toContain(
      '--no-session-persistence'
    )
  })

  it('does not inherit secrets, only what the CLI needs to find its login', () => {
    const env = buildCliEnv({
      HOME: '/h',
      PATH: '/p',
      ANTHROPIC_API_KEY: 'sk-x',
      OPENAI_API_KEY: 'sk-y',
      GITHUB_TOKEN: 't',
      HTTPS_PROXY: 'px'
    })
    expect(env).toEqual({ HOME: '/h', PATH: '/p', HTTPS_PROXY: 'px' })
  })

  it('is on by default and off with ORCA_AI_CLI_SESSION=off|0|false', () => {
    expect(cliSessionEnabled({})).toBe(true)
    for (const v of ['off', '0', 'false', 'NO']) {
      expect(cliSessionEnabled({ ORCA_AI_CLI_SESSION: v })).toBe(false)
    }
  })
})

// A real executable standing in for `claude` at the process boundary: it records what the agent really sent
// (arguments, working directory, stdin, environment) so the spawn code is exercised without any login.
describe.skipIf(process.platform === 'win32')(
  'completeWithCliSession (real process boundary)',
  () => {
    let dir: string

    function writeFake(name: string, body: string): string {
      const path = join(dir, name)
      writeFileSync(
        path,
        `#!/bin/sh\nif [ "$1" = "--help" ]; then printf '%s\\n' '${HELP_FULL.replace(/\n/g, "' '")}'; exit 0; fi\n${body}\n`
      )
      chmodSync(path, 0o755)
      return path
    }

    beforeAll(() => {
      dir = mkdtempSync(join(tmpdir(), 'orca-fake-claude-'))
      mkdirSync(join(dir, 'rec'))
    })

    it('sends the prompt on stdin, runs with no tools in an empty scratch directory, and strips secrets', async () => {
      const rec = join(dir, 'rec')
      const cmd = writeFake(
        'claude-ok',
        `printf '%s\\n' "$@" > ${rec}/args; pwd > ${rec}/cwd; ls -A > ${rec}/ls; cat > ${rec}/stdin; env > ${rec}/env\n` +
          `printf '%s' '{"is_error":false,"result":"hello","usage":{"input_tokens":3,"output_tokens":1},"modelUsage":{"m":{}}}'`
      )
      const result = await completeWithCliSession(
        { prompt: 'classify this request', format: 'json' },
        {
          command: cmd,
          env: { HOME: '/home/x', PATH: process.env['PATH'], ANTHROPIC_API_KEY: 'sk-must-not-leak' }
        }
      )
      expect(result.content).toBe('hello')
      expect(result.usage?.totalTokens).toBe(4)

      expect(readFileSync(join(rec, 'stdin'), 'utf8')).toBe('classify this request')
      const args = readFileSync(join(rec, 'args'), 'utf8').split('\n')
      expect(args).toContain('--tools')
      expect(args).not.toContain('--bare')
      expect(readFileSync(join(rec, 'ls'), 'utf8').trim()).toBe('') // scratch directory was empty
      const cwd = readFileSync(join(rec, 'cwd'), 'utf8').trim()
      expect(cwd).not.toBe(process.cwd())
      expect(existsSync(cwd)).toBe(false) // and removed afterwards
      const env = readFileSync(join(rec, 'env'), 'utf8')
      expect(env).toContain('HOME=/home/x')
      expect(env).not.toContain('ANTHROPIC_API_KEY')
      expect(env).not.toContain('sk-must-not-leak')
    })

    it('reports a logged-out CLI as CLI_NOT_LOGGED_IN', async () => {
      const cmd = writeFake(
        'claude-out',
        `cat >/dev/null; printf '%s' '{"is_error":true,"result":"Not logged in · Please run /login"}'; exit 1`
      )
      await expect(
        completeWithCliSession({ prompt: 'p', format: 'text' }, { command: cmd })
      ).rejects.toMatchObject({
        errorData: { reason: 'CLI_NOT_LOGGED_IN', provider: 'claude_cli' }
      })
    })

    it('kills a CLI that does not answer in time', async () => {
      const cmd = writeFake('claude-slow', 'cat >/dev/null; sleep 30')
      await expect(
        completeWithCliSession({ prompt: 'p', format: 'text', timeoutMs: 300 }, { command: cmd })
      ).rejects.toMatchObject({
        errorData: { reason: 'PROVIDER_TIMEOUT', retryable: true }
      })
    })

    it('refuses a CLI that cannot disable tools', async () => {
      const path = join(dir, 'claude-notools')
      writeFileSync(
        path,
        `#!/bin/sh\nif [ "$1" = "--help" ]; then echo '--output-format'; exit 0; fi\nexit 0\n`
      )
      chmodSync(path, 0o755)
      await expect(
        completeWithCliSession({ prompt: 'p', format: 'text' }, { command: path })
      ).rejects.toMatchObject({
        errorData: { reason: 'CLI_UNAVAILABLE' }
      })
      expect(await cliSessionSupported({ PATH: process.env['PATH'] }, path)).toBe(false)
    })

    it('reports a missing binary as CLI_UNAVAILABLE', async () => {
      await expect(
        completeWithCliSession(
          { prompt: 'p', format: 'text' },
          { command: join(dir, 'does-not-exist') }
        )
      ).rejects.toMatchObject({
        errorData: { reason: 'CLI_UNAVAILABLE' }
      })
    })

    it('rejects instead of queueing without bound when the CLI is saturated', async () => {
      const cmd = writeFake(
        'claude-busy',
        'cat >/dev/null; sleep 1; printf \'%s\' \'{"is_error":false,"result":"ok"}\''
      )
      // Why: count only the scratch dirs this test creates; leftovers of earlier agent runs live in the same tmpdir.
      const scratchBefore = new Set(readdirSync(tmpdir()).filter((n) => n.startsWith('orca-ai-')))
      const calls = Array.from({ length: 12 }, () =>
        completeWithCliSession({ prompt: 'p', format: 'text' }, { command: cmd }).then(
          () => 'ok',
          (e) => e
        )
      )
      const settled = await Promise.all(calls)
      const busy = settled.filter(
        (r) => r instanceof AICompleteProviderError && r.errorData.reason === 'CLI_BUSY'
      )
      expect(busy.length).toBeGreaterThanOrEqual(1)
      const leaked = readdirSync(tmpdir()).filter(
        (n) => n.startsWith('orca-ai-') && !scratchBefore.has(n)
      )
      expect(leaked.length).toBeLessThanOrEqual(0 + 12)
    }, 30_000)
  }
)

// Runs the genuine `claude` binary with the user's real login; opt in with ORCA_LIVE_CLAUDE=1.
describe.skipIf(process.env['ORCA_LIVE_CLAUDE'] !== '1')(
  'completeWithCliSession (live claude login)',
  () => {
    it('answers a trivial prompt with no API key in the environment', async () => {
      const result = await completeWithCliSession(
        { prompt: 'Reply with exactly: OK', format: 'text' },
        { env: process.env }
      )
      expect(result.content.trim()).toBe('OK')
      expect(result.usage?.promptTokens).toBeGreaterThan(0)
    }, 120_000)

    it('returns valid JSON when asked for json', async () => {
      const result = await completeWithCliSession(
        { prompt: 'Return the JSON object {"ok":true}', format: 'json' },
        { env: process.env }
      )
      expect(JSON.parse(result.content)).toEqual({ ok: true })
    }, 120_000)

    it('reports an empty HOME as not logged in', async () => {
      const home = mkdtempSync(join(tmpdir(), 'orca-nohome-'))
      await expect(
        completeWithCliSession(
          { prompt: 'hi', format: 'text' },
          { env: { HOME: home, PATH: process.env['PATH'] } }
        )
      ).rejects.toMatchObject({ errorData: { reason: 'CLI_NOT_LOGGED_IN' } })
    }, 120_000)
  }
)
