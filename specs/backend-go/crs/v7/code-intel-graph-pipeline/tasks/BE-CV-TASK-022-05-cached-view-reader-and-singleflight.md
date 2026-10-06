# BE-CV-TASK-022-05: `CachedViewReader` (hit/miss/offline), singleflight tách ngữ cảnh, giới hạn 20 s

**From Solution:** BE-CV-SOL-022-snapshot-cache
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/usecase/cached_view_reader.go` (mới) và test
**Depends on:** TASK-022-03, TASK-022-04
**Status:** [ ] TODO

---

## Context

Luồng C của solution; PQ-13 (20 s + hoàn tất nền).

## Việc cần làm

1. Cài luồng 1–7 (mục 2.C) bọc `ViewReader`.
2. `singleflight.DoChan` với `context.WithoutCancel` + `CODEINTEL_COLLECT_TIMEOUT`; chờ tối đa 20 s theo ctx người gọi → `CODEINTEL_TIMEOUT | {"retryAfterMs":3000,"inProgress":true}`.
3. Phục vụ snapshot cũ chỉ cho `DEV_SERVER_OFFLINE`/`TIMEOUT`.
4. `STATUS`/`SYMBOL` không xuống DB; LRU `SYMBOL`.

## Kiểm thử

- `go test ./internal/usecase/ -run CachedViewReader -race`; hit không gọi agent; 50 yêu cầu → 1 thu thập; người dẫn huỷ; offline có/không snapshot; `INDEX_MISSING` không bị che; quá 20 s.

## Tiêu chí hoàn thành

- [ ] Các ca trên xanh. - [ ] Không ghi `graph_snapshots` cho STATUS/SYMBOL.

## Rủi ro và lưu ý

- Hai replica có thể cùng thu thập (chấp nhận).
