# CR-PW-008 — Agent/terminal session trong Project Workspace bị "mất việc" khi disconnect; lỗi `GITGATEWAY_STATUS_FAILED` không chẩn đoán được vì thiếu log

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-PW-008 |
| **Tên** | (a) Xác minh/củng cố cơ chế grace-period cho worktree agent session; (b) mở rộng `apperrors` cause-logging sang `git-gateway-service` |
| **Loại** | Reliability / Observability |
| **Priority** | 🟡 P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-15 |
| **Trạng thái** | 🟡 Hedged — (a) nghi vấn chưa confirm bằng repro thật; (b) 🔲 Proposed, chưa code |
| **Tác giả** | User report: *"nhánh worktree đang làm việc đang là sync theo terminal => nhanh bị ngắt và khi out app bị ngắt"*; lỗi `GITGATEWAY_STATUS_FAILED` lặp lại nhiều lần trong phiên làm việc |
| **Tác động HLD** | `agent/src/relay/agent-session.ts`, `agent-pty-registry.ts`, `pty-agent-bridge.ts`; `git-gateway-service` |
| **Tác động Features** | Project Workspace (Beta) — tab Agent, tab Git |

---

## Bối cảnh & Vấn đề gốc

### (a) User cần: 1 worktree = 1 session bền vững, không mất việc khi mất kết nối/thoát app

Đọc source xác nhận: **đã có cơ chế đúng ý này** — [CR-STORAGE-008](./../storage/CR-STORAGE-008-reconnect-resume-semantics.md) phần (b), implement qua `SOL-AG-STORAGE-003`: grace-period 120s cho cả terminal PTY (`pty-agent-bridge.ts`) và AI-agent CLI PTY (`agent-pty-registry.ts`), arm khi WS đóng, huỷ khi reconnect đúng session trong 120s.

**Vấn đề**: user vẫn report bị ngắt "nhanh" — mâu thuẫn với thiết kế đã có. 3 giả thuyết CHƯA xác định được cái nào đúng (xem đầy đủ ở [BUG-AG-ORCH-014](../../../../specs/agent/bugs/agent-orchestration/BUG-AG-ORCH-014-worktree-terminal-session-interrupted-on-disconnect.md)):
1. UI báo "đã ngắt" sớm hơn thực tế backend (PTY vẫn sống trong grace window).
2. Reconnect không match đúng `ptyId`/session cũ → tạo phiên mới thay vì reattach.
3. Trường hợp thoát app hẳn có thể không đi đúng luồng như network blip thường.

Phát hiện phụ: doc `SOL-AG-STORAGE-003` đang ghi sai trạng thái "chưa implement" trong khi code đã implement thật (2026-09-07) — cần đồng bộ lại.

**Phân biệt quan trọng cho user**: cơ chế 120s này chỉ dành cho phiên **tương tác trực tiếp** (Agent tab, PTY). Với việc "giao task rồi để đó" thật sự — `orchestration-service`'s `CoordinatorRun`/`OrchestrationTask` (DAG, Postgres-persisted, polling-driven, **không phụ thuộc kết nối WS nào**) mới là mô hình đúng cho async task bền vững vô thời hạn — khác hẳn Agent tab. Cần làm rõ trong tài liệu/UI để user chọn đúng mô hình cho đúng nhu cầu.

### (b) `GITGATEWAY_STATUS_FAILED` — không chẩn đoán được vì `git-gateway-service` không có cause-logging

`apperrors.SetLogger` (SOL-009) chỉ wired ở `infra-fleet-service`. `git-gateway-service`'s `GetStatus` wrap lỗi thật (`err`) vào `GITGATEWAY_STATUS_FAILED` nhưng không log ra đâu cả — xác nhận qua log: relay call (`RelayByDevServer`) **thành công** (`rpc ok`), nghĩa là lỗi nằm ở tầng xử lý response của `git-gateway-service`, nhưng không cách nào thấy được nội dung thật. Xem [BUG-012](../../../../specs/backend-go/bugs/missing-v2/BUG-012-gitgateway-status-failed-opaque-relay-error.md).

## Giải pháp đề xuất

1. **(a) Xác minh bằng log thật** — lần tới tái hiện được, đối chiếu đúng thời điểm: log agent (`"Session closed code=..."`, `"scheduleAgentSpawnGracePeriod: armed..."`) vs. thời điểm UI báo "ngắt" — xác định đúng giả thuyết trong 3 cái trên trước khi sửa code (tránh sửa sai chỗ).
2. **(a) Doc fix**: cập nhật `SOL-AG-STORAGE-003` từ "chưa implement" → "đã implement 2026-09-07".
3. **(a) UX**: làm rõ trong UI (tooltip/help text) sự khác biệt Agent tab (tương tác, giới hạn ~120s reconnect) vs. Tasks/Automations (async thật, không giới hạn) — để user chọn đúng mô hình.
4. **(b) SOL-012**: wire `apperrors.SetLogger` vào `git-gateway-service/cmd/server/main.go`, đúng pattern SOL-009. Sau deploy, tái hiện lỗi để lấy nguyên nhân thật, cập nhật BUG-012.
5. **(b) Cân nhắc lâu dài**: 1 helper bootstrap dùng chung (`common/serverboot`) tự wire `apperrors.SetLogger` cho MỌI service — đây đã là gap thứ 3 lặp lại (infra-fleet-service, project-service qua BUG-010, giờ git-gateway-service) theo đúng pattern lỗi lặp lại nhiều lần trong `missing-v2/solutions/README.md`'s "Cross-cutting design theme".

## Không thuộc phạm vi CR này

- Chuyển hẳn Agent tab sang mô hình chạy vô thời hạn không giới hạn 120s — đây là quyết định sản phẩm cần bàn riêng (đánh đổi resource leak vs. bền vững), không tự quyết trong CR này.

## Liên quan

- [CR-STORAGE-008](../storage/CR-STORAGE-008-reconnect-resume-semantics.md) — CR gốc, CR này là phần theo dõi/xác minh tiếp
- [BUG-AG-ORCH-011](../../../../specs/agent/bugs/agent-orchestration/BUG-AG-ORCH-011-pty-registry-orphaned-on-ws-disconnect.md), [BUG-AG-ORCH-014](../../../../specs/agent/bugs/agent-orchestration/BUG-AG-ORCH-014-worktree-terminal-session-interrupted-on-disconnect.md)
- [BUG-012](../../../../specs/backend-go/bugs/missing-v2/BUG-012-gitgateway-status-failed-opaque-relay-error.md) / [SOL-012](../../../../specs/backend-go/bugs/missing-v2/solutions/SOL-012-git-gateway-service-cause-logging.md)
- [BUG-009](../../../../specs/backend-go/bugs/missing-v2/BUG-009-infra-agent-exec-failed-generic-relay-error.md) / [SOL-009](../../../../specs/backend-go/bugs/missing-v2/solutions/SOL-009-apperrors-optional-cause-logging.md) — nguồn gốc pattern cause-logging
