# BUG-AG-TM-003: `pty.spawn` shell resolve không từ `env.SHELL` — dùng `resolveDefaultShell()` bypass $SHELL env

## Mức độ: MEDIUM

## Tóm tắt

HLD mô tả (BR-TM-04): "Shell path resolve từ `$SHELL` env trên Dev Server, không hardcode". Nhưng trong `pty-handler.ts::spawn()`, shell được resolve bằng `resolveDefaultShell()` — **không phải từ `env.SHELL` do Backend inject**.

## File liên quan

- [`src/relay/pty-handler.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/pty-handler.ts) — Lines 626-629

## Code thực tế

```typescript
// Lines 626-629
const shellOverride =
  typeof params.shellOverride === 'string' ? params.shellOverride.trim() : ''
const resolvedShellOverride = resolvePtyShellOverride(shellOverride)
const shell = resolvedShellOverride || resolveDefaultShell()
// ↑ resolveDefaultShell() đọc process.env.SHELL của relay process
//   KHÔNG phải env.SHELL từ params (do Backend inject)
```

## Vấn đề

1. Backend inject `env: { SHELL: '/usr/local/bin/zsh', ... }` trong `pty.spawn` params.
2. Nhưng `pty-handler.ts` không đọc `env.SHELL` để resolve shell path.
3. `resolveDefaultShell()` đọc `process.env.SHELL` của relay process (là shell của user chạy relay, không phải shell mong muốn từ config).
4. Theo HLD: "shell = env.SHELL ?? '/bin/bash'; resolveDefaultShell(platform) → verify binary exists".

## Code đúng theo HLD

```typescript
// Đúng theo HLD:
const shellFromEnv = (params.env as Record<string, string>)?.SHELL
const shell = resolvedShellOverride 
  || shellFromEnv 
  || resolveDefaultShell()
```

## Ảnh hưởng

- Nếu user config shell `/usr/bin/fish` qua profile, Backend sẽ inject `env.SHELL = /usr/bin/fish` nhưng agent vẫn spawn `/bin/bash` hoặc `/bin/zsh` theo relay process env.
- Shell integration (OSC 133) có thể không hoạt động với shell không đúng.
- **BR-TM-04** violated.

## Liên quan đến luồng

- **BL-TM-01**: Bước 5 — SHELL RESOLVE (BR-TM-04).

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** pty-handler.ts: Shell resolution reads params.env.SHELL first, falls back to process.env.SHELL, then /bin/sh.
