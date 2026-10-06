# 09 — Yêu cầu dữ liệu từ nguồn khác (cần bổ sung)

GitNexus và CodeGraph chỉ biết **code đã được index**. Các view trong [08](./08-views-and-review-models.md) cần thêm dữ liệu dưới đây. Danh sách này để bạn biết cần cung cấp/xác nhận gì.

Cột "Trạng thái": **Có trong repo** = đã thấy file/thư mục thật (xem được qua `fs.*` của agent); **Chưa xác nhận** = chưa kiểm tra có hay không; **Cần bạn cung cấp** = con người/hệ thống khác nắm.

| # | Nguồn | Cần cho | Chi tiết cần | Lấy từ đâu | Trạng thái |
|---|---|---|---|---|---|
| E1 | **Migration SQL** | ERD (4), lưu trữ (5), R4 | Toàn bộ `*.up.sql` theo thứ tự; dialect (postgres/mysql) | `backend-go/services/*/migrations/{postgres,mysql}/` | Có trong repo (17 service, 588 file `.sql`) |
| E2 | **File proto** | Luồng (3), C4 (2), R4 | Định nghĩa service/RPC để nối client↔server | `backend-go/proto/orca/**/*.proto` | Có trong repo |
| E3 | **Compose / deploy** | Lưu trữ (5) | Postgres/MySQL/Redis/object store/Vault, volume, mạng, biến môi trường | `backend-go/docker-compose.yml`, `deploy/dev`, `deploy/prod`, `deploy/old` | Có trong repo; `deploy/prod` chỉ có `orca-server` → **cần bạn xác nhận topology prod thật** |
| E4 | **Cấu hình từng service** | Lưu trữ (5), luồng (3) | DSN/schema, tên topic event bus, Vault path, timeout | `internal/config/` và biến môi trường của từng service | Có trong repo (cấu trúc); giá trị thật **cần bạn cung cấp** (không đưa secret vào tài liệu) |
| E5 | **Lịch sử git** | Hotspot (R3), change overlay (R1), thứ tự đọc (R6) | `git log` (tần suất đổi, tác giả), diff giữa hai commit | RPC `git.*` của agent (đã có) | Có; cần chốt khoảng thời gian (ví dụ 90 ngày) |
| E6 | **Quy tắc C4 do con người** | C4 (2) | Tên component, gộp/tách package, mô tả, ranh giới container, thành phần ngoài | File `c4.yaml` (đề xuất đặt trong repo) | **Cần bạn cung cấp / duyệt** — bản mặc định suy từ thư mục chỉ là bản nháp |
| E7 | **Quan hệ logic giữa service (ERD)** | ERD (4) | Cột nào tham chiếu bảng của service khác (không có FK thật) | Chú thích trong migration hoặc khai báo tay | **Cần bạn cung cấp** hoặc quy ước đặt tên (`*_id`, `tenant_id`) |
| E8 | **Danh mục kênh `wscompat`** | Luồng (3), contract (R4) | Tên kênh → gRPC method | `backend-go/services/api-gateway/internal/adapter/wscompat/channels_*.go` | Có trong repo (nhiều file `channels_*.go`) |
| E9 | **Định nghĩa workflow** | Luồng (3) | Workflow/step do `workflow-service` chạy (nếu lưu trong DB hoặc seed) | `workflow-service` (migration, seed, mã nguồn) | **Chưa xác nhận** workflow nằm ở đâu |
| E10 | **Bản đồ topic event bus** | Lưu trữ (5), luồng (3) | Topic, publisher, subscriber, payload | `adapter/eventbus` mỗi service | Có trong repo (chưa liệt kê đầy đủ); cần thu thập khi làm |
| E11 | **Kết quả test/coverage** | Khoảng trống test (R5) | Độ phủ thật theo hàm/file | CI (báo cáo coverage), hoặc dùng phỏng đoán của GitNexus/CodeGraph | **Chưa xác nhận** CI có xuất coverage không; mặc định dùng cạnh test→hàm của GitNexus |
| E12 | **CODEOWNERS / owner** | Owner (R8), review | Ánh xạ đường dẫn → người/nhóm | `CODEOWNERS` hoặc danh sách do bạn cung cấp | **Chưa xác nhận** có file hay không |
| E13 | **Index `--pdg` của GitNexus** | Bảo mật (R7) | Chạy `gitnexus analyze --pdg` trên dev server | Dev server | Cần bật; tốn thời gian/dung lượng index |
| E14 | **DB thật (tuỳ chọn)** | ERD (4) | `information_schema` để đối chiếu với migration (phát hiện lệch) | DB môi trường dev/prod | **Cần bạn cung cấp quyền** nếu muốn đối chiếu; mặc định không dùng |
| E15 | **ADR/spec kiến trúc** | C4 (2), lưu trữ (5) | Quyết định kiến trúc đã duyệt (ví dụ nguyên tắc "service sở hữu DB riêng") | `specs/backend-go/architecture/*` | Có trích dẫn trong migration (ví dụ `05-data-architecture.md`); chưa đọc |
| E16 | **Ánh xạ project/worktree → repo index** | Mọi view | Project nào ↔ repo GitNexus/CodeGraph nào (registry GitNexus có nhiều repo) | `project-service` + `infra-fleet-service` | Cần thiết kế quy tắc ([06 §4](./06-gaps-risks-roadmap.md)) |
| E17 | **Nguồn dữ liệu của hệ thống ngoài Orca (nếu cần)** | C4 (2) | Dịch vụ bên thứ ba (Jira, GitHub/GitLab, Vault, LLM) hiển thị làm "external" | Cấu hình tích hợp; `scm-integration-service`, `issue-tracking-service`… | Có trong repo; cần liệt kê |

## Ưu tiên cung cấp

Tối thiểu để làm MVP của view 1, 4 và R1:
- E1, E5 (đã có trong repo/agent, không cần bạn làm gì ngoài xác nhận phạm vi).
- E16 (quy tắc ánh xạ project → repo).

Cần quyết định/cung cấp từ bạn để làm các view khác:
- E6 (quy tắc C4) và E7 (quan hệ logic ERD): hai thứ máy không tự suy ra đúng được.
- E3 (topology prod) và E4 (giá trị cấu hình thật, không kèm secret) cho view lưu trữ.
- E9 (workflow nằm ở đâu), E11 (coverage), E12 (owner): chưa xác nhận có hay không.

## Ghi chú về an toàn dữ liệu

- Không đưa secret (DSN có mật khẩu, token Vault, khoá API) vào tài liệu hoặc cache; chỉ lưu tên khoá/đường dẫn Vault.
- Các file ngoài (E1–E4, E8, E10) được đọc qua agent trong phạm vi workspace đã đăng ký, không đọc ngoài worktree.
- Mã nguồn chỉ trả về UI khi người dùng mở một symbol cụ thể ([03 §5](./03-command-and-data-flow.md)).
