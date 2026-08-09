# TASK-FE2E-004 — Cập nhật test `LoginPage.test.tsx`

**Source Solution:** [SOL-FE2E-002](../solutions/SOL-FE2E-002-remove-paircode-fallback-from-login.md) §3.1
**Priority:** P0
**Loại:** Sửa/thêm test
**Depends on:** TASK-FE2E-002
**Estimated:** 15 phút
**Status:** ✅ DONE — 2026-08-09

---

## Context

```bash
cat frontend/src/renderer/src/web/login/__tests__/LoginPage.test.tsx
```

## Thay đổi cần thực hiện

**File:** `frontend/src/renderer/src/web/login/__tests__/LoginPage.test.tsx`

1. Xoá mọi test case render/tương tác với `PairCodeFallback` bên trong `LoginPage` (nếu có sẵn — kiểm tra bằng grep trước).
2. Thêm test mới:

```tsx
it('does not render the pairing code fallback', () => {
  render(<LoginPage availableProviders={[]} onLoginSuccess={vi.fn()} />)
  expect(screen.queryByLabelText(/pairing url or code/i)).not.toBeInTheDocument()
  expect(screen.queryByText(/pairing url or code/i)).not.toBeInTheDocument()
})
```

> [!IMPORTANT]
> Xác nhận import `render`/`screen` đã có sẵn trong file (Testing Library) trước khi thêm — không thêm import trùng.

## Verify

```bash
cd frontend
grep -n "PairCodeFallback" src/renderer/src/web/login/__tests__/LoginPage.test.tsx
# kỳ vọng: 0 kết quả (đã xoá test case cũ nếu có)

node_modules/.bin/vitest run --config config/vitest.config.ts src/renderer/src/web/login/__tests__/LoginPage.test.tsx
```

## Definition of Done

- [x] Không còn test case nào tham chiếu `PairCodeFallback` — cũng xoá luôn 2 `vi.mock()` (`web-pairing`, `web-runtime-environment`) không còn cần thiết
- [x] Test mới `'does not render the pairing code fallback'` pass
- [x] Toàn bộ test file pass: **8/8** (7 test cũ giữ nguyên + 1 test mới thay cho test PairCode fallback cũ đã xoá)
