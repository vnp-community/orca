# AG-CV-TASK-081-13: Ghi đè theo host `~/.orca/quality/profiles.json` (thêm/vô hiệu, `replace:true`)

**From Solution:** [AG-CV-SOL-081-quality-profile-catalog-and-preflight](../solutions/AG-CV-SOL-081-quality-profile-catalog-and-preflight.md) mục 5.2
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-profile-host-overrides.ts` (mới), `.test.ts` (mới)
**Depends on:** AG-CV-TASK-081-10, 081-12
**Status:** [ ] TODO

## Context

Hợp đồng §9.5: L2 do quản trị dev server; thư mục `0700`, tệp `0600`; chỉ thêm hoặc vô hiệu; ghi đè `argv` chỉ khi `replace:true` + lý do (log). `.orca/quality.json` trong repo là P1, **không làm** ở task này.

## Việc cần làm

1. `loadHostOverrides(home, deps?)`: đọc tệp (không có → rỗng); từ chối nếu quyền rộng hơn `0600` hoặc thư mục rộng hơn `0700` (POSIX; log, bỏ qua tệp, không ném); JSON lỗi → bỏ qua + log.
2. Dạng tệp: `{ version:1, add:[profile], disable:[id], replace:[{ id, reason, profile }] }`; `add` validate bằng schema (id không trùng builtin); `disable` chỉ id có thật; `replace` bắt buộc `reason` ≥ 10 ký tự, log `warn` kèm `definitionHash` cũ/mới.
3. `applyOverrides(catalog, overrides) → { catalog, source: Record<id, "builtin"|"host"> }`; `source` đi vào `listProfiles`.
4. Không đọc `.orca/quality.json`.

## Kiểm thử

Thư mục tạm làm `HOME`: tệp 0600 hợp lệ; 0644 bị bỏ qua; `add` trùng id; `disable` id lạ; `replace` thiếu lý do; `argv` có token cấm bị từ chối; không có tệp. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-profile-host-overrides.test.ts`.

## Tiêu chí hoàn thành

- [ ] Không ca nào cho phép đổi argv mà không `replace:true` + lý do.
- [ ] Windows: bỏ kiểm quyền, ghi chú.

## Rủi ro

Quản trị dev server vẫn có thể thêm profile chạy mã tuỳ ý: đúng thiết kế (người tin cậy), ghi rõ trong tài liệu.
