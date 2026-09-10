// src/index.ts
// Public entry point for orca-dev-agent-transport.
//
// Re-exports the shared wire-protocol codec (relay-protocol.ts, agent-wire.ts)
// so agent/ and emulator/ import a single package name instead of relative
// cross-package paths. Pure codec only — see relay-protocol.ts/agent-wire.ts
// for the framing/seq-ack/keepalive implementation. No dispatcher/domain
// logic lives here or should ever be added here.

// relay-protocol.ts's `DecodedFrame` (the generic multi-purpose frame type
// used by ssh-channel-multiplexer.ts) and agent-wire.ts's `DecodedFrame`
// (agent-wire's own seq/ack-aware frame type) are two distinct types that
// happen to share a name. `export *` from both would be ambiguous, so
// agent-wire's exports are listed explicitly here (its DecodedFrame type is
// not currently imported by name anywhere — callers use decodeFrame()'s
// inferred return type) while relay-protocol's full surface, including its
// DecodedFrame, is re-exported as-is.
export * from './relay-protocol'
export {
  HEADER_SIZE,
  type WireState,
  createWireState,
  encodeDataFrame,
  encodeKeepaliveFrame,
  decodeFrame,
  parseJsonPayload
} from './agent-wire'
