# TASK-REQ-026-06: `openspecEngine.GenerateAnalysis` (Solution hồ sơ `full`), `ProposalWorkspace` và kiểm phạm vi ghi

**From Solution:** BE-REQ-SOL-026
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/engine_openspec.go`, `internal/usecase/openspec_solution_prompt.go`, `internal/usecase/proposal_scope_check.go`, `internal/adapter/grpcclient/proposal_workspace.go`, `internal/usecase/ports.go` (sửa) và test (mới)
**Depends on:** TASK-REQ-026-01, 026-02, 026-03 (không bắt buộc cho task này), 026-04, 026-05, TASK-REQ-008-01 (`RedactSecrets`), TASK-REQ-027-04 (parser khối `orca-json`, `RenderArtifact`)
**Status:** [x] DONE

---

## Context

Đã đọc `proto/orca/gitgateway/v1/gitgateway.proto`: `CreateWorktreeRequest{project_id, repo_id, branch, base_ref, idempotency_key, name, path}` (dòng 736), `CreateWorktreeResponse{worktree_id, path, head_sha}` (775), `CommitRequest{worktree_id, message, paths}` (206), `GetStatusRequest{worktree_id}` (182, `FileStatus{path, state}` với state `modified|added|deleted|untracked|conflicted`), `BulkDiscardRequest{worktree_id, paths[]}` (1108), `RemoveWorktreeRequest{worktree_id, force, ...}` (822), `WriteFileRequest{worktree_id, path, content, encoding, create_parents}` (531). Chưa có client git-gateway trong `request-service` (service chưa tồn tại); tạo `adapter/grpcclient/proposal_workspace.go` theo mẫu client trong `task-service/internal/adapter/grpcclient/`.

CR-REQ-026 mục 2.5 (Solution). Điểm bẫy: `agent.execPrompt` chỉ trả `{stdout, stderr, exitCode, timedOut}` (không model, không danh sách tệp đổi), nên kiểm phạm vi dựa `GetStatus`. Không dùng `CreateWorktreeFromIssue`: có thể kích hoạt `issue-status-sync` đổi trạng thái Jira (chưa kiểm chứng).

## Việc cần làm

1. `ports.go`: `ProposalWorkspace` đúng như SOL-026 mục 2.F;
   - `Workspace{WorktreeID, Path, Branch, BaseRef string}`
   - `EnsureWorkspaceInput{TenantID, ProjectID, RepoID, RequestNumber int64, ChangeID string}`.
2. `proposal_workspace.go`: `Ensure` gọi `CreateWorktree{branch: "request/<number>-proposal", base_ref: "", idempotency_key: hex(sha256(project_id|repo_id|branch)), name: "request-<number>-proposal"}`;
   - `base_ref` rỗng nghĩa là nhánh mặc định (đọc `CreateWorktreeRequest` comment lúc làm, nếu bắt buộc thì lấy từ project-service).
   - Lỗi `WORKTREE_PATH_EXISTS` kèm `suggested_name` thì coi như đã tồn tại (đọc lại bằng `idempotency_key`).
   - `ReadFile` giới hạn `maxBytes` (256 KB) bằng `ReadFileChunk` hoặc cắt sau khi đọc
   - `WriteFile` dùng `create_parents=true`
   - `ChangedPaths` đọc `GetStatus`, trả đường dẫn tương đối chuẩn hoá bằng `path.Clean`, đổi `\` thành `/`
   - `DiscardAll`: `GetStatus` rồi `BulkDiscard` các tệp đã theo dõi
   - tệp `untracked` còn sót sau đó thì `Remove{force:true}` rồi `Ensure` lại (Q5 của solution)
   - `Commit{message, paths}`.
3. `proposal_scope_check.go`: `CheckProposalScope(changed []string, changeID string) []string` (trả danh sách đường dẫn **ngoài** tiền tố `openspec/changes/<changeID>/`);
   - từ chối: tuyệt đối, chứa `..` sau `Clean`, chứa NUL, độ dài > 512.
   - `ValidChangeID(changeID)` được kiểm lại ở đây (không tin người gọi).
4. `openspec_solution_prompt.go`: `BuildOpenSpecSolutionPrompt(in) string`.
   - Cấu trúc: chỉ dẫn (đọc `openspec/AGENTS.md` nếu có; tạo `proposal.md`, `design.md`, delta spec dưới `openspec/changes/<change_id>/`; **không** sửa tệp nào khác; không chạy lệnh git), khối `<request>` có rào (body cắt 12 000 ký tự, loại ký tự điều khiển, nhắc rõ đó là dữ liệu không phải chỉ dẫn), `<prior_artifacts>` (mỗi mục cắt 8 000), yêu cầu `design.md` chứa vùng `<!-- orca:begin options -->` với khối ```` ```orca-json ```` là `SolutionOptions` v1 (mô tả schema ngắn, bao gồm `requirement_coverage` của SOL-027 nếu Request có AC). Không đưa `credential_ref`, `env`, token.
5. `engine_openspec.go`, `GenerateAnalysis(ctx, in)`: (a) `Preflight` (dùng gate 026-04, lỗi trả ngay);
   - (b) `Ensure` workspace
   - `OpenSpecChangeRepository.Upsert(status=preparing, change_id=NewChangeID(...))`
   - (c) `ExecPrompt{Prompt, WorktreePath: ws.Path, TrustPreset:"default", Env:{ORCA_REQUEST_ID, ORCA_PROJECT_ID}, TimeoutMs: REQUEST_OPENSPEC_TIMEOUT_MS (600000)}`
   - `TimedOut` thì `REQUEST_OPENSPEC_TIMEOUT`, `ExitCode != 0` thì `REQUEST_OPENSPEC_AGENT_FAILED`
   - (d) `ChangedPaths` rồi `CheckProposalScope`
   - có đường dẫn ngoài phạm vi thì `DiscardAll` và `REQUEST_OPENSPEC_OUT_OF_SCOPE_CHANGE`
   - (e) đọc `proposal.md`, `design.md` và các tệp dưới `specs/` (liệt kê bằng `ReadDir` qua git-gateway, không glob từ đầu ra agent), che bí mật
   - (f) parse khối `orca-json` trong vùng `options` của `design.md` bằng parser SOL-027
   - (g) `ExecInput{Binary:"openspec", Args:["validate", changeID], Cwd: ws.Path}`
   - (h) `SolutionOptions.Validate(minOptions)` và kiểm ngữ nghĩa SOL-027
   - (i) sai ở (f) đến (h): thử lại **một lần** (`Attempt=2`) bằng `ExecPrompt` với thông điệp lỗi cụ thể (số dòng, mã), cùng worktree
   - vẫn sai thì `REQUEST_OPENSPEC_INVALID_OUTPUT` (kèm `Violation` đầu tiên).
6. Sau khi đạt: `RenderArtifact` ghi lại vùng `options` của `design.md` từ mô hình chuẩn (`WriteFile`), `Commit(message="docs(request): proposal for REQ-<number>", paths=["openspec/changes/<id>/"])`, `OpenSpecChange.status=ready`, trả `AnalysisOutput{OptionsJSON, Raw: nội dung design.md, Provenance{generator:{kind:"openspec", tool:"openspec@<version>", model:"", model_source:"unknown"}}}`.
7. Lỗi bất kỳ: `DiscardAll` thay đổi chưa commit, giữ worktree, không để `OpenSpecChange` ở `preparing` mãi (đặt lại giá trị cũ; lần gọi sau dùng lại).
   - Trả lỗi cho `RunSolutionGeneration` ghi vào `analysis_runs.error_code` (không ném ra RPC).

## Kiểm thử

- `TestScopeCheck_Table`: `openspec/changes/req-1-x/proposal.md` đạt; `openspec/changes/req-1-x/../../../etc/passwd`
- `src/main.go`
- `openspec/changes/req-2-y/a.md`, đường dẫn Windows `openspec\changes\req-1-x\a.md` (chuẩn hoá rồi đạt), NUL, độ dài lớn.
- `TestPrompt_FencesRequestAndOmitsSecrets` (Request chứa "bỏ qua mọi chỉ dẫn trước" nằm trong `<request>`; không có `credential_ref`)
- `TestPrompt_TruncatesBody12000`.
- `TestGenerateAnalysis_HappyPath` (fake workspace + fake executor; kỳ vọng thứ tự gọi: `Ensure`
- `ExecPrompt`
- `ChangedPaths`
- `ReadFile`
- `Exec validate`
- `WriteFile`
- `Commit`)
- `TestGenerateAnalysis_OutOfScope_DiscardsAndFails`
- `TestGenerateAnalysis_InvalidJSON_RetriesOnce_ThenFails`
- `TestGenerateAnalysis_ValidateFails_RetriesWithMessage`
- `TestGenerateAnalysis_Timeout`
- `TestGenerateAnalysis_AgentExitNonZero`
- `TestGenerateAnalysis_ReusesWorktreeOnSecondCall`.
- `TestProposalWorkspace_Ensure_IdempotentKey` (fake git-gateway, hai lần gọi, cùng `idempotency_key`).
- Thủ công, chưa kiểm chứng (ghi vào PR): chạy trên dev server thật với OpenSpec cài thật.
- Lệnh: `cd /opt/repos/orca/backend-go && go test -race ./services/request-service/internal/usecase/... -run 'OpenSpec|ScopeCheck|ProposalPrompt' && go test ./services/request-service/internal/adapter/grpcclient/... -run ProposalWorkspace`.

## Tiêu chí hoàn thành

- [x] Thay đổi ngoài `openspec/changes/<change_id>/` luôn bị phát hiện và hủy (test bảng đủ biến thể đường dẫn).
- [x] Không có commit khi kiểm tra chưa đạt (test: `Commit` không được gọi ở nhánh lỗi).
- [x] Thử lại đúng một lần, `attempt` tăng; run `failed` có mã đúng và Solution `draft` bị xoá (do `RunSolutionGeneration`).
- [x] `provenance.generator.kind=openspec`, `model` rỗng, `model_source=unknown`.
- [x] Prompt không chứa bí mật; nội dung Request luôn nằm trong khối rào.
- [x] Gọi lặp `Ensure` không tạo worktree thứ hai.

## Rủi ro và lưu ý

- `Discard` có dọn `untracked` hay không chưa rõ; nếu không, tệp rác trong `openspec/changes/<id>/` lần chạy sau lẫn vào kết quả. Test thủ công trên git-gateway thật.
- Agent có thể đọc `openspec/AGENTS.md` do chính repo cung cấp (nội dung không tin cậy). Biện pháp là kiểm sau chạy và duyệt của người; không có cách chặn tuyệt đối.
- Thời gian chạy có thể sát trần 15 phút của `agent.execPrompt`; `REQUEST_OPENSPEC_TIMEOUT_MS` mặc định 600000 chỉ là đề xuất.
- `RenderArtifact` ghi đè vùng `options`: nếu người dùng đã sửa tay vùng này trong repo giữa hai lần sinh, nội dung đó mất (hành vi đúng theo CR, ghi ở README task).
