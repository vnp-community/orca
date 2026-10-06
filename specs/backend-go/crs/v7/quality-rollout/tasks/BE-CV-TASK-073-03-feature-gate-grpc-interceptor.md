# BE-CV-TASK-073-03: Interceptor gRPC `feature_gate` với bảng phân loại mọi RPC

**From Solution:** BE-CV-SOL-073
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/grpc/feature_gate.go` (mới), `.../internal/adapter/grpc/feature_gate_test.go` (mới), `.../cmd/server/main.go`
**Depends on:** BE-CV-TASK-073-02, BE-CV-SOL-010 (chuỗi interceptor, `internalcaller.Guard`), BE-CV-SOL-071 task 02 (bộ đếm từ chối)
**Status:** `[ ] TODO`

---

## Context

- Thứ tự (hợp đồng §3 đầu): guard nội bộ → tenant+user → **cờ** → OPA → selector → cổng agent → …
- Mẫu: v6 `TASK-REQ-025-02` (bảng `flowMethodClass`), `common/internalcaller.Guard` (lọc theo `fullMethods`), `common/grpcmw`.
- PQ-01: cờ gốc ⇒ `CODEINTEL_DISABLED`; `QualityGateService` khi cờ chất lượng tắt ⇒ `CODEINTEL_QUALITY_GATE_DISABLED` (`FailedPrecondition`); AI ⇒ `CODEINTEL_AI_REVIEW_DISABLED`; profile bảo mật khi cờ quét tắt ⇒ `CODEINTEL_PROFILE_UNKNOWN` (xử lý ở use case `StartQualityRun`, không ở interceptor).
- Ngoại lệ khi tắt (PQ-24): `GetSettings`, `SetSettings`, `GetReindexJob`.

## Việc cần làm

1. `feature_gate.go`: `type gateClass int` (`gateOpen`, `gateCore`, `gateQuality`, `gateAI` — `gateAI` cũng yêu cầu `gateQuality`); bảng `methodGate map[string]gateClass` đủ **49** RPC của hai service (+ các RPC thêm sau).
2. Interceptor unary + stream: tra `EffectiveFlags` theo `tenant.RequireTenantID`; lỗi đọc cờ ⇒ coi tắt; RPC không có trong bảng ⇒ từ chối như `gateCore` và `slog.Warn`.
3. `StreamCodeIntelEvents` khi tắt: từ chối mở luồng; luồng đang mở khi cờ tắt giữa chừng ngừng phát sự kiện mới (kiểm lại cờ mỗi sự kiện, cache 5 s).
4. Nối vào `main.go` sau `internalcaller.StreamGuard`/`Guard` và trích tenant.
5. Mã lỗi dạng `apperrors` (`CODEINTEL_DISABLED: …`, `KindFailedPrecondition`).

## Kiểm thử

- `TestEveryRPCHasGateClass` (đỏ khi RPC trong `ServiceDesc` thiếu dòng, và ngược lại).
- Tắt: `gateOpen` qua; `gateCore`, `gateQuality`, `gateAI` bị chặn đúng mã; chỉ cờ chất lượng tắt ⇒ `CodeIntelService` vẫn chạy; lỗi đọc ⇒ chặn; tenant A bật không ảnh hưởng B.
- `go test ./internal/adapter/grpc/... -run FeatureGate`.

## Tiêu chí hoàn thành

- [ ] Mọi RPC có phân loại.
- [ ] Ba RPC ngoại lệ chạy khi tắt; consumer sự kiện/outbox (không qua gRPC) không bị chặn.

## Rủi ro và lưu ý

- Ranh giới với BE-CV-SOL-013 (Q1 của SOL-073): thống nhất một bộ đọc cờ trước khi viết.
- Metric `disabled_rejections_total` nối ở task 071-02.
