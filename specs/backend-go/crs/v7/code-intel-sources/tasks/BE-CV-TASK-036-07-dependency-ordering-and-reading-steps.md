# BE-CV-TASK-036-07: Kahn có ưu tiên, gộp bước theo tệp, `stepKey`, `reason`, bước generated/overflow

**From Solution:** BE-CV-SOL-036-reading-order-and-risk
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/readingorder/{layer_rank.go, dependency_ordering.go, reading_steps.go}` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-036-06
**Status:** [x] DONE

---

## Context

Solution reading-order §2.B bước 3–8; PQ-31 (`stepKey` là khoá tiến độ của CR-052).

## Việc cần làm

1. `layer_rank.go`: `LayerOf(path string, componentLayer string) (name string, rank int)`: `.proto` và `backend-go/services/*/migrations/**` ⇒ `contract`(0); `internal/domain` ⇒ 1; `internal/usecase` ⇒ 2; `internal/adapter` ⇒ 3; test ⇒ 5; doc/cấu hình ⇒ 6; còn lại `edge`(4).
2. `dependency_ordering.go`: `Order(dag, rankOf, positionOf) []NodeID`: Kahn với hàng đợi ưu tiên (`container/heap`) khoá `(layerRank, path, startLine, key)`; nút SCC nén phát một lần, thành viên sắp `(path, startLine)`. Tiền nhiệm trước (callee trước caller).
3. `reading_steps.go`: `BuildReadingSteps(order, files, impacts, tests) []ReadingStep`: gộp symbol liên tiếp cùng tệp; tệp không symbol thành bước riêng theo `layerRank`; tệp generated gộp **một** bước cuối `generated`; `stepKey = hex(sha256(file+"\n"+join(sorted keys,"\n")))[:16]` (bước không symbol: `sha256(file)[:16]`); `reason` theo bảng (`contract|dependency-of|leaf|cycle|no-edges|test|doc|generated|overflow`; `dependency-of` kèm `reasonParams.caller`); `dependsOn`, `tests` (≤ 10), `hunks` từ `changedFiles`; ≤ 300 bước, phần dư gom vào bước `overflow` (`reasonParams.count`).
4. Đánh số `n` từ 1 sau cùng; `stepKey` **không** phụ thuộc `n`.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/readingorder/... -race`: A→B→C ra C,B,A; vòng A↔B với C gọi A ra {A,B} liền nhau và đứng trước C; hai nút sẵn sàng khác tầng ⇒ tầng thấp trước; symbol không dữ liệu cạnh ⇒ `no-edges`; tệp proto đứng trước domain; generated luôn cuối; 400 bước ⇒ 300 + `overflow`; `stepKey` không đổi khi thêm symbol ở tệp khác hoặc đổi thứ tự bước; 100 lần chạy ⇒ byte-by-byte giống; test property: mọi cạnh `S→C` trong DAG thoả `pos(S) < pos(C)` trừ khi cùng SCC.

## Tiêu chí hoàn thành

- [x] Tiêu chí thứ tự/`stepKey`/xác định của §9 đạt.
- [x] `reason` luôn thuộc tập hợp lệ; `overflow` đúng số lượng.

## Rủi ro và lưu ý

- Thêm symbol vào tệp đã xem làm đổi `stepKey` của tệp đó (mất trạng thái "đã xem" cho bước này); chấp nhận theo PQ-31, báo `FE-CV-SOL-052`.
