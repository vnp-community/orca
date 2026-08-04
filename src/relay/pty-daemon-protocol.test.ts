import { describe, it, expect, vi } from 'vitest'
import {
  DaemonMessageDecoder,
  encodeDaemonMessage,
  isDaemonRequest,
  isDaemonResponse,
  type DaemonMessage
} from './pty-daemon-protocol'

describe('pty-daemon-protocol', () => {
  describe('isDaemonRequest / isDaemonResponse', () => {
    it('classifies a request (has id and method)', () => {
      const msg: DaemonMessage = { id: 1, method: 'pty.create', params: {} }
      expect(isDaemonRequest(msg)).toBe(true)
      expect(isDaemonResponse(msg)).toBe(false)
    })

    it('classifies a response (has id, no method)', () => {
      const msg: DaemonMessage = { id: 1, result: { ok: true } }
      expect(isDaemonRequest(msg)).toBe(false)
      expect(isDaemonResponse(msg)).toBe(true)
    })

    it('classifies a notification (has method, no id) as neither', () => {
      const msg: DaemonMessage = { method: 'pty.data', params: { id: 'x', data: 'y' } }
      expect(isDaemonRequest(msg)).toBe(false)
      expect(isDaemonResponse(msg)).toBe(false)
    })
  })

  describe('encodeDaemonMessage', () => {
    it('serializes to JSON followed by a single trailing newline', () => {
      const line = encodeDaemonMessage({ id: 1, method: 'daemon.ping' })
      expect(line).toBe('{"id":1,"method":"daemon.ping"}\n')
    })
  })

  describe('DaemonMessageDecoder', () => {
    it('emits one message per complete line fed in a single chunk', () => {
      const onMessage = vi.fn()
      const decoder = new DaemonMessageDecoder(onMessage)
      decoder.feed('{"id":1,"method":"a"}\n{"id":2,"method":"b"}\n')
      expect(onMessage).toHaveBeenCalledTimes(2)
      expect(onMessage).toHaveBeenNthCalledWith(1, { id: 1, method: 'a' })
      expect(onMessage).toHaveBeenNthCalledWith(2, { id: 2, method: 'b' })
    })

    it('buffers a message split across multiple chunks', () => {
      const onMessage = vi.fn()
      const decoder = new DaemonMessageDecoder(onMessage)
      decoder.feed('{"id":1,"met')
      expect(onMessage).not.toHaveBeenCalled()
      decoder.feed('hod":"a"}\n')
      expect(onMessage).toHaveBeenCalledExactlyOnceWith({ id: 1, method: 'a' })
    })

    it('drops a malformed line silently and keeps decoding subsequent lines', () => {
      const onMessage = vi.fn()
      const decoder = new DaemonMessageDecoder(onMessage)
      decoder.feed('not json\n{"id":1,"method":"a"}\n')
      expect(onMessage).toHaveBeenCalledExactlyOnceWith({ id: 1, method: 'a' })
    })

    it('ignores blank lines', () => {
      const onMessage = vi.fn()
      const decoder = new DaemonMessageDecoder(onMessage)
      decoder.feed('\n\n{"id":1,"method":"a"}\n')
      expect(onMessage).toHaveBeenCalledExactlyOnceWith({ id: 1, method: 'a' })
    })

    it('retains an incomplete trailing line across feeds without emitting it', () => {
      const onMessage = vi.fn()
      const decoder = new DaemonMessageDecoder(onMessage)
      decoder.feed('{"id":1,"method":"a"}\n{"id":2,"method":"b"}')
      expect(onMessage).toHaveBeenCalledExactlyOnceWith({ id: 1, method: 'a' })
      decoder.feed('\n')
      expect(onMessage).toHaveBeenCalledTimes(2)
      expect(onMessage).toHaveBeenNthCalledWith(2, { id: 2, method: 'b' })
    })
  })
})
