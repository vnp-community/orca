# BE-CV-TASK-020-06: Hợp nhất symbol và cạnh từ hai nguồn (`MergeSymbolSets`, `MergeEdges`)

**From Solution:** BE-CV-SOL-020-canonical-graph-model
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/symbol_merge.go` (mới), `symbol_merge_test.go` (mới)
**Depends on:** TASK-020-04
**Status:** [x] DONE

---

## Context

CR mục 2.4 "Khớp hai nguồn". Agent trả tối đa hai nguồn trong `sources[]` (agent §2.2); backend hợp nhất. Độ chính xác khớp phụ chưa đo (CR-CV-070 đo).

## Việc cần làm

1. `MergeSymbolSets(a, b []SymbolRef) (merged []SymbolRef, report MergeReport)`; `MergeReport{PrimaryMatches, SecondaryMatches, Unmatched int; Disagreements []string}` để tính tỉ lệ khớp cho metric/fixture.
2. Khớp chính theo `Key`. Khớp phụ chỉ khi khớp chính thất bại, mỗi phía đúng một ứng viên, cùng `FilePath`, cùng `Name`, cùng nhóm kind (callable = function|method|value; type riêng), `|StartLine chuẩn hoá chênh| ≤ 2`.
3. Khi khớp: một `SymbolRef`, giữ cả `GitNexusID` và `CodeGraphID`, `StartLine/EndLine` ưu tiên GitNexus, `Signature/Language` lấy CodeGraph, `NativeKind` ghép `Function|function`; `CodeGraphID` không vào khoá. Hai nguồn cùng `Key` mà `StartLine` lệch > 2 → thêm `sources_disagree` vào `Disagreements`.
4. `Route` không hợp nhất: giữ hai `RouteNode` riêng theo thẻ nguồn (hàm `MergeRoutes` chỉ nối và khử trùng theo `(source,id)`).
5. `MergeEdges(a, b []SymbolEdge) []SymbolEdge`: khoá `(FromKey,ToKey,Kind)`, `Sources` hợp, `Confidence` lấy max; sắp xếp xác định `(FromKey,ToKey,Kind)` để ETag ổn định (SOL-022).
6. Đầu ra xác định: cùng đầu vào (bất kể thứ tự nguồn) cho cùng kết quả; sắp theo `Key`.

## Kiểm thử

- `cd backend-go/services/code-intel-service && go test ./internal/domain/ -run Merge -race -count=2`.
- Ca: cặp `prefetchManagedWorktreeCreateBase` (dòng 1041/1042) thành một nút có hai id; hai ứng viên cùng tên cùng tệp ở một phía → không khớp phụ; lệch dòng 3 → `sources_disagree`; cạnh trùng từ hai nguồn giữ một; hoán đổi `a,b` ra cùng kết quả.
- Benchmark nhỏ: hợp nhất 1 500 nút × 2 nguồn không cấp phát vượt O(n) (kiểm `testing.AllocsPerRun`, ngưỡng ghi lại làm đường cơ sở, không chặn CI).

## Tiêu chí hoàn thành

- [x] Mọi ca ở trên xanh và xác định (`-count=2`).
- [x] `MergeReport` đủ để SOL-021 đẩy metric `codeintel_symbol_merge_total{kind=primary|secondary|unmatched}`.
- [x] Không import proto/DB.

## Rủi ro và lưu ý

- Khớp phụ có thể gộp nhầm các hàm cùng tên trong một tệp (ngưỡng ±2 dòng giảm nhưng không loại bỏ); ghi tỉ lệ trên fixture làm đường cơ sở.
- Số nút lớn: nếu 2 nguồn × 1 500 nút gây chậm, chỉ mục theo `(FilePath, Name)` thay vì lặp lồng.
