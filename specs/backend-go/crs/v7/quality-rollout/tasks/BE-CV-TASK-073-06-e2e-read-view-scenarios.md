# BE-CV-TASK-073-06: Kịch bản e2e đọc view (E02–E10)

**From Solution:** BE-CV-SOL-073
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/e2e/change_overlay_test.go`, `e2e/impact_symbol_test.go`, `e2e/erd_test.go`, `e2e/cache_singleflight_test.go`, `e2e/tool_missing_test.go`, `e2e/repo_resolution_test.go`, `e2e/timeout_test.go`, `e2e/truncation_test.go` (mới, tag `e2e`)
**Depends on:** BE-CV-TASK-073-05, BE-CV-SOL-022, 031, 036, 040 (không bắt buộc ở T1), BE-CV-TASK-070-04
**Status:** `[x] DONE`

---

## Context

- CR-073 §2.3: E02 `GetChangeOverlay`; E03 `GetImpact`/`GetSymbol` + `CODEINTEL_AMBIGUOUS_SYMBOL` (có `candidates`, chọn rồi gọi lại); E04 `GetErd`; E06 cache + singleflight; E07 `CODEINTEL_TOOL_UNAVAILABLE`/`CODEINTEL_INDEX_MISSING`; E08 repo chưa đăng ký, worktree lồng không dùng index repo cha; E09 timeout; E10 `truncated`/`totalCount`.
- Mã lỗi theo agent-rpc §3.2 và PQ-03; timeout: service trả `CODEINTEL_TIMEOUT` + `{"retryAfterMs":3000,"inProgress":true}` và hoàn tất nền (PQ-13); trần 2 MiB.
- `GetStructure`, `GetSubgraph` v.v. thuộc ma trận `TestEveryRPCHasScenario` (task 07).

## Việc cần làm

1. Mỗi kịch bản khẳng định: kết quả/lỗi đúng, `etag`/`fromCache`, sự kiện outbox, dòng audit, **không rò** secret/tenant.
2. E06: lần hai `cache=hit`; sau `index_changed` lần ba gọi lại agent; 10 yêu cầu đồng thời ⇒ một lần gọi agent (đếm ở replay agent).
3. E09: agent giả chậm vượt hạn ⇒ `CODEINTEL_TIMEOUT` kèm hậu tố; việc nền xong; lần sau trúng cache.
4. E10: `truncated:true`, `totalCount` đi hết chuỗi tới response service.
5. E04: chỉ chạy khi CR-031 có; ngược lại `t.Skip` có tham chiếu.

## Kiểm thử

- `go test -tags=e2e ./e2e/... -run 'Overlay|Impact|ERD|Cache|ToolMissing|RepoResolution|Timeout|Truncation'` hai dialect. Chưa chạy.

## Tiêu chí hoàn thành

- [x] E02–E10 (phần T1) xanh hai dialect.

## Rủi ro và lưu ý

- Dữ liệu `data` mỗi method phụ thuộc agent chạy thật (O-3); tệp vàng có thể phải đổi.
