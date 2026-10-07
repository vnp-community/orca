import { describe, it, expect } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { buildCodeIntelChildEnv } from '../codeintel-child-env'
import { CODEINTEL_DEFAULT_LIMITS, readCodeIntelLimits } from '../codeintel-limits'
import { redactForClient, tailForStderr } from '../codeintel-secret-redaction'
import { readSymbolSource } from '../codeintel-symbol-source-reader'

describe('security-process-limits-and-env (Task 072-07)', () => {
  const canaryFixturePath = path.resolve(__dirname, '__fixtures__/canary-secrets.json')
  const { canaries, mockEnv } = JSON.parse(fs.readFileSync(canaryFixturePath, 'utf8'))

  it('Child process env strictly strips all secrets and retains only safe vars', () => {
    const childEnv = buildCodeIntelChildEnv({ toolEnv: mockEnv })

    expect(childEnv['PATH']).toBe(mockEnv.PATH)
    expect(childEnv['HOME']).toBe(mockEnv.HOME)
    expect(childEnv['NO_COLOR']).toBe('1')

    // Disallowed secret env vars
    expect(childEnv['ANTHROPIC_API_KEY']).toBeUndefined()
    expect(childEnv['GITHUB_TOKEN']).toBeUndefined()
    expect(childEnv['GH_TOKEN']).toBeUndefined()
    expect(childEnv['AGENT_TOKEN']).toBeUndefined()
    expect(childEnv['ORCA_DATABASE_URL']).toBeUndefined()
    expect(childEnv['SSH_AUTH_SOCK']).toBeUndefined()
    expect(childEnv['AWS_SECRET_ACCESS_KEY']).toBeUndefined()
    expect(childEnv['MY_SECRET_THING']).toBeUndefined()
  })

  it('Enforces default process and memory limits from contract', () => {
    expect(CODEINTEL_DEFAULT_LIMITS.toolMaxOutputBytes).toBe(16 * 1024 * 1024) // 16 MiB
    expect(CODEINTEL_DEFAULT_LIMITS.resultMaxBytes).toBe(8 * 1024 * 1024) // 8 MiB
    expect(CODEINTEL_DEFAULT_LIMITS.toolTimeoutMs).toBe(20000) // 20s
    expect(CODEINTEL_DEFAULT_LIMITS.killGraceMs).toBe(5000) // 5s

    const limits = readCodeIntelLimits({}, { warn: () => {} })
    expect(limits.toolMaxOutputBytes).toBe(16 * 1024 * 1024)
    expect(limits.toolTimeoutMs).toBe(20000)
  })

  it('Redacts canary tokens from CLI output and stderr', () => {
    for (const secret of canaries) {
      const rawText = `Error occurred with secret: ${secret} while processing`
      const redacted = redactForClient(rawText, { home: '/Users/canary-user' })

      if (secret.startsWith('ghp_') || secret.startsWith('AKIA') || secret.startsWith('sk-') || secret.includes('://')) {
        expect(redacted).not.toContain(secret)
      }
    }

    const testStderr = `Process failed at /Users/canary-user/repo with token ghp_CanaryGitHubToken1234567890abcdef`
    const tail = tailForStderr(testStderr, { home: '/Users/canary-user' })
    expect(tail).not.toContain('/Users/canary-user')
    expect(tail).not.toContain('ghp_CanaryGitHubToken1234567890abcdef')
    expect(tail).toContain('~')
  })

  it('Symbol source reader strictly omits source for sensitive files', async () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-sensitive-files-'))
    try {
      fs.writeFileSync(path.join(tmpDir, '.env'), 'DATABASE_PASSWORD=supersecret')
      fs.writeFileSync(path.join(tmpDir, '.env.production'), 'API_KEY=prodsecret')
      fs.writeFileSync(path.join(tmpDir, 'server.pem'), 'CERTIFICATE')
      fs.writeFileSync(path.join(tmpDir, 'private.key'), 'PRIVATE_KEY')
      fs.writeFileSync(path.join(tmpDir, 'id_rsa'), 'RSA_KEY')
      fs.writeFileSync(path.join(tmpDir, 'normal.ts'), 'export const x = 1;')

      const envRes = await readSymbolSource(tmpDir, '.env', 1, 10)
      expect(envRes.sourceOmitted).toBe('sensitive')
      expect(envRes.text).toBeUndefined()

      const envProdRes = await readSymbolSource(tmpDir, '.env.production', 1, 10)
      expect(envProdRes.sourceOmitted).toBe('sensitive')

      const pemRes = await readSymbolSource(tmpDir, 'server.pem', 1, 10)
      expect(pemRes.sourceOmitted).toBe('sensitive')

      const keyRes = await readSymbolSource(tmpDir, 'private.key', 1, 10)
      expect(keyRes.sourceOmitted).toBe('sensitive')

      const rsaRes = await readSymbolSource(tmpDir, 'id_rsa', 1, 10)
      expect(rsaRes.sourceOmitted).toBe('sensitive')

      const normalRes = await readSymbolSource(tmpDir, 'normal.ts', 1, 10)
      expect(normalRes.sourceOmitted).toBeUndefined()
      expect(normalRes.text).toContain('export const x = 1;')
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true })
    }
  })
})
