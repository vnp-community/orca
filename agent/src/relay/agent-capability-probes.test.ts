import { describe, expect, it } from 'vitest'
import { buildHostInfo, createVersionRunner, readClaudeLogin } from './agent-capability-probes'

describe('readClaudeLogin', () => {
  const env = {}
  it('reads only the loggedIn boolean', async () => {
    expect(
      await readClaudeLogin(async () => ({ stdout: '{"loggedIn":true,"email":"a@b"}' }), env)
    ).toBe('logged_in')
    expect(await readClaudeLogin(async () => ({ stdout: '{"loggedIn":false}' }), env)).toBe(
      'logged_out'
    )
  })

  it('is unknown for a missing binary, a garbled answer or a thrown probe', async () => {
    expect(await readClaudeLogin(async () => null, env)).toBe('unknown')
    expect(await readClaudeLogin(async () => ({ stdout: 'not json' }), env)).toBe('unknown')
    expect(await readClaudeLogin(async () => ({ stdout: '{"loggedIn":"yes"}' }), env)).toBe(
      'unknown'
    )
    expect(
      await readClaudeLogin(async () => {
        throw new Error('boom')
      }, env)
    ).toBe('unknown')
  })
})

describe('createVersionRunner', () => {
  it('returns null when the binary does not exist', async () => {
    const run = createVersionRunner('linux')
    expect(await run('orca-no-such-binary-xyz', ['--version'], process.env)).toBeNull()
  })

  it('returns stdout for a binary that exists', async () => {
    const run = createVersionRunner(process.platform)
    const res = await run('node', ['--version'], process.env)
    expect(res?.stdout).toMatch(/^v\d+/)
  })
})

describe('buildHostInfo', () => {
  it('uses the injected statfs and reports the platform it was given', async () => {
    const info = await buildHostInfo('/w', 'linux', (async () => ({
      bavail: 10,
      bsize: 1024 * 1024
    })) as never)
    expect(info.platform).toBe('linux')
    expect(info.diskFreeMb).toBe(10)
    expect(info.cpuCount).toBeGreaterThan(0)
  })

  it('omits diskFreeMb when statfs fails', async () => {
    const info = await buildHostInfo('/w', 'linux', (async () => {
      throw new Error('nope')
    }) as never)
    expect(info).not.toHaveProperty('diskFreeMb')
  })
})
