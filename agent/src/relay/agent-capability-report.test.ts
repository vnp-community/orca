import { describe, it, expect } from 'vitest'
import {
  buildCapabilityReport,
  validateCapabilityParams,
  handleAgentCapabilities
} from './agent-capability-report'
import type { AgentConfig } from './agent-config'

const dummyConfig: AgentConfig = {
  port: 8080,
  token: 'tok',
  workDir: '/tmp',
  toolPath: '/custom/bin',
  accountId: 'acct-1'
}

const dummyLogger = {
  info: () => {},
  warn: () => {},
  error: () => {},
  debug: () => {}
} as any

describe('agent-capability-report', () => {
  describe('validateCapabilityParams', () => {
    it('validates default params when empty object', () => {
      const res = validateCapabilityParams({})
      expect(res.ok).toBe(true)
      if (res.ok) {
        expect(res.value.refresh).toBe(false)
        expect(res.value.tools.length).toBeGreaterThan(0)
        expect(res.value.unknownTools).toEqual([])
        expect(res.value.rejectedEnvNames).toEqual([])
      }
    })

    it('rejects non-array tools with INVALID_CAPABILITY_PARAMS', () => {
      const res = validateCapabilityParams({ tools: 'invalid' as any })
      expect(res.ok).toBe(false)
      if (!res.ok) {
        expect(res.code).toBe('INVALID_CAPABILITY_PARAMS')
      }
    })

    it('rejects more than 64 envNames with TOO_MANY_ENV_NAMES', () => {
      const names = Array.from({ length: 65 }, (_, i) => `ENV_${i}`)
      const res = validateCapabilityParams({ envNames: names })
      expect(res.ok).toBe(false)
      if (!res.ok) {
        expect(res.code).toBe('TOO_MANY_ENV_NAMES')
      }
    })

    it('classifies unknown tools and invalid envNames', () => {
      const res = validateCapabilityParams({
        tools: ['node', 'rm -rf /', 'curl'],
        envNames: ['VALID_ENV', 'lower_case', 'A'.repeat(65)]
      })
      expect(res.ok).toBe(true)
      if (res.ok) {
        expect(res.value.tools).toEqual(['node'])
        expect(res.value.unknownTools).toEqual(['rm -rf /', 'curl'])
        expect(res.value.envNames).toEqual(['VALID_ENV'])
        expect(res.value.rejectedEnvNames).toEqual(['lower_case', 'A'.repeat(65)])
      }
    })
  })

  describe('buildCapabilityReport with mock deps', () => {
    it('runs only allowlisted tools and uses --version / go version', async () => {
      const runCalls: Array<{ id: string; args: readonly string[] }> = []
      const mockRun = async (id: string, args: readonly string[]) => {
        runCalls.push({ id, args })
        if (id === 'go') {
          return { stdout: 'go version go1.22.1 darwin/arm64\n' }
        }
        if (id === 'node') {
          return { stdout: 'v22.0.0\n' }
        }
        return { stdout: '1.0.0\n' }
      }

      const validated = validateCapabilityParams({ tools: ['go', 'node', 'malicious_cmd'] })
      expect(validated.ok).toBe(true)
      if (!validated.ok) return

      const report = await buildCapabilityReport(validated.value, dummyConfig, {
        run: mockRun
      })

      // Verify malicious_cmd was not executed
      expect(runCalls.some((c) => c.id === 'malicious_cmd')).toBe(false)
      expect(report.unknownTools).toContain('malicious_cmd')

      // Verify go used 'version' argument
      const goCall = runCalls.find((c) => c.id === 'go')
      expect(goCall?.args).toEqual(['version'])

      // Verify node used '--version' argument
      const nodeCall = runCalls.find((c) => c.id === 'node')
      expect(nodeCall?.args).toEqual(['--version'])

      // Check extracted versions
      const goEntry = report.tools.find((t) => t.id === 'go')
      expect(goEntry?.installed).toBe(true)
      expect(goEntry?.version).toContain('1.22.1')
    })

    it('marks ENOENT as installed:false', async () => {
      const mockRun = async () => null // null simulates ENOENT

      const validated = validateCapabilityParams({ tools: ['git'] })
      if (!validated.ok) return

      const report = await buildCapabilityReport(validated.value, dummyConfig, {
        run: mockRun
      })

      const gitEntry = report.tools.find((t) => t.id === 'git')
      expect(gitEntry?.installed).toBe(false)
      expect(gitEntry?.version).toBeUndefined()
    })

    it('never includes any env var values in report and reports booleans', async () => {
      const mockEnv = {
        ANTHROPIC_API_KEY: 'sk-super-secret-123456',
        EMPTY_VAR: '',
        OTHER_KEY: 'value'
      }

      const validated = validateCapabilityParams({
        envNames: ['ANTHROPIC_API_KEY', 'EMPTY_VAR', 'UNSET_KEY']
      })
      if (!validated.ok) return

      const report = await buildCapabilityReport(validated.value, dummyConfig, {
        env: mockEnv,
        run: async () => ({ stdout: '1.0.0\n' })
      })

      const jsonStr = JSON.stringify(report)
      expect(jsonStr).not.toContain('sk-super-secret-123456')

      const anthropic = report.env.find((e) => e.name === 'ANTHROPIC_API_KEY')
      expect(anthropic).toEqual({ name: 'ANTHROPIC_API_KEY', present: true })

      const empty = report.env.find((e) => e.name === 'EMPTY_VAR')
      expect(empty).toEqual({ name: 'EMPTY_VAR', present: false })

      const unset = report.env.find((e) => e.name === 'UNSET_KEY')
      expect(unset).toEqual({ name: 'UNSET_KEY', present: false })
    })

    it('reads claude auth as a boolean and never copies other fields', async () => {
      const mockRun = async (id: string, args: readonly string[]) => {
        if (id === 'claude' && args.includes('auth')) {
          return {
            stdout: JSON.stringify({
              loggedIn: true,
              email: 'secret@corp.com',
              organizationId: 'org-secret-999'
            })
          }
        }
        return { stdout: 'claude 1.2.3\n' }
      }

      const validated = validateCapabilityParams({ tools: ['claude'] })
      if (!validated.ok) return

      const report = await buildCapabilityReport(validated.value, dummyConfig, {
        run: mockRun,
        detectFlags: async () => ({ tools: true, permissionMode: true, disallowedTools: false })
      })

      expect(report.claude.installed).toBe(true)
      expect(report.claude.auth).toBe('logged_in')
      expect((report.claude as any).email).toBeUndefined()
      expect((report.claude as any).organizationId).toBeUndefined()
    })

    it('reports auth unknown when claude auth status is not JSON or missing loggedIn', async () => {
      const mockRun = async (id: string, args: readonly string[]) => {
        if (id === 'claude' && args.includes('auth')) {
          return { stdout: 'Not JSON format at all' }
        }
        return { stdout: 'claude 1.2.3\n' }
      }

      const validated = validateCapabilityParams({ tools: ['claude'], refresh: true })
      if (!validated.ok) return

      const report = await buildCapabilityReport(validated.value, dummyConfig, {
        run: mockRun,
        detectFlags: async () => null
      })

      expect(report.claude.auth).toBe('unknown')
    })
  })

  describe('handleAgentCapabilities RPC handler', () => {
    it('returns error for invalid parameters', async () => {
      const res = (await handleAgentCapabilities('req-1', { tools: 'invalid' }, dummyConfig, dummyLogger)) as any
      expect(res.jsonrpc).toBe('2.0')
      expect(res.id).toBe('req-1')
      expect(res.error.code).toBe(-32602)
      expect(res.error.data.reason).toBe('INVALID_CAPABILITY_PARAMS')
    })

    it('returns result under same id when valid', async () => {
      const res = (await handleAgentCapabilities('req-2', {}, dummyConfig, dummyLogger)) as any
      expect(res.jsonrpc).toBe('2.0')
      expect(res.id).toBe('req-2')
      expect(res.result).toBeDefined()
      expect(res.result.schemaVersion).toBe(1)
      expect(res.result.agent.protocolVersion).toBe(2)
    })
  })
})
