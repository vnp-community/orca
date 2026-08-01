# BUG-AG-ORCH-007: `AgentHookParser` và OSC 133 parser cho agent orchestration không tồn tại — BL-AG-05 broken

## Mức độ: 🔴 HIGH

## Tóm tắt

HLD (BL-AG-05) mô tả:
```
[Orca Server — AgentHookParser]
    Parse OSC 133 sequences:
        ESC]133;A ST → command started  → status = "running"
        ESC]133;D;<code> ST → finished → check exit code
    Pattern match text:
        "waiting for input"  → status = "waiting"
        RATE_LIMIT_PATTERNS  → emit: agent:rateLimited
        "task completed"     → status = "completed"
    emit: agent:statusChanged { sessionId, status, detail }
```

Grep toàn bộ `src/main`:
```
AgentHookParser       → No results
agent:statusChanged   → No results
agent:rateLimited     → No results
agent:started         → No results
'133;A'               → No results (agent orchestration context)
RATE_LIMIT_PATTERNS   → exists in rate-limits/ nhưng không related đến agent orchestration
```

OSC 133 parsing **có tồn tại** nhưng chỉ trong:
- `orca-runtime.ts`: Terminal emulator OSC parsing cho local terminals
- `powershell-osc133-bootstrap.ts`: PowerShell shell integration
- `daemon/shell-ready.ts`: Shell prompt detection

**Không có OSC 133 parser nào trong agent output pipeline.**

## Root Cause

`AgentHookParser` phụ thuộc vào `agent.output` stream hoạt động (BUG-AG-ORCH-006). Vì stream không có receiver, AgentHookParser cũng không thể được trigger.

## Ảnh hưởng

1. **BL-AG-05**: Status monitoring broken hoàn toàn
2. Agent card trên UI không bao giờ chuyển từ "idle" → "running" → "completed"
3. Rate limit detection (BL-AG-04) phụ thuộc vào AgentHookParser → cũng broken
4. Mobile App push (TweetNaCl) không được trigger vì agent:statusChanged không emit

## Cần implement

```typescript
// src/main/agent/AgentHookParser.ts
export class AgentHookParser {
  private oscBuffer = ''
  
  parse(ptyId: string, rawData: string): AgentStatus | null {
    // OSC 133 patterns
    if (rawData.includes('\x1b]133;A')) return { status: 'running' }
    if (rawData.includes('\x1b]133;D;0')) return { status: 'idle', exitCode: 0 }
    if (/\x1b\]133;D;(\d+)/.test(rawData)) return { status: 'error' }
    
    // Text patterns
    if (/waiting for input/i.test(rawData)) return { status: 'waiting' }
    if (/task completed/i.test(rawData)) return { status: 'completed' }
    if (RATE_LIMIT_RE.test(rawData)) return { status: 'rate_limited' }
    
    return null
  }
}
```

## Liên quan đến luồng

- **BL-AG-05**: Monitor Status — AgentHookParser missing
- **BL-AG-04**: Switch Account — rate limit detection depends on parser
- **BL-AG-01**: Start Agent — session `INSERT` depends on parser detecting "idle" status

---

## ⏸ Fix Status: DEFERRED

**Reason:** AgentHookParser is complex feature requiring hook protocol implementation. Deferred to Phase 3 backlog.
