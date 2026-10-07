# AG-CV-TASK-083-05: Profile `coverage-go`, bước theo module, bộ thu coverage sau run

**From Solution:** [AG-CV-SOL-083-coverage-collection](../solutions/AG-CV-SOL-083-coverage-collection.md) mục 5.2
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-coverage-collector.ts` (mới), điền profile trong `quality-profile-catalog.ts` qua `registerBuiltinProfiles`, `.test.ts`
**Depends on:** AG-CV-TASK-083-02..04, AG-CV-TASK-081-12, 081-15, 081-06
**Status:** [x] DONE

## Context

Hợp đồng §5.6 lệnh Go; bước chạy qua executor (AG-CV-SOL-081-A) nên có giới hạn, huỷ cây, env sạch.

## Việc cần làm

1. Đăng ký profile `coverage-go` (5.2), `perModule`, `go-modules`, `requires go`; `-coverprofile={tmp:<module>.out}`; không `-coverpkg`, không `-tags`.
2. `collectGoCoverage(run, steps)`: sau khi mọi bước coverage xong, đọc `<tmp>/<module>.out` từng module → task 02/03/04 → lưu vào kho kết quả (`coverageByRun`), TTL như kết quả (1 h).
3. `scope=changed`: module do planner chọn; `diff.partial:true`, `diff.modules` = module đã đo; `scope=worktree`: mọi module, `partial:false`. `scope` không có `base` cho diff → chỉ `totals` (diff `null`, `reason:"base_required"`).
4. Bước thất bại/timeout: module đó không vào `modules`, `diff.partial:true`.
5. Không ghi tệp nào trong worktree (`git status --porcelain` trước/sau bằng nhau).

## Kiểm thử

Executor giả ghi tệp `.out` từ fixture; ca một module fail; scope changed 1 module; kiểm `git status` không đổi bằng repo tạm thật + `go` thật nếu có (`it.skipIf(!hasGo)`). Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-coverage-collector.test.ts`.

## Tiêu chí hoàn thành

- [x] Không có tệp mới trong worktree sau run.
- [x] Profile qua kiểm token cấm của catalog.

## Rủi ro

Thời gian/RAM của 21 module chưa đo; `heavy:true` + cổng nặng 1.
