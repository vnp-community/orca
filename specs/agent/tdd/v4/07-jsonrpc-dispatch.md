# TDD-AG-07: JSON-RPC Dispatch

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `deploy/dev/agent/agent.js`

---

## 1. JSON-RPC 2.0 over Binary Frames

Agent nhận JSON-RPC 2.0 requests từ Orca Server qua binary frames:

```
Frame:    TYPE=0x01, CHANNEL_ID=N, SEQ=M, PAYLOAD=<JSON-RPC request>

Payload:  {
            "jsonrpc": "2.0",
            "id":      "req-uuid",
            "method":  "tools/call",
            "params":  { "name": "shell", "arguments": { "command": "ls", "args": ["-la"] } }
          }
```

---

## 2. Supported Methods

| Method | Description |
|--------|-------------|
| `tools/list` | List available tools |
| `tools/call` | Execute a tool |
| `ping` | Health check |

---

## 3. Dispatch Loop

```javascript
async function handleRpc(frame, msg) {
  const { id, method, params } = msg

  // Track ACK
  lastReceivedSeq = Math.max(lastReceivedSeq, frame.seqNo)

  try {
    let result
    switch (method) {
      case 'tools/list':
        result = { tools: getToolList() }
        break

      case 'tools/call':
        result = await callTool(params.name, params.arguments, frame.channelId)
        break

      case 'ping':
        result = { pong: true, timestamp: Date.now() }
        break

      default:
        throw { code: -32601, message: `Method not found: ${method}` }
    }

    sendResponse(frame.channelId, id, result)

  } catch (err) {
    sendError(frame.channelId, id, err)
  }
}
```

---

## 4. Streaming Tool Results

```javascript
async function callTool(toolName, args, channelId) {
  const tool = toolRegistry.get(toolName)
  if (!tool) {
    throw { code: -32601, message: `Tool not found: ${toolName}` }
  }

  const chunks = []

  // Chunk callback — stream partial results
  const onChunk = ({ text }) => {
    chunks.push(text)
    // Send intermediate notification (no id = notification)
    sendNotification(channelId, 'tools/progress', {
      toolName,
      chunk: text
    })
  }

  const result = await tool.handler(args, { onChunk })
  return result
}
```

---

## 5. Response Format

### Success

```json
{
  "jsonrpc": "2.0",
  "id": "req-uuid",
  "result": {
    "content": [{ "type": "text", "text": "output here" }]
  }
}
```

### Error

```json
{
  "jsonrpc": "2.0",
  "id": "req-uuid",
  "error": {
    "code":    -32603,
    "message": "Internal error",
    "data":    "Error details"
  }
}
```

### Progress Notification (no id)

```json
{
  "jsonrpc": "2.0",
  "method": "tools/progress",
  "params": {
    "toolName": "shell",
    "chunk": "partial output"
  }
}
```

---

## 6. Error Codes

| Code | Meaning |
|------|---------|
| -32700 | Parse error |
| -32600 | Invalid request |
| -32601 | Method not found |
| -32602 | Invalid params |
| -32603 | Internal error |

---

## 7. sendResponse / sendError

```javascript
function sendResponse(channelId, requestId, result) {
  const payload = JSON.stringify({
    jsonrpc: '2.0',
    id:      requestId,
    result
  })
  const frame = encodeFrame(channelId, ++outgoingSeq, payload)
  ws.send(frame)
}

function sendError(channelId, requestId, err) {
  const payload = JSON.stringify({
    jsonrpc: '2.0',
    id:      requestId,
    error:   {
      code:    err.code ?? -32603,
      message: err.message ?? 'Internal error',
      data:    err.data
    }
  })
  const frame = encodeFrame(channelId, ++outgoingSeq, payload)
  ws.send(frame)
}

function sendNotification(channelId, method, params) {
  const payload = JSON.stringify({ jsonrpc: '2.0', method, params })
  const frame   = encodeFrame(channelId, ++outgoingSeq, payload)
  ws.send(frame)
}
```

---

## 8. Concurrency Control

```javascript
// Per-channel call tracking
const inFlightCalls = new Map()   // Map<channelId, AbortController>

// Cancel on channel close:
ws.on('message', (data) => {
  const { channelId, payload } = decodeFrame(data)
  const msg = JSON.parse(payload)

  if (msg.type === 'cancel') {
    const ctrl = inFlightCalls.get(channelId)
    ctrl?.abort()
  }
})
```
