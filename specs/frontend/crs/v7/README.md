# Frontend: Solutions và Tasks cho series v7 "Xem code & kiểm soát chất lượng"

> **Trạng thái: 🚧 In Progress.** Rà soát ngày 2026-10-07. Tiến độ code thực tế: ~16% (~29/178 tasks). `quality-rollout` 14%, `quality-gate` 25%, `quality-visualization` 7%, `review-frontend` 16%. Nền (SOL-050) ~60%, UI lens và components hầu hết chưa bắt đầu. Spec đã hoàn chỉnh (26 solutions), code đang triển khai. Chưa chạy test toàn bộ, fake backend chưa tồn tại.

**CR nguồn:** [docs/crs/v7](../../../../docs/crs/v7/README.md) (59 CR, mục 8 ghi các điều chỉnh hợp đồng)
**Hợp đồng chuẩn tắc (theo thứ tự ưu tiên khi lệch):** [CONTRACT-codeintel-proto-and-data-map.md](../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (37 phán quyết PQ-01..37, bảng ánh xạ CR → khu vực → solution ở mục 8.2), [CONTRACT-codeintel-agent-rpc.md](../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md), [CONTRACT-codeintel-ui-api.md](../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md)
**Điểm hợp đồng còn thiếu/mâu thuẫn do các solution báo:** [CONTRACT-open-issues.md](../../../backend-go/crs/v7/CONTRACT-open-issues.md)

## Quy ước tên

- Solution: `FE-CV-SOL-<CR>-<slug>.md` trong `<feature>/solutions/`; Task: `FE-CV-TASK-<CR>-<NN>-<slug>.md` trong `<feature>/tasks/`.
- `<NN>` tăng liên tục trong cùng (khu vực, CR) kể cả khi một CR có nhiều solution. Mọi task mở đầu `Status: [ ] TODO`.
- `<feature>` trùng tên thư mục trong `docs/crs/v7/`.

## Danh sách feature

| Feature | Nội dung | Solution | Task (done/total) | Liên kết |
|---|---|---|---|---|
| [`quality-gate`](./quality-gate/solutions/README.md) | Cổng chất lượng, dấu vết agent, báo cáo, truy vết yêu cầu, tóm tắt AI, telemetry | 6 | ~10/42 (25%) 🚧 | [solutions](./quality-gate/solutions/README.md) · [tasks](./quality-gate/tasks/README.md) |
| [`quality-rollout`](./quality-rollout/solutions/README.md) | Fixture vàng, hiệu năng/metrics, kiểm thử bảo mật, cờ + E2E + rollout | 1 | 1/7 (14%) 🔴 | [solutions](./quality-rollout/solutions/README.md) · [tasks](./quality-rollout/tasks/README.md) |
| [`quality-visualization`](./quality-visualization/solutions/README.md) | Frontend chất lượng và nền đồ hoạ | 4 | ~2/29 (7%) 🔴 | [solutions](./quality-visualization/solutions/README.md) · [tasks](./quality-visualization/tasks/README.md) |
| [`review-frontend`](./review-frontend/solutions/README.md) | Màn Review: nền, khung, thứ tự đọc, các lens (ảnh hưởng, cấu trúc, C4, luồng, ERD, lưu trữ, hợp đồng), ghi chú, điểm vào, mobile | 15 | ~16/100 (16%) 🚧 | [solutions](./review-frontend/solutions/README.md) · [tasks](./review-frontend/tasks/README.md) |
| **Tổng** | | **26** | **~29/178 (~16%)** | |

## Thứ tự thực thi

Xem mục 7 của hợp đồng ([CONTRACT-codeintel-proto-and-data-map.md](../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md)): cổng đồng bộ G0–G4, thứ tự theo khu vực (backend → agent → frontend) và 9 đợt. Frontend chỉ dùng kênh thật sau cổng G3; trước đó dùng fake backend (task `FE-CV-TASK-073-02`).
