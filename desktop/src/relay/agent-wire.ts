// src/relay/agent-wire.ts
// Binary frame codec for Agent Wire Protocol v1.
//
// Frame format (13-byte header):
//   [TYPE u8][SEQ u32 BE][ACK u32 BE][LENGTH u32 BE][PAYLOAD bytes]
//
// TYPE: MessageType.Regular (1) = JSON-RPC data frame
//       MessageType.KeepAlive (9) = ping/pong frame, always empty payload
//
// SEQ: outgoing frame sequence counter (pre-incremented before each send, starts at 1)
// ACK: highest incoming SEQ seen from peer (updated by decodeFrame, sent in each outgoing frame)
//
// Rules:
//   - Create one WireState per WebSocket connection via createWireState()
//   - NEVER share a WireState between connections
//   - decodeFrame() updates state.highestAck as a side effect

import { MessageType } from '../main/ssh/relay-protocol'

export const HEADER_SIZE = 13  // 1 (type) + 4 (seq) + 4 (ack) + 4 (length)

/**
 * Per-connection mutable state.
 * Call createWireState() at the start of each new WebSocket connection.
 */
export type WireState = {
  /** Incremented before each outgoing frame (starts at 0, so first frame has seq=1) */
  seqCounter: number
  /** Highest SEQ received from peer — sent as ACK field in our outgoing frames */
  highestAck: number
}

export function createWireState(): WireState {
  return { seqCounter: 0, highestAck: 0 }
}

/**
 * Encode a JSON-RPC data frame (TYPE = Regular = 1).
 * Payload can be a UTF-8 string or a pre-built Buffer.
 */
export function encodeDataFrame(state: WireState, payload: string | Buffer): Buffer {
  return encodeFrame(state, MessageType.Regular, payload)
}

/**
 * Encode a keepalive frame (TYPE = KeepAlive = 9, LENGTH = 0, no payload).
 * Total size is always HEADER_SIZE (13 bytes).
 */
export function encodeKeepaliveFrame(state: WireState): Buffer {
  return encodeFrame(state, MessageType.KeepAlive, Buffer.alloc(0))
}

function encodeFrame(state: WireState, type: number, payload: string | Buffer): Buffer {
  const payloadBuf = typeof payload === 'string'
    ? Buffer.from(payload, 'utf8')
    : payload
  const frame = Buffer.allocUnsafe(HEADER_SIZE + payloadBuf.length)
  const seq = ++state.seqCounter
  frame.writeUInt8(type, 0)
  frame.writeUInt32BE(seq, 1)
  frame.writeUInt32BE(state.highestAck, 5)
  frame.writeUInt32BE(payloadBuf.length, 9)
  if (payloadBuf.length > 0) {payloadBuf.copy(frame, HEADER_SIZE)}
  return frame
}

export type DecodedFrame = {
  type: number
  seq: number
  ack: number
  length: number
  payload: Buffer
}

/**
 * Decode an incoming binary frame.
 * Side effect: updates state.highestAck if incoming seq > current highestAck.
 * Returns null if buf is shorter than HEADER_SIZE (malformed frame — caller should ignore it).
 */
export function decodeFrame(state: WireState, buf: Buffer): DecodedFrame | null {
  if (buf.length < HEADER_SIZE) {return null}
  const type   = buf.readUInt8(0)
  const seq    = buf.readUInt32BE(1)
  const ack    = buf.readUInt32BE(5)
  const length = buf.readUInt32BE(9)
  const payload = buf.subarray(HEADER_SIZE, HEADER_SIZE + length)
  // Track highest peer SEQ so our outgoing ACK field stays current
  if (seq > state.highestAck) {state.highestAck = seq}
  return { type, seq, ack, length, payload }
}

/**
 * Parse JSON from a frame payload buffer.
 * Returns null on malformed JSON or empty buffer (never throws).
 */
export function parseJsonPayload<T = unknown>(payload: Buffer): T | null {
  if (payload.length === 0) {return null}
  try {
    return JSON.parse(payload.toString('utf8')) as T
  } catch {
    return null
  }
}
