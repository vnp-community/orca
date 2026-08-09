# TASK-FE2E-002 — Bỏ `PairCodeFallback` khỏi `LoginPage.tsx`

**Source Solution:** [SOL-FE2E-002](../solutions/SOL-FE2E-002-remove-paircode-fallback-from-login.md)
**Priority:** P0
**Loại:** Sửa file hiện có
**Depends on:** TASK-FE2E-001
**Estimated:** 10 phút
**Status:** ✅ DONE — 2026-08-09

---

## Context

```bash
cat frontend/src/renderer/src/web/login/LoginPage.tsx
```

## Thay đổi cần thực hiện

**File:** `frontend/src/renderer/src/web/login/LoginPage.tsx`

**TÌM:**
```tsx
import { PairCodeFallback } from './PairCodeFallback'
```
và
```tsx
        {/* PairCode backward-compat section */}
        <div className="login-divider" aria-hidden="true">
          or
        </div>
        <PairCodeFallback />
      </main>
```

**THAY BẰNG:** xoá dòng import; xoá 3 dòng JSX (`login-divider` + `<PairCodeFallback />`), giữ nguyên thẻ đóng `</main>`.

> [!IMPORTANT]
> Không sửa gì khác trong file — `LoginForm`, `SsoButton` giữ nguyên 100%.

## Verify

```bash
cd frontend
node_modules/.bin/vitest run --config config/vitest.config.ts src/renderer/src/web/login/__tests__/LoginPage.test.tsx
grep -n "PairCodeFallback" src/renderer/src/web/login/LoginPage.tsx
# kỳ vọng: 0 kết quả
```

## Definition of Done

- [x] Import + JSX của `PairCodeFallback` đã xoá khỏi `LoginPage.tsx`
- [x] `LoginForm`/`SsoButton` không đổi
- [x] Test hiện có của `LoginPage.test.tsx` vẫn pass (cập nhật thêm ở TASK-FE2E-004) — 8/8 pass

## Kết quả thực thi

Ngoài đúng diff của solution, đã thêm 1 đoạn comment "Why" ở đầu `LoginPage.tsx` giải thích lý do bỏ (điều kiện `sessionUser === null` khiến pairing luôn thừa) — không nằm trong plan gốc nhưng phù hợp quy ước "Document the Why" của `AGENTS.md`.
