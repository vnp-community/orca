import { describe, it, expect } from 'vitest'
import { buildCodeIntelChildEnv } from './codeintel-child-env'

describe('codeintel-child-env', () => {
  it('filters out secrets and forbidden prefixes', () => {
    const toolEnv = {
      PATH: '/usr/bin',
      HOME: '/home/user',
      LANG: 'en_US.UTF-8',
      ANTHROPIC_API_KEY: 'sk-ant-123',
      GITHUB_TOKEN: 'ghp_123',
      GH_TOKEN: 'gho_123',
      AWS_SECRET_ACCESS_KEY: 'aws123',
      SSH_AUTH_SOCK: '/tmp/ssh-agent.sock',
      ORCA_FOO: 'bar',
      ORCA_CODEINTEL_TOOL_TIMEOUT_MS: '1000',
      NORMAL_VAR: 'value'
    }

    const env = buildCodeIntelChildEnv({ toolEnv })

    expect(env.PATH).toBe('/usr/bin')
    expect(env.HOME).toBe('/home/user')
    expect(env.LANG).toBe('en_US.UTF-8')
    expect(env.NORMAL_VAR).toBe('value')
    expect(env.NO_COLOR).toBe('1')

    expect(env.ANTHROPIC_API_KEY).toBeUndefined()
    expect(env.GITHUB_TOKEN).toBeUndefined()
    expect(env.GH_TOKEN).toBeUndefined()
    expect(env.AWS_SECRET_ACCESS_KEY).toBeUndefined()
    expect(env.SSH_AUTH_SOCK).toBeUndefined()
    expect(env.ORCA_FOO).toBeUndefined()
    expect(env.ORCA_CODEINTEL_TOOL_TIMEOUT_MS).toBeUndefined()

    // Original toolEnv should not be modified
    expect(toolEnv.ANTHROPIC_API_KEY).toBe('sk-ant-123')
  })
})
