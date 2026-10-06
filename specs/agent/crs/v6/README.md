# agent v6: năng lực của Dev Server Agent cho Request flow

> **Trạng thái: 📋 Proposed.** Chưa có dòng code nào. Soạn ngày 2026-10-06 từ đọc code `agent/src/` và TDD ở [`specs/agent/tdd/v4/`](../../tdd/v4/00-index.md). Chưa chạy `vitest`, `tsc`, hay `claude` thật.

Chỉ **một CR** chạm tới `agent/`: [CR-REQ-033](../../../../docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md). Các CR khác (008, 026, 029) dùng các thay đổi này qua backend. Phần backend tương ứng: [`specs/backend-go/crs/v6/agent-capabilities/`](../../../backend-go/crs/v6/agent-capabilities/solutions/README.md).

## 1. Nội dung

Feature [`agent-capabilities/`](./agent-capabilities/solutions/README.md): 3 solution, 13 task. ID `AG-REQ-SOL-033-*` và `AG-REQ-TASK-033-<01..13>`.

| Solution | Task | Nội dung |
|---|---|---|
| A: `exec-prompt-readonly-and-workspace` | 01 đến 04 | Tham số chế độ chỉ đọc (`accessMode`), chính sách công cụ chỉ đọc, kiểm tra `workspaceKind` và đường dẫn, nối vào handler |
| B: `result-block-changes-and-output-cap` | 05 đến 08 | Bộ đệm đầu ra có giới hạn, bộ phân tích khối kết quả `ORCA_RESULT_BEGIN/END <nonce>`, ảnh chụp thay đổi worktree, nối vào handler |
| C: `capability-report-handshake-and-ai-complete` | 09 đến 13 | Báo cáo năng lực dev server, RPC `agent.capabilities`, `ai.complete` trả `usage`/`maxTokens`/`error.data`, phiên bản giao thức và `features` trong handshake, nâng phiên bản agent |

Bảng giao diện method và tham số chung với backend nằm ở [`solutions/README.md`](./agent-capabilities/solutions/README.md) mục "Hợp đồng chung với backend".

## 2. Thứ tự và phụ thuộc

Làm solution A và B trước (độc lập với backend), solution C sau. Backend chỉ được bật chế độ chỉ đọc bắt buộc (`REQUEST_REQUIRE_ENFORCED_READONLY`) sau khi **test e2e đối kháng với `claude` thật** (task 04, tắt mặc định bằng `ORCA_REAL_CLAUDE_E2E=1`) đã chạy và đạt.

## 3. Điểm cần chốt

| # | Vấn đề |
|---|---|
| 1 | **Chế độ chỉ đọc dựa trên cờ của claude CLI chưa chạy thử**: `--permission-mode plan`, `--tools Read,Glob,Grep`. Chưa biết có chặn ghi thật, `plan` có kẹt chờ phê duyệt không, dạng dấu phẩy, tên công cụ. Agent từ chối chạy chỉ đọc nếu CLI thiếu cờ, không hạ cấp âm thầm |
| 2 | **Agent cũ bỏ qua tham số lạ im lặng**: `accessMode=readonly` gửi cho agent cũ sẽ chạy chế độ GHI. Đã thêm trường `applied` (echo tham số đã áp dụng) để backend phát hiện. Đây là bổ sung ngoài CR cần backend xác nhận |
| 3 | **Chỉ cần sửa `agent/`**: `desktop/src/relay` là bản đã lệch, không có `execPrompt`, và không có script đồng bộ; xác minh bằng `grep execPrompt desktop/src/relay` |
| 4 | **Ba số phiên bản agent khác nhau** (handshake cứng `5.0.0`, build `2.1.0`, `package.json` `1.4.138-rc.6`) nên `ORCA_AGENT_MIN_VERSION` vô tác dụng. Đề xuất thêm `protocolVersion`, `buildVersion`, `features`, không đổi `agentVersion` |
| 5 | **Nâng `AGENT_VERSION` có buộc đẩy lại bundle qua SSH hay không không chắc**: `sshrelay/provisioner.go:141` so với `ORCA_VERSION` của backend, không phải phiên bản agent mong muốn. Task 13 ghi quy trình thủ công và cảnh báo |
| 6 | **`agent.exec` không có shell** (binary và args): `command -v` phải bọc `sh -c`, Windows dùng `where.exe`; `go` dùng `go version`. `execPrompt` chạy `claude` với `PATH = config.toolPath` nên mọi dò `claude` phải dùng cùng PATH |
| 7 | **`MAX_MESSAGE_SIZE` 16 MiB**: sau khi JSON thoát ký tự, trần 12 MiB của CR có thể vượt khung; task 05 thêm `fitResultToFrame`. Đổi mặc định `maxOutputBytes` 4 MiB làm đổi hành vi cho người gọi cũ có đầu ra lớn |
| 8 | **`ai.complete`**: `max_tokens: 4096` cố định có thể cắt cụt JSON Plan; `buildAgentEnv` đặt `GEMINI_API_KEY` còn `ai.complete` đọc `GOOGLE_API_KEY`; không bao giờ backend gửi `resolvedApiKey` |
| 9 | **Handshake**: chưa rõ trường mới (`features`...) tới Go ở relay-websocket và relay-ssh; `usecase.HandshakeInfo` và `devserveragent.Client.LastHandshakeInfo` phải sửa phía backend |
| 10 | **Dispatcher có tuần tự hoá RPC trên một kết nối không**: nếu có, `agent.capabilities` mất tới 8 giây sẽ chặn RPC sau nó (task 10 ghi cần kiểm) |
| 11 | **`agent.execPromptStream` không truyền `taskId`/`projectId` vào `buildAgentEnv`**, khác handler thường; có sẵn, không sửa trong đợt này |

## 4. Giới hạn

- Không chạy `pnpm test` toàn gói và `tsc --noEmit` (tài liệu v4 ghi 53 lỗi kiểu có sẵn, chưa chạy lại). Không thử Windows, WSL, hiệu năng ảnh chụp worktree trên repo lớn. Tên trường `loggedIn` của `claude auth status --json`, tên trường `usage` của các nhà cung cấp, dạng `--version` của `openspec`, `codegraph`, `gitnexus`, `rg`, `semgrep` chưa kiểm chứng.
