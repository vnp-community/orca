# BE-CV-TASK-024-04: Bộ gộp sự kiện, token bucket, throttle tiến trình, `event_id`

**From Solution:** BE-CV-SOL-024-event-distribution
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/usecase/{index_event_coalescer.go,index_event_identity.go}` (mới) và test
**Depends on:** TASK-024-01
**Status:** [x] DONE

---

## Context

Chống bão (2.D): gộp 2/10 s, 20 sự kiện/s/dev server, ≤ 2 tin/s/job, hàng đợi 256.

## Việc cần làm

1. `Coalescer` theo `(tenant,binding)`, huỷ cache ngay (callback) và phát sau yên lặng.
2. Token bucket theo dev server; vượt → một `resync`.
3. Throttle tiến trình: cho qua tin đầu/đổi stage/kết thúc, còn lại ≤ 2/s; ghi `reindex_jobs` ≤ 1/2 s.
4. `EventID(tenant,binding,tools,commit,indexedAt,kind)` = UUID v5.

## Kiểm thử

- `go test ./internal/usecase/ -run 'Coalesc|Bucket|EventID' -race` (đồng hồ giả): 30 tin/1 s → 1 phát; chuỗi liên tục → phát ≤ 10 s; cùng đầu vào → cùng UUID; 100 tin/s tiến trình → ≤ 2/s + stage + kết thúc.

## Tiêu chí hoàn thành

- [x] Xác định. - [x] Giới hạn đúng.

## Rủi ro và lưu ý

- Thời gian dùng đồng hồ tiêm vào.
