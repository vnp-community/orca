# Kế hoạch triển khai backend v6

| Trường | Giá trị |
|---|---|
| **Ngày** | 2026-10-07 |
| **Nhánh** | `feat/request-flow-backend` (worktree `.claude/worktrees/request-flow-backend`) |
| **Nền** | [Kiểm toán 2026-10-07](../../../../docs/research/receive-request/audit-2026-10-07/README.md): 14/199 task đủ khi bắt đầu; trạng thái đã được đưa về đúng thực tế |
| **Định nghĩa "xong"** | Xem mục 4 |

## 1. Quyết định trung tâm

| # | Quyết định | Lý do |
|---|---|---|
| D1 | **Kế hoạch số migration của `request-service`:** service chưa từng triển khai, nên cho phép **đánh số lại một lần** các migration hiện có (0001 đến 0007) theo thứ tự phụ thuộc đúng (ví dụ bảng `requests` trước `approvals`); sau đợt R1 số được đóng băng, mỗi đợt sau chỉ thêm số kế tiếp. Chỉ một agent sửa migrations của `request-service` tại một thời điểm | `0002_approvals` có FK tới `request.requests` chỉ tạo ở `0005`; số chồng giữa các CR |
| D2 | **`task-service`:** số kế tiếp `0015` (`task_type`), `0016` (`request_id`); các đợt sau thêm số kế tiếp, sửa tuần tự | Đã xác nhận bằng ls |
| D3 | **Gọi AI không đồng bộ qua WS:** `ClassifyRequest` và `GeneratePlan` (PROPOSE) trả ngay `{run_id}` và chạy nền như `GenerateSolution` (lease, phục hồi); kết quả đọc qua `GetRequest`/`ListSolutions` và sự kiện. Không nâng timeout 25 giây của `wscompat` | Tránh vượt timeout, nhất quán với CR-007 |
| D4 | **Mặc định cho điểm mở (README v6 O1 đến O4):** trạng thái Plan/Phase suy ra từ con; Plan/Phase không nhận `TaskNumber`; gỡ `backlog` khỏi `TaskStatus`; không hợp nhất `DecisionGate` với Approval | Theo `docs/crs/v6/README.md` |
| D5 | **Agent:** `request-service` gọi `agent.execPrompt` qua `infra-fleet-service` với `RelayByDevServer` khi `connectionID` theo project không tra được (BUG-025 của `task-service`); chế độ chỉ đọc dùng `accessMode` (CR-REQ-033) và kiểm `features` | Theo audit và CR-033 |
| D6 | **Quyền mức Request:** theo CR-REQ-035 mục 2.3 (`owner|member` của project-service, `reporter`, `admin`, `actor_type`) | Chốt tạm để làm tiếp, ghi trong IMPLEMENTATION-NOTES |
| D7 | **RLS:** theo mẫu `mcp-service`; sửa `0006_analysis_runs` (thiếu `FORCE`, GUC sai `orca.tenant_id`, thiếu `WITH CHECK`) | Audit |

## 2. Các đợt (tuần tự trong cùng module, song song giữa các module rời nhau)

| Đợt | Module | CR và task | Phụ thuộc |
|---|---|---|---|
| **R1a** | `request-service` | 001, 002: nền, migration (đánh số lại), repository Postgres và MySQL, giao dịch và RLS, RPC `GetRequest`/`ListRequests` thật, wiring `main.go`, CI workflow, compose | không |
| **T1** (song song R1a) | `task-service` | 011: sửa build, `type` plan/phase, lọc, cascade, `request_id` (proto), không số task, chặn Execute container | không |
| **I1** (song song R1a) | `infra-fleet-service` | 033 backend: nối `GetDevServerCapabilities`, wiring, test | không |
| **R1b** | `request-service` | 003 đến 006: máy trạng thái, tiếp nhận, phân loại, trả backlog | R1a |
| **R2** | `request-service` (+ `notification-service`) | 009, 010 (Approval, chính sách, thông báo), 007, 008 (Solution, Chẩn đoán), 026 (OpenSpec) | R1b, I1 |
| **R3** | `request-service` + `task-service` | 012, 013, 014, 015 (Plan, thực thi, chính sách theo loại, backlog) | R2, T1 |
| **R4** | `request-service` + `task-service` | 027, 028, 029, 030 (lược đồ, hỏi lại, hợp đồng thực thi, tác động rủi ro) | R3 |
| **R5** | `api-gateway`, `mcp-service`, `issue-status-sync`, `common` | 016, 017, 024, 025 (kênh WS, MCP, đồng bộ Jira, e2e, cờ) | R2 trở đi (RPC có thật) |
| **R6** | `request-service`, `mcp-service`, `common` | 031, 034, 035 (nguồn dữ liệu, quản trị AI, bảo mật) | R4 |

Sau mỗi đợt người điều phối: build và test toàn bộ module đã chạm, kiểm tra không tạo hồi quy ở module khác, chạy lại kiểm toán script, commit trên nhánh.

## 3. Quy tắc cập nhật trạng thái

Tóm tắt: `[x] DONE` chỉ sau khi kiểm chứng thật (có ngày và lệnh test); solution chỉ "Đã triển khai" khi mọi task của nó xong; mỗi feature có `IMPLEMENTATION-NOTES.md`.

## 4. Định nghĩa "xong"

1. Code thật, không stub. 2. Nối vào wiring thật. 3. Có test đúng mục Kiểm thử và PASS thật. 4. `go build`, `go vet`, `go test` PASS, `gofmt` sạch ở mọi module đã sửa. 5. Mục tiêu chí kiểm chứng được đã kiểm chứng; phần cần môi trường không có ghi rõ "chưa kiểm chứng".

## 5. Việc nằm ngoài bộ này

Frontend (54 task) và phần còn lại của agent (CR-033: 3 task một phần). `update_v7.js` ở gốc repo thuộc v7, không xử lý ở đây.

## 6. Chạy song song nhiều agent trong `request-service` (từ đợt R1b)

Sau khi R1a xong (migration đã đánh số lại và đóng băng, repository, giao dịch, RLS, wiring cơ bản), nhiều agent chạy song song, **mỗi agent trong git worktree riêng** (nhánh `feat/request-flow-backend/<feature>` tách từ nhánh chính của đợt), người điều phối hợp nhất tuần tự. Để giảm xung đột:

| Quy tắc | Nội dung |
|---|---|
| Dải số migration dành riêng | `request-service`: 004 đến 006 → `0020`..`0029`; 007, 008, 026 → `0030`..`0039`; 009, 010 → `0040`..`0049`; 012 đến 015 → `0050`..`0059`; 027, 028 → `0060`..`0069`; 029, 030 → `0070`..`0079`; 031, 034, 035 → `0080`..`0089`. (golang-migrate cho phép số cách quãng; mỗi migration chỉ tham chiếu bảng đã có ở số nhỏ hơn hoặc ở R1a.) |
| Wiring | Mỗi feature thêm `cmd/server/wire_<feature>.go` với hàm `wire<Feature>(...)`; `main.go` chỉ thêm đúng một lời gọi. Không sửa lẫn wiring của feature khác |
| Proto | Một agent "proto-first" định nghĩa toàn bộ RPC và message của `RequestService`, `ApprovalService` ... theo `CONTRACT-request-ui-api.md` và CR (server trả `Unimplemented` mặc định) trước khi các agent feature chạy; agent feature chỉ cài đặt, KHÔNG sửa `request.proto` (nếu cần bổ sung, ghi vào IMPLEMENTATION-NOTES và báo người điều phối) |
| Ports | Mỗi feature đặt cổng (interface) trong file riêng `internal/usecase/ports_<feature>.go`, không sửa `ports.go` chung ngoài việc thêm |
| Trạng thái task | Mỗi agent chỉ sửa file task/solution của feature mình và `IMPLEMENTATION-NOTES.md` của feature mình |

## 7. Dải số cho đợt 2 (đã cấp)

- `request-service` migration: lifecycle 003, 006 → `0020`..`0024`; 004, 005 → `0025`..`0029`; các đợt sau theo mục 6.
- `task-service` migration: 0015, 0016 đã dùng; CR-012 và CR-013 (phần task-service) → `0017`..`0019`; CR-027, CR-029 (phần task-service) → `0020`..`0022`.
- Mỗi agent đợt 2 làm trong worktree riêng `.claude/worktrees/rf-<tên>` (nhánh `feat/request-flow-backend/<tên>`); người điều phối hợp nhất và sinh lại mã proto sau khi hợp nhất, không hợp nhất mã sinh.

## 8. Dải số cho đợt 3 và 4 (đã cấp)

- `request-service` migration: lifecycle 0020..0029, Solution 0030..0039, Approval 0040..0049, thực thi/Plan/backlog 0050..0059, artifact 0060..0069, hợp đồng thực thi và tác động 0070..0079, bảo mật 0080..0089, rollout và đồng bộ 0090..0099, nguồn dữ liệu và AI 0100..0119.
- Mỗi agent làm trong worktree `rf-<tên>`; người điều phối hợp nhất tuần tự và chạy lại build, vet, test sau mỗi lần hợp nhất.
