# TASK-REQ-030-03: Repository hai dialect cho `impact_assessments`, `impact_tool_runs`, `risk_acceptances`, `risk_policies`, `risk_outcomes` (lease, một `collecting`, hiệu lực theo digest)

**From Solution:** [BE-REQ-SOL-030](../solutions/BE-REQ-SOL-030-impact-assessment-and-risk-scoring.md) mục 2.B, 2.D
**Priority:** P1
**Service/Area:** `request-service` (mới) / usecase port, adapter postgres và mysql
**File:** `internal/usecase/impact_ports.go` (mới), `internal/adapter/postgres/impact_repository.go` (mới), `internal/adapter/postgres/risk_acceptance_repository.go` (mới), `internal/adapter/postgres/risk_policy_repository.go` (mới), `internal/adapter/postgres/risk_outcome_repository.go` (mới), bản `internal/adapter/mysql/*` tương ứng, và các `_test.go`
**Depends on:** TASK-REQ-030-01 (bảng), TASK-REQ-030-02 (kiểu domain), TASK-REQ-001-04 (executor trong ctx, `TxRunner`)
**Status:** [x] DONE

---

## Context

- `request-service` dùng executor trong ctx: `InTx` tham gia giao dịch khi lồng (SOL-001 mục 2.D, TASK-REQ-001-04), để một use case ghi nhiều bảng cùng commit (ví dụ `AcceptRisk` ghi `risk_acceptances` + outbox). Mọi truy vấn qua `exec(ctx)`; Postgres đặt `set_config('app.tenant_id', $1, true)` mỗi giao dịch; MySQL lọc `tenant_id` ở mọi `WHERE`.
- Mẫu lease hai dialect: `task-service/internal/adapter/postgres/execution_leases.go` (`ClaimForExecution`, `ReleaseExecution`) và `adapter/mysql/execution_leases.go`; SOL-007 `RecoverInterruptedAnalysisRuns` dùng Postgres `UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED) RETURNING`, MySQL `SELECT ... FOR UPDATE SKIP LOCKED` trong giao dịch rồi `UPDATE` (MySQL ≥ 8.0.1). Đồng hồ lease lấy từ **DB** (`now()`/`CURRENT_TIMESTAMP(6)`), không từ máy ứng dụng.
- `impact_assessments` là append-only theo `revision`: chỉ `INSERT` bản mới; bản cũ chuyển `status=superseded` (cột duy nhất được `UPDATE` sau khi `ready`, cùng `narrative`, `finished_at`, lease). Một `collecting` mỗi chủ thể do chỉ mục duy nhất (task 01).
- `risk_acceptance` có hiệu lực **chỉ khi** `assessment_digest` trùng bản đánh giá hiện hành (CR 2.1).
- `risk_policies`: tối đa một `active` và một `shadow` mỗi tenant (chỉ mục duy nhất); chuyển trạng thái trong một giao dịch (đặt bản cũ `retired` rồi bản mới `active`).
- Tên phương thức khác nhau giữa các cổng trên cùng struct `Repository` (bài học `CreateExecutionLink` của `task-service`): đặt tên có tiền tố thực thể.

## Việc cần làm

1. `impact_ports.go`:
   ```go
   type ImpactAssessmentRepository interface {
       InsertCollecting(ctx context.Context, a domain.ImpactAssessment, leaseTTL time.Duration) (domain.ImpactAssessment, bool, error) // bool=true nếu tạo mới; false trả bản collecting đang có
       NextRevision(ctx context.Context, tenantID string, st domain.SubjectType, subjectID string) (int, error)
       GetImpactAssessment(ctx context.Context, tenantID, id string) (domain.ImpactAssessment, error)
       LatestImpactAssessment(ctx context.Context, tenantID string, st domain.SubjectType, subjectID string) (domain.ImpactAssessment, bool, error)
       ListImpactAssessmentsByRequest(ctx context.Context, tenantID, requestID string, st []domain.SubjectType) ([]domain.ImpactAssessment, error)
       RenewImpactLease(ctx context.Context, tenantID, id, owner string, ttl time.Duration) (bool, error)
       CompleteImpactAssessment(ctx context.Context, in CompleteImpactInput) error // ready|partial|failed, ghi dimensions, triggers, findings, level, score, confidence, digest, finished_at; supersede các bản cũ cùng chủ thể trong cùng giao dịch
       SetImpactNarrative(ctx context.Context, tenantID, id string, narrative []byte) error
       ClaimExpiredImpactAssessments(ctx context.Context, limit int) ([]domain.ImpactAssessment, error) // FOR UPDATE SKIP LOCKED
   }
   type ImpactToolRunRepository interface {
       InsertToolRun(ctx context.Context, r domain.ImpactToolRun) error
       ListToolRuns(ctx context.Context, tenantID, assessmentID string) ([]domain.ImpactToolRun, error)
       FindRecentToolRun(ctx context.Context, tenantID, commandDigest string, notBefore time.Time) (domain.ImpactToolRun, bool, error)
   }
   type RiskAcceptanceRepository interface {
       InsertRiskAcceptance(ctx context.Context, a domain.RiskAcceptance) error // UNIQUE trùng thì domain.ErrAlreadyExists
       ListRiskAcceptances(ctx context.Context, tenantID, assessmentID string) ([]domain.RiskAcceptance, error)
       ListValidAcceptances(ctx context.Context, tenantID, requestID, currentDigest string) ([]domain.RiskAcceptance, error)
   }
   type RiskPolicyRepository interface {
       GetActiveRiskPolicy(ctx context.Context, tenantID string) (domain.RiskPolicy, bool, error)
       GetShadowRiskPolicy(ctx context.Context, tenantID string) (domain.RiskPolicy, bool, error)
       InsertRiskPolicy(ctx context.Context, p domain.RiskPolicy) (domain.RiskPolicy, error) // version = max+1 trong giao dịch
       SetRiskPolicyStatus(ctx context.Context, tenantID, id string, to domain.PolicyStatus, now time.Time) error // retire bản cũ cùng giao dịch
   }
   type RiskOutcomeRepository interface {
       UpsertRiskOutcome(ctx context.Context, o domain.RiskOutcome) error
       IncrementOverrideCount(ctx context.Context, tenantID, requestID string) error
       CountOutcomes(ctx context.Context, tenantID string) (OutcomeStats, error) // cho điều kiện bật enforce (task 08)
   }
   ```
2. `InsertCollecting` Postgres: giao dịch: `SELECT ... WHERE status='collecting'` cho chủ thể:
   - có thì trả `(bản đó, false)`
   - không thì `INSERT` với `revision = NextRevision`, `lease_expires_at = now() + $ttl::interval`
   - bắt `unique_violation` (23505) do đua thì đọc lại và trả bản `collecting` hiện có. MySQL: `INSERT` rồi bắt lỗi 1062, đọc lại.
3. `ClaimExpiredImpactAssessments`: Postgres `WITH c AS (SELECT id FROM request.impact_assessments WHERE status='collecting' AND lease_expires_at < now() ORDER BY lease_expires_at LIMIT $1 FOR UPDATE SKIP LOCKED) UPDATE ... SET status='failed', finished_at=now() FROM c WHERE ... RETURNING ...`; MySQL: `SELECT ... FOR UPDATE SKIP LOCKED` trong giao dịch rồi `UPDATE`. Trả các bản vừa chuyển `failed` (mã lỗi lưu vào `findings` bằng `INTERRUPTED` ở use case, không ở đây).
4. `CompleteImpactAssessment`: một giao dịch: `UPDATE ... WHERE id=$1 AND tenant_id=$2 AND status='collecting' AND lease_owner=$owner` (0 dòng thì `ErrLeaseLost`); rồi `UPDATE ... SET status='superseded' WHERE tenant_id AND subject_type AND subject_id AND id <> $1 AND status IN ('ready','partial')`.
5. `FindRecentToolRun`: theo `(tenant_id, command_digest)` với `created_at >= notBefore` và `status='ok'` (chỉ dùng lại kết quả thành công).
6. `ListValidAcceptances`: join logic ở SQL: `risk_acceptances.assessment_digest = $currentDigest` **và** `request_id` khớp; không join sang `impact_assessments` (digest hiện hành được truyền vào).
7. `InsertRiskPolicy`: `INSERT ... SELECT COALESCE(MAX(version),0)+1` trong giao dịch; `SetRiskPolicyStatus(to=active)`: đặt bản `active` hiện có thành `retired` (đặt `activated_at` cho bản mới) trước, vi phạm chỉ mục thì lỗi `ErrPolicyConflict`; quy tắc chuyển trạng thái kiểm bằng `PolicyStatus.CanTransitionTo` (task 02).
8. Kiểm biên dịch: `var _ usecase.ImpactAssessmentRepository = (*Repository)(nil)` v.v. cho cả hai dialect; không import `common/outbox` ở đây (ghi outbox qua `OutboxWriter` của TASK-REQ-001-04).

## Kiểm thử

- Integration hai dialect: `TestInsertCollecting_ReturnsExistingWhenCollecting`, `_ConcurrentCallsOneCreated` (8 goroutine, đúng một `created=true`), `_RevisionIncrements`, `TestCompleteImpactAssessment_SupersedesOlder`, `_LeaseLost`, `TestRenewImpactLease_OnlyOwner`, `TestClaimExpired_SkipLocked` (hai worker không nhận cùng một bản), `TestClaimExpired_ClockFromDB`.
- `TestToolRun_FindRecent_OnlyOk_AndWithinWindow`.
- `TestRiskAcceptance_UniqueConstraint`, `_ListValid_ByDigest` (digest đổi thì không còn hiệu lực), `_TenantIsolation`.
- `TestRiskPolicy_OneActiveOneShadow`, `_VersionIncrements`, `_ActivateRetiresPrevious`, `_InvalidTransitionRejected`.
- `TestRiskOutcome_UpsertAndOverrideCount`, `_CountOutcomes`.
- Postgres RLS: `TestRLS_ImpactRepositories_TenantIsolation` (role không phải superuser).
- JSON Unicode (tiếng Việt trong `findings.message`) qua cả hai dialect.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... && go test -tags=integration ./services/request-service/internal/adapter/postgres/... ./services/request-service/internal/adapter/mysql/...`.

## Tiêu chí hoàn thành

- [x] Chỉ một `collecting` mỗi chủ thể kể cả khi gọi đồng thời; `InsertCollecting` idempotent.
- [x] `CompleteImpactAssessment` đặt bản cũ `superseded` cùng giao dịch và từ chối khi mất lease.
- [x] Quét lease dùng `SKIP LOCKED` ở cả hai dialect, thời gian từ DB.
- [x] `ListValidAcceptances` chỉ trả chấp nhận có digest hiện hành.
- [x] Mọi truy vấn có `tenant_id`; không có `UPDATE` nào trên cột `dimensions`/`findings` của bản `ready` (append-only).
- [x] Hai bản cài thoả cùng bộ test bảng.

## Rủi ro và lưu ý

- `SKIP LOCKED` cần MySQL ≥ 8.0.1; TiDB chưa kiểm.
- Đua giữa `InsertCollecting` và `CompleteImpactAssessment` cho cùng chủ thể: bản `ready` chưa chèn được `collecting` mới ngay trước khi bản cũ `superseded`; chấp nhận vì UNIQUE chỉ áp cho `collecting`.
- `FindRecentToolRun` có thể trả kết quả của cùng lệnh nhưng khác `index_commit`: người gọi (task 05) phải so `index_commit` trước khi dùng lại.
- Chưa có chính sách dọn `impact_tool_runs.output`; ghi vào câu hỏi mở.
