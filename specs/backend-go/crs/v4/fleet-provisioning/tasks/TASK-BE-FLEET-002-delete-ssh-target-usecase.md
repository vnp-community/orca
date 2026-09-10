# TASK-BE-FLEET-002: Usecase `DeleteSshTarget` — dọn record mồ côi (compensating rollback)

**Solution:** BE-FLEET-SOL-001 | **CR:** CR-FLEET-001
**Service:** `infra-fleet-service`
**Depends on:** Không (làm trước TASK-BE-FLEET-001, vì `BulkProvisionFleet` gọi usecase này)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** `SshTargetRepository` (ports.go) thêm `Delete(ctx,
> tenantID, id) error` đúng như thiết kế. `impact({target:
> "SshTargetRepository", direction: "upstream", repo: "orca"})` chạy trước khi
> sửa: MEDIUM risk, impactedCount 11 (8 direct) — implementer thật là
> `SshTargetStore` (postgres/repository.go) + fake test double duy nhất
> `fakeSshTargetRepository` (định nghĩa 1 chỗ trong `create_ssh_target_test.go`,
> dùng chung bởi `establish_connection_test.go`/`get_ssh_state_test.go`/
> `list_ssh_targets_test.go` trong cùng package) — cả 2 đã cập nhật thêm
> `Delete`, không có implementer nào khác bị bỏ sót (xác nhận bằng
> `grep -rn "SshTargetRepository"`).
>
> **Sai khác so với mô tả task:** code mẫu trong task dùng `r.db.ExecContext`
> nhưng implementer thật (`SshTargetStore`) dùng `pgxpool.Pool` (`s.pool.Exec`)
> — đã điều chỉnh theo code thật, giữ nguyên logic
> `DELETE ... WHERE id = $1 AND tenant_id = $2`.
>
> **Build/test thật đã chạy**: `go build ./...` sạch. `go test
> ./internal/usecase/... ./internal/adapter/postgres/... -run SshTarget -v` —
> 9/9 PASS (bao gồm 3 test mới `TestDeleteSshTarget_*`). `go vet
> -tags=integration ./internal/adapter/postgres/...` sạch (xác nhận code
> integration-tagged biên dịch được). Docker sẵn có trong môi trường này —
> ĐÃ CHẠY THẬT `go test -tags=integration ./internal/adapter/postgres/... -run
> TestRepository_Delete -v`: 2/2 PASS
> (`TestRepository_Delete_RemovesRow`, `TestRepository_Delete_ScopedByTenant`,
> qua testcontainers postgres:16-alpine, ~9s). `gofmt -l` sạch trên mọi file
> đã sửa/thêm.

---

## Mục tiêu

Thêm usecase `DeleteSshTarget` — xoá 1 `SshTarget` theo `id` (scoped theo tenant), dùng bởi
`BulkProvisionFleet` (TASK-BE-FLEET-001) để dọn record mồ côi khi `RegisterDevServer` fail sau khi
`CreateSshTarget` đã thành công. Thêm method `Delete` vào `SshTargetRepository` port + implement Postgres
adapter.

## Files cần sửa

1. `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (MODIFY — `SshTargetRepository` thêm `Delete`)
2. `backend-go/services/infra-fleet-service/internal/usecase/delete_ssh_target.go` (MỚI)
3. `backend-go/services/infra-fleet-service/internal/usecase/delete_ssh_target_test.go` (MỚI)
4. `backend-go/services/infra-fleet-service/internal/adapter/postgres/repository.go` (MODIFY — thêm `Delete`)
5. `backend-go/services/infra-fleet-service/internal/adapter/postgres/repository_test.go` (MODIFY hoặc file test riêng nếu convention repo yêu cầu — kiểm tra file test Postgres hiện có trước khi quyết định)

## `ports.go` — `SshTargetRepository` thêm `Delete`

```go
// SshTargetRepository is the persistence port for SSH target registration.
type SshTargetRepository interface {
	Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error)
	Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error)
	List(ctx context.Context, tenantID string) ([]domain.SshTarget, error)
	// Delete removes an SSH target scoped to tenantID — used by
	// DeleteSshTarget's compensating-rollback path in BulkProvisionFleet
	// (CR-FLEET-001) when RegisterDevServer fails after CreateSshTarget
	// already succeeded.
	Delete(ctx context.Context, tenantID, id string) error
}
```

**Cảnh báo:** thêm method vào interface này ảnh hưởng MỌI implementer/fake hiện có của `SshTargetRepository`
— chạy `impact()` trước khi sửa (xem mục gitnexus bên dưới), và cập nhật **mọi** fake test double đang
implement interface này (tìm bằng `grep -rn "SshTargetRepository" backend-go/services/infra-fleet-service`
trước khi sửa, không chỉ sửa file được liệt kê ở trên nếu grep phát hiện thêm).

## `delete_ssh_target.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// DeleteSshTarget removes an SshTarget by ID, scoped to the caller's
// tenant. Used as a compensating action by BulkProvisionFleet when
// RegisterDevServer fails after CreateSshTarget already committed —
// see CR-FLEET-001's "Rollback semantics" (no cross-usecase DB
// transaction exists, so orphaned rows are cleaned up explicitly).
type DeleteSshTarget struct {
	repo SshTargetRepository
}

func NewDeleteSshTarget(repo SshTargetRepository) *DeleteSshTarget {
	return &DeleteSshTarget{repo: repo}
}

func (uc *DeleteSshTarget) Execute(ctx context.Context, id string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := uc.repo.Delete(ctx, tenantID, id); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_DELETE_SSH_TARGET_FAILED", "failed to delete ssh target", err)
	}
	return nil
}
```

## Postgres adapter — `repository.go`

```go
// Delete removes the ssh_targets row scoped to tenantID. Cạnh INSERT hiện
// có (dòng ~203 tại thời điểm viết task — xác nhận lại vị trí thật khi
// implement, số dòng có thể lệch nếu file đã đổi).
func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM infra.ssh_targets WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	return err
}
```

## Test cases cần cover

- `TestDeleteSshTarget_RequiresTenantContext` — mirror `TestCreateSshTarget_RequiresTenantContext` đã có (ctx
  không có tenant → lỗi `INFRA_NO_TENANT`).
- `TestDeleteSshTarget_CallsRepositoryDeleteWithTenantFromContext` — fake repo, xác nhận `Delete` được gọi
  đúng `(tenantID, id)`.
- `TestDeleteSshTarget_RepositoryErrorWrapped` — fake repo trả lỗi → usecase trả `apperrors` kind `Internal`,
  code `INFRA_DELETE_SSH_TARGET_FAILED`.
- Postgres adapter test (nếu repo có test tầng adapter chạy DB thật/testcontainer — kiểm tra convention hiện
  có trước khi viết, mirror cách `repository_test.go` test `Create`/`Get`/`List` nếu tồn tại):
  `TestRepository_Delete_RemovesRow`, `TestRepository_Delete_ScopedByTenant` (xoá của tenant A không ảnh
  hưởng tenant B, dù cùng id — trường hợp khó xảy ra vì `id` là UUID nhưng vẫn nên test đúng WHERE clause).

## Verify

```bash
cd backend-go/services/infra-fleet-service && go build ./... && go test ./internal/usecase/... ./internal/adapter/postgres/... -run SshTarget -v
gofmt -l internal/usecase/delete_ssh_target.go internal/usecase/ports.go internal/adapter/postgres/repository.go
```

## gitnexus

`impact({target: "SshTargetRepository", direction: "upstream"})` **bắt buộc trước khi sửa interface** — xác
nhận toàn bộ implementer/fake hiện có (Postgres adapter, mọi fake test double dùng interface này trong
`create_ssh_target_test.go` và các file test khác) để không phá build khi thêm method `Delete`. `DeleteSshTarget`
là symbol mới — chạy `impact()` cho nó ngay sau khi tạo, trước khi TASK-BE-FLEET-001 thêm caller.

## Blocking

TASK-BE-FLEET-001 (`BulkProvisionFleet`) phụ thuộc cứng `DeleteSshTarget` đã tồn tại ở đây.
