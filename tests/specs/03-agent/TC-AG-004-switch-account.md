# TC-AG-004 — Switch Account / Provider

**BL Reference:** BL-AG-04  
**Flow Reference:** docs/flows/logic/agent-orchestration.md#BL-AG-04  
**Priority:** P1  
**Type:** Integration  
**Actor:** Alex, Sam

---

## TC-AG-004-01: Rate limit detection — Pattern match

**Priority:** P1

### Steps
1. PTY output stream emit rate-limit text:
   `"Rate limit exceeded. Please retry after 2026-08-02 10:00:00"`
2. Verify event

### Expected Results
- `AgentHookParser` detect rate-limit pattern
- Event: `agent:rateLimited { sessionId, pattern: 'rate limit exceeded', resetAt: '...' }`
- UI: alert "Claude Code bị rate limited. Reset lúc 10:00"

### Assertions
```
devServer.emit('agent.output', {
  ptyId,
  data: 'Rate limit exceeded. Please retry after 2026-08-02 10:00:00'
})
event = await events.next('agent:rateLimited')
assert event.sessionId === sessionId
assert event.resetAt !== undefined
```

---

## TC-AG-004-02: Switch tới account khác

**Priority:** P1

### Steps
1. User nhận `agent:rateLimited` alert
2. User chọn "Switch account 2"
3. RPC: `agent.switchAccount { sessionId, newAccountId: 'account-2' }`

### Expected Results
- Stop current agent: `agent.kill { ptyId }` → Dev Server
- Resolve new provider: account-2 credentials
- Spawn new agent với new env: `agent.spawn { newEnv }` → Dev Server

### Assertions
```
await ipc.invoke('agent.switchAccount', { sessionId, newAccountId: 'account-2' })
assert spyJsonRpc.calledWith('agent.kill', { ptyId })
newSpawn = spyJsonRpc.lastCall('agent.spawn')
assert newSpawn.args.env.ANTHROPIC_API_KEY !== oldApiKey // different key
```

---

## TC-AG-004-03: Switch provider (Claude → OpenAI)

**Priority:** P1

### Steps
1. User nhận rate limit
2. User chọn "Switch to OpenAI GPT-4"
3. `agent.switchAccount { sessionId, newProvider: 'openai', newAccountId: '...' }`

### Expected Results
- Old agent killed
- `AIProviderResolver.resolve()` với openai account
- New spawn: `agentBinary='openai-codex'` và `OPENAI_API_KEY` in env

---

## TC-AG-004-04: Resume compatible session sau switch

**Priority:** P1

### Preconditions
- Old agent: Claude, có session context
- New agent: Claude account-2 (compatible)

### Steps
1. Switch account (Claude account-2)
2. Auto-resume session nếu compatible

### Expected Results
- `agent.spawn` với `--resume sess-id` args (BL-AG-03 flow)

---

*TC-AG-004 — Orca v5.0 — 2026-08-01*
