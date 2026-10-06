# BE-CV-TASK-022-04: `HeadProbe` (15 s, singleflight) và tính `stale` A/B

**From Solution:** BE-CV-SOL-022-snapshot-cache
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/usecase/head_probe.go` (mới) và test
**Depends on:** TASK-022-02, BE-CV-TASK-021-06
**Status:** [ ] TODO

---

## Context

Hai nguồn độ cũ (SOL-022 2.B); ưu tiên `headCommit` của phong bì (C7).

## Việc cần làm

1. `HeadProbe.Get(ctx, target)`: bộ nhớ theo `(tenant,binding)` TTL `CODEINTEL_HEAD_PROBE_TTL`; `Observe(headCommit, sources)` cập nhật từ mọi kết quả; hết hạn mới gọi `status`.
2. `Staleness.Evaluate(sources, head, status)` → `(stale, miss)` theo A/B.
3. `Invalidate` xoá mục.

## Kiểm thử

- `go test ./internal/usecase/ -run 'HeadProbe|Staleness' -race`; ca: commit lệch → stale; `pendingChanges>0` → stale; version/`indexedAt` đổi → miss; 20 gọi đồng thời → 1 `status`.

## Tiêu chí hoàn thành

- [ ] Nhánh A, B có test. - [ ] `stale` không tự kích hoạt thu thập lại.

## Rủi ro và lưu ý

- `pendingChanges` cần agent đúng (chưa kiểm).
