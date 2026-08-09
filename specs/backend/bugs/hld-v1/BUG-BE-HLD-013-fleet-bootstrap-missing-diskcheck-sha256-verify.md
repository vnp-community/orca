# BUG-BE-HLD-013 — `bootstrapServer()` thiếu disk-space check và SHA256 verify cho relay binary

**Mức độ:** 🟡 MEDIUM (Reliability/Security gap)
**Status:** 🔴 Open
**Module:** `backend/src/main/ssh/fleet-bootstrap-service.ts`, `ssh-relay-deploy.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.10/F31, CR-004)

---

## Mô tả

`docs/features/F31-fleet-provisioning.md` (CR-004) mô tả `FleetBootstrapService.bootstrap()` gồm 7 bước, trong đó có: check disk (`df -h .`, ≥5GB) và verify SHA256 checksum của relay binary trước khi cài đặt.

Thực tế:
- Class `FleetBootstrapService` không tồn tại — chỉ có hàm đơn `bootstrapServer()` (`fleet-bootstrap-service.ts:50-216`).
- Đối chiếu từng bước: check Node.js ✅, check Git ✅ — nhưng **check disk space hoàn toàn không có** (không tìm thấy ở `fleet-bootstrap-service.ts` lẫn `fleet-remote-commands.ts`).
- **Verify SHA256 binary cũng không có** — grep `sha256|checksum` trong `ssh-relay-deploy.ts`: 0 kết quả liên quan tới integrity-check của relay binary (các chỗ khác dùng sha256 cho mục đích khác — hash token/endpoint, không phải verify binary).
- Bước "install relay qua SFTP + chmod" và "start relay" cũng **không nằm trong `bootstrapServer()`** — là cơ chế riêng biệt (`ssh-relay-deploy.ts`), không được gọi tích hợp từ hàm bootstrap.

## Hậu quả

- Bootstrap có thể thành công trên server sắp hết dung lượng đĩa, dẫn tới lỗi khó chẩn đoán sau này (clone repo, tạo worktree fail giữa chừng).
- Không xác minh được relay binary tải về không bị hỏng/can thiệp giữa đường — rủi ro bảo mật (supply-chain) nếu kênh tải xuống bị compromise.

## Bằng chứng

- `backend/src/main/ssh/fleet-bootstrap-service.ts:50-216` — không có class `FleetBootstrapService`, không có disk-check.
- `backend/src/main/ssh/fleet-remote-commands.ts` — không có hàm check disk.
- Grep `sha256|checksum` trong `backend/src/main/ssh/ssh-relay-deploy.ts`: 0 kết quả liên quan verify binary.

## Đề xuất fix

1. Thêm bước `df -h <remoteWorkDir> | parse available >= 5GB` trong `bootstrapServer()`, fail sớm nếu không đủ.
2. Publish checksum SHA256 cùng với relay binary release, thêm bước verify sau khi SFTP upload, trước khi chmod +x / chạy.
3. Cân nhắc gộp bước install/start relay (hiện ở `ssh-relay-deploy.ts`) vào cùng luồng `bootstrapServer()` để tránh 2 luồng rời rạc dễ desync trạng thái.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.10 (F31, CR-004), §6 mục 9 (Top 10)
- Doc gốc: `docs/features/F31-fleet-provisioning.md`
- Liên quan: [BUG-BE-HLD-012](./BUG-BE-HLD-012-fleet-provision-cli-not-implemented.md)
