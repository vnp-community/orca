# agent-capabilities (v6) solutions: index (agent)

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mọi nhận định "đã đọc" là đọc code ở `agent/` và `backend-go/services/infra-fleet-service`, chưa chạy hệ thống.

Solution phía `agent/` (Dev Server Agent) cho feature `agent-capabilities` của series v6 Request → Solution → Plan → Phase → Task. Feature này chỉ có MỘT CR: [CR-REQ-033](../../../../../../docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md), và đó là CR duy nhất của series v6 chạm `agent/`. Theo README v6 mục 8 (thắng mục 3) và CR-033, phần infra-fleet (proto, migration `0039`, hồ sơ năng lực) thuộc solution backend, không có ở đây.

## Bảng CR → Solution

CR-REQ-033 chạm `agent/` nhiều nhóm thay đổi, nên có ba solution (cùng số 033, khác slug). Quy ước ID: solution `AG-REQ-SOL-033-<slug>.md`, task `AG-REQ-TASK-033-<NN>-<slug>.md` với NN đánh số liên tục 01 đến 13 xuyên suốt ba solution để ID không trùng.

| CR | Solution | Nhóm trong CR-033 | Tasks |
|---|---|---|---|
| CR-REQ-033 | [AG-REQ-SOL-033-exec-prompt-readonly-and-workspace](./AG-REQ-SOL-033-exec-prompt-readonly-and-workspace.md) (A) | 2.1 (`accessMode`, `workspaceKind`), 2.2, 2.3 | 01, 02, 03, 04 |
| CR-REQ-033 | [AG-REQ-SOL-033-result-block-changes-and-output-cap](./AG-REQ-SOL-033-result-block-changes-and-output-cap.md) (B) | 2.1 (`reportChanges`, `resultBlock`, `maxOutputBytes`), 2.4, 2.5 | 05, 06, 07, 08 |
| CR-REQ-033 | [AG-REQ-SOL-033-capability-report-handshake-and-ai-complete](./AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md) (C) | 2.6, 2.7 (phía agent), 2.8 | 09, 10, 11, 12, 13 |

## Các CR khác trong v6

| CR | Khu vực `agent` | Ghi chú |
|---|---|---|
| CR-REQ-001 đến 025, 027, 028, 030 đến 032, 035, 036 | N/A | Không chạm `agent/`. Một số là người gọi RPC của agent qua `request-service`/`task-service`/`infra-fleet-service` (CR-REQ-005, 007, 008, 012, 013, 026, 029, 034) nhưng không đổi mã agent |
| CR-REQ-008 | N/A (người dùng) | `AgentReadonlyRunner` gọi `agent.execPrompt` với tham số mới: cần `accessMode`, `workspaceKind=repo_root`, `reportChanges`, `resultBlock.nonce`; đọc `applied`, `changes`, `warnings`, `parsed` |
| CR-REQ-029 | N/A (người dùng) | `SimpleExecutor` gửi `resultBlock`, `reportChanges` (Q4 của CR-029: tên tham số được CR-033 chốt tại đây, xem mục "Hợp đồng") |
| CR-REQ-034 | N/A (người dùng) | Đọc `usage`, `provider`, `latencyMs`, `error.data` của `ai.complete`, gửi `maxTokens` |
| CR-REQ-026 | N/A (người dùng) | Kiểm `openspec` qua `agent.exec` hiện có hoặc qua `agent.capabilities` (`tools[].id == "openspec"`) |

## Thứ tự phụ thuộc

```
A:  01 parseExecPromptOptions ──┬── 02 readonly tool policy ──┐
                                ├── 03 workspace validation ──┼── 04 nối vào hai handler ──┐
                                │                             │                             │
B:                              ├── 05 bounded buffer ────────┼─────────────────────────────┤
                                │   06 result block parser (độc lập)   ────────────────────┤
                                │   07 change snapshot (độc lập)       ────────────────────┤
                                │                                                          08 nối B vào hai handler
C:  02 ──► 09 capability report ──► 10 đăng ký agent.capabilities ───────────────────────────┤
    11 ai.complete (độc lập) ────────────────────────────────────────────────────────────────┤
                                                                                            12 handshake features
                                                                                            13 phiên bản và tương thích
```

Task 06, 07, 11 không phụ thuộc task nào và có thể làm đầu tiên, song song. Task 12 chỉ thêm tên vào `features` sau khi mã tương ứng (04, 08, 10, 11) đã có: không quảng cáo thứ chưa làm.

## Hai bản agent: xác minh lại (theo yêu cầu của CR-033)

CR-033 kết luận chỉ sửa `agent/`. Đã kiểm lại ngày 2026-10-06:

- `agent/src/relay/agent-print-mode-exec.ts` (382 dòng), `agent-binary-specs.ts`, `ai-complete-handler.ts`, `agent-rpc-dispatch-agent-exec.ts` tồn tại; `grep -rn execPrompt desktop/src/relay` không trả file nào, tức bản `desktop/src/relay/` không có `agent.execPrompt`. `desktop/src/relay/ai-complete-handler.ts` có nhưng là bản lệch (khác doc comment của bản `agent/`).
- `agent/build.mjs` (`AGENT_VERSION = '2.1.0'`) build `agent/src/relay/agent-entry.ts`; `desktop/config/scripts/build-agent-only.mjs` build bản riêng.
- Triển khai trỏ `agent/out/agent.js`: `deploy/agent/README.md` (`scp agent/out/agent.js`); `sshrelay.Config.BundlePath` đặt "agent/out/agent.js" (theo CR; chưa đọc lại `config.go` dòng đó trong đợt này).
- Kết luận giữ nguyên: chỉ sửa `agent/`. Bundle dựng từ `desktop/` không có method mới và không có `features`; backend coi như agent cũ.

## Quyết định chung

1. Mọi tham số mới đều tuỳ chọn; không có tham số thì kết quả giống hệt hiện nay (ngoại lệ có chủ ý: giới hạn `maxOutputBytes` mặc định 4 MiB, solution B).
2. An toàn đóng (fail closed): tham số an toàn sai kiểu thì lỗi (`InvalidParams` kèm `error.data.reason`), `readonly` mà CLI thiếu cờ thì `READONLY_MODE_UNSUPPORTED` và không chạy.
3. Logic mới ở file mới, tên cụ thể (không `helpers`, `utils`, `common`); không thêm `max-lines` disable; handler cũ chỉ thêm lời gọi.
4. Agent chạy trên chính host nên đúng cho SSH, WSL; chỉ dùng lệnh Git cơ bản dưới baseline 2.25 (`git rev-parse --show-toplevel`, `git --no-optional-locks status --porcelain=v1 -z --untracked-files=all`, `git rev-parse HEAD`).
5. Chỉ dùng cờ CLI `claude` mà CR-033 nêu: `--permission-mode plan`, `--tools`, và lệnh `claude auth status --json`, `claude --help`, `claude --version`. Hành vi thật của chúng với `--print` CHƯA KIỂM CHỨNG; thử nghiệm đối kháng trên `claude` thật (task 04) là điều kiện trước khi backend bật `REQUEST_REQUIRE_ENFORCED_READONLY`.
6. Agent cũ phải suy giảm được: backend phát hiện bằng `protocolVersion`/`features` ở handshake và bằng `applied` trong kết quả; không bao giờ suy luận từ `agentVersion` (luôn `5.0.0`).

## Hợp đồng chung với backend (tên chính xác)

| Hạng mục | Tên | Kiểu | Mặc định | Backend phải làm |
|---|---|---|---|---|
| Tham số | `accessMode` | `"write"`, `"readonly"` | `"write"` | Chỉ gửi `readonly` khi `features` có `agent.execPrompt.readonly`; đọc `applied.accessMode` |
| Tham số | `workspaceKind` | `"worktree"`, `"repo_root"`, `"scratch"` | `"worktree"` | `repo_root` bắt buộc đi với `readonly` |
| Tham số | `reportChanges` | boolean | `false` | Chỉ tin `changes` khi có mặt |
| Tham số | `resultBlock` | `{ nonce: string }`, nonce `^[A-Za-z0-9]{16,64}$` | vắng | Sinh nonce mới mỗi lần chạy; agent cũ thì tự phân tích `stdout` |
| Tham số | `maxOutputBytes` | số nguyên 65536 đến 12582912 (kẹp trên) | 4194304 | Chấp nhận `truncated` |
| Kết quả mới | `applied`, `changes`, `parsed`, `truncated`, `warnings` | xem solution A, B | vắng khi không dùng | `applied` vắng nghĩa agent cũ đã bỏ qua tham số |
| Frame `stream.end` | cùng các trường trên, cạnh `exitCode` | | | Decoder Go phải chấp nhận trường thừa ở `stream.end` (chưa xác nhận) |
| RPC mới | `agent.capabilities` params `{ tools?, envNames?, refresh? }` | | | Hồ sơ `profile_json` giữ nguyên JSON; `-32601` thì `handshake_only` |
| Handshake | `protocolVersion` (số, vắng = 1), `buildVersion`, `features` | | | Mở rộng `usecase.HandshakeInfo`, `inboundHandshakeParams`, `sshrelay` |
| `ai.complete` | `maxTokens` (4096, trần 16384); kết quả `provider`, `latencyMs`, `usage?`; lỗi `error.data { provider, httpStatus, retryable, reason }` | | | Chỉ tin khi `features` có `ai.complete.usage` |
| Mã lỗi | `INVALID_ACCESS_MODE`, `INVALID_WORKSPACE_KIND`, `INVALID_REPORT_CHANGES`, `INVALID_RESULT_BLOCK`, `INVALID_RESULT_BLOCK_NONCE`, `INVALID_MAX_OUTPUT_BYTES`, `READONLY_MODE_UNSUPPORTED`, `REPO_ROOT_REQUIRES_READONLY`, `WORKSPACE_PATH_NOT_FOUND`, `WORKSPACE_NOT_A_DIRECTORY`, `WORKSPACE_NOT_A_GIT_REPO`, `SCRATCH_OUTSIDE_ALLOWED_ROOTS`, `INVALID_CAPABILITY_PARAMS`, `TOO_MANY_ENV_NAMES`, `INVALID_MAX_TOKENS` | JSON-RPC `InvalidParams -32602`, mã trong `message` và `error.data.reason` | | Map sang `REQUEST_*` ở backend |
| Cảnh báo | `TRUST_PRESET_IGNORED_READONLY`, `READONLY_VIOLATION` | trong `warnings[]` | | CR-REQ-008 loại kết quả khi `READONLY_VIOLATION` |
| Mã khối kết quả | `RESULT_BLOCK_MISSING`, `RESULT_BLOCK_INVALID_JSON`, `RESULT_BLOCK_TOO_LARGE`, `RESULT_BLOCK_NOT_OBJECT` | trong `parsed.code` | | CR-REQ-029 quyết định thử lại hay `agent_defect` |

Bổ sung ngoài CR-033 (cần backend xác nhận): `applied` echo; `truncated` là `{ stdout, stderr }`; `error.data.reason` cho `ai.complete`; `unknownTools`, `rejectedEnvNames`, `installed: null` trong `agent.capabilities`; `stream.end` mang trường mới.

## Mâu thuẫn và sai khác phát hiện

1. **`sshrelay` không so với "phiên bản agent mong muốn"** (khác CR-033 mục 2.7): `provisioner.go:141` so `AGENT_VERSION` của bundle xa với `p.cfg.OrcaVersion` (biến `ORCA_VERSION` của backend). Tăng `AGENT_VERSION` lên `2.2.0` không chắc kích hoạt đẩy lại; chưa biết `ORCA_VERSION` thật. Task 13 ghi quy trình thủ công và cảnh báo; backend cần chốt cách buộc đẩy lại.
2. **`go --version` không tồn tại**: CR ghi mọi công cụ dùng `--version`; `go` dùng `go version` (task 09).
3. **`claude` chạy với `PATH = config.toolPath`**, không phải `PATH` của tiến trình; mọi dò `claude` phải dùng cùng PATH (task 02, 09).
4. **Handler stream lệch handler thường**: `agent.execPromptStream` không truyền `taskId`/`projectId` vào `buildAgentEnv`. Không thuộc CR-033; ghi ở solution A (câu hỏi mở 1).
5. **Agent cũ bỏ qua tham số lạ im lặng**: backend gửi `accessMode=readonly` cho agent cũ sẽ chạy chế độ ghi. CR chỉ nói "backend gọi method mới khi `features` có"; solution thêm `applied` echo để chốt hai lớp (A, mục 3.5).
6. **Đường handshake ở relay-websocket và relay-ssh chưa kiểm chứng**: trong `agent/` chỉ có phía agent khởi xướng `agent.handshake`; Go `runInitiatorHandshake` gửi request rồi đọc phản hồi. Chưa rõ trường mới tới Go ở hai chế độ này (solution C mục 6, task 12).
7. **`GEMINI_API_KEY` và `GOOGLE_API_KEY`** lệch giữa `buildAgentEnv` và `ai.complete` (solution C, câu hỏi mở 4).
8. **Khung 16 MiB sau khi JSON thoát ký tự**: trần `maxOutputBytes` 12 MiB của CR có thể vượt khung sau mã hoá; task 05 thêm `fitResultToFrame`.
9. **README v6 mục 8** chưa có dòng về ba nguồn số phiên bản agent (CR-033 mục 8 đã đề nghị thêm); không sửa ở đây theo quy định.
10. CR-REQ-029 Q4 (tên tham số `result_nonce`, `resultBlock`, `reportChanges`) được chốt ở đây là `resultBlock.nonce`, `reportChanges`; nếu `task-service` dùng tên khác ở proto thì ánh xạ khi gọi relay.

## Điểm chưa kiểm chứng (toàn feature)

- Hành vi `claude --print --permission-mode plan --tools Read,Glob,Grep` (có chặn ghi, có kẹt chờ phê duyệt, dạng dấu phẩy có được nhận): chưa chạy.
- Tên trường `loggedIn` của `claude auth status --json`; tên trường `usage` của Anthropic, OpenAI, Google; dạng `--version` của `openspec`, `codegraph`, `gitnexus`, `rg`, `semgrep`: chưa kiểm chứng.
- `typecheck` toàn gói (`npx tsc --noEmit`): tài liệu v4 ghi 53 lỗi có sẵn ngày 2026-09-09, chưa chạy lại.
- Test Windows/WSL của `agent-workspace-validation` và `agent-capability-report`: chưa chạy.
- Kết quả `pnpm test` toàn gói hiện tại: chưa chạy trong đợt soạn tài liệu này.

## Tài liệu liên quan

- CR: [CR-REQ-033](../../../../../../docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md), [README feature](../../../../../../docs/crs/v6/agent-capabilities/README.md), `docs/crs/v6/solution-analysis/CR-REQ-008-diagnosis-findings-answer-analysis.md`, `docs/crs/v6/execution-contract/CR-REQ-029-execution-contract-and-readiness-gate.md`, `docs/crs/v6/ai-governance/CR-REQ-034-ai-governance-budgets-evals-prompt-versioning.md`
- TDD: [v5/00-index](../../../../tdd/v5/00-index.md), [v5/04](../../../../tdd/v5/04-handshake-session.md), [v5/07](../../../../tdd/v5/07-jsonrpc-dispatch.md), [v5/08](../../../../tdd/v5/08-deployment.md), [v5/09](../../../../tdd/v5/09-ai-credential-relay.md), [v5/12](../../../../tdd/v5/12-agent-spawner.md)
- Tasks: [../tasks/README.md](../tasks/README.md)
- Mẫu: `specs/agent/crs/v4/task-graph/solutions/README.md`
