import { describe, expect, it } from 'vitest'
import { maskSensitiveRecord, maskSensitiveText } from './sensitive-text-masking'

const HEX40 = 'a94a8fe5ccb19ba61c4c0873d391e987982fbbd3'

describe('maskSensitiveText', () => {
  const masked: [string, string, string][] = [
    ['DSN userinfo', 'postgres://u:p4ss@db.local/app', 'postgres://•••@db.local/app'],
    ['user only userinfo', 'redis://token123@cache:6379', 'redis://•••@cache:6379'],
    ['password=', 'host=a password=abc port=1', 'host=a password=••• port=1'],
    ['json token', '{"token":"abc123"}', '{"token":"•••"}'],
    ['yaml secret', 'api_key: sk-live-xyz', 'api_key: •••'],
    ['hex after key', `token: ${HEX40}`, 'token: •••']
  ]
  for (const [name, input, expected] of masked) {
    it(`masks ${name}`, () => {
      const r = maskSensitiveText(input)
      expect(r.text).toBe(expected)
      expect(r.masked).toBe(true)
    })
  }

  it('masks a multi-line private key block', () => {
    const pem = 'x\n-----BEGIN RSA PRIVATE KEY-----\nMIIabc\ndef\n-----END RSA PRIVATE KEY-----\ny'
    expect(maskSensitiveText(pem).text).toBe('x\n•••\ny')
  })

  it('masks very long hex and mixed-case base64-like tokens', () => {
    expect(maskSensitiveText(`v ${'ab12'.repeat(16)} v`).masked).toBe(true)
    expect(maskSensitiveText('k Zm9vYmFyQmF6MTIzNDU2Nzg5MEFiQ2RFZkdoSWo= k').masked).toBe(true)
    expect(maskSensitiveText('eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.sig').masked).toBe(
      true
    )
  })

  const unchanged: [string, string][] = [
    ['URL without userinfo', 'https://example.com/a?b=1'],
    ['UUID', '123e4567-e89b-12d3-a456-426614174000'],
    ['Vault path', 'secret/data/infra/ssh'],
    ['commit hash', `commit ${HEX40}`],
    ['snake_case name', 'infra_fleet_service_dev_servers_table_2'],
    ['plain prose', 'status of the dev server'],
    ['empty', '']
  ]
  for (const [name, input] of unchanged) {
    it(`leaves ${name} unchanged`, () => {
      expect(maskSensitiveText(input)).toEqual({ text: input, masked: false })
    })
  }

  it('is idempotent', () => {
    const once = maskSensitiveText('postgres://u:p@h/db password=abc {"token":"x"}').text
    const twice = maskSensitiveText(once)
    expect(twice.text).toBe(once)
    expect(twice.masked).toBe(false)
  })

  it('stays fast on 1 MB hostile inputs', () => {
    const inputs = [
      'a'.repeat(1_000_000),
      'a-'.repeat(500_000),
      'password'.repeat(125_000),
      'password="'.repeat(100_000),
      'http://'.repeat(140_000),
      '-----BEGIN PRIVATE KEY-----'.repeat(37_000)
    ]
    const start = Date.now()
    for (const input of inputs) {
      maskSensitiveText(input)
    }
    expect(Date.now() - start).toBeLessThan(5000)
  })
})

describe('maskSensitiveRecord', () => {
  it('masks only the listed string fields without mutating the input', () => {
    const input = { defaultExpr: "'password=abc'", comment: 'ok', name: 'password=keep', n: 1 }
    const { value, masked } = maskSensitiveRecord(input, ['defaultExpr', 'comment'])
    expect(masked).toBe(true)
    expect(value.defaultExpr).toContain('•••')
    expect(value.name).toBe('password=keep')
    expect(input.defaultExpr).toBe("'password=abc'")
  })

  it('reports masked=false and skips non-strings', () => {
    const r = maskSensitiveRecord({ a: undefined as string | undefined }, ['a'])
    expect(r.masked).toBe(false)
  })
})
