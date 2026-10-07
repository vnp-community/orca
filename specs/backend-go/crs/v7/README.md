# Backend (backend-go): Solutions và Tasks cho series v7 "Xem code & kiểm soát chất lượng"

> **Trạng thái: ✅ Đã hoàn thành (Implemented & Verified).**

**CR nguồn:** [docs/crs/v7](../../../../docs/crs/v7/README.md) (59 CR, mục 8 ghi các điều chỉnh hợp đồng)
**Hợp đồng chuẩn tắc (theo thứ tự ưu tiên khi lệch):** [CONTRACT-codeintel-proto-and-data-map.md](../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (37 phán quyết PQ-01..37, bảng ánh xạ CR → khu vực → solution ở mục 8.2), [CONTRACT-codeintel-agent-rpc.md](../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md), [CONTRACT-codeintel-ui-api.md](../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md)
**Điểm hợp đồng còn thiếu/mâu thuẫn do các solution báo:** [CONTRACT-open-issues.md](../../../backend-go/crs/v7/CONTRACT-open-issues.md)

## Quy ước tên

- Solution: `BE-CV-SOL-<CR>-<slug>.md` trong `<feature>/solutions/`; Task: `BE-CV-TASK-<CR>-<NN>-<slug>.md` trong `<feature>/tasks/`.
- `<NN>` tăng liên tục trong cùng (khu vực, CR) kể cả khi một CR có nhiều solution. Mọi task mở đầu `Status: [ ] TODO`.
- `<feature>` trùng tên thư mục trong `docs/crs/v7/`.

## Danh sách feature

| Feature | Nội dung | Solution | Task | Liên kết |
|---|---|---|---|---|
| [`code-intel-gateway`](./code-intel-gateway/solutions/README.md) | Kênh `codeIntel.*` ở api-gateway và tool MCP (tuỳ chọn) | 5 | 36 | [solutions](./code-intel-gateway/solutions/README.md) · [tasks](./code-intel-gateway/tasks/README.md) |
| [`code-intel-graph-pipeline`](./code-intel-graph-pipeline/solutions/README.md) | Mô hình graph chuẩn, collector, cache snapshot, vận chuyển ở infra-fleet, phân phối sự kiện | 5 | 43 | [solutions](./code-intel-graph-pipeline/solutions/README.md) · [tasks](./code-intel-graph-pipeline/tasks/README.md) |
| [`code-intel-service-foundation`](./code-intel-service-foundation/solutions/README.md) | Dựng `code-intel-service`, mô hình dữ liệu và repository, ánh xạ project/worktree → repo, phân quyền/audit/hạn mức | 7 | 44 | [solutions](./code-intel-service-foundation/solutions/README.md) · [tasks](./code-intel-service-foundation/tasks/README.md) |
| [`code-intel-sources`](./code-intel-sources/solutions/README.md) | Cổng đọc file repo, ERD từ SQL, hợp đồng proto/wscompat, C4, luồng dữ liệu, lưu trữ, change overlay, phân tích cấu trúc, contract diff | 13 | 84 | [solutions](./code-intel-sources/solutions/README.md) · [tasks](./code-intel-sources/tasks/README.md) |
| [`quality-gate`](./quality-gate/solutions/README.md) | Cổng chất lượng, dấu vết agent, báo cáo, truy vết yêu cầu, tóm tắt AI, telemetry | 6 | 41 | [solutions](./quality-gate/solutions/README.md) · [tasks](./quality-gate/tasks/README.md) |
| [`quality-rollout`](./quality-rollout/solutions/README.md) | Fixture vàng, hiệu năng/metrics, kiểm thử bảo mật, cờ + E2E + rollout | 4 | 33 | [solutions](./quality-rollout/solutions/README.md) · [tasks](./quality-rollout/tasks/README.md) |
| [`quality-signals`](./quality-signals/solutions/README.md) | Index bắt kịp, bộ chạy kiểm tra, phát hiện + parser, coverage, rule pack, gộp CI, quét bảo mật | 6 | 49 | [solutions](./quality-signals/solutions/README.md) · [tasks](./quality-signals/tasks/README.md) |
| **Tổng** | | **46** | **330** | |

## Thứ tự thực thi

Xem mục 7 của hợp đồng ([CONTRACT-codeintel-proto-and-data-map.md](../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md)): cổng đồng bộ G0–G4, thứ tự theo khu vực (backend → agent → frontend) và 9 đợt. Frontend chỉ dùng kênh thật sau cổng G3; trước đó dùng fake backend (task `FE-CV-TASK-073-02`).
