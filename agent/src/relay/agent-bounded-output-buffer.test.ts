import { describe, it, expect } from 'vitest'
import {
  BoundedOutputBuffer,
  splitOutputBudget,
  fitResultToFrame
} from './agent-bounded-output-buffer'

describe('agent-bounded-output-buffer', () => {
  describe('BoundedOutputBuffer', () => {
    it('keeps all output under budget without truncation', () => {
      const buf = new BoundedOutputBuffer(100)
      buf.append(Buffer.from('hello '))
      buf.append(Buffer.from('world'))
      expect(buf.truncated).toBe(false)
      expect(buf.byteLength).toBe(11)
      expect(buf.toString()).toBe('hello world')
    })

    it('truncates from head when exceeding budget and preserves byteLength === maxBytes', () => {
      const buf = new BoundedOutputBuffer(10)
      buf.append(Buffer.from('12345678')) // 8 bytes
      buf.append(Buffer.from('ABCDEFGH')) // +8 bytes = 16 bytes total
      expect(buf.truncated).toBe(true)
      expect(buf.byteLength).toBe(10)
      // Tail 10 bytes: last 2 bytes of first chunk ('78') + 'ABCDEFGH'
      expect(buf.toString()).toBe('78ABCDEFGH')
    })

    it('handles a single chunk larger than maxBytes', () => {
      const buf = new BoundedOutputBuffer(5)
      buf.append(Buffer.from('abcdefghij')) // 10 bytes
      expect(buf.truncated).toBe(true)
      expect(buf.byteLength).toBe(5)
      expect(buf.toString()).toBe('fghij')
    })

    it('preserves valid UTF-8 boundary without replacement char at truncated start', () => {
      // Vietnamese character "ế" is 3 bytes in UTF-8: 0xE1 0xBA 0xBF
      // Prefix with 4 ASCII bytes 'abcd' -> total 7 bytes.
      // If maxBytes is 4, cutting naive 4 bytes leaves continuation byte 0xBA 0xBF...
      const str = 'abcdế'
      const raw = Buffer.from(str, 'utf8') // 4 + 3 = 7 bytes
      const buf = new BoundedOutputBuffer(5) // Leaves 2 ASCII bytes + 'ế' (total 5 bytes)
      buf.append(raw)
      expect(buf.truncated).toBe(true)
      // toString() strips leading continuation bytes
      const out = buf.toString()
      expect(out).not.toContain('\uFFFD')
    })

    it('returns empty string for zero chunks or empty buffers', () => {
      const buf = new BoundedOutputBuffer(10)
      expect(buf.toString()).toBe('')
      buf.append(Buffer.alloc(0))
      expect(buf.toString()).toBe('')
      expect(buf.byteLength).toBe(0)
    })
  })

  describe('splitOutputBudget', () => {
    it('allocates 75% to stdout and 25% to stderr', () => {
      const split = splitOutputBudget(4 * 1024 * 1024)
      expect(split.stdoutBytes).toBe(3 * 1024 * 1024)
      expect(split.stderrBytes).toBe(1 * 1024 * 1024)
    })
  })

  describe('fitResultToFrame', () => {
    it('does nothing if already within frameLimitBytes', () => {
      const res = { stdout: 'small', stderr: 'small' }
      const fitted = fitResultToFrame(res, 1000)
      expect(fitted).toEqual(res)
      expect(fitted.truncated).toBeUndefined()
    })

    it('sheds stdout iteratively until fitting within limit and marks truncated', () => {
      const bigStdout = 'x'.repeat(2000)
      const res = { stdout: bigStdout }
      const fitted = fitResultToFrame(res, 1000)
      expect(Buffer.byteLength(JSON.stringify(fitted))).toBeLessThanOrEqual(1000)
      expect(fitted.truncated?.stdout).toBe(true)
    })
  })
})
