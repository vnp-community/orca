# BUG-FE-TM-004: Default viewport hardcode `cols: 80, rows: 24` — không match HLD quy định `cols: 120, rows: 40`

## Mức độ: LOW

## Tóm tắt

HLD (terminal-create-flow.md §Bước 3) chỉ rõ Backend spawn PTY với:
```
ptyController.spawn({
  cols: 120, rows: 40,
  ...
})
```

Nhưng Browser transport sử dụng default `cols: 80, rows: 24` nếu options không chỉ định:

```typescript
// Lines 669-672
desiredViewport = {
  cols: options.cols ?? 80,  // ← HLD yêu cầu 120
  rows: options.rows ?? 24   // ← HLD yêu cầu 40
}
```

## File liên quan

- [`src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts) — Lines 669-672, 712-715, 230-233

## Code

```typescript
// Xuất hiện ở 3 nơi:
// Lines 669-672 (connect)
desiredViewport = { cols: options.cols ?? 80, rows: options.rows ?? 24 }

// Lines 712-715 (attach)
desiredViewport = { cols: options.cols ?? 80, rows: options.rows ?? 24 }

// Lines 230-233 (attachHostSessionMirror)
desiredViewport = { cols: options.cols ?? 80, rows: options.rows ?? 24 }
```

## Phân tích

Trong thực tế, Browser sẽ gọi `resize()` ngay sau khi PTY được spawn để sync kích thước thực của pane. Vì vậy default 80x24 chỉ tồn tại trong một khoảnh khắc ngắn.

Tuy nhiên, nếu có race condition hoặc resize call bị miss, PTY sẽ chạy với 80x24 thay vì kích thước thực — gây layout vấn đề cho các ứng dụng TUI (vim, tmux, htop).

## Cách fix đề xuất

```typescript
// Align với HLD default, hoặc tốt hơn là lấy từ pane container size
desiredViewport = {
  cols: options.cols ?? 120,  // HLD default
  rows: options.rows ?? 40    // HLD default
}
```

## Liên quan đến luồng

- **BR-TM-02**: Resize propagation phải đồng bộ viewport giữa Browser và PTY.
- **BR-TM-06**: Minimum size 80 cols × 10 rows.
