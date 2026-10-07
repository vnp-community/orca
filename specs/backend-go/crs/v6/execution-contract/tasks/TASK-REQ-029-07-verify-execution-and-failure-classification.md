# TASK-REQ-029-07: `VerifyExecution` (chạy lại Check, kiểm phạm vi, quét bí mật) và `ClassifyFailure`

**From Solution:** [BE-REQ-SOL-029](../solutions/BE-REQ-SOL-029-execution-contract-and-readiness-gate.md) mục 2.F
**Priority:** P0
**Service/Area:** `request-service` (mới) / domain, usecase
**File:** `internal/domain/execution_verdict.go` (mới), `internal/domain/failure_class.go` (mới), `internal/domain/verification_findings.go` (mới), `internal/usecase/verify_execution.go` (mới), `internal/usecase/verify_execution_steps.go` (mới), `internal/usecase/classify_failure.go` (mới), `internal/usecase/secret_scan_ports.go` (mới), và các `_test.go`
**Depends on:** TASK-REQ-029-05 (`TaskSpecV2`, `ScopeMatcher`), TASK-REQ-029-06 (`AgentRelay`), TASK-REQ-029-02 (`ListExecutionRecords`), CR-REQ-035 (`common/secretscan`), TASK-REQ-014-01/02 (`request_checks` repository)
**Status:** [x] DONE

---

## Context

Đã đọc ngày 2026-10-06:
- CR-REQ-029 mục 2.6: Orca không tin lời agent. Thứ tự: (1) bản ghi `task_execution_records` mới nhất phải `parse_status=ok`; (2) **chạy lại Check** bằng `agent.exec` trong worktree, so `exit` với `expect.exit` và `expect.match` (regex trên 4 KB cuối), tuần tự, tổng ≤ `REQUEST_VERIFY_BUDGET` (8 phút đề xuất); lệch với `checks_run` của agent ghi `CHECK_MISMATCH`; (3) phạm vi: `git diff --name-only -z <base_sha>` cộng danh sách file chưa theo dõi, đối chiếu `changes.changedFiles`; file ngoài `scope.include`, khớp `exclude`, file mới ngoài `create`, hoặc vượt `max_files` là `SCOPE_VIOLATION`; danh sách do git đo là kết luận; (4) quét bí mật `git diff <base_sha>` cắt 1 MB bằng `secretscan.Scan`, ghi loại và vị trí, không ghi giá trị (`SECRET_FOUND`); (5) gộp `ExecutionVerdict{status passed|failed, findings[]}` ghi vào `task_run_outcomes.verdict`.
- `agent.exec` tối đa 5 phút mỗi lệnh; `timeout_seconds ≤ 300` đã bị `Validate` chặn (task 05). Không qua shell. `git.exec` không có `ls-files` và cấm một số ký tự: dùng `agent.exec binary="git"`.
- `git diff --name-only -z <base_sha>` so cây làm việc với `base_sha` **bao gồm** commit mà agent tự tạo (`headMoved`), đúng ý CR. `ls-files --others --exclude-standard -z` cho file chưa theo dõi; hai lệnh đều có trước Git 2.25 (`rev-parse`, `diff --name-only`, `ls-files`), không cần `GitCapabilityCache` (`guides/reference/git-compatibility.md`).
- `base_sha` lấy từ báo cáo sẵn sàng của lần thử đó (`task_readiness_reports.base_sha`, task 06); task đầu chưa có worktree lúc cổng chạy thì `base_sha` rỗng: `VerifyExecution` tự đọc `git rev-parse HEAD` **trước khi** có commit của agent là không thể; thay vào đó dùng `merge-base` với nhánh gốc của worktree (`project-service.GetWorktree.base_ref`, trường 11 của `Worktree`) và ghi `finding BASE_SHA_DERIVED` mức `warn`. Chốt: nếu cả hai không có thì `VerifyExecution` trả `failed` với `BASE_SHA_UNKNOWN` (không tha thứ im lặng).
- `request_checks` (CR-REQ-014): `source=orca_verified`; `tests_modified` do luật `no_test_files_modified` quyết định.
- `common/secretscan` chưa có trong repo ngày 2026-10-06 (CR-REQ-035 tạo); dùng cổng `SecretScanner` ở usecase để task này không bị chặn bởi nó.

## Việc cần làm

1. `execution_verdict.go`: `VerdictStatus` (`VerdictPassed|VerdictFailed`), `VerificationFinding{Code string; CheckID, Path string; Detail string}` (không bao giờ có giá trị bí mật), `ExecutionVerdict{Status VerdictStatus; Findings []VerificationFinding; FilesChanged []string; ChecksRun []VerifiedCheck; BaseSHA string; DurationMS int; BudgetExceeded bool}`, `VerifiedCheck{ID string; Exit int; TimedOut bool; MatchOK bool; AgentClaimedExit *int}`. Mã finding: `CHECK_FAILED`, `CHECK_MISMATCH`, `CHECK_TIMEOUT`, `CHECK_NOT_FOUND` (exit 126/127), `SCOPE_VIOLATION`, `SCOPE_FILE_COUNT`, `SECRET_FOUND`, `RESULT_INVALID`, `BASE_SHA_UNKNOWN`, `BASE_SHA_DERIVED`, `FILES_CLAIM_DIFFERS` (cảnh báo).
2. `failure_class.go`: `FailureClass` và `Valid()` (cùng năm giá trị với `task-service`, bản sao riêng):
   - `FailureInput{ParseStatus, ResultStatus string; TimedOut bool; RelayTransient bool; Verdict *ExecutionVerdict; BaselineRedAtHead bool; HeadSHA string; PriorMissingCount int; ExitCodes map[string]int}`
   - hàm thuần `ClassifyFailure(in FailureInput) ClassifiedFailure{Class FailureClass; Reason string; CountsAttempt bool; GoStraightToBacklog bool; RetryWithFormatReminder bool}`. Quy tắc (bảng CR 2.7): `TimedOut` hoặc `RelayTransient` thì `retryable`, tính lần thử
   - `ResultStatus` `needs_info|blocked` thì `needs_info`, không tính
   - verdict có `CHECK_NOT_FOUND` (exit 126/127) thì `spec_defect`, không tính
   - verdict `CHECK_FAILED` mà Check đó đã đỏ ở Check nền cùng `HeadSHA` (`BaselineRedAtHead`) thì `env_defect`, không tính
   - `SECRET_FOUND` thì `agent_defect` với `GoStraightToBacklog=true` (không thử lại)
   - `ParseStatus` `missing|invalid` thì `agent_defect`, `CountsAttempt=false` và `RetryWithFormatReminder=true` khi `PriorMissingCount == 0`, ngược lại tính lần thử
   - `SCOPE_VIOLATION`, Check đỏ do code, `CHECK_MISMATCH` kèm đỏ thì `agent_defect`, tính lần thử, thử lại có phản hồi. Mọi hằng và danh sách mã nằm trong bảng ở một tệp (dễ cấu hình sau).
3. `secret_scan_ports.go`: `SecretScanner{Scan(text string) []SecretFinding}` với `SecretFinding{Kind string; Line int}` (không có `Value`); adapter cho `common/secretscan.Scan` ở TASK-REQ-029-08.
4. `verify_execution.go`: `VerifyExecution{Specs TaskSpecReader; Records ExecutionRecordReader; Relay AgentRelay; Worktrees WorktreeResolver; Reports ReadinessReportRepository; Scanner SecretScanner; Clock Clock; Cfg VerifyConfig}`; `Execute(ctx, VerifyInput{RequestID, TaskID string; Attempt int}) (ExecutionVerdict, error)`:
   a. đọc spec (`GetTaskSpecs`), bản ghi mới nhất (`ListExecutionRecords latest_only`), `worktree_id` và đường dẫn (`GetTask`, `WorktreeResolver.Path`), `base_sha` từ `Reports.Latest` (hoặc dẫn xuất như Context);
   b. `parse_status != ok` hoặc `result.status != done` thì verdict `failed` với `RESULT_INVALID`, bỏ qua bước c đến e (không chạy Check khi chưa có kết quả hợp lệ);
   c. Check: với mỗi Check chạy được (không `manual`, không `diff_rule`) `RunCommand` tuần tự (`Binary`/`Args` tách bằng bộ tách lệnh của task 06; lệnh có toán tử shell thì `sh -c <command>` trên Linux/macOS hoặc `cmd /c` trên Windows, ghi `finding SHELL_WRAPPED` mức `info`), `Cwd` = `check.cwd` nối vào worktree (từ chối `..`), `TimeoutMS = min(timeout_seconds*1000, phần ngân sách còn lại)`; so `exit` (mặc định 0) và `match` trên 4 KB cuối (`regexp`); so với `checks_run` của agent: khác nhau thì `CHECK_MISMATCH`; vượt ngân sách thì dừng, `BudgetExceeded=true`, các Check chưa chạy được đánh `CHECK_TIMEOUT`;
   d. phạm vi: `git diff --name-only -z <base_sha>` và `git ls-files --others --exclude-standard -z` (qua `RunCommand`), hợp thành tập `measured`; `ScopeMatcher.Violations(measured, create, max_files)`; `diff_rule` Check được đánh giá ở đây bằng chính tập này (`no_test_files_modified`: tệp khớp `*_test.go|*.test.ts|*.spec.ts|test_*.py|__tests__/**` — danh sách mẫu cấu hình, mặc định theo ngôn ngữ repo); lệch giữa `measured` và `changes.changedFiles`/`files_changed` ghi `FILES_CLAIM_DIFFERS` (cảnh báo, không đổi kết luận);
   e. quét bí mật: `git diff <base_sha>` (qua `RunCommand`, cắt 1 MB phía Go; diff nhị phân bị bỏ) rồi `Scanner.Scan`; mỗi phát hiện một `SECRET_FOUND{Kind, Path (từ header diff), Detail="line N"}`, **không** ghi giá trị;
   f. gộp: `failed` nếu có finding mức chặn (`CHECK_FAILED`, `CHECK_MISMATCH` kèm Check đỏ, `CHECK_TIMEOUT`, `SCOPE_VIOLATION`, `SECRET_FOUND`, `RESULT_INVALID`, `BASE_SHA_UNKNOWN`), ngược lại `passed`.
5. Khi `passed`: trả thêm danh sách số đo cho `request_checks` (`RequestCheckMeasurement{Kind, Name string; Passed bool; Source "orca_verified"; TestsModified *bool}`) để use case của task 08 ghi cùng giao dịch; **không** ghi trong `VerifyExecution` (giữ hàm này không có tác dụng phụ ngoài `agent.exec` đọc).
6. `classify_failure.go`: bọc `domain.ClassifyFailure` thành use case nhận `outcome` từ consumer (`ReportTaskOutcome`) và verdict, trả `ClassifiedFailure`; ánh xạ `category` cho `ReturnToBacklog` (CR mục 9): `needs_info` thành `missing_info`, `spec_defect` thành `other`, `env_defect` thành `blocked_dependency`, `agent_defect` thành `other`.
7. Giới hạn bộ nhớ và log: `stdout` của Check chỉ giữ 4 KB cuối; mọi log chỉ có mã finding, id Check, độ dài; không log diff, `stdout`, hay giá trị.

## Kiểm thử

- `TestClassifyFailure_Table`: đủ năm lớp theo bảng CR 2.7, gồm exit 126/127, Check đỏ trùng Check nền, `SECRET_FOUND` đi thẳng backlog, thiếu khối lần đầu (không tính lần thử, nhắc định dạng) và lần hai (tính), `needs_info` không tính, `retryable` tính.
- `TestVerifyExecution_AgentClaimsExit0ButOrcaMeasures1_FailsAsMismatch` (Check đỏ khi agent khai xanh: task không sang `done`), `_CheckGreen_Passes`, `_MatchRegexOnTail4KB`, `_BudgetExceededStopsAndMarksTimeout`, `_ManualCheckSkipped`, `_DiffRuleNoTestFilesModified`.
- Phạm vi: `_FileOutsideScopeInclude`, `_FileMatchesExclude`, `_NewFileOutsideCreate`, `_TooManyFiles`, `_AgentCommittedStillCounted` (`headMoved`: diff theo `base_sha`), `_UntrackedFilesIncluded`, `_ClaimDiffersIsWarningOnly`.
- Bí mật: `_SecretInDiff_PEM_FailsNoRetryNoValue` (kiểm `Detail` không chứa nội dung khoá), `_BinaryDiffSkipped`, `_DiffTruncatedAt1MB`.
- `_BaseSHAUnknown_FailsClosed`, `_BaseSHADerivedWarns`, `_ResultNotOk_SkipsChecks`, `_NoSideEffects` (relay giả chỉ nhận lệnh trong danh sách cho phép).
- Mọi lệnh Git qua relay giả khẳng định đúng danh sách tham số: `["diff","--name-only","-z",base]`, `["ls-files","--others","--exclude-standard","-z"]`, `["diff",base]`, và tuỳ chọn toàn cục (nếu có) đứng trước subcommand.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... -run "Verify|Classify"`.

## Tiêu chí hoàn thành

- [x] Agent khai `exit=0` cho Check mà Orca đo `exit=1`: verdict `failed`, task không sang `done`.
- [x] File ngoài `scope.include`, vượt `max_files`, file mới ngoài `create` là `SCOPE_VIOLATION`, kể cả khi agent tự commit.
- [x] Diff chứa khoá PEM: `SECRET_FOUND`, `agent_defect`, không thử lại; log và payload không chứa giá trị.
- [x] Chỉ `retryable` và `agent_defect` tính lần thử (test bảng).
- [x] `VerifyExecution` không có tác dụng phụ ghi DB; chỉ đọc qua relay.
- [x] Không file nào tên `helpers`/`utils`/`common`/`misc`; không `max-lines` disable (tách `verify_execution_steps.go` nếu thân hàm lớn).

## Rủi ro và lưu ý

- Chạy lại Check có cùng môi trường (PATH, env, cache build) với lần agent chạy hay không chưa kiểm chứng; `agent.execPrompt` nhận `env` riêng còn `agent.exec` ở đây không có biến đó. Nếu lệch, Check xanh với agent nhưng đỏ ở đây sẽ bị gắn `agent_defect` oan: đây là rủi ro lớn nhất của task, đo bằng thử nghiệm thủ công trên dev server thật (chưa chạy).
- Heuristic phân biệt `spec_defect` (126/127) với `agent_defect` chưa kiểm chứng trên dữ liệu thật; cấu hình được.
- `git diff <base_sha>` có thể lớn hơn 1 MB (bị cắt: bí mật nằm ngoài phần cắt bị bỏ sót); `common/secretscan` bỏ sót nhiều dạng (CR-REQ-035).
- `trustPreset=full` cho agent ghi tuỳ ý; kiểm chứng chỉ phát hiện sau, không hoàn tác (README v6 mục 6: Orca không tự `git checkout`).
- `no_test_files_modified` bằng danh sách mẫu tên tệp là heuristic; ghi vào `request_checks` kèm `source=orca_verified` nhưng không nên coi là bảo đảm.
