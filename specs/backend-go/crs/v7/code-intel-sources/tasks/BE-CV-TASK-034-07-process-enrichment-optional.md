# BE-CV-TASK-034-07: Làm giàu `related_processes` từ GitNexus (tuỳ chọn, không đổi bước)

**From Solution:** BE-CV-SOL-034-data-flow-model
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/enrich_flow_with_processes.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-034-05; BE-CV-SOL-021 (collector `codeintel.processes/process`)
**Status:** [ ] TODO

---

## Context

CR 2.3.5; README v7 mục 8 điểm 9. Agent contract §4.3 có `codeintel.processes`/`process`.

## Việc cần làm

1. Cổng `ProcessLookup{ProcessesContainingSymbol(ctx, RepoRef, []SymbolRef)}` (cài đặt qua collector; không có → `nil` implement no-op).
2. Với mỗi bước có `symbol`, tra Process chứa symbol; đưa `RelatedProcess{process_id, label, step_count, relation:"contains-symbol"}`; **không** thêm bước.
3. Lỗi collector (offline, `INDEX_MISSING`) → bỏ qua, ghi `stale`/cảnh báo, luồng không lỗi.
4. Mẫu Cypher CR §5 chưa chạy: **không** viết Cypher ở backend (agent chỉ nhận method hẹp, H3); dùng `codeintel.process`.

## Kiểm thử

`go test ... -run EnrichProcess` (chưa chạy) với lookup giả: có/không/lỗi.

## Tiêu chí hoàn thành

- [ ] Lookup lỗi không làm hỏng luồng.
- [ ] Không thêm bước.

## Rủi ro và lưu ý

- Không có đường nối kênh↔Process frontend (Q3).
