# BE-CV-TASK-013-07: Bảng chính sách theo RPC, interceptor pipeline và ánh xạ lỗi gRPC (`CODEINTEL_X: msg | {json}`)

**From Solution:** BE-CV-SOL-013-authorization-flags-and-audit
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/grpc/{rpc_policy_table.go,policy_interceptor.go,status_mapping.go}` và `_test.go` (mới); `internal/domain/coded_data.go` (mới); `cmd/server/main.go` / `buildServerOptions` (sửa)
**Depends on:** BE-CV-TASK-013-03, 013-04, 013-05, 013-06, 010-02
**Status:** [ ] TODO

---

## Context

Hợp đồng §3 (chuỗi 9 bước), PQ-02 (3) (mã ở đầu `status.Message`, dữ liệu ở hậu tố `" | {json}"` ≤ 2 KiB, không `status.Details`), PQ-01, PQ-03. Bảng "Action OPA" ở §3.1/§3.2.

## Việc cần làm

1. `rpc_policy_table.go`: `map[string]rpcPolicy` theo `FullMethod` cho **mọi** RPC hiện có (ở thời điểm này: `BindRepo`, `ListRepoBindings`, `GetIndexStatus`; thêm dòng khi CR khác thêm RPC) với `Action`, `FlagScope` (`CodeIntel|QualityGate|AIReview`), `AllowWhenDisabled`, `ProjectFrom` (hàm lấy `project_id` từ request qua interface `ProjectScoped{GetProjectId() string}` hoặc `GetSelector().GetProjectId()`), `AuditAction`. Test: tập khoá bằng tập method của các `ServiceDesc` (thiếu/thừa đều đỏ).
2. `policy_interceptor.go`: unary + stream theo SOL 2.D bước 2–7: identity (`CODEINTEL_NO_TENANT`), L1 (cổng tiêm nil-an-toàn; SOL-013-gate), cờ (`FlagReader`), `ProjectAuthorizer`, gọi handler, audit. Stream: kiểm cờ/quyền cho `selectors` ở handler (SOL-024), interceptor chỉ kiểm identity và cờ.
3. `status_mapping.go`: `toStatus(err) error`: lấy `*apperrors.AppError`; `code = status mã theo Kind` (qua `apperrors.ToGRPCStatus`), thêm hậu tố `" | " + json` khi lỗi mang `domain.CodedData` (map chuỗi→giá trị, ≤ 2 KiB: bỏ phần tử đến khi vừa; JSON hợp lệ; không xuống dòng; phần người đọc ≤ 200 ký tự, không chứa `" | {"`).
4. Bảng mã → Kind/gRPC: `NOT_AUTHORIZED` PermissionDenied, `DISABLED`/`QUALITY_GATE_DISABLED`/`AI_REVIEW_DISABLED` FailedPrecondition, `AUTHZ_UNAVAILABLE`/`DEV_SERVER_OFFLINE`/`UNAVAILABLE` Unavailable, `NO_TENANT` Unauthenticated, hạn mức ResourceExhausted, `NOT_FOUND` NotFound.
5. Nối vào `buildServerOptions` đúng thứ tự: `ChainUnary` (recovery, tenant, log) → `internalcaller` (013-03) → pipeline.
6. Không log payload/`finding_key` dài/ghi chú review; log `method`, `code`, `duration`.

## Kiểm thử

- Unit (bufconn): từng bước từ chối đúng mã và **không** đi bước sau (fake đếm gọi `project-service`/handler); cờ trước quyền; `AllowWhenDisabled` chạy khi tắt; hậu tố JSON ≤ 2 KiB và hợp lệ; parse bằng regex PQ-02 `^(CODEINTEL_[A-Z0-9_]+): (.*?)(?: \| (\{.*\}))?$` khớp mọi lỗi sinh ra.
- An toàn: id project tenant khác không lộ khác biệt tồn tại; RPC không có tenant bị từ chối.
- `go test ./services/code-intel-service/internal/adapter/grpc/...`

## Tiêu chí hoàn thành

- [ ] Mọi RPC có dòng chính sách; thêm RPC thiếu dòng làm test đỏ.
- [ ] Thứ tự guard → identity → cờ → quyền đúng.
- [ ] Lỗi tới gateway đúng cú pháp PQ-02.

## Rủi ro và lưu ý

- `LoggingInterceptor` chỉ unary: log RPC stream ở handler.
- Mã `CODEINTEL_DISABLED` làm frontend ẩn toàn bộ tính năng (PQ-01); không dùng nhầm cho cờ phụ.
