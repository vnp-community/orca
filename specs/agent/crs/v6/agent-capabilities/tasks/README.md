# agent-capabilities (v6) tasks: index (agent)

**Solutions:** [../solutions/](../solutions/README.md)

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mỗi task viết sau khi đọc lại code thật ở `agent/src/relay/` (đã đọc: `agent-print-mode-exec.ts`, `agent-rpc-dispatch-agent-exec.ts`, `agent-rpc-dispatch-misc.ts`, `agent-rpc-dispatch-ai.ts`, `agent-session-handshake.ts`, `agent-session-capabilities.ts`, `ai-complete-handler.ts`, `agent-spawn-env.ts`, `agent-binary-specs.ts`, `agent-config.ts`, `agent-entry.ts`, `build.mjs`, `package.json`, `vitest.config.ts`) và phía Go `infra-fleet-service` để giao diện khớp. Task nào lệch với solution cha đều ghi chỗ lệch trong mục Context của nó.

Tất cả task chạy trong `agent/` (gói `orca-agent`, TypeScript, vitest). Lệnh test: trong `/opt/repos/orca/agent`, `pnpm exec vitest run <đường dẫn test>` cho từng file, `pnpm test` (script `vitest run`) cho toàn gói; `vitest.config.ts` gom `src/**/*.test.ts`, môi trường node. Kiểm kiểu: `npx tsc --noEmit` (chỉ so sánh trước và sau; không có script typecheck trong `package.json`).

## Solution → Task

| Solution | Task | Tên | Ưu tiên | File chính |
|---|---|---|---|---|
| [A: exec-prompt-readonly-and-workspace](../solutions/AG-REQ-SOL-033-exec-prompt-readonly-and-workspace.md) | [01](./AG-REQ-TASK-033-01-exec-prompt-options-parser.md) | Bộ đọc và kiểm tham số mới | P1 | `agent-exec-prompt-options.ts` (mới) |
| | [02](./AG-REQ-TASK-033-02-readonly-tool-policy.md) | Chính sách công cụ chỉ đọc, dò cờ `claude` | P1 | `agent-readonly-tool-policy.ts` (mới) |
| | [03](./AG-REQ-TASK-033-03-workspace-validation.md) | Kiểm vùng làm việc `worktree/repo_root/scratch` | P1 | `agent-workspace-validation.ts` (mới) |
| | [04](./AG-REQ-TASK-033-04-wire-readonly-and-workspace-into-handlers.md) | Nối chỉ đọc và vùng làm việc vào hai handler, thử nghiệm đối kháng | P1 | `agent-print-mode-exec.ts`, `agent-readonly-adversarial.e2e.test.ts` (mới) |
| [B: result-block-changes-and-output-cap](../solutions/AG-REQ-SOL-033-result-block-changes-and-output-cap.md) | [05](./AG-REQ-TASK-033-05-bounded-output-buffer.md) | Bộ đệm đầu ra có giới hạn, `maxOutputBytes` | P1 | `agent-bounded-output-buffer.ts` (mới) |
| | [06](./AG-REQ-TASK-033-06-result-block-parser.md) | Bộ phân tích khối `ORCA_RESULT` | P1 | `agent-result-block-parser.ts` (mới) |
| | [07](./AG-REQ-TASK-033-07-worktree-change-snapshot.md) | Chụp và so sánh file thay đổi | P1 | `agent-worktree-change-snapshot.ts` (mới) |
| | [08](./AG-REQ-TASK-033-08-wire-result-block-and-changes-into-handlers.md) | Nối khối kết quả, `changes`, cảnh báo vào hai handler | P1 | `agent-print-mode-exec.ts` |
| [C: capability-report-handshake-and-ai-complete](../solutions/AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md) | [09](./AG-REQ-TASK-033-09-capability-report-core.md) | Lõi báo cáo năng lực | P1 | `agent-capability-report.ts`, `agent-build-version.ts` (mới) |
| | [10](./AG-REQ-TASK-033-10-capabilities-rpc-dispatch.md) | Đăng ký `agent.capabilities`, golden JSON | P1 | `agent-rpc-dispatch-misc.ts` |
| | [11](./AG-REQ-TASK-033-11-ai-complete-usage-maxtokens-error-data.md) | `ai.complete`: `usage`, `maxTokens`, `error.data` | P1 | `ai-complete-handler.ts`, `agent-rpc-dispatch-ai.ts` |
| | [12](./AG-REQ-TASK-033-12-handshake-protocol-version-and-features.md) | Handshake: `protocolVersion`, `buildVersion`, `features` | P1 | `agent-protocol-features.ts` (mới), `agent-session-handshake.ts` |
| | [13](./AG-REQ-TASK-033-13-agent-version-bump-and-compatibility-check.md) | Tăng `AGENT_VERSION`, triển khai, kiểm tương thích | P1 | `build.mjs`, `agent-entry.ts`, `deploy/agent/*` |

Trạng thái mọi task: `[ ] TODO`.

## Thứ tự phụ thuộc

```
Làm trước, song song (không phụ thuộc):   06   07   11   01   02   03   05*   09**
                                          (* 05 cần 01;  ** 09 cần 02)

01 ──► 02 ┐
01 ──► 03 ┴──► 04 ──┐
01 ──► 05 ───────────┤
       06 ───────────┼──► 08 ──┐
       07 ───────────┘         │
02 ──► 09 ──► 10 ──────────────┼──► 12 ──► 13
       11 ─────────────────────┤
                    04 ────────┘
```

Rút gọn: 12 phụ thuộc 04, 08, 10, 11; 13 phụ thuộc 12. Lý do: handshake `features` là cam kết nên chỉ thêm sau khi mã đã có; tăng phiên bản và kiểm tương thích là bước cuối.

Gợi ý chia việc cho hai kỹ sư: một người làm 01, 02, 03, 04, 05, 08 (luồng `execPrompt`), một người làm 06, 07, 09, 10, 11 (khối nền độc lập) rồi cả hai gặp ở 12 và 13. 06, 07, 11 có thể giao ngay.

## Hợp đồng chung với backend (tóm tắt; bảng đầy đủ ở README solutions)

Mọi task dùng đúng các tên sau, khớp CR-REQ-033 và README v6 mục 8: tham số `accessMode`, `workspaceKind`, `reportChanges`, `resultBlock.nonce`, `maxOutputBytes`; kết quả `applied`, `changes`, `parsed`, `truncated`, `warnings`; RPC `agent.capabilities`; handshake `protocolVersion`, `buildVersion`, `features`; `ai.complete` `maxTokens`, `provider`, `latencyMs`, `usage`, `error.data`.

Phần backend PHẢI khớp (không nằm trong các task này):

1. `infra-fleet-service`: mở rộng `usecase.HandshakeInfo` (`ports.go`) thêm `Capabilities`, `Features`, `ProtocolVersion`, `BuildVersion`; đọc ở `agentwsserver` (`inboundHandshakeParams`), `sshrelay`, `devserveragent`; RPC `GetDevServerCapabilities`, migration `0039` hai dialect, use case `RefreshDevServerCapabilities` (agent cũ `-32601` thì `handshake_only`). Dùng tệp golden của task 10 cho decoder.
2. `request-service` (CR-REQ-008): chọn đường `agent_readonly` theo `features`; truyền `accessMode=readonly`, `workspaceKind=repo_root`, `reportChanges=true`, `resultBlock.nonce`; ĐỌC `applied.accessMode` trước khi tin chỉ đọc; loại kết quả khi `warnings` có `READONLY_VIOLATION`.
3. `task-service` (`SimpleExecutor`, CR-REQ-029): gửi `resultBlock`, `reportChanges`; đọc `parsed`; với agent cũ tự phân tích `stdout` bằng đúng thuật toán của task 06; chấp nhận `truncated` và giới hạn 4 MiB mặc định (thay đổi hành vi cho người gọi cũ, task 05).
4. Người gọi `ai.complete` (`task-service` `aidecompose_relay.go`, `git-gateway-service` `generate_commit_message.go`, CR-REQ-034): gửi `maxTokens` chỉ khi `features` có `ai.complete.usage`; đọc `error.data`.
5. `sshrelay` (`provisioner.go:141`): chốt cách buộc đẩy lại bundle khi nâng `AGENT_VERSION` (task 13).
6. Decoder `stream.end` ở `devserveragent`: chấp nhận trường thừa (task 08).

## Triển khai và tương thích

- Thứ tự: agent mới trước, backend mới sau. Backend mới chỉ gọi method hoặc tham số mới khi `features` có; nên không có cửa sổ lỗi. Backend cũ gọi agent mới không đổi gì (trừ giới hạn `maxOutputBytes` mặc định).
- Agent cũ (không có `features`, hoặc bundle build từ `desktop/`): `agent.capabilities` trả `-32601`; tham số mới bị bỏ qua IM LẶNG (nguy hiểm nhất là `accessMode=readonly` chạy ở chế độ ghi); phát hiện bằng `protocolVersion`/`features` và bằng việc kết quả thiếu `applied`.
- Task 13 chứa quy trình nâng cấp và ma trận tương thích; kiểm tay trên dev server thật là điều kiện trước khi bật `request_flow_enabled` (CR-REQ-025).

## Kiểm với `claude` thật hay giả lập

| Nội dung | Cách kiểm | Trạng thái |
|---|---|---|
| Dựng `args` chỉ đọc, không cờ YOLO, từ chối khi thiếu cờ | `spawn` giả (`FakeChild`) và probe `claude --help` giả (task 02, 04) | tự động, chưa chạy |
| Script `claude` giả ghi `argv` | thư mục tạm có script Node, đặt `PATH` qua `params.env` (task 04); bỏ qua Windows; việc `spawn` tìm `claude` theo `env.PATH` của con chưa kiểm chứng trên Node 22 | tự động, chưa chạy |
| Chặn ghi thật của `--permission-mode plan` và `--tools Read,Glob,Grep` | `agent-readonly-adversarial.e2e.test.ts`, bật bằng `ORCA_REAL_CLAUDE_E2E=1` trên dev server có `claude` đã đăng nhập | chưa chạy; bắt buộc trước khi bật `REQUEST_REQUIRE_ENFORCED_READONLY` |
| `claude auth status --json`, `claude --version`, `claude --help` | gọi tay và dán đầu ra làm fixture (task 02, 09) | chưa chạy; tên trường `loggedIn` chưa kiểm chứng |
| Khối `ORCA_RESULT` với mô hình thật | gọi tay `agent.execPrompt` kèm nonce (task 08) | chưa chạy |
| `usage` của nhà cung cấp `ai.complete` | một lần gọi tay với khoá thử (task 11) | chưa chạy |

Không dùng cờ CLI nào ngoài các cờ CR-033 đã nêu (`--permission-mode plan`, `--tools`; lệnh `claude auth status --json`, `claude --help`, `claude --version`).

## Ghi chú chung

- Mọi file mới đặt tên theo khái niệm cụ thể (theo `AGENTS.md`); không thêm `max-lines` disable.
- Mọi tên RPC, tham số, mã lỗi trong task trùng nguyên văn với bảng "Hợp đồng chung" ở README solutions.
- Các TDD cần cập nhật SAU khi triển khai (người điều phối làm, không sửa trong task): `specs/agent/tdd/v5/04-handshake-session.md`, `07-jsonrpc-dispatch.md`, `08-deployment.md`, `09-ai-credential-relay.md`.
- Mẫu định dạng: `specs/agent/crs/v4/task-graph/tasks/README.md`.
