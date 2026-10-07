# BE-CV-TASK-010-03: Module `code-intel-service`, `config.go`, đăng ký `go.work` và `Makefile`

**From Solution:** BE-CV-SOL-010-scaffold-code-intel-service
**Priority:** P0
**Service:** `code-intel-service` (mới)
**File:** `backend-go/services/code-intel-service/go.mod` (mới), `internal/config/config.go` (mới), `internal/config/config_test.go` (mới), `backend-go/go.work`, `backend-go/Makefile`
**Depends on:** BE-CV-TASK-010-01
**Status:** [x] DONE

---

## Context

Mẫu cấu hình: `services/mcp-service/internal/config/config.go` (có `boolEnv` dòng 54, `intEnv` dòng 66) và `common/config` (`LoadBase`, `StringEnv`; không có `BoolEnv`/`DurationEnv`). Danh sách biến và mặc định ở SOL-010 mục 2.C (đã áp hợp đồng PQ-14, PQ-23, PQ-24). `go.mod` các service ghi `go 1.25.0`, `go.work` ghi `go 1.26.0`.

## Việc cần làm

1. Tạo `go.mod`: `module github.com/stablyai/orca-go/services/code-intel-service`, `go 1.25.0`; `replace`/`require` theo cách `task-service/go.mod` (module `common`, `proto` qua `go.work`). Chạy `make tidy-all` khi triển khai.
2. `config.go`: `type Config struct { commonconfig.Base; DatabaseCredentialsFile, NATSURL, ProjectServiceAddr, InfraFleetServiceAddr, GitGatewayServiceAddr, AuthServiceAddr, OPABundlePath string; Enabled, QualityGateEnabled, AIReviewEnabled, TenantDefaultEnabled, TenantDefaultQualityGateEnabled bool; InternalCallerToken string; SnapshotMaxBytes, SnapshotTenantQuotaBytes, SnapshotBindingQuotaBytes int64; SnapshotTTL, MaintenanceInterval, OrphanRetention, ReindexStaleAfter, BindingIdleRetention, StatusTTL, StatusTimeout, FlagCacheTTL time.Duration }`.
3. `Load()` dùng `commonconfig.LoadBase("code-intel-service")`; hàm nội bộ `boolEnv`, `int64Env`, `durationEnv` **trả lỗi** khi giá trị sai (không nuốt). `SnapshotMaxBytes` vượt `8388608` → lỗi (PQ-14 (2)).
4. Mặc định đúng bảng SOL-010 2.C: `SnapshotMaxBytes=3145728`, `SnapshotTTL=168h`, quota tenant `536870912`, binding `67108864`, `MaintenanceInterval=10m`, `OrphanRetention=168h`, `ReindexStaleAfter=45m`, `BindingIdleRetention=2160h`, `StatusTTL=15s`, `StatusTimeout=10s`, `FlagCacheTTL=5s`; ba công tắc và hai `TenantDefault*` mặc định `false`.
5. Thêm `./services/code-intel-service` vào `backend-go/go.work` giữa `./services/automation-service` và `./services/credential-broker-service` (giữ thứ tự chữ cái).
6. Thêm `code-intel-service` vào `SERVICES` của `backend-go/Makefile` (dòng 7–11), giữ định dạng nhiều dòng.
7. `InternalCallerToken` rỗng: `Load` không lỗi (log WARN ở `main`), nhưng `Config` có phương thức `InternalCallerConfigured() bool`.

## Kiểm thử

- `cd backend-go && go test ./services/code-intel-service/internal/config/...`: mặc định; ghi đè từng biến; `CODEINTEL_ENABLED=maybe` → lỗi; `CODEINTEL_SNAPSHOT_TTL=abc` → lỗi; `CODEINTEL_SNAPSHOT_MAX_BYTES=9000000` → lỗi; ba công tắc độc lập nhau.
- `make build` thấy module mới (khi triển khai).

## Tiêu chí hoàn thành

- [x] `config.Load()` trả mặc định theo hợp đồng; giá trị sai trả lỗi.
- [x] `go.work`, `Makefile` có module mới; `make vet` chạy được trên module.
- [x] Không tên file `helpers|utils|common|misc`.

## Rủi ro và lưu ý

- Tên env của api-gateway (`CODE_INTEL_SERVICE_ADDR`…) **không** ở đây (SOL-040).
- `go.work.sum` có thể đổi khi thêm module; commit cùng PR.
