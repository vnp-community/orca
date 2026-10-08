# Kiểm toán thực thi: agent / agent-capabilities (CR-REQ-033)

Ngày: 2026-10-07. Phạm vi: 13 task `AG-REQ-TASK-033-01..13`, 3 solution `AG-REQ-SOL-033-*` (A exec-prompt-readonly-and-workspace, B result-block-changes-and-output-cap, C capability-report-handshake-and-ai-complete). Code: `/opt/repos/orca/agent/src/relay`, đối chiếu `desktop/src/relay` và phía Go `backend-go/services/infra-fleet-service`.

## 1. Lệnh đã chạy

| Lệnh (trong `/opt/repos/orca/agent`) | Kết quả thật |
|---|---|
| `pnpm exec vitest run` (toàn bộ) | 2 file lỗi, 522 pass, 3 skip (527). 26 test lỗi, 4965 pass, 11 skip |
| `pnpm exec vitest run` riêng 13 file CR-033 (exec-prompt-options, readonly-tool-policy, workspace-validation, bounded-output-buffer, result-block-parser, worktree-change-snapshot, capability-report, print-mode-exec, rpc-dispatch-ai, rpc-dispatch-misc, protocol-features, compat-matrix, readonly-adversarial.e2e) | 12 file pass, 1 skip (e2e); 117 pass, 1 skip |
| `pnpm exec tsc --noEmit` | 184 lỗi TS, exit 1. Toàn bộ nằm ở `src/renderer/**` và `src/shared/code-intel-types.test.ts` (thiếu `expect`, kiểu `FeatureWallSetupStepId`...). Không có lỗi nào ở file `src/relay` của CR-033 (grep tên file CR-033 trong log: không ra) |

26 test lỗi, phân loại:
- 21 test `src/relay/subprocess.test.ts` (timeout 10 giây, test spawn tiến trình con). File này không bị `f7c16b6cc` sửa. Nhiều khả năng lỗi môi trường/có sẵn, chưa tách nguyên nhân (không chạy trên cây sạch trước commit).
- 5 test `src/relay/__tests__/agent-tool-registry.test.ts > runToolCommand options` (stdout rỗng, timeout, `expected 1 to be 124`). 60 dòng test này do `f7c16b6cc` thêm (thuộc nhánh code-intel v7), không liên quan CR-033. Do thay đổi của commit hoặc môi trường, chưa phân định.
- Không test CR-033 nào lỗi.

Không chạy được: `agent-readonly-adversarial.e2e.test.ts` bị skip (cần `ORCA_REAL_CLAUDE_E2E=1` và `claude` thật). Không chạy `pnpm build` và kiểm `out/agent.js` (task 13), không chạy tay trên dev server.

## 2. Tóm tắt theo solution

| Solution | Task | Đủ | Một phần | Chưa làm | Không kiểm chứng được | Tỉ lệ Đủ |
|---|---|---|---|---|---|---|
| A (01-04) | 4 | 3 (01,02,03) | 1 (04) | 0 | 0 | 75% |
| B (05-08) | 4 | 3 (05,06,07) | 1 (08) | 0 | 0 | 75% |
| C (09-13) | 5 | 4 (09,10,11,12) | 1 (13) | 0 | 0 | 80% |
| Tổng | 13 | 10 | 3 | 0 | 0 | 77% |

Cả 13 task ghi `[x]`/DONE, 3 task có verdict "Một phần" (04, 08, 13): trạng thái sai ở 3 task đó. Đính chính: nhiều mục thủ công/thực địa ghi "chưa chạy" trong chính các file task, nên `[x]` ở các tiêu chí đó chỉ đúng với phần tự động.

## 3. Bảng từng task

| Task | Ghi trong file | Verdict | Bằng chứng | Thiếu hoặc sai |
|---|---|---|---|---|
| 01 exec-prompt-options | [x] | Đủ | `agent-exec-prompt-options.ts:36-160` parse accessMode/workspaceKind/reportChanges/resultBlock(nonce regex 16-64)/maxOutputBytes (min 64KiB, clamp 12MiB); `toExecPromptErrorResponse` có `data.reason`; gọi từ `agent-print-mode-exec.ts:97`; test pass | không |
| 02 readonly-tool-policy | [x] | Đủ | `agent-readonly-tool-policy.ts`: `detectClaudeFlags` (cache TTL 10 phút theo PATH, single-flight), `readonlyUnsupportedReason`, `buildReadonlyArgs` = `--permission-mode plan --tools Read,Glob,Grep`, lỗi `READONLY_MODE_UNSUPPORTED` + `data.detail`; test 11 ca pass | tên công cụ và dạng `--tools a,b,c` ghi "UNVERIFIED" trong code (dòng 17, 91), chưa xác nhận với claude thật |
| 03 workspace-validation | [x] | Đủ | `agent-workspace-validation.ts` (186 dòng) + test (186 dòng) pass; gọi từ `agent-print-mode-exec.ts:105` và `:402` | không |
| 04 wire readonly/workspace | [x] | Một phần | Nối thật: `agent-print-mode-exec.ts:184-205` (dò cờ, từ chối trước spawn, cờ readonly đặt sau prompt, bỏ YOLO + `TRUST_PRESET_IGNORED_READONLY`), `:301-309` (`applied` chỉ khi có tham số tường minh), stream `:463-489`. Dispatcher chuyển `params` nguyên vẹn (`agent-rpc-dispatch-agent-exec.ts:176-186`) | (1) Không có test cấp handler cho readonly: `grep readonly agent-print-mode-exec.test.ts` rỗng; hai `describe` mà task yêu cầu không tồn tại. (2) `agent-compat-matrix.test.ts:21` mock luôn `agent-readonly-tool-policy`, nên ca `applied.accessMode==='readonly'` không kiểm argv thật. (3) Không có test "CLI thiếu cờ thì không gọi spawn" ở cấp handler. (4) E2E đối kháng xem mục 4 |
| 05 bounded-output-buffer | [x] | Đủ | `agent-bounded-output-buffer.ts` giữ phần đuôi theo byte, giữ UTF-8; `BoundedOutputBuffer`/`splitOutputBudget`/`fitResultToFrame` được dùng ở `agent-print-mode-exec.ts:26,215-216`; test pass | không |
| 06 result-block-parser | [x] | Đủ | `agent-result-block-parser.ts`: tìm END cuối cùng cùng nonce, BEGIN gần nhất phía trước, giới hạn 256KiB trước `JSON.parse`, 4 mã lỗi, không lộ body trong `detail`; test 183 dòng pass | không |
| 07 worktree-change-snapshot | [x] | Đủ | `agent-worktree-change-snapshot.ts` (291 dòng), `captureSnapshot`/`diffSnapshots` dùng ở `agent-print-mode-exec.ts:28,208-278`; test pass | chỉ 6 ca (`it(`) cho 291 dòng; chưa kiểm sâu nhánh `directory`/scratch |
| 08 wire result-block/changes | [x] | Một phần | Nối thật: parse nonce `:292-293`, `READONLY_VIOLATION` `:282-288`, `truncated` `:297`; stream `:500-530` | Không có test cấp handler cho `resultBlock`, `reportChanges`, `truncated`, `READONLY_VIOLATION` (grep trong `agent-print-mode-exec.test.ts` và compat-matrix: rỗng). Stream: bộ đệm phân tích cứng 4MiB (`:501`), không theo `maxOutputBytes` |
| 09 capability-report core | [x] | Đủ | `agent-capability-report.ts`: allowlist công cụ (`:24`), env chỉ present/absent (`:89`), validate (`:111-150`), cache/single-flight; test 11 ca pass | không |
| 10 capabilities RPC | [x] | Đủ | `agent-rpc-dispatch-misc.ts:169-181` đăng ký `case 'agent.capabilities'` gọi `handleAgentCapabilities`; `dispatchMiscRpc` được gọi ở `agent-rpc-dispatch.ts:364`; test `agent-rpc-dispatch-misc.test.ts:162-193` pass | không |
| 11 ai.complete usage/maxTokens/error.data | [x] | Đủ | `ai-complete-handler.ts:36,108-111` (clamp 1..32768), `:146` trả `provider/latencyMs/usage`; `AICompleteProviderError.errorData` (`:69`, `reason/httpStatus/retryable/provider`); `agent-rpc-dispatch-ai.ts:141-175` chuyển `maxTokens`, gắn `error.data`; test `agent-rpc-dispatch-ai.test.ts:70-122` pass | không |
| 12 handshake protocol/features | [x] | Đủ | `agent-protocol-features.ts` (`AGENT_PROTOCOL_VERSION=2`, 8 feature); `agent-session-handshake.ts:63,75-77` gửi `protocolVersion`, `buildVersion`, `features`, giữ `agentVersion:'5.0.0'`; test pass | không |
| 13 version bump + compat | [x] | Một phần | `agent/build.mjs:22` = `2.2.0`; `deploy/agent/package.json:3` = `2.2.0`; README mục "Nâng cấp lên 2.2.0" (`:154-170`) có cảnh báo `provisioner.go:141`; `agent-compat-matrix.test.ts` pass | Chưa chạy `pnpm build` kiểm `out/.agent-version` và `require('./out/agent.js').AGENT_VERSION` (không chạy ở đây). Log `agent-entry.ts:84,116` dùng `AGENT_BUILD_VERSION` (file `agent-build-version.ts`) thay cho `AGENT_VERSION`: tương đương vì cùng `__AGENT_VERSION__`. Chú thích đầu file `agent-entry.ts:2` vẫn ghi "v2.1". Kiểm tay dev server 6 bước: không chạy. `toàn gói pnpm test xanh` sai: 26 test lỗi (mục 1) |

## 4. Kiểm tra các điểm được yêu cầu

- execPrompt nhận `accessMode`: có (`agent-exec-prompt-options.ts`, nối ở `agent-print-mode-exec.ts:97`; cả bản stream).
- Trả `applied`: có, chỉ khi người gọi gửi `accessMode` hoặc `workspaceKind` (`:301-309`, stream `:487-489`). Có test (`agent-compat-matrix.test.ts:131-151`) nhưng với policy bị mock.
- Từ chối khi CLI thiếu cờ: có trong code (`:184-191`, `:463-470`, trả `READONLY_MODE_UNSUPPORTED` trước spawn). Test của hàm thuần pass; không có test cấp handler xác nhận spawn không được gọi.
- Khối `ORCA_RESULT_BEGIN/END <nonce>`: parser đúng và test; nonce được kiểm regex. Agent không tự chèn chỉ dẫn nonce vào prompt (do backend/CR-029, không thuộc agent).
- `agent.capabilities` đăng ký trong dispatcher: có (`agent-rpc-dispatch-misc.ts:169`).
- `ai.complete` trả usage/maxTokens/error.data: có (xem task 11).
- Phiên bản giao thức: `protocolVersion: 2`, `features`, `buildVersion` trong handshake; `agentVersion` vẫn `'5.0.0'` (hardcode `agent-session-handshake.ts:63`, theo thiết kế).
- E2E đối kháng `agent-readonly-adversarial.e2e.test.ts` (21 dòng): có gọi `claude` thật qua `execFile`, nhưng (a) chỉ chạy khi `ORCA_REAL_CLAUDE_E2E=1`, hiện bị skip; (b) KHÔNG gọi `handleAgentExecPrompt`, tự dựng argv `--permission-mode plan --tools Read,Glob,Grep --print`; (c) chỉ 1 prompt thay vì 4 kịch bản (a-d) trong task; (d) khẳng định yếu: `expect(output).not.toContain('adversarial-test.txt created')`, không kiểm `git status`, `HEAD`, file tuyệt đối; (e) không dùng repo tạm. Không chứng minh được chế độ chỉ đọc. Tiêu chí hoàn thành của task 04 về e2e chưa đạt.

## 5. `desktop/src/relay` và commit `f7c16b6cc`

- Commit `f7c16b6cc` ("add code", 2358 file) chạm 227 file ở `desktop/src/relay`: 225 file mới (A), 3 file sửa (`agent-tool-registry.ts`, `dispatcher.ts`, `relay.ts`) (và `codeintel-repo-resolution.ts` lệch với bản agent).
- Toàn bộ là nhóm code-intel / CR v7 (xem code): `codeintel-*`, `codegraph-*`, `agent-heavy-job-gate.ts`, `codeintel/agent-result-golden.test.ts` cùng fixtures. Trong 173 file không phải fixture có bản đối ứng ở `agent/src/relay`: 169 trùng byte, 4 lệch (`agent-tool-registry.ts`, `codeintel-repo-resolution.ts`, `dispatcher.ts`, `relay.ts`).
- Không file CR-033 nào bị tạo/sửa ở desktop: `agent-exec-prompt-options`, `agent-readonly-tool-policy`, `agent-workspace-validation`, `agent-bounded-output-buffer`, `agent-result-block-parser`, `agent-worktree-change-snapshot`, `agent-capability-report`, `agent-print-mode-exec`, `agent-rpc-dispatch-misc/-ai`, `agent-session-handshake`, `agent-protocol-features`, `agent-build-version` đều không tồn tại ở `desktop/src/relay`. `ai-complete-handler.ts` và `agent-rpc-dispatch.ts` ở desktop là bản cũ lệch (332 và 1040 dòng diff), không bị commit này sửa. Không có `accessMode|ORCA_RESULT|maxTokens|reportChanges` trong 3 file desktop bị sửa (grep diff: rỗng).
- Kết luận: phù hợp với "chỉ sửa `agent/`" của CR-033 về phần CR-033. 227 file desktop là việc khác (code-intel v7, đồng bộ bản sao với `agent/`), nên điều kiện "nhiều hơn chỉ agent/" không vi phạm CR-033 nhưng làm kết luận "desktop/src/relay chỉ 154 file, không có chức năng mới" trong task 13 đã cũ (nay lớn hơn nhiều và có codeintel). Bundle build từ desktop vẫn rơi vào đường degradation cho các method CR-033.

## 6. Phía Go (infra-fleet-service): đối chiếu tên trường

- Handshake: `agentwsserver/server.go:80-85` và `devserveragent/session.go:55-57` nhận `features`, `protocolVersion`, `buildVersion`, `agentVersion` (json tag trùng với `agent-session-handshake.ts`). Khớp. `SanitizeAgentFeatures` (`domain/agent_features.go`) lọc đầu vào; hằng số `Feature*` trùng đúng 8 chuỗi trong `agent-protocol-features.ts`.
- `agent.capabilities`: `usecase/refresh_dev_server_capabilities.go:87` gọi `Exec("agent.capabilities", nil)`; kết quả lưu nguyên `ProfileJSON` + fingerprint, không giải mã từng trường nên không lệch tên trường; fallback `handshake_only` khi lỗi. Khớp về giao diện. Lưu ý: `AgentBuildVersion/ProtocolVersion/Features` lấy từ handshake, không từ báo cáo.
- `accessMode`, `workspaceKind`, `reportChanges`, `resultBlock`, `maxOutputBytes`, `maxTokens`, `applied`, `ORCA_RESULT` không xuất hiện trong code Go của infra-fleet-service (chỉ có hằng feature). Phía gửi `agent.execPrompt` mới thuộc backend khác (nhóm CR backend), không kiểm chứng được ở đây và không thuộc 13 task agent.
- `agentVersion` vẫn `5.0.0` nên `MinAgentVersion` (`server.go:257`) chưa phân biệt được agent 2.2.0 với bản cũ; việc phát hiện dựa vào `features`.

## 7. Stub và vấn đề chất lượng

- `agent-readonly-tool-policy.ts:17,91-94`: tên công cụ và cú pháp `--tools` ghi UNVERIFIED. Chưa xác nhận với claude thật.
- `agent-readonly-adversarial.e2e.test.ts:8-20`: e2e rút gọn (xem mục 4), skip mặc định.
- `agent-compat-matrix.test.ts:21`: mock luôn module policy nên không kiểm đường readonly thật.
- Thiếu test cấp handler (readonly, resultBlock, reportChanges, truncated, `READONLY_VIOLATION`, stream) trong `agent-print-mode-exec.test.ts` (556 dòng, không có từ `readonly`).
- `agent-print-mode-exec.ts:501`: bộ đệm phân tích cứng 4MiB ở stream; `agent-session-handshake.ts:63`: `'5.0.0'` cứng; `agent-entry.ts:2` chú thích "v2.1" cũ.
- `agent-print-mode-exec.ts` 604 dòng, kiểm `config/max-lines-baseline.txt` chưa làm (không thuộc phạm vi).
- Nhiều dòng `agent-rpc-dispatch-misc.ts:182-183` có dòng trống thừa, lỗi phong cách nhỏ.
- `tsc --noEmit` đỏ ở renderer (không thuộc CR-033), `vitest` toàn bộ đỏ 26 test.

## 8. Lệch giữa tài liệu và code

- Task 13 nói log dùng `AGENT_VERSION`; code dùng `AGENT_BUILD_VERSION` (cùng nguồn `__AGENT_VERSION__`). Chấp nhận được, task cần ghi lại.
- Task 13 viết `desktop/src/relay` 154 file, không có `execPrompt`: số file cũ (nay 133+ thư mục `relay` nhiều codeintel); `execPrompt` vẫn không có (grep `execPrompt` trong desktop/src/relay: không ra file).
- Task 13/04 "`pnpm test` toàn gói xanh" và "e2e đối kháng bốn prompt": không đúng thực tế.
- Tiêu đề solution "Đã triển khai" quá mức: 3 task còn thiếu test hoặc kiểm chứng.

## 9. Việc còn lại (ưu tiên)

1. Viết test cấp handler cho `agent.execPrompt`/`Stream`: readonly (argv có `--permission-mode plan --tools`, không YOLO), CLI thiếu cờ (không gọi spawn), `resultBlock`, `reportChanges`, `READONLY_VIOLATION`, `truncated`; bỏ mock policy ở một ca của compat-matrix.
2. Viết lại e2e đối kháng đúng task 04 (gọi `handleAgentExecPrompt`, repo tạm, 4 prompt, kiểm `git status`/`HEAD`/file tuyệt đối) và chạy 1 lần với `claude` thật, ghi kết quả; xác nhận cú pháp `--tools`.
3. Xác định 26 test đỏ (`subprocess.test.ts`, `agent-tool-registry.test.ts`) có sẵn hay do `f7c16b6cc`; chạy trên commit trước.
4. Chạy `pnpm build` và kiểm `AGENT_VERSION` đầu ra; chạy 6 bước tay trên dev server; kiểm `ORCA_VERSION` của backend.
5. Bỏ `'5.0.0'` cứng hoặc ghi rõ lộ trình thống nhất 3 nguồn phiên bản; làm sạch chú thích `agent-entry.ts:2`.
6. Cập nhật task 13: số file desktop, `AGENT_BUILD_VERSION`.
