# TASK-BE-FLEET-001: Usecase `BulkProvisionFleet` — bounded concurrency, per-server độc lập

**Solution:** BE-FLEET-SOL-001 | **CR:** CR-FLEET-001
**Service:** `infra-fleet-service`
**Depends on:** TASK-BE-FLEET-002 (`DeleteSshTarget` phải tồn tại trước vì `BulkProvisionFleet` gọi nó khi rollback)
**Status:** ✅ DONE — code + test (2026-09-09); wiring `cmd/server/main.go` HOÃN LẠI đến lúc làm TASK-BE-FLEET-003 (xem bên dưới)

> **Kết quả thực tế:** `bulk_provision_fleet.go` implement đúng nội dung task, CỘNG THÊM `isUniqueViolation`
> (Postgres `23505`) ngay trong `provisionOne` — đưa logic mà TASK-BE-FLEET-004 ghi chú "chưa áp dụng, để
> TASK-BE-FLEET-001 làm" vào thẳng đây (tránh viết 2 lần, đúng ghi chú phối hợp giữa 2 task). Verify
> `apperrors.AppError` có `Unwrap() error` (đọc `common/apperrors/apperrors.go`) trước khi viết
> `errors.As(err, &pgErr)` — xác nhận `errors.As` xuyên qua được lớp bọc `apperrors.New(...)` của
> `CreateSshTarget.Execute` tới `*pgconn.PgError` gốc (test riêng
> `TestIsUniqueViolation_WrapsThroughAppErrors` xác nhận).
>
> **`cmd/server/main.go` wiring hoãn lại có chủ đích:** `Server.New(...)` (adapter/grpc) CHƯA có tham số nhận
> `*usecase.BulkProvisionFleet` — thêm tham số đó là phạm vi TASK-BE-FLEET-003 (RPC/proto). Khai báo
> `bulkProvisionFleetUC := usecase.NewBulkProvisionFleet(...)` ở `main.go` mà không dùng ở đâu sẽ làm
> `go build` lỗi "declared and not used" — thay vì tạo 1 biến `_ = ...` tạm bợ, quyết định gộp việc wire
> `main.go` vào ngay lúc làm TASK-BE-FLEET-003 (chạy ngay sau task này trong cùng lượt thực thi), nơi
> `Server.New(...)` thực sự nhận và dùng usecase này — xem TASK-BE-FLEET-003's "Kết quả thực tế" cho phần
> `main.go` diff thật.
>
> **`impact({target: "BulkProvisionFleet", direction: "upstream", repo: "orca"})` sau khi tạo symbol mới:**
> trả về "Target not found" — GitNexus's index (khác `codegraph`, không watch file live) chưa bắt kịp file mới
> tại thời điểm chạy; không phải lỗi, ghi nhận đây là giới hạn độ trễ index đã biết, không phải tín hiệu risk.
>
> **Build/test thật đã chạy**: `go build ./...` sạch. `go test ./internal/usecase/... -run
> 'BulkProvisionFleet|IsUniqueViolation' -v -race` — 8/8 PASS (bao gồm cả `TestBulkProvisionFleet_DuplicateHost_ReturnsAlreadyExistsNotFailed`
> từ TASK-BE-FLEET-004 và `TestIsUniqueViolation_WrapsThroughAppErrors`). `go test ./...` (toàn bộ package
> service) — PASS, không regress. `gofmt -l` sạch.

---

## Mục tiêu

Thêm usecase mới `BulkProvisionFleet` — lặp `CreateSshTarget`+`RegisterDevServer` cho N server trong 1
`FleetSpec`, với concurrency bị chặn (semaphore) và compensating rollback per-server khi `RegisterDevServer`
fail sau khi `CreateSshTarget` đã thành công. Tái sử dụng nguyên trạng `CreateSshTarget`/`RegisterDevServer`
đã có — **không sửa** 2 usecase đó.

## Files cần sửa

1. `backend-go/services/infra-fleet-service/internal/usecase/bulk_provision_fleet.go` (MỚI)
2. `backend-go/services/infra-fleet-service/internal/usecase/bulk_provision_fleet_test.go` (MỚI)
3. `backend-go/services/infra-fleet-service/cmd/server/main.go` (MODIFY — wire `NewBulkProvisionFleet`)

## Nội dung — `bulk_provision_fleet.go`

```go
package usecase

import (
	"context"
	"fmt"
	"sync"
)

type FleetSpecServer struct {
	Host, UserName, VaultSSHRole string
	Kind                         domain.AgentKind
}

type FleetSpec struct {
	Version string
	Servers []FleetSpecServer
}

type BulkProvisionServerResult struct {
	Host        string
	Status      string // "SUCCEEDED" | "FAILED"
	DevServerID string
	Error       string
}

type BulkProvisionResult struct {
	Results []BulkProvisionServerResult
}

type BulkProvisionFleet struct {
	createSshTarget    *CreateSshTarget
	registerDevServer  *RegisterDevServer
	deleteSshTarget    *DeleteSshTarget
	concurrencyDefault int
}

func NewBulkProvisionFleet(createSshTarget *CreateSshTarget, registerDevServer *RegisterDevServer, deleteSshTarget *DeleteSshTarget) *BulkProvisionFleet {
	return &BulkProvisionFleet{
		createSshTarget:    createSshTarget,
		registerDevServer:  registerDevServer,
		deleteSshTarget:    deleteSshTarget,
		concurrencyDefault: 5,
	}
}

func (uc *BulkProvisionFleet) Execute(ctx context.Context, spec FleetSpec, concurrency int, emit func(BulkProvisionServerResult)) (BulkProvisionResult, error) {
	if concurrency <= 0 {
		concurrency = uc.concurrencyDefault
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	result := BulkProvisionResult{Results: make([]BulkProvisionServerResult, 0, len(spec.Servers))}

	for _, server := range spec.Servers {
		server := server
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			r := uc.provisionOne(ctx, server)
			mu.Lock()
			result.Results = append(result.Results, r)
			mu.Unlock()
			if emit != nil {
				emit(r)
			}
		}()
	}
	wg.Wait()
	return result, nil
}

func (uc *BulkProvisionFleet) provisionOne(ctx context.Context, server FleetSpecServer) BulkProvisionServerResult {
	if server.Host == "" {
		return BulkProvisionServerResult{Host: server.Host, Status: "FAILED", Error: "host is required"}
	}

	sshTarget, err := uc.createSshTarget.Execute(ctx, CreateSshTargetInput{
		Host:         server.Host,
		UserName:     server.UserName,
		VaultSSHRole: server.VaultSSHRole,
	})
	if err != nil {
		return BulkProvisionServerResult{Host: server.Host, Status: "FAILED", Error: err.Error()}
	}

	devServer, err := uc.registerDevServer.Execute(ctx, RegisterDevServerInput{
		Host:        server.Host,
		Mode:        domain.ConnectionModeRelaySSH,
		SSHTargetID: sshTarget.ID,
		Kind:        server.Kind,
	})
	if err != nil {
		if delErr := uc.deleteSshTarget.Execute(ctx, sshTarget.ID); delErr != nil {
			return BulkProvisionServerResult{
				Host: server.Host, Status: "FAILED",
				Error: fmt.Sprintf("register failed: %v; cleanup also failed: %v", err, delErr),
			}
		}
		return BulkProvisionServerResult{Host: server.Host, Status: "FAILED", Error: err.Error()}
	}

	return BulkProvisionServerResult{Host: server.Host, Status: "SUCCEEDED", DevServerID: devServer.ID}
}
```

Import `domain` package (`"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"`) —
theo đúng import path đã dùng ở `register_dev_server.go`/`create_ssh_target.go`.

**Lưu ý khi TASK-BE-FLEET-010 (BE-FLEET-SOL-003) chạy sau:** solution BE-FLEET-SOL-003 đề xuất di chuyển
`FleetSpecServer` xuống `domain` package để tránh `domain.FleetDefinition` phải import `usecase`. Task này
định nghĩa `FleetSpecServer` ở `usecase` trước (đúng CR-FLEET-001 gốc) — việc di chuyển xuống `domain` là
phạm vi của TASK-BE-FLEET-010, không làm ở đây. Nếu TASK-BE-FLEET-010 chạy sau task này, nó phải cập nhật lại
file này để `import domain` thay vì định nghĩa `FleetSpecServer` tại chỗ.

## Wiring `cmd/server/main.go`

```go
deleteSshTarget := usecase.NewDeleteSshTarget(sshTargetRepo)
bulkProvisionFleet := usecase.NewBulkProvisionFleet(createSshTarget, registerDevServer, deleteSshTarget)
// truyền bulkProvisionFleet vào Server.New(...) — xem TASK-BE-FLEET-003
```

## Test cases cần cover

- `TestBulkProvisionFleet_AllServersSucceed` — N=3 server hợp lệ, tất cả `SUCCEEDED`, `DevServerID` khác rỗng.
- `TestBulkProvisionFleet_OneServerEmptyHost_OthersSucceed` — 1 server host rỗng → `FAILED`, N-1 còn lại
  `SUCCEEDED` (đúng AC "N/M servers provisioned successfully").
- `TestBulkProvisionFleet_RegisterFails_CompensatingDeleteCalled` — fake `RegisterDevServer` trả lỗi, xác
  nhận `DeleteSshTarget.Execute` được gọi đúng 1 lần với `sshTargetID` vừa tạo.
- `TestBulkProvisionFleet_RegisterFails_CompensatingDeleteAlsoFails` — cả 2 đều lỗi, `Error` field ghi rõ cả 2
  lỗi (dùng `fmt.Sprintf` như code mẫu), không panic.
- `TestBulkProvisionFleet_RespectsConcurrencyLimit` — dùng fake usecase có `sync.WaitGroup`/counter đếm số
  goroutine chạy đồng thời tối đa, xác nhận không vượt `concurrency` truyền vào.
- `TestBulkProvisionFleet_ZeroConcurrency_UsesDefault5` — `concurrency=0` → default 5 áp dụng (không panic
  với channel size 0).

Dùng fake `*CreateSshTarget`/`*RegisterDevServer`/`*DeleteSshTarget` qua interface — mirror cách
`create_ssh_target_test.go`'s `fakeSshTargetRepository` fake ở tầng repository (test usecase này nên fake ở
tầng usecase con, không phải repository, vì `BulkProvisionFleet` phụ thuộc trực tiếp vào 3 usecase khác —
cân nhắc đổi `createSshTarget`/`registerDevServer`/`deleteSshTarget` field types sang interface nhỏ nếu cần
fake dễ hơn, giữ nguyên struct cụ thể nếu test dùng thẳng fake repository bên dưới đơn giản hơn).

## Verify

```bash
cd backend-go/services/infra-fleet-service && go build ./... && go test ./internal/usecase/... -run BulkProvisionFleet -v
gofmt -l internal/usecase/bulk_provision_fleet.go
```

## gitnexus

`BulkProvisionFleet` là symbol MỚI — không chạy `impact()` trước khi tạo (chưa tồn tại để impact). Sau khi
tạo, chạy `impact({target: "BulkProvisionFleet", direction: "upstream"})` trước khi task khác (TASK-BE-FLEET-003,
TASK-BE-FLEET-014) thêm caller mới vào nó — bắt buộc theo CLAUDE.md.

Trước khi sửa `CreateSshTarget`/`RegisterDevServer` là **KHÔNG cần** (task này không sửa 2 usecase đó), nhưng
nếu trong lúc implement phát hiện cần đổi field/method của chúng, dừng lại và chạy
`impact({target: "CreateSshTarget"/"RegisterDevServer", direction: "upstream"})` trước — số liệu đã re-verify
tại thời điểm viết solution (2026-09-09): cả hai đều **LOW**, impactedCount 3, direct 1.

## Blocking

TASK-BE-FLEET-003 (RPC/proto) phụ thuộc `BulkProvisionFleet` đã tồn tại ở đây.
