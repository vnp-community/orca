# BE-CV-TASK-034-08: Xuất `SequenceModel` và `DfdModel` (hàm thuần, hai mức `detail`)

**From Solution:** BE-CV-SOL-034-data-flow-model
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/export_flow_models.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-034-03
**Status:** [x] DONE

---

## Context

Solution 2.D.

## Việc cần làm

1. `ExportSequence(flow, detail)`: participants theo thứ tự xuất hiện; `SeqMessage` mỗi bước; `dashed_return` cho `rpc` đồng bộ; `event` bất đồng bộ; `note` cho `unimplemented`/suy luận.
2. `ExportDfd(flow, detail)`: node ui/gateway/service/store/queue/external; cạnh gộp `(from,to,kind)`, `label` = danh sách RPC/kênh, `data` = tên message proto/bảng, `count`.
3. `detail=service` gộp component cùng container, ẩn bước nội bộ.
4. Ngân sách ≤ 60 bước/40 participant → `truncated` + `DEPTH_LIMIT`.
5. Không sinh Mermaid.

## Kiểm thử

`go test ... -run Export` (chưa chạy): ví dụ 5 bước hai mức; số participant/node; `data` có tên message thật.

## Tiêu chí hoàn thành

- [x] Hai mức `detail` đúng.
- [x] Không chuỗi Mermaid.

## Rủi ro và lưu ý

- Quy ước `group` = service; thống nhất với FE 056.
