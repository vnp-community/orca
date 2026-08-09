# TASK-FE2E-009 — Thêm comment giải thích `canGeneratePairingUrl={!isWebClient}`

**Source Solution:** [SOL-FE2E-004](../solutions/SOL-FE2E-004-share-link-decision.md) §3
**Priority:** P2 — không khẩn cấp, thuần tài liệu hoá trong code
**Loại:** Sửa comment (không đổi logic)
**Depends on:** TASK-FE2E-001
**Estimated:** 5 phút
**Status:** ✅ DONE — 2026-08-09

---

## Context

```bash
grep -n "canGeneratePairingUrl" frontend/src/renderer/src/components/settings/Settings.tsx
```

## Thay đổi cần thực hiện

**File:** `frontend/src/renderer/src/components/settings/Settings.tsx`

**TÌM:**
```tsx
canGeneratePairingUrl={!isWebClient}
```

**THAY BẰNG:**
```tsx
// Why: "Share this Orca server" only makes sense from the app that OWNS a
// runtime to advertise — Desktop only. isWebClient is true for both the
// multi-user backend path AND the bare Desktop-pair-code path (same
// web-index.html bundle, see docs/crs/v2/frontend-e2ee/), so this hides the
// section in both, not just the multi-user one. Confirmed via
// specs/frontend/crs/frontend-e2ee/solutions/SOL-FE2E-004.
canGeneratePairingUrl={!isWebClient}
```

> [!IMPORTANT]
> Chỉ thêm comment — không đổi giá trị/logic dòng này.

## Verify

```bash
grep -n "SOL-FE2E-004" frontend/src/renderer/src/components/settings/Settings.tsx
```

## Definition of Done

- [x] Comment thêm đúng vị trí, không đổi behavior
- [x] Không có thay đổi nào khác trong file — chỉ thêm 6 dòng comment trước `canGeneratePairingUrl={!isWebClient}`
