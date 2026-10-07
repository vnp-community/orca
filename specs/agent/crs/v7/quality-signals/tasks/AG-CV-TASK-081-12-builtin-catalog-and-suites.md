# AG-CV-TASK-081-12: Catalog tích hợp cho Orca, suite, API đăng ký

**From Solution:** [AG-CV-SOL-081-quality-profile-catalog-and-preflight](../solutions/AG-CV-SOL-081-quality-profile-catalog-and-preflight.md) mục 5.3
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-profile-catalog.ts` (mới), `.test.ts` (mới)
**Depends on:** AG-CV-TASK-081-10, 081-11
**Status:** [x] DONE

## Context

Hợp đồng §5.1 chốt catalog mặc định; solution chốt bảng ở 5.3. Profile của 083/084/091 đăng ký qua API (không sửa file này khi thêm).

## Việc cần làm

1. `BUILTIN_PROFILES: readonly QualityCheckProfile[]` và `BUILTIN_SUITES` (`fast`, `standard`, `full`) đúng bảng 5.3; mã TS (không YAML).
2. `registerBuiltinProfiles(extra: readonly QualityCheckProfile[])` (gọi lúc khởi tạo module; id trùng → ném) để AG-CV-SOL-083/084/091 cấp định nghĩa.
3. `getCatalog(): { profiles, suites }` đã qua `validateProfile`; profile `enabled:false` (vd `ts-typecheck-agent` nếu task 11 thấy không chạy) không liệt kê.
4. Mọi `requires` dùng đường dẫn đã `ls`.

## Kiểm thử

Test: mọi profile hợp lệ theo schema; id duy nhất; mỗi `requires.file` tồn tại trong repo thật (đọc đĩa tương đối `/opt/repos/orca` qua `path` từ `import.meta`); quét argv: không chứa `install`, `rebuild`, `mod`+`download`, `--runtime=`, `--init`, `--prune`, `analyze`, `clean`; suite chỉ tham chiếu id có thật. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-profile-catalog.test.ts`.

## Tiêu chí hoàn thành

- [x] Catalog khớp hợp đồng §5.1; test cấm token pass.
- [x] Thêm profile mới không cần sửa file này.

## Rủi ro

Test đọc repo thật làm test gắn với cấu trúc thư mục; nếu repo tái cấu trúc test báo đúng chỗ cần đổi catalog (có chủ ý).
