import { describe, expect, it } from 'vitest'
import { getPowerShellOsc133Bootstrap, isPowerShellExecutableName, encodePowerShellCommand } from './pty-osc133-bootstrap'

describe('getPowerShellOsc133Bootstrap', () => {
  it('embeds OSC 133 A/B/C/D markers', () => {
    const script = getPowerShellOsc133Bootstrap()
    expect(script).toContain(']133;A')
    expect(script).toContain(']133;B')
    expect(script).toContain(']133;C')
    expect(script).toContain(']133;D')
  })

  it('base64/utf16le-encodes for -EncodedCommand', () => {
    const encoded = encodePowerShellCommand(getPowerShellOsc133Bootstrap())
    const decoded = Buffer.from(encoded, 'base64').toString('utf16le')
    expect(decoded).toContain('Orca OSC 133 shell integration for PowerShell')
  })
})

describe('isPowerShellExecutableName', () => {
  it('matches all four PowerShell executable name spellings', () => {
    expect(isPowerShellExecutableName('pwsh')).toBe(true)
    expect(isPowerShellExecutableName('PWSH.EXE')).toBe(true)
    expect(isPowerShellExecutableName('powershell')).toBe(true)
    expect(isPowerShellExecutableName('powershell.exe')).toBe(true)
    expect(isPowerShellExecutableName('bash')).toBe(false)
  })
})
