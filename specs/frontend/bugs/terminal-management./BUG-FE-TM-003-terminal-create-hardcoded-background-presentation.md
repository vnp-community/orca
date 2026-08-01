# BUG-FE-TM-003: `terminal.create` hardcode `presentation: 'background'` — bỏ qua user intent khi mở terminal focused

## Mức độ: LOW

## Tóm tắt

Trong `remote-runtime-pty-transport.ts::connect()`, `presentation` được hardcode là `'background'` bất kể options từ caller:

```typescript
// Lines 653-657
focus: false,
// Why: this transport is backing an already-mounted renderer pane;
// activation here is local state, not permission for remote UI reveal.
presentation: 'background',
...(activate === true ? { activate: true } : {})
```

Comment giải thích "transport đã mount renderer pane nên không cần reveal". Nhưng HLD mô tả:
```
presentation: 'background' | 'focused',
focus: boolean
```
phải được gửi đúng theo intent của caller.

## File liên quan

- [`src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts) — Lines 640-658

## Code

```typescript
const created = await callRuntime<{ terminal: RuntimeTerminalCreate }>('terminal.create', {
  worktree: toRuntimeTerminalWorktreeSelector(worktreeId),
  // ...
  focus: false,                // ← luôn false, không phụ thuộc opts
  presentation: 'background', // ← luôn background
  ...(activate === true ? { activate: true } : {})
})
```

## Phân tích

Comment trong code giải thích lý do: transport đã mount renderer pane (tab đã mở), nên không cần Backend "reveal" thêm tab nữa — activation là local state. Logic này **có thể đúng** trong Electron mode.

Tuy nhiên trong Web Server mode (headless, không có renderer window), `presentation: 'focused'` sẽ trigger `notifier.revealTerminalSession()` → tạo ra terminal tab trên client khác. Nếu Browser luôn gửi `'background'`, sẽ không bao giờ trigger reveal.

## Ảnh hưởng

- Nếu Electron renderer gọi transport với intent "open focused terminal" → Backend nhận `'background'` → không reveal trên mobile companion hoặc other clients.
- `publishPtyBackedMobileSessionTerminal` với `selectIfNoActiveTab` sẽ không được set đúng.
- **Severity thấp** vì trong Electron mode, reveal được handle bởi renderer trực tiếp.

## Liên quan đến luồng

- **BL-TM-01**: Response Path — `notifier.revealTerminalSession()`.
- **BR-TM**: presentation mode.
