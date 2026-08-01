# BUG-AG-TM-004: `pty.spawn` response thiếu fields `handle`, `cols`, `rows`, `cwd` theo HLD

## Mức độ: MEDIUM

## Tóm tắt

HLD (terminal-create-flow.md — Response Path) chỉ rõ sau khi `pty.spawn` hoàn thành, Agent trả về:
```
{ ptyId, handle, cols, rows, cwd }
```

Nhưng `pty-handler.ts::spawn()` chỉ trả về `{ id }`:

```typescript
// Line 747
return { id }
```

## File liên quan

- [`src/relay/pty-handler.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/pty-handler.ts) — Line 747

## Code thực tế

```typescript
// Lines 747
return { id }
// Thiếu: handle, cols, rows, cwd
```

## Ảnh hưởng

1. Backend (`OrcaRuntimeService`) nhận `ptyId` là `result.id` — OK vì `id` được map thành `ptyId`.
2. Nhưng Backend không nhận được `cols`, `rows` thực tế mà PTY được spawn → không thể verify hoặc sync viewport.
3. `cwd` của PTY sau spawn không được xác nhận → nếu cwd resolve khác mong đợi, Backend không biết.
4. `handle` là terminal handle được pre-allocate từ Backend — nếu Agent không echo lại, Backend không thể verify alignment.

## Cách fix đề xuất

```typescript
// Trả về đủ fields theo HLD:
return {
  id,                         // ptyId
  handle: terminalHandle,     // pre-allocated handle từ env.ORCA_TERMINAL_HANDLE
  cols,
  rows,
  cwd                         // actual resolved cwd
}
```

## Liên quan đến luồng

- **BL-TM-01**: Response Path — `{ ptyId, handle, cols, rows, cwd }`.
- **BL-TM-03**: Scrollback Persistence — backend cần biết cols/rows để serialize snapshot đúng.

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** pty-handler.ts: spawn() returns {id, cols, rows, cwd, shell}. Return type explicitly typed.
