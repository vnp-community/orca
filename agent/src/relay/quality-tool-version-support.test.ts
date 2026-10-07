import { describe, it, expect } from 'vitest'
import { classifyToolVersion, readToolVersion } from './quality-tool-version-support'

describe('quality-tool-version-support', () => {
  it('classifies versions', () => {
    expect(classifyToolVersion('oxlint', '1.71.0')).toBe('verified')
    expect(classifyToolVersion('oxlint', '1.72.0')).toBe('untested')
    expect(classifyToolVersion('oxlint', '2.0.0')).toBe('incompatible')
    expect(classifyToolVersion('unknown-tool', '1.0.0')).toBe('untested')
  })

  it('reads tool version', async () => {
    const execute = async (cmd: string, args: string[]) => {
      if (cmd === 'oxlint' && args[0] === '--version') return { stdout: '1.71.0\n', stderr: '', exitCode: 0 }
      if (cmd === 'go' && args[0] === 'version') return { stdout: 'go version go1.26.0 linux/amd64\n', stderr: '', exitCode: 0 }
      return { stdout: '', stderr: '', exitCode: 1 }
    }

    expect(await readToolVersion('oxlint', execute)).toBe('1.71.0')
    expect(await readToolVersion('go', execute)).toBe('go version go1.26.0 linux/amd64')
    expect(await readToolVersion('orca-check', execute)).toBe('1.0.0')
    expect(await readToolVersion('not-found', execute)).toBeNull()
  })
})
