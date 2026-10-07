import { describe, it, expect } from 'vitest'
import { createRedactor, secretEnvValuesFrom, redactTail } from './quality-output-redaction'

describe('quality-output-redaction', () => {
  it('extracts secret values correctly', () => {
    const env = {
      MY_TOKEN: '12345678', // length 8 -> yes
      MY_SECRET: '1234567', // length 7 -> no
      FOO: '123456789' // no match
    }
    const secrets = secretEnvValuesFrom(env)
    expect(secrets).toContain('12345678')
    expect(secrets).not.toContain('1234567')
    expect(secrets).not.toContain('123456789')
  })

  it('redacts tokens and URI auth', () => {
    const r = createRedactor({ repoRoot: '/repo', home: '/home', tmpRoot: '/tmp', secretEnvValues: [] })
    
    expect(r('foo ghp_12345678901234567890 bar')).toBe('foo *** bar')
    expect(r('github_pat_abcdefghijklmnopqrstuvwx')).toBe('***')
    expect(r('AKIA0123456789ABCDEF')).toBe('***')
    expect(r('sk-12345678901234567890')).toBe('***')
    expect(r('eyJhbGciOiJIUzI1NiIsInR5cCI.eyJzdWIiOiIxMjM0NTY3ODkwIiwibm.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c')).toBe('***')
    
    // URI
    expect(r('connect to http://user:password@localhost:8080')).toBe('connect to http://user:***@localhost:8080')
    expect(r('ftp://admin:secret@domain.com/path')).toBe('ftp://admin:***@domain.com/path')
  })

  it('redacts private keys', () => {
    const r = createRedactor({ repoRoot: '/repo', home: '/home', tmpRoot: '/tmp', secretEnvValues: [] })
    const key = `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEA
-----END RSA PRIVATE KEY-----`
    expect(r(`some text ${key} more text`)).toBe('some text *** more text')
  })

  it('redacts env secrets and paths', () => {
    const r = createRedactor({ 
      repoRoot: 'C:\\Users\\space user\\repo', 
      home: 'C:\\Users\\space user', 
      tmpRoot: 'C:\\tmp', 
      secretEnvValues: ['secret_value_123'] 
    })

    const text = 'error in C:\\Users\\space user\\repo\\src\\main.ts with secret_value_123!'
    const expected = 'error in <repo>/src/main.ts with ***!'
    expect(r(text)).toBe(expected)

    // Note paths are normalized to /
    expect(r('C:\\Users\\space user\\docs')).toBe('~/docs')
  })

  it('handles multiple occurrences', () => {
    const r = createRedactor({ repoRoot: '/repo', home: '/home', tmpRoot: '/tmp', secretEnvValues: [] })
    expect(r('ghp_12345678901234567890 and sk-12345678901234567890')).toBe('*** and ***')
  })

  it('handles CRLF and normalizes paths', () => {
    const r = createRedactor({ repoRoot: '/repo', home: '/home', tmpRoot: '/tmp', secretEnvValues: [] })
    expect(r('line1\r\n/repo/foo\r\n')).toBe('line1\r\n<repo>/foo\r\n')
  })

  it('redactTail cuts correctly', () => {
    const r = createRedactor({ repoRoot: '/repo', home: '/home', tmpRoot: '/tmp', secretEnvValues: ['SECRET123'] })
    
    // "abcSECRET123" -> 12 bytes. Cut 10 bytes -> "cSECRET123" -> redacted to "c***"
    expect(redactTail('abcSECRET123', 10, r)).toBe('c***')
    
    // multibyte
    const str = 'ab\u2603SECRET123' // 2 + 3 + 9 = 14 bytes
    // cut 11 bytes: \u2603 is 3 bytes. start = 14 - 11 = 3 -> pointing to middle of \u2603?
    // Let's test standard cut
    const cut = redactTail(str, 12, r) // \u2603 starts at 2. 14-12=2. Should keep \u2603SECRET123
    expect(cut).toBe('\u2603***')
  })

  it('executes under 200ms for 1 MiB string (catastrophic backtracking)', () => {
    const r = createRedactor({ repoRoot: '/repo', home: '/home', tmpRoot: '/tmp', secretEnvValues: [] })
    // Build 1 MiB string
    const chunk = 'this is a normal log line with some text and no secrets\n'
    const targetSize = 1024 * 1024
    let str = ''
    while (str.length < targetSize) {
      str += chunk
    }
    
    const start = performance.now()
    r(str)
    const end = performance.now()
    
    expect(end - start).toBeLessThan(200)
  })
})
