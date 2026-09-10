# CR-EVM-011 — Container (Docker/OCI) runtime type — backlog, chưa xác nhận scope

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-EVM-011 |
| **Tên** | Thêm `connection_type: 'container'` như lựa chọn thay thế `ssh`/`orca-server` |
| **Loại** | New Capability (subsystem lớn) |
| **Priority** | P3 — backlog, **chưa xác nhận nhu cầu sản phẩm** |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Proposed — chưa xác nhận scope, KHÔNG bắt đầu code |
| **Tác giả** | Audit trực tiếp mã nguồn sau khi xác nhận CR-EVM-001..005 đã Done |
| **Tác động HLD** | Agent transport layer, Infra-Fleet domain |
| **Tác động Features** | F18 (Ephemeral VM) — "Runtime Options" liệt Container như lựa chọn thay thế SSH |

---

## Bối cảnh & Vấn đề gốc

F18's spec
([`docs/features/F18-ephemeral-vm.md:40-43`](../../../features/F18-ephemeral-vm.md))
liệt 2 Runtime Option:

> - **SSH-based**: VM được access qua SSH
> - **Container**: Docker/OCI container (alternative)

SSH-based (+ orca-server) đã triển khai đầy đủ (CR-EVM-001..005,
CR-EVM-008). Container **hoàn toàn chưa tồn tại**, không phải thiếu 1
phần:

- `connection_type` (`agent/src/shared/ephemeral-vm-runtimes.ts:28`) —
  `EphemeralVmRuntimeConnectionModeSchema = z.enum(['orca-server', 'ssh'])`
  — đúng 2 giá trị.
- `infrafleet.proto:231,267` — cùng union 2 giá trị string, không có
  `docker`/`container`.
- Grep `docker`/`Docker`/`OCI`/`container` (case-insensitive) trong toàn
  bộ schema/handler/domain file ephemeral-vm: **0 kết quả**.

Đây là scope hoàn toàn mới, không phải "hoàn thiện nốt phần dở" như
CR-EVM-006..010.

## Vì sao CR này KHÔNG có "Giải pháp đề xuất" chi tiết

Theo đúng thận trọng mà CR-EVM-005 gốc đã áp dụng cho nhánh SSH (từng để
ngỏ Option A/B, chỉ chốt 1 hướng sau khi có đủ thông tin) — CR này **cố
ý dừng ở mức đặt vấn đề**, không cam kết thiết kế kỹ thuật, cho tới khi
có xác nhận nhu cầu sản phẩm thật. Lý do:

1. Container runtime đặt ra câu hỏi kiến trúc khác hẳn SSH: chạy ở đâu
   (agent host tự chạy Docker daemon? hay 1 dịch vụ container
   orchestration riêng?), image từ đâu (registry nào, ai build), network
   isolation model khác hoàn toàn SSH tunnel.
2. Chưa có tín hiệu nhu cầu nào được audit ghi nhận (không backlog item,
   không user request nào được tìm thấy khi khảo sát `docs/backlog/`).
3. Xây trước khi xác nhận nhu cầu có rủi ro y hệt cảnh báo gốc của
   `docs/backlog/BACKLOG-001-ephemeralvm-ssh-outbound-client.md` đã né
   tránh cho SSH — không lặp lại sai lầm ở nhánh Container.

## Việc cần làm trước khi CR này được "Proposed" đúng nghĩa

- Xác nhận với product/roadmap: có nhu cầu thật cho Container runtime
  không, hay SSH-based đã đủ cho mọi use case ephemeral VM hiện tại?
- Nếu có nhu cầu: viết lại CR này với thiết kế kỹ thuật đầy đủ (theo
  đúng khuôn CR-EVM-001..010), bao gồm Option A/B cho các quyết định
  kiến trúc nêu ở mục trên.
- Nếu không có nhu cầu: đóng CR này, cân nhắc sửa F18 spec bỏ mục
  Container khỏi "Runtime Options" (hoặc giữ như "future consideration",
  tuỳ quyết định product).

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Bắt đầu code trước khi xác nhận nhu cầu | Cao | CR này tồn tại chính là để CHẶN việc đó xảy ra ngoài ý muốn — giữ trong README để không mất dấu, nhưng đánh dấu rõ "chưa xác nhận scope" |

## Không thuộc phạm vi CR này

- Mọi thiết kế kỹ thuật cụ thể — cố ý để trống, xem "Vì sao CR này
  không có Giải pháp đề xuất chi tiết" ở trên.

## Liên quan

- `docs/features/F18-ephemeral-vm.md:40-43`
- `agent/src/shared/ephemeral-vm-runtimes.ts:28` (`EphemeralVmRuntimeConnectionModeSchema`)
- `infrafleet.proto:231,267`
- [CR-EVM-005](./CR-EVM-005-ssh-connection-type-outbound-client.md) (tiền lệ Option A/B cho quyết định kiến trúc runtime mới)
