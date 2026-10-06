# AG-CV-TASK-083-07: Giai đoạn B: báo cáo `coverage-final.json` của vitest (chỉ sau khi duyệt O12)

**From Solution:** [AG-CV-SOL-083-coverage-collection](../solutions/AG-CV-SOL-083-coverage-collection.md) mục 5.1
**Priority:** P2
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-coverage-vitest-report.ts` (mới), `.test.ts`; profile `coverage-ts`
**Depends on:** AG-CV-TASK-083-04; **duyệt `@vitest/coverage-v8` (O12) — cần duyệt, không thêm phụ thuộc trong task này**
**Status:** [ ] TODO

## Context

Repo không có `@vitest/coverage-v8` (đã `ls node_modules/@vitest`, theo CR). Nếu chưa duyệt, task chỉ làm phần parser (hàm thuần trên fixture) và để profile `enabled:false`.

## Việc cần làm

1. Parser `coverage-final.json` (istanbul: `statementMap`, `s`, `fnMap`, `f`) → cùng kiểu khối với task 02; đường dẫn tuyệt đối → cắt `workspaceRoot`; ngoài → bỏ.
2. Profile `coverage-ts` (mặc định `enabled:false` tới khi duyệt): `{bin:vitest} run --coverage.enabled --coverage.provider=v8 --coverage.reporter=json --coverage.reportsDirectory={tmp:<pkg>}` theo gói `agent`, `frontend`, `desktop`.
3. Preflight: thiếu `@vitest/coverage-v8` (`node_modules/@vitest/coverage-v8` hoặc `require.resolve`) → `missing.reason:"coverage_provider_missing"`; khi chạy → `ENV_NOT_READY` (không ra số 0). Không bao giờ `pnpm add`.
4. Phiên bản provider phải bằng `vitest` (4.1.5): sai → `tool_incompatible`.

## Kiểm thử

Fixture `coverage-final.json` nhỏ (tạo tay, ghi rõ đã tạo tay vì provider chưa được duyệt); ca tệp ngoài workspace; preflight thiếu provider. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-coverage-vitest-report.test.ts`.

## Tiêu chí hoàn thành

- [ ] Không thay đổi `package.json`/`pnpm-lock.yaml`.
- [ ] Thiếu provider → lỗi rõ, không số 0.

## Rủi ro

Cần quyết định duyệt (O12/O-9); vitest 4 + v8 + alias có thể sai source map (chưa thử); tệp không test thiếu nếu không có `coverage.include`.
