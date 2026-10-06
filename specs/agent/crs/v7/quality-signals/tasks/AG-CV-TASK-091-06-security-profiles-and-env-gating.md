# AG-CV-TASK-091-06: Đăng ký 4 profile bảo mật/phụ thuộc, preflight công cụ và chính sách mạng

**From Solution:** [AG-CV-SOL-091-security-and-dependency-profiles](../solutions/AG-CV-SOL-091-security-and-dependency-profiles.md) mục 5.3
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-security-profiles.ts` (mới), `.test.ts`; catalog qua `registerBuiltinProfiles`; preflight (081-16)
**Depends on:** AG-CV-TASK-091-03, 091-04, AG-CV-TASK-081-12, 081-16
**Status:** [ ] TODO

## Context

PQ-01(4): agent luôn liệt kê. Công cụ ngoài `enabled:false` tới khi duyệt (O12) — profile vẫn có trong catalog nhưng `ready:false`.

## Việc cần làm

1. Đăng ký 4 profile (5.3); `security-secrets-diff` và `dependency-diff` `ready` mặc định; `security-go-vuln`/`security-deps-osv` cần `govulncheck`/`osv-scanner` (`binary_missing`), mạng.
2. Preflight bổ sung: `ORCA_QUALITY_NETWORK=deny` + `network:true` → `network_policy`; build lỗi module Go (thiếu mod cache) → `go_modules_unavailable` (phân loại sau chạy từ stderr).
3. `scope: changed-*`: planner chọn module/lockfile có tệp đổi; không có → `skipped (scope_unchanged)`.
4. `scope:"worktree"` quét toàn bộ (quyền do backend quyết).
5. Hạn mức: `heavy:true` cho `govulncheck`.

## Kiểm thử

Catalog test: 4 id có, qua kiểm token cấm, không `install|curl`; preflight: thiếu công cụ, `deny`; planner: không tệp phụ thuộc đổi → skipped. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-security-profiles.test.ts`.

## Tiêu chí hoàn thành

- [ ] Không lệnh cài nào; `ENV_NOT_READY` có `missingTools[]`.

## Rủi ro

Cờ CLI của công cụ ngoài chưa xác nhận (task 01); profile đó đặt `enabled:false` cho tới khi xác nhận.
