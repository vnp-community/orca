# agent-capabilities: solutions backend (BE-REQ-SOL-033)

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Tài liệu ngày 2026-10-06; số dòng và đường dẫn đã đối chiếu với code `backend-go/services/infra-fleet-service` cùng ngày. `request-service` chưa có thư mục: mọi đường dẫn của nó là "(mới)".

Nguồn: [docs/crs/v6/agent-capabilities](../../../../../../docs/crs/v6/agent-capabilities/README.md). README v6 [mục 8](../../../../../../docs/crs/v6/README.md) thắng mục 3 khi mâu thuẫn. Tài liệu thiết kế tham chiếu: [`tdd/README.md`](../../../../tdd/README.md), `architecture/03, 05, 08, 09`, [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md), [`services/task-service.md`](../../../../tdd/services/task-service.md).

## Bảng CR → Solution → Task

| CR | Solution | Service / Area | Task (xem [tasks/README](../tasks/README.md)) |
|---|---|---|---|
| [CR-REQ-033](../../../../../../docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md) chỉ phần backend: hồ sơ năng lực, `GetDevServerCapabilities`, `HandshakeInfo`, client `agent.execPrompt` | [BE-REQ-SOL-033](./BE-REQ-SOL-033-dev-server-capability-profile-and-agent-client.md) | `infra-fleet-service`, `request-service` (mới), `proto` | TASK-REQ-033-01 đến 06 |

Phần trong `agent/` của CR-REQ-033 (chế độ chỉ đọc, vùng làm việc, `changes`, khối kết quả, `agent.capabilities`, handshake, `ai.complete` mở rộng) do khu vực `agent` soạn: [`specs/agent/crs/v6/agent-capabilities/`](../../../../../agent/crs/v6/agent-capabilities/solutions/) (file `AG-REQ-SOL-033-*`). Bảng giao diện method, tham số và trường khớp CR nằm ở mục 2.K của solution.

## Thứ tự phụ thuộc

```
[agent: AG-REQ-SOL-033-*]   (độc lập, làm trước; backend suy giảm được nếu chưa có)
        │
TASK-033-01 (migration 0039, domain, repo)   TASK-033-02 (HandshakeInfo)   ← song song
        └──────────────┬─────────────────────────┘
                       ▼
               TASK-033-03 (Refresh/Get use case, RPC, kích hoạt)
                       ▼
               TASK-033-04 (request-service đọc hồ sơ, chọn đường)  ◀── cần TASK-REQ-001-01..05
                       ▼
               TASK-033-05 (client Go agent.execPrompt)  ◀── cần TASK-REQ-008-02
                       ▼
               TASK-033-06 (golden hợp đồng, kịch bản suy giảm)
```

Mở khoá: SOL-029 (`ReadinessGate` tầng môi trường, `result_nonce`), SOL-008 (đường `agent_readonly` ép được), SOL-005 (báo thiếu khoá API sớm), SOL-030 (đọc `tools` làm nền cho bộ thu thập).

## Quyết định chung

| # | Quyết định | Lý do |
|---|---|---|
| A1 | Hồ sơ lưu `profile_json` nguyên văn, chỉ tách `features`, `protocol_version`, `source` | Schema agent đổi nhanh; không migrate mỗi lần |
| A2 | `fingerprint` loại số đo biến động; sự kiện chỉ khi fingerprint đổi | Tránh bão sự kiện |
| A3 | Không probe khi dev server không kết nối; trả hồ sơ cũ kèm `connected=false` | Không dial chỉ để dò |
| A4 | Hồ sơ `handshake_only` mang `Known=false`, không bao giờ `Installed=false` | "Không biết" khác "không có" |
| A5 | `features` tách khỏi `capabilities`; không đổi `agentVersion` | `ResumeAgentSession`, `MinAgentVersion`, `ptyReady` phụ thuộc giá trị cũ |
| A6 | Chọn đường theo `features` **trước** khi gửi tham số mới | Agent cũ bỏ qua khoá lạ và sẽ chạy ở chế độ ghi |
| A7 | Hợp đồng JSON giữa agent và Go khoá bằng golden copy hai nơi, so hash ở CI | Hai khu vực không import nhau |
| A8 | Không sửa `desktop/src/relay` | Bản đã lệch, không có `execPrompt` |

## Đã kiểm chứng và điểm khác CR gốc

- `HandshakeInfo` của `usecase` thiếu `Capabilities` đúng như CR; nhưng phép chuyển ở `devserveragent.Client.LastHandshakeInfo` (`client.go:489`) cũng phải sửa (SOL-033 mục 1, điều 1).
- `agent.exec` không có shell: `command -v` (CR-REQ-029) phải bọc `sh -c`, và Windows dùng `where.exe`. `git.exec` của agent không cho `ls-files` và cấm ký tự `\ ! < > | & ; $` trong tham số (liên quan SOL-029).
- `fs.glob` của agent chỉ khớp phần tên cuối bằng `find -name` (`fs-agent-extensions.ts` dòng 240 đến 255), không thực sự hiểu `**` (liên quan SOL-029).
- `ResolveConnection` trả `repo_path` và `worktree_id`, **không** trả đường dẫn worktree (trả lời Q3 của CR-REQ-029).
- Khoá `readOnly:true` giả định ở SOL-008 mục G và TASK-REQ-008-02 phải đổi thành `accessMode` (CR-REQ-033 chốt tên). Không sửa file đó; ghi ở báo cáo.
- Số migration `0039` đúng ngày 2026-10-06; luôn `ls` lại.
- Chưa chạy bất kỳ test nào; mọi lệnh test là lệnh dự kiến.
