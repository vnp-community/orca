# TC-AWS-002 — direct-websocket Mode (Agent → Orca)

**BL Reference:** BL-AWS-02  
**Priority:** P1

---

## TC-AWS-002-01: Agent connects to Orca WS Server

### Steps
1. Agent: `ws://orca:6768/agent`
2. Send handshake: `{ jsonrpc: '2.0', method: 'agent.handshake', params: { agentToken: '...' } }`

### Expected Results
- Orca verifies agentToken
- Response: `{ result: { sessionId: '...' } }`
- AgentConnectionManager stores connection

---

## TC-AWS-002-02: agentToken — Invalid

### Steps
1. Handshake với invalid agentToken

### Expected Results
- WS closed: `{ error: 'INVALID_AGENT_TOKEN' }`

---

## TC-AWS-002-03: JSON-RPC 2.0 — Bidirectional

### Steps
1. Agent sends method call
2. Orca sends response
3. Orca sends request
4. Agent sends response

### Expected Results
- Full bidirectional JSON-RPC over single WS connection

