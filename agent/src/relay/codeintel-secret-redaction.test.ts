import { describe, it, expect } from 'vitest'
import { redactForClient, tailForStderr } from './codeintel-secret-redaction'

describe('codeintel-secret-redaction', () => {
  it('redacts tokens and keys', () => {
    const text = 'Here is a ghp_1234567890abcdefGHIJKLMNOPQRSTUVwx token, and github_pat_11AAAAAAA0abcdefGHIJKL_MNOPQRSTUVWXYZ1234567890abcdefGHIJKLMNOPQRSTUVWXYZ1234567890, AKIA1234567890ABCDEF, and sk-ant-api03-123-abc. Also JWT eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c'
    const redacted = redactForClient(text, {})
    expect(redacted).not.toContain('ghp_')
    expect(redacted).not.toContain('github_pat_')
    expect(redacted).not.toContain('AKIA')
    expect(redacted).not.toContain('sk-ant-')
    expect(redacted).not.toContain('eyJ')
    expect(redacted).toContain('<REDACTED>')
  })

  it('redacts private keys', () => {
    const pem = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAw...
-----END RSA PRIVATE KEY-----`
    const text = `Error: \n${pem}\nDone.`
    const redacted = redactForClient(text, {})
    expect(redacted).not.toContain('MIIE')
    expect(redacted).toContain('<REDACTED>')
  })

  it('redacts urls with credentials', () => {
    const url = 'https://user:password123@github.com/repo.git'
    const redacted = redactForClient(url, {})
    expect(redacted).toBe('https://<REDACTED>@github.com/repo.git')
  })

  it('replaces $HOME with ~', () => {
    const text = 'File found at /Users/binhnt/Work/repo/file.ts'
    const redacted = redactForClient(text, { home: '/Users/binhnt' })
    expect(redacted).toBe('File found at ~/Work/repo/file.ts')
  })

  it('tailForStderr removes ANSI and caps at 2048 bytes', () => {
    const ansiText = '\u001b[36mℹ Symbol "X" not found\u001b[0m'
    const longText = 'a'.repeat(3000)
    
    expect(tailForStderr(ansiText)).toBe('ℹ Symbol "X" not found')
    
    const tail = tailForStderr(longText)
    expect(tail.length).toBe(2048 + 3) // ... + 2048
    expect(tail.startsWith('...a')).toBe(true)
  })
})
