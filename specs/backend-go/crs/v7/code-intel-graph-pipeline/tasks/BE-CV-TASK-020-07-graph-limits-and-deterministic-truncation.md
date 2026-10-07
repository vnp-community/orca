# BE-CV-TASK-020-07: Giới hạn kích thước và cắt xác định (`graph_limits.go`), kiểu đồ thị domain

**From Solution:** BE-CV-SOL-020-canonical-graph-model
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/graph_limits.go`, `architecture_graph.go`, `module_graph.go`, `symbol_graph.go`, `flow_graph.go`, `impact_graph.go`, `route_map.go`, `symbol_detail.go`, `result_meta.go`, `tool_index_status.go` (mới) và test
**Depends on:** TASK-020-04
**Status:** [x] DONE

---

## Context

Trần mỗi response ≤ 2 MiB, `GetSymbol` ≤ 320 KiB (PQ-14, khác 3 MiB của CR). Cắt phải **xác định** để ETag của SOL-022 ổn định. Domain không import proto nên kích thước đo qua hàm `SizeFunc` do adapter cấp.

## Việc cần làm

1. Khai báo struct domain tương ứng proto (SOL-020 mục 2.C) và `ViewKind` kèm `CacheName()`/`ParseViewKind` (mục 2.D).
2. `graph_limits.go`: hằng giới hạn (solution 2.C); `type SizeFunc func(v any) int`; `LimitArchitecture`, `LimitModuleGraph`, `LimitSymbolGraph`, `LimitFlowGraph`, `LimitImpactGraph`, `LimitRouteMap` mỗi hàm trả `(kết quả, truncated bool, totalBefore int64)`.
3. Thứ tự cắt: cụm theo `SymbolCount` giảm dần rồi `ID`; cạnh theo `Weight` giảm dần rồi `(From,To)`; nút `SymbolGraph` theo khoảng cách tới `Center` tăng dần rồi bậc giảm dần rồi `Key` (tâm không bao giờ bị cắt); loại cạnh mồ côi; `ImpactGraph` theo `depth` rồi `direct` rồi `Key` ≤ 300 nút tổng.
4. Sau cắt số lượng: nếu `SizeFunc(result) > ResponseMaxBytes` thì cắt tiếp cùng thứ tự theo bước 10% số nút cho tới khi vừa; nếu vẫn vượt khi chỉ còn tâm → trả lỗi `CODEINTEL_RESPONSE_TOO_LARGE` kèm `{bytes, limit}` (mã do gateway cũng sinh, PQ-03(4); ở đây chỉ trả sentinel `ErrResponseTooLarge`).
5. `ViewKind` song ánh test (mục 2.D).

## Kiểm thử

- `cd backend-go/services/code-intel-service && go test ./internal/domain/ -run 'Limit|ViewKind' -race -count=2`.
- Ca: 5 000 nút `SymbolGraph` → 1 500 nút, `truncated=true`, `totalBefore=5000`, tâm còn, chạy hai lần cho cùng thứ tự; 9 039 cụm → 500; `SizeFunc` giả vượt 2 MiB → cắt tiếp; `SymbolDetail` chỉ giới hạn 320 KiB (`SymbolResponseMaxBytes`).
- Property: cạnh sau cắt luôn có cả hai đầu mút trong tập nút.

## Tiêu chí hoàn thành

- [x] Mọi `Limit*` trả đúng số lượng, `truncated`, `totalBefore`, xác định.
- [x] `ViewKind.CacheName()` khớp 21 chuỗi hợp đồng §4 (`status` và `symbol` có tên nhưng SOL-022 không lưu).
- [x] Không có import proto/DB.

## Rủi ro và lưu ý

- 2 MiB là ngân sách hợp đồng; chưa đo kích thước thực của `SymbolGraph` 1 500 nút/4 000 cạnh.
- Ngưỡng bước cắt 10% là lựa chọn của task, đổi được nếu đo cho thấy lặp nhiều.
