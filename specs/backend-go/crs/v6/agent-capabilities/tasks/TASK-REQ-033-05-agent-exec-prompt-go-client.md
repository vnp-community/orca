# TASK-REQ-033-05: Client Go của `agent.execPrompt` mới: `accessMode`, `workspaceKind`, `reportChanges`, `resultBlock`, `parsed`, `changes`

**From Solution:** [BE-REQ-SOL-033](../solutions/BE-REQ-SOL-033-dev-server-capability-profile-and-agent-client.md) mục 2.I, 2.K
**Priority:** P1
**Service/Area:** `request-service` (mới) / usecase port, adapter grpcclient. Cùng hợp đồng JSON được `task-service` dùng ở TASK-REQ-029-03.
**File:** `internal/usecase/ports.go` (sửa `AgentPromptInput`, `AgentPromptResult` do TASK-REQ-008-02 tạo), `internal/adapter/grpcclient/agent_prompt_relay.go` (sửa), `internal/adapter/grpcclient/agent_prompt_params.go` (mới), `internal/adapter/grpcclient/agent_prompt_result.go` (mới), `internal/domain/agent_prompt_errors.go` (mới), và các `_test.go`
**Depends on:** TASK-REQ-008-02 (tạo `AgentPromptRunner` và adapter), TASK-REQ-033-04 (chọn đường)
**Status:** [x] DONE

---

## Context

- Hợp đồng agent (CR-REQ-033 mục 2.1 đến 2.5), mọi tham số tuỳ chọn, thiếu thì hành vi cũ:

  | Tham số | Giá trị |
  |---|---|
  | `accessMode` | `"write"` (mặc định) hoặc `"readonly"` |
  | `workspaceKind` | `"worktree"` (mặc định) \| `"repo_root"` \| `"scratch"` |
  | `reportChanges` | `true` \| `false` (mặc định) |
  | `resultBlock` | `{"nonce":"<16 đến 64 ký tự [A-Za-z0-9]>"}` |
  | `maxOutputBytes` | số nguyên, mặc định 4 MiB, trần 12 MiB |

  Kết quả thêm: `truncated`, `warnings[]` (`READONLY_VIOLATION`, `TRUST_PRESET_IGNORED_READONLY`), `changes{available,reason,headBefore,headAfter,headMoved,changedFiles[{path,change}],truncated}`, `parsed{ok:true,value}` hoặc `{ok:false,code,detail}` với `code` thuộc `RESULT_BLOCK_MISSING|RESULT_BLOCK_INVALID_JSON|RESULT_BLOCK_TOO_LARGE|RESULT_BLOCK_NOT_OBJECT`. Không có `resultBlock` thì không có `parsed`.
- Lỗi agent (JSON-RPC `InvalidParams`, `error.data.reason`): `READONLY_MODE_UNSUPPORTED`, `REPO_ROOT_REQUIRES_READONLY`, mã từ chối vùng làm việc `scratch` (tên cụ thể do `AG-REQ-SOL-033-*` chốt; dùng tiền tố `WORKSPACE_`). `-32601` là method không có (agent cũ): với tham số mới, agent cũ **bỏ qua** khoá lạ nên sẽ chạy ở chế độ ghi, đó là lý do `SelectReadonlyRoute` (task 04) phải quyết định **trước** khi gửi.
- `TASK-REQ-008-02` đã nêu: hằng `trustPresetReadonly = "default"`, không nhận `trustPreset` từ bên gọi, `env` chỉ hai khoá `ORCA_REQUEST_ID`, `ORCA_PROJECT_ID`, giới hạn đọc `stdout` 256 KB, và cờ `REQUEST_AGENT_READONLY_USE_AGENT_FLAG` với khoá giả định `readOnly:true`. **Mâu thuẫn:** CR-REQ-033 chốt tên là `accessMode`. Task này thay khoá giả định bằng `accessMode`, giữ tên cờ cấu hình.
- Phía `task-service` có struct riêng `agentExecPromptParams` (`simple_executor.go` dòng 254 đến 280); hai service không import nhau nên hợp đồng JSON được khoá bằng test golden ở TASK-REQ-033-06.
- Kết nối: `Relay{connection_id}` hoặc `RelayByDevServer{dev_server_id}` với `method="agent.execPrompt"` (`connection_id = projectID` luôn trượt, BUG-025: dùng `AIConnectionResolver` của SOL-005/007).

## Việc cần làm

1. `agent_prompt_params.go`: struct dựng JSON
   ```go
   type agentExecPromptParams struct {
       StepID         string            `json:"stepId,omitempty"`
       Prompt         string            `json:"prompt"`
       WorktreePath   string            `json:"worktreePath"`
       TrustPreset    string            `json:"trustPreset,omitempty"` // chỉ "default"; rỗng khi readonly
       Env            map[string]string `json:"env,omitempty"`
       TimeoutMS      int               `json:"timeoutMs,omitempty"`
       AccessMode     string            `json:"accessMode,omitempty"`    // chỉ "readonly"; write bỏ khỏi JSON
       WorkspaceKind  string            `json:"workspaceKind,omitempty"` // chỉ khác "worktree"
       ReportChanges  bool              `json:"reportChanges,omitempty"`
       ResultBlock    *resultBlockParam `json:"resultBlock,omitempty"`
       MaxOutputBytes int               `json:"maxOutputBytes,omitempty"`
   }
   type resultBlockParam struct{ Nonce string `json:"nonce"` }
   ```
   Hàm `buildParams(in AgentPromptInput) (agentExecPromptParams, error)` kiểm: `ResultNonce` khớp `^[A-Za-z0-9]{16,64}$` (lỗi `ErrInvalidResultNonce`); `Workspace=RepoRoot` mà `AccessMode!=Readonly` thì lỗi `ErrRepoRootRequiresReadonly` (kiểm cả phía Go để không phụ thuộc agent); `MaxOutputBytes` trần 12 MiB; `Env` chỉ hai khoá cho phép (như 008-02).
2. `agent_prompt_result.go`: giải `agentExecPromptResult` mở rộng (`Stdout, Stderr, ExitCode *int, TimedOut, Truncated bool, Warnings []string, Changes *agentChanges, Parsed *agentParsed`) rồi ánh xạ sang `usecase.AgentPromptResult` (`ExitCode` nil thì `-1` và `TimedOut=false`, giữ ngữ nghĩa của 008-02). `Parsed.Value` giữ `json.RawMessage` nguyên văn. `ChangedFiles` cắt tối đa 2000 (đã giới hạn ở agent, chặn lại phía Go).
3. `agent_prompt_errors.go`: kiểu lỗi `AgentError{Reason string; Message string}`:
   - hàm `classifyRelayError(err error) error` đọc `status.Convert(err)` và JSON `error.data.reason` nếu `infra-fleet-service` chuyển được (kiểm `RelayResponse`/lỗi; nếu chỉ chuyển chuỗi thì so khớp tiền tố mã trong thông điệp và ghi vào Rủi ro): `READONLY_MODE_UNSUPPORTED` thành `ErrAgentReadonlyUnsupported`
   - `REPO_ROOT_REQUIRES_READONLY` thành `ErrAgentWorkspaceRejected`
   - mã `WORKSPACE_*` thành `ErrAgentWorkspaceRejected`
   - lỗi method không có thành `ErrAgentMethodNotFound`.
4. `agent_prompt_relay.go`: `ExecPrompt` dùng `buildParams`, gọi relay, giải kết quả, gắn `classifyRelayError`:
   - khi `AccessMode==Readonly` thì **không gửi** `trustPreset`
   - khi không readonly thì `trustPreset="default"`. Mọi nhánh không có giá trị `"full"` (kiểm bằng test chạy qua mọi tổ hợp của `AccessMode x Workspace x ReportChanges x ResultNonce`).
5. Khi có `ResultNonce`: use case gọi sinh nonce bằng `crypto/rand` (hàm `NewResultNonce() string` trả 32 ký tự chữ-số, đặt ở `internal/domain/result_nonce.go`):
   - nonce được đưa vào prompt (việc dựng prompt thuộc SOL-008 và SOL-029, không thuộc task này) và vào `ResultNonce` của input
   - không bao giờ ghi nonce vào log (chỉ băm `nonce_hash` ở `execution_packets` của SOL-029).
6. Chuyển cờ: `REQUEST_AGENT_READONLY_USE_AGENT_FLAG` bật mới gửi `accessMode`; kết hợp với `SelectReadonlyRoute`: chỉ gửi khi route là `RouteAgentEnforced`.
7. Giới hạn bộ nhớ: đọc `stdout` tối đa `maxOutputBytes` do agent đã cắt; phía Go cắt thêm theo giới hạn của use case (256 KB cho phân tích chỉ đọc, như 008-02) sau khi lấy `Parsed`.

## Kiểm thử

- `TestBuildParams_Table`: write mặc định chỉ có `prompt`, `worktreePath`, `trustPreset:"default"`; readonly có `accessMode:"readonly"` và không có `trustPreset`; `repo_root` kèm write thì `ErrRepoRootRequiresReadonly`; nonce sai định dạng bị từ chối; `Env` ngoài hai khoá bị từ chối.
- `TestBuildParams_NeverTrustFull_AllCombinations` duyệt đủ tổ hợp.
- `TestExecPrompt_ParsesChangesAndWarnings`, `_ParsedOk`, `_ParsedMissingCode`, `_TruncatedFlag`, `_ExitCodeNil`.
- `TestClassifyRelayError_Table`: `READONLY_MODE_UNSUPPORTED`, `REPO_ROOT_REQUIRES_READONLY`, `-32601`.
- `TestGoldenParams_MatchesAgentContract` đọc `testdata/execprompt_params.golden.json` (cùng file với TASK-REQ-033-06).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/adapter/grpcclient/... -run "AgentPrompt|BuildParams|ClassifyRelay"`.

## Tiêu chí hoàn thành

- [x] Agent cũ nhận đúng JSON như trước khi có task này khi dùng tham số mặc định (golden so với `simple_executor.go` doc comment).
- [x] Không có đường mã nào đặt `trustPreset="full"` (test và `grep` trong review).
- [x] Route `agent_readonly` không gửi `accessMode` tới agent cũ (kiểm bằng `SelectReadonlyRoute`).
- [x] Mọi mã lỗi agent trong bảng được dịch sang lỗi có kiểu, không còn so chuỗi ở use case.
- [x] Nonce không xuất hiện trong log hay lỗi.

## Rủi ro và lưu ý

- Chưa kiểm chứng `infra-fleet-service.Relay` có chuyển `error.data` của JSON-RPC lỗi hay chỉ thông điệp; nếu chỉ thông điệp, `classifyRelayError` so khớp tiền tố mã. Đọc `usecase/relay.go` trước khi viết và ghi kết luận vào PR.
- Hành vi thật của `--permission-mode plan` với `--print` chưa kiểm chứng (CR-REQ-033 mục 6); client này chỉ truyền tham số, không bảo đảm chỉ đọc.
- `maxOutputBytes` mặc định 4 MiB lớn hơn giới hạn 256 KB của use case: tốn băng thông; đề xuất truyền `MaxOutputBytes` bằng giới hạn use case cộng 64 KB dư cho khối kết quả.
