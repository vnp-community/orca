# TASK-FE-HLD-005 — Làm rõ phạm vi chuẩn #3 (IPlatformServices) trong docs

**Solution:** [SOLUTION-FE-HLD-005](../solutions/SOLUTION-FE-HLD-005-iplatformservices-scope.md)
**Bug:** [BUG-FE-HLD-005](../BUG-FE-HLD-005-iplatformservices-electron-adapter-missing.md)
**File:** `docs/features/README.md`
**Estimated:** 10 phút
**Status:** ✅ DONE — 2026-08-09

---

## Mục tiêu

Sửa câu chữ chuẩn #3 trong "Coding Standards cho v5.0" để phản ánh đúng phạm vi thật (5 module v5.0 mới, không áp hồi tố cho toàn bộ `src/main`) — dựa trên xác nhận từ [tdd/v5/03-runtime-client-layer.md](../../../tdd/v5/03-runtime-client-layer.md) rằng `platform/adapters/` từ đầu chỉ thiết kế cho web target.

---

## Context

```bash
grep -n "IPlatformServices" docs/features/README.md
```

---

## Thay đổi cần thực hiện

**File:** `docs/features/README.md`

**TÌM:**
```
3. **IPlatformServices**: không import electron trực tiếp
```

**THAY BẰNG:**
```
3. **IPlatformServices**: code v5.0 mới (Profile/Project/AI Provider/Workflow/Task —
   `src/main/profile`, `project`, `ai-providers`, `workflow`, `task`) không import
   electron trực tiếp. Áp dụng cho code **mới** thuộc các domain này; không áp dụng
   hồi tố cho code Electron/desktop main-process đã tồn tại trước restructure_v1
   (xem `specs/frontend/crs/v1/restructure_v1/README.md` — nguyên tắc "Additive only").
```

---

## Verify

```bash
grep -n "IPlatformServices" docs/features/README.md
# Xác nhận câu mới đã thay thế, không còn câu cũ mơ hồ
```

---

## Definition of Done

- [x] Câu chữ chuẩn #3 đã cập nhật, nêu rõ phạm vi 5 module + tham chiếu nguyên tắc "Additive only"
- [x] Không có thay đổi code nào khác trong task này (đây là fix doc thuần)

## Kết quả thực thi

Đã sửa `docs/features/README.md` đúng như đề xuất. Xác nhận qua audit trước đó (và grep lại): `src/main/profile`, `project`, `task`, `workflow` tồn tại và sạch (0 import electron); `ai-providers` **không tồn tại** ở `frontend/src/main` (chỉ có ở `backend/`) — giữ nguyên đúng như solution đã ghi nhận, không thêm path không tồn tại vào câu doc.
