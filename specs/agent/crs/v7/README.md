# Agent: Solutions và Tasks cho series v7 "Xem code & kiểm soát chất lượng"

> 📋 Proposed. Chưa triển khai, chưa chạy test hay build nào. Mọi nhận định về code là kết quả đọc code ngày 2026-10-06; chỗ chưa kiểm chứng được ghi rõ trong từng solution.

**CR nguồn:** [docs/crs/v7](../../../../docs/crs/v7/README.md) (59 CR, mục 8 ghi các điều chỉnh hợp đồng)
**Hợp đồng chuẩn tắc (theo thứ tự ưu tiên khi lệch):** [CONTRACT-codeintel-proto-and-data-map.md](../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (37 phán quyết PQ-01..37, bảng ánh xạ CR → khu vực → solution ở mục 8.2), [CONTRACT-codeintel-agent-rpc.md](../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md), [CONTRACT-codeintel-ui-api.md](../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md)
**Điểm hợp đồng còn thiếu/mâu thuẫn do các solution báo:** [CONTRACT-open-issues.md](../../../backend-go/crs/v7/CONTRACT-open-issues.md)

## Quy ước tên

- Solution: `AG-CV-SOL-<CR>-<slug>.md` trong `<feature>/solutions/`; Task: `AG-CV-TASK-<CR>-<NN>-<slug>.md` trong `<feature>/tasks/`.
- `<NN>` tăng liên tục trong cùng (khu vực, CR) kể cả khi một CR có nhiều solution. Mọi task mở đầu `Status: [ ] TODO`.
- `<feature>` trùng tên thư mục trong `docs/crs/v7/`.

## Danh sách feature

| Feature | Nội dung | Solution | Task | Liên kết |
|---|---|---|---|---|
| [`agent-codeintel`](./agent-codeintel/solutions/README.md) | Nền `codeintel.*` trên agent, trích xuất GitNexus/CodeGraph, làm mới index, `detectChanges`, relay-ssh | 6 | 50 | [solutions](./agent-codeintel/solutions/README.md) · [tasks](./agent-codeintel/tasks/README.md) |
| [`code-intel-sources`](./code-intel-sources/solutions/README.md) | Cổng đọc file repo, ERD từ SQL, hợp đồng proto/wscompat, C4, luồng dữ liệu, lưu trữ, change overlay, phân tích cấu trúc, contract diff | 1 | 7 | [solutions](./code-intel-sources/solutions/README.md) · [tasks](./code-intel-sources/tasks/README.md) |
| [`quality-rollout`](./quality-rollout/solutions/README.md) | Fixture vàng, hiệu năng/metrics, kiểm thử bảo mật, cờ + E2E + rollout | 4 | 31 | [solutions](./quality-rollout/solutions/README.md) · [tasks](./quality-rollout/tasks/README.md) |
| [`quality-signals`](./quality-signals/solutions/README.md) | Index bắt kịp, bộ chạy kiểm tra, phát hiện + parser, coverage, rule pack, gộp CI, quét bảo mật | 7 | 56 | [solutions](./quality-signals/solutions/README.md) · [tasks](./quality-signals/tasks/README.md) |
| **Tổng** | | **18** | **144** | |

## Thứ tự thực thi

Xem mục 7 của hợp đồng ([CONTRACT-codeintel-proto-and-data-map.md](../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md)): cổng đồng bộ G0–G4, thứ tự theo khu vực (backend → agent → frontend) và 9 đợt. Frontend chỉ dùng kênh thật sau cổng G3; trước đó dùng fake backend (task `FE-CV-TASK-073-02`).
