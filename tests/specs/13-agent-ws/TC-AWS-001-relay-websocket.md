# TC-AWS-001 — relay-websocket Mode (Orca → Agent)

**BL Reference:** BL-AWS-01  
**Priority:** P1  
**Actor:** Agent Developer

---

## TC-AWS-001-01: Orca connects to Agent WS Server

### Steps
1. Agent WS Server listening at `ws://agent-host:6799/orca-relay`
2. Orca: `agentWS.connectRelay { url: 'ws://agent-host:6799/orca-relay', bearerToken: '...' }`

### Expected Results
- Orca establishes WS connection as CLIENT
- Bearer token sent in Authorization header
- Handshake: Orca → Agent: `{ mode: 'relay' }`

---

## TC-AWS-001-02: Bearer token auth — Invalid token

### Steps
1. Connect với invalid bearer token

### Expected Results
- Connection rejected: 401

---

## TC-AWS-001-03: Binary wire protocol — 13-byte header

### Steps
1. Send JSON-RPC message
2. Capture WS frame

### Expected Results
- Frame = 13-byte header + JSON-RPC payload
- Header: `[TYPE(1)][SEQ u32 BE][ACK u32 BE][LEN u32 BE]`
- `LEN` matches actual payload length

