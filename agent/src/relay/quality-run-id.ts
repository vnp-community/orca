import { randomBytes } from 'crypto'

const CROCKFORD_BASE32 = '0123456789ABCDEFGHJKMNPQRSTVWXYZ'

export function generateRunId(): string {
  const bytes = randomBytes(16)
  let str = ''
  for (let i = 0; i < bytes.length; i++) {
    const val = bytes[i]
    str += CROCKFORD_BASE32[val % 32]
  }
  return `qr_${str.toLowerCase()}`
}

export function isValidRunId(runId: string): boolean {
  return /^qr_[0-9a-z]{8,40}$/i.test(runId)
}
