# BE-CV-TASK-036-06: Đồ thị đọc từ `impact` độ sâu 1 và Tarjan SCC lặp

**From Solution:** BE-CV-SOL-036-reading-order-and-risk
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/readingorder/reading_graph.go`, `strongly_connected.go` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-036-03 (kiểu `ChangedSymbol`)
**Status:** [x] DONE

---

## Context

Solution reading-order C1: cạnh lấy từ `levels[0]` của `impact` upstream (caller trực tiếp). Domain thuần, stdlib.

## Việc cần làm

1. `reading_graph.go`: `BuildReadingGraph(nodes []ChangedSymbol, impacts map[string]ImpactLevels) Graph`; nút chỉ gồm symbol thực thi (function/method/type; không test/doc/generated/deleted); với `S` có `impacts[S]`, mỗi `C` trong `levels[0].symbols` thuộc tập nút và `via ∈ {calls, accesses}` (so không phân biệt hoa thường) ⇒ cạnh `S → C`. Loại tự trỏ, khử trùng, sắp danh sách kề theo `key`. `Graph.HasEdgeData[key]` đánh dấu symbol đã có `impact` (để `reason: no-edges`).
2. `strongly_connected.go`: Tarjan **lặp** (stack tường minh) trả `[]SCC` theo thứ tự xác định (duyệt nút theo `key` tăng dần); `Condense(g) (dag, memberOf)`; SCC cỡ 1 không tự trỏ giữ nguyên nút.
3. Không đệ quy; không `map` iteration trong đường xác định (sắp khoá trước).

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/readingorder/ -run 'Graph|SCC' -race`: đồ thị rỗng; một nút; chuỗi; kim cương; vòng 2 và vòng 3; hai vòng nối nhau; tự gọi bị bỏ; cạnh tới symbol ngoài tập nút bị bỏ; `via:"imports"` bị bỏ; 2 000 nút dây chuyền không tràn stack; hoán vị đầu vào 100 lần ⇒ SCC giống hệt.

## Tiêu chí hoàn thành

- [x] Các ca trên xanh; thời gian 2 000 nút/10 000 cạnh < 100 ms (đo trong test benchmark, không là cổng chặn).
- [x] Không import ngoài stdlib.

## Rủi ro và lưu ý

- Chỉ ≤ K symbol có `HasEdgeData`; phần còn lại không có cạnh vào/ra đáng tin ⇒ đừng suy từ vắng cạnh rằng "không phụ thuộc".
