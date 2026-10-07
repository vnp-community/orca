import { describe, it, expect } from 'vitest'
import { buildQualityChildEnv, SECRET_ENV_NAME_PATTERN } from './quality-child-env'

describe('quality-child-env', () => {
  it('filters out forbidden variables and copies allowed ones', () => {
    const sourceEnv = {
      ANTHROPIC_API_KEY: 'sk-123',
      GITHUB_TOKEN: 'ghp_123',
      GH_TOKEN: 'ghp_456',
      AGENT_TOKEN: 'ag_789',
      FOO_SECRET: 'secret',
      ORCA_URL: 'http://localhost',
      AWS_ACCESS_KEY_ID: 'AKIA...',
      SSH_AUTH_SOCK: '/tmp/ssh-agent',
      XDG_SESSION_ID: '1234',
      HOME: '/home/user',
      LANG: 'en_US.UTF-8',
      FOO: 'bar'
    }

    const { env, rejectedExtra } = buildQualityChildEnv({
      sourceEnv,
      qualityToolPath: '/bin',
      tmpDir: '/tmp',
      profileEnv: { set: {}, allowExtra: ['MY_API_KEY', 'FOO'] },
      limits: { gomaxprocs: 4 }
    })

    expect(env['HOME']).toBe('/home/user')
    expect(env['LANG']).toBe('en_US.UTF-8')
    expect(env['FOO']).toBe('bar')
    expect(env['PATH']).toBe('/bin')
    expect(env['TMPDIR']).toBe('/tmp')
    expect(env['GOMAXPROCS']).toBe('4')
    expect(env['CI']).toBe('1')

    // Check denied
    expect(env['ANTHROPIC_API_KEY']).toBeUndefined()
    expect(env['GITHUB_TOKEN']).toBeUndefined()
    expect(env['GH_TOKEN']).toBeUndefined()
    expect(env['AGENT_TOKEN']).toBeUndefined()
    expect(env['FOO_SECRET']).toBeUndefined()
    expect(env['ORCA_URL']).toBeUndefined()
    expect(env['AWS_ACCESS_KEY_ID']).toBeUndefined()
    expect(env['SSH_AUTH_SOCK']).toBeUndefined()
    expect(env['XDG_SESSION_ID']).toBeUndefined()

    expect(rejectedExtra).toContain('MY_API_KEY')
  })

  it('sets GOCACHE only if set in sourceEnv', () => {
    let res = buildQualityChildEnv({
      sourceEnv: {},
      qualityToolPath: '/bin',
      tmpDir: '/tmp',
      profileEnv: { set: {}, allowExtra: [] },
      limits: { gomaxprocs: 1 }
    })
    expect(res.env['GOCACHE']).toBeUndefined()

    res = buildQualityChildEnv({
      sourceEnv: { GOCACHE: '/cache' },
      qualityToolPath: '/bin',
      tmpDir: '/tmp',
      profileEnv: { set: {}, allowExtra: [] },
      limits: { gomaxprocs: 1 }
    })
    expect(res.env['GOCACHE']).toBe('/cache')
  })

  it('rejects NODE_OPTIONS with --require', () => {
    const { env, rejectedExtra } = buildQualityChildEnv({
      sourceEnv: {},
      qualityToolPath: '/bin',
      tmpDir: '/tmp',
      profileEnv: { set: {}, allowExtra: [] },
      limits: { gomaxprocs: 1 },
      nodeOptions: '--require x'
    })
    expect(env['NODE_OPTIONS']).toBeUndefined()
    expect(rejectedExtra).toContain('NODE_OPTIONS')
  })

  it('rejects NODE_OPTIONS in profileEnv.set with loader', () => {
    const { env, rejectedExtra } = buildQualityChildEnv({
      sourceEnv: {},
      qualityToolPath: '/bin',
      tmpDir: '/tmp',
      profileEnv: { set: { NODE_OPTIONS: '--loader ts-node/esm' }, allowExtra: [] },
      limits: { gomaxprocs: 1 }
    })
    expect(env['NODE_OPTIONS']).toBeUndefined()
    expect(rejectedExtra).toContain('NODE_OPTIONS')
  })

  it('combines nodeOptions and nodeOldSpaceMb', () => {
    const { env } = buildQualityChildEnv({
      sourceEnv: {},
      qualityToolPath: '/bin',
      tmpDir: '/tmp',
      profileEnv: { set: {}, allowExtra: [] },
      limits: { gomaxprocs: 1, nodeOldSpaceMb: 2048 },
      nodeOptions: '--no-warnings'
    })
    expect(env['NODE_OPTIONS']).toBe('--no-warnings --max-old-space-size=2048')
  })
})
