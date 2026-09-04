// src/relay/__tests__/agent-wire.test.ts
import { describe, it, expect } from 'vitest'
import {
  createWireState,
  encodeDataFrame,
  encodeKeepaliveFrame,
  decodeFrame,
  parseJsonPayload,
  HEADER_SIZE,
} from 'orca-dev-agent-transport'

// Wire protocol TYPE values
const TYPE_REGULAR   = 1  // MessageType.Regular
const TYPE_KEEPALIVE = 9  // MessageType.KeepAlive

describe('HEADER_SIZE', () => {
  it('is 13 bytes (1 type + 4 seq + 4 ack + 4 length)', () => {
    expect(HEADER_SIZE).toBe(13)
  })
})

describe('createWireState', () => {
  it('initializes seqCounter=0', () => {
    expect(createWireState().seqCounter).toBe(0)
  })

  it('initializes highestAck=0', () => {
    expect(createWireState().highestAck).toBe(0)
  })

  it('returns independent objects on each call', () => {
    const s1 = createWireState()
    const s2 = createWireState()
    s1.seqCounter = 99
    expect(s2.seqCounter).toBe(0)
  })
})

describe('encodeDataFrame', () => {
  it('sets TYPE byte to Regular (1)', () => {
    const f = encodeDataFrame(createWireState(), 'hello')
    expect(f.readUInt8(0)).toBe(TYPE_REGULAR)
  })

  it('pre-increments seqCounter before encoding (first frame has seq=1)', () => {
    const s = createWireState()
    const f = encodeDataFrame(s, 'x')
    expect(f.readUInt32BE(1)).toBe(1)
  })

  it('increments seqCounter on each call', () => {
    const s = createWireState()
    encodeDataFrame(s, 'a')
    encodeDataFrame(s, 'b')
    expect(s.seqCounter).toBe(2)
  })

  it('SEQ field == seqCounter after increment', () => {
    const s = createWireState()
    const f = encodeDataFrame(s, 'x')
    expect(f.readUInt32BE(1)).toBe(s.seqCounter)
  })

  it('ACK field == state.highestAck', () => {
    const s = createWireState()
    s.highestAck = 42
    const f = encodeDataFrame(s, 'x')
    expect(f.readUInt32BE(5)).toBe(42)
  })

  it('LENGTH field == payload byte length (ASCII)', () => {
    const s = createWireState()
    const payload = 'hello'
    const f = encodeDataFrame(s, payload)
    expect(f.readUInt32BE(9)).toBe(Buffer.byteLength(payload, 'utf8'))
  })

  it('LENGTH field == payload byte length (multi-byte UTF-8)', () => {
    const s = createWireState()
    const payload = '日本語'
    const f = encodeDataFrame(s, payload)
    expect(f.readUInt32BE(9)).toBe(Buffer.byteLength(payload, 'utf8'))
  })

  it('total frame size == HEADER_SIZE + payload length', () => {
    const s = createWireState()
    const payload = 'test'
    const f = encodeDataFrame(s, payload)
    expect(f.length).toBe(HEADER_SIZE + Buffer.byteLength(payload))
  })

  it('accepts Buffer payload', () => {
    const s = createWireState()
    const buf = Buffer.from('buf')
    const f = encodeDataFrame(s, buf)
    expect(f.readUInt32BE(9)).toBe(3)
  })

  it('accepts empty string payload (length=0)', () => {
    const s = createWireState()
    const f = encodeDataFrame(s, '')
    expect(f.readUInt32BE(9)).toBe(0)
    expect(f.length).toBe(HEADER_SIZE)
  })

  it('two connections have independent seqCounters', () => {
    const s1 = createWireState()
    const s2 = createWireState()
    encodeDataFrame(s1, 'a')
    encodeDataFrame(s1, 'b')
    encodeDataFrame(s2, 'c')
    expect(s1.seqCounter).toBe(2)
    expect(s2.seqCounter).toBe(1)
  })
})

describe('encodeKeepaliveFrame', () => {
  it('sets TYPE byte to KeepAlive (9)', () => {
    const f = encodeKeepaliveFrame(createWireState())
    expect(f.readUInt8(0)).toBe(TYPE_KEEPALIVE)
  })

  it('LENGTH field == 0', () => {
    const f = encodeKeepaliveFrame(createWireState())
    expect(f.readUInt32BE(9)).toBe(0)
  })

  it('total frame size == HEADER_SIZE (no payload)', () => {
    const f = encodeKeepaliveFrame(createWireState())
    expect(f.length).toBe(HEADER_SIZE)
  })

  it('increments seqCounter', () => {
    const s = createWireState()
    encodeKeepaliveFrame(s)
    expect(s.seqCounter).toBe(1)
  })
})

describe('decodeFrame', () => {
  it('returns null for buffer shorter than HEADER_SIZE', () => {
    expect(decodeFrame(createWireState(), Buffer.alloc(5))).toBeNull()
  })

  it('returns null for empty buffer', () => {
    expect(decodeFrame(createWireState(), Buffer.alloc(0))).toBeNull()
  })

  it('decodes type, seq, ack, length fields correctly', () => {
    const inBuf = Buffer.allocUnsafe(HEADER_SIZE)
    inBuf.writeUInt8(1, 0)
    inBuf.writeUInt32BE(7, 1)
    inBuf.writeUInt32BE(3, 5)
    inBuf.writeUInt32BE(0, 9)
    const frame = decodeFrame(createWireState(), inBuf)!
    expect(frame.type).toBe(1)
    expect(frame.seq).toBe(7)
    expect(frame.ack).toBe(3)
    expect(frame.length).toBe(0)
  })

  it('updates state.highestAck from incoming seq', () => {
    const s = createWireState()
    const inBuf = Buffer.allocUnsafe(HEADER_SIZE)
    inBuf.writeUInt8(1, 0)
    inBuf.writeUInt32BE(7, 1)
    inBuf.writeUInt32BE(0, 5)
    inBuf.writeUInt32BE(0, 9)
    decodeFrame(s, inBuf)
    expect(s.highestAck).toBe(7)
  })

  it('does NOT lower highestAck if incoming seq < current highestAck', () => {
    const s = createWireState()
    s.highestAck = 10
    const inBuf = Buffer.allocUnsafe(HEADER_SIZE)
    inBuf.writeUInt8(1, 0)
    inBuf.writeUInt32BE(3, 1)  // seq=3 < highestAck=10
    inBuf.writeUInt32BE(0, 5)
    inBuf.writeUInt32BE(0, 9)
    decodeFrame(s, inBuf)
    expect(s.highestAck).toBe(10)
  })
})

describe('parseJsonPayload', () => {
  it('returns parsed object', () => {
    expect(parseJsonPayload(Buffer.from('{"ok":true}'))).toEqual({ ok: true })
  })

  it('returns parsed array', () => {
    expect(parseJsonPayload(Buffer.from('[1,2,3]'))).toEqual([1, 2, 3])
  })

  it('returns null for malformed JSON', () => {
    expect(parseJsonPayload(Buffer.from('{bad}'))).toBeNull()
  })

  it('returns null for empty buffer', () => {
    expect(parseJsonPayload(Buffer.alloc(0))).toBeNull()
  })

  it('returns null for non-JSON string', () => {
    expect(parseJsonPayload(Buffer.from('hello world'))).toBeNull()
  })
})

describe('round-trip encode → decode', () => {
  it('decodes payload back to original string', () => {
    const s1 = createWireState()
    const s2 = createWireState()
    const payload = JSON.stringify({ method: 'tools/list', id: 42 })
    const frame   = encodeDataFrame(s1, payload)
    const decoded = decodeFrame(s2, frame)!
    expect(decoded.payload.toString('utf8')).toBe(payload)
  })

  it('decoded type matches TYPE_REGULAR', () => {
    const s1 = createWireState()
    const s2 = createWireState()
    const frame   = encodeDataFrame(s1, 'x')
    const decoded = decodeFrame(s2, frame)!
    expect(decoded.type).toBe(TYPE_REGULAR)
  })

  it('decoded seq == encoder seqCounter', () => {
    const s1 = createWireState()
    const s2 = createWireState()
    const frame   = encodeDataFrame(s1, 'x')
    const decoded = decodeFrame(s2, frame)!
    expect(decoded.seq).toBe(s1.seqCounter)
  })

  it('keepalive round-trip: type=9, length=0', () => {
    const s1 = createWireState()
    const s2 = createWireState()
    const frame   = encodeKeepaliveFrame(s1)
    const decoded = decodeFrame(s2, frame)!
    expect(decoded.type).toBe(TYPE_KEEPALIVE)
    expect(decoded.length).toBe(0)
  })
})
