# BE-CV-TASK-013-11: `ReindexAdmission` (một job/binding, dev server, tenant, cooldown, giới hạn người dùng)

**From Solution:** BE-CV-SOL-013-agent-call-gate-and-quotas
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/reindex_admission.go`, `reindex_admission_test.go` (mới)
**Depends on:** BE-CV-TASK-011-08, 011-10, 013-09, 013-06
**Status:** [x] DONE

---

## Context

SOL-013-gate mục 2.E; `ReindexJobRepository` (`Create`, `CountActiveByDevServer`, `CountActiveByTenant`, `LastSucceededFinishedAt`) từ SOL-011. `RequestReindex` (RPC, SOL-021) gọi `Admit` rồi tạo job và gọi agent `codeintel.reindex` (hợp đồng PQ-16: UI chỉ gửi `mode`; `trigger='manual'` do service đặt). Hợp đồng không có `CancelReindex` (O-17).

## Việc cần làm

1. `ReindexAdmission.Admit(ctx, target Target, trigger Trigger) error` theo thứ tự: giới hạn người dùng (`rate.Limiter` 6/giờ/user, chỉ `manual`) → cooldown (`now - LastSucceededFinishedAt < CODEINTEL_REINDEX_COOLDOWN`, chỉ `manual`) → `CountActiveByDevServer ≥ CODEINTEL_REINDEX_PER_DEV_SERVER` → `CountActiveByTenant ≥ CODEINTEL_REINDEX_PER_TENANT`.
2. Lỗi: `CODEINTEL_RATE_LIMITED`, `CODEINTEL_REINDEX_COOLDOWN` (+ `retryAfterSeconds` còn lại), `CODEINTEL_CONCURRENCY_LIMIT` (`scope: devServer|tenant`); `Create` vi phạm `active_key` → `CODEINTEL_REINDEX_IN_PROGRESS` (`CodedData{jobId, stage}`: lấy job đang chạy của binding).
3. Audit: `codeintel.reindex.request` (allowed) khi `Create` thành công; `codeintel.reindex.limit` (denied) khi bị hạn mức chặn; trigger tự động không audit "request".
4. Tham số `trigger`: `manual` chịu mọi kiểm; `agent_done|head_change|schedule` chỉ kiểm (3)(4)(5) và `active_key` (Q1 SOL: chốt ở SOL-080).
5. Tài liệu hoá cuộc đua đếm→tạo không nguyên tử (chú thích ngắn).

## Kiểm thử

- Unit (repo giả, đồng hồ giả): bảng từng nhánh; thứ tự ưu tiên (người dùng trước cooldown…); `retryAfterSeconds`; audit đúng.
- Integration hai dialect: 20 `Admit`+`Create` đồng thời cùng binding → đúng một thành công (nhờ `active_key`); sau `Finish` bị cooldown 5 phút; dev server khác không bị ảnh hưởng.
- `go test ./services/code-intel-service/... -run ReindexAdmission`; `go test -tags=integration … -run ReindexAdmission`

## Tiêu chí hoàn thành

- [x] Các tiêu chí reindex ở SOL mục 4 đạt ở hai dialect.
- [x] Không có cách tạo hai job `queued/running` cho một binding.

## Rủi ro và lưu ý

- Vượt tạm 1 job/dev server trong cuộc đua hiếm (chấp nhận).
- Job mồ côi giải phóng `active_key` nhờ bảo trì (SOL-011 task 13).
