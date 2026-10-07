# BE-CV-TASK-072-08: Agent không tin cậy và giới hạn từ chối dịch vụ

**From Solution:** BE-CV-SOL-072
**Priority:** P1
**Service:** `code-intel-service`
**File:** `.../internal/usecase/agent_forgery_test.go` (mới), `.../internal/usecase/dos_limits_test.go` (mới)
**Depends on:** BE-CV-SOL-021, 022, 013-agent-call-gate-and-quotas, 024, BE-CV-TASK-070-04
**Status:** `[x] DONE`

---

## Context

- S10/S11 (CR-072). Trần: kết quả agent > 12 MiB ⇒ `CODEINTEL_OUTPUT_TOO_LARGE` (agent-rpc §3.3); phản hồi UI ≤ 2 MiB (PQ-14); client nhận `MaxCallRecvMsgSize(16 MiB)`; `CODEINTEL_RATE_LIMITED`/`CODEINTEL_CONCURRENCY_LIMIT`, `CODEINTEL_REINDEX_IN_PROGRESS`; `StreamCodeIntelEvents` ≤ 16 luồng/(tenant, dev server) ở infra-fleet (PQ-18), gateway `CODE_INTEL_MAX_STREAMS`.

## Việc cần làm

1. Agent giả trả: kết quả khổng lồ, sai schema, `truncated:false` nhưng vượt trần, `workspaceRoot`/đường dẫn khác yêu cầu, `sources[].commit` giả. Khẳng định: từ chối đúng mã, không panic, không cache, không phản chiếu đường dẫn tuyệt đối, khoá cache dùng `headCommit` do service lấy (nếu CR-022 chốt; nếu chưa, test ghi `t.Skip` có lý do tham chiếu Q3).
2. 10 yêu cầu đồng thời cùng worktree ⇒ một lần gọi agent (singleflight); vượt hạn mức đồng thời tenant/dev server ⇒ `RATE_LIMITED`/`CONCURRENCY_LIMIT`.
3. Reindex lần hai khi đang chạy ⇒ `CODEINTEL_REINDEX_IN_PROGRESS`.
4. `GetSymbol` > 200 KiB mã nguồn bị cắt với `truncated`.
5. Vượt số luồng subscribe ⇒ `CODEINTEL_RATE_LIMITED`.

## Kiểm thử

- `go test ./internal/usecase/... -run 'Forgery|DoS'`; không DB (repo/cache giả) trừ bước cache (dùng giả trong bộ nhớ).

## Tiêu chí hoàn thành

- [x] Mọi ca trả mã đúng, không panic, không cache dữ liệu không hợp lệ.

## Rủi ro và lưu ý

- Số hạn mức là giả định chưa đo (O-15).
