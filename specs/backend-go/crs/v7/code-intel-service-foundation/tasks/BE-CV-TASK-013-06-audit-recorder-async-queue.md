# BE-CV-TASK-013-06: `AuditRecorder` bất đồng bộ bọc `auditclient.Append`

**From Solution:** BE-CV-SOL-013-authorization-flags-and-audit
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/audit_recorder.go`, `internal/adapter/grpcclient/audit_recorder.go`, `_test.go` (mới); `cmd/server/main.go` (nối)
**Depends on:** BE-CV-TASK-012-04 (`dial.go`), 013-01
**Status:** [x] DONE

---

## Context

`auditclient.Client.Append(ctx, tenantID, actorID, action, target, outcome, ip)` đồng bộ và **nuốt lỗi**; `outcome` ngoài `allowed|denied` bị `auth-service` từ chối và mất lặng lẽ (đã đọc `common/auditclient/client.go`). Không sửa `common`. Bảng hành động ở SOL-013-authorization 2.F.

## Việc cần làm

1. `usecase/audit_recorder.go`: `AuditOutcome` (hằng `AuditAllowed`, `AuditDenied`), `AuditEntry{Action, Target string; Outcome AuditOutcome}`, `AuditRecorder.Record(ctx, e)`; hàm dựng `NewAuditEntry` cắt `Target` ≤ 200 ký tự, kiểm `Action` không rỗng.
2. `grpcclient/audit_recorder.go`: hàng đợi chan 256, 2 worker; mỗi mục gọi `Append` với `context.WithTimeout(2s)` (ctx **tách huỷ** khỏi RPC gốc: `context.WithoutCancel`); sao `tenantID/userID/ip` từ ctx lúc `Record` (không đọc ctx sau); đầy → bỏ mục + tăng bộ đếm `DroppedTotal()` (nối SOL-071 sau); `Close(ctx)` xả tối đa 5 s.
3. Không log nội dung; log `audit=true`, `action`, `outcome`, `trace_id`.
4. `main.go`: dựng `auditclient.New(authv1.NewAuthServiceClient(conn))` tới `AUTH_SERVICE_ADDR`; nil client → recorder no-op (test không cần auth).
5. Hằng tên hành động `codeintel.*` đặt ở một file domain (`audit_actions.go`).

## Kiểm thử

- Unit: `auth-service` giả chậm 5 s → `Record` quay lại ngay; hàng đợi đầy → bộ đếm tăng, RPC không chặn; shutdown xả; `Outcome` ngoài hằng không biên dịch/được dựng; `Target` cắt 200; tenant/user lấy đúng tại thời điểm `Record`.
- `go test ./services/code-intel-service/... -run AuditRecorder`

## Tiêu chí hoàn thành

- [x] RPC không chờ `auth-service`.
- [x] Mất bản ghi đo được qua bộ đếm.

## Rủi ro và lưu ý

- Audit có thể mất khi `auth-service` lỗi hoặc `target` vượt giới hạn cột (chưa kiểm chứng, 013-01).
- Không có `metadata_json`; chi tiết vào log (Q3).
