# TASK-REQ-035-02: `x-orca-actor-type` (grpcmw, tenant, gateway) và token nội bộ gateway → `request-service`

**From Solution:** BE-REQ-SOL-035 (mục B, C1, C2)
**Priority:** P0
**Service:** `backend-go/common`, `api-gateway`
**File:** `backend-go/common/grpcmw/grpcmw.go` (sửa: hằng), `backend-go/common/tenant/tenant.go` (sửa: `WithActorType`, `ActorType`), `backend-go/common/tenant/tenant_test.go` (sửa), `backend-go/services/api-gateway/internal/adapter/grpc/dial.go` (sửa: `AttachIdentity`), `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/executor.go` (sửa: `CallTool`), `backend-go/services/api-gateway/cmd/server/request_wiring.go` (sửa/mới: token và dial, do TASK-REQ-016-01 tạo), và `_test.go` tương ứng
**Depends on:** TASK-REQ-016-01 (dial `request-service` ở gateway)
**Status:** [ ] TODO (một phần: phần `common` và `api-gateway` đã làm và có test; còn thiếu nối `dialRequestService` vào `main.go`, xem mục Tiến độ)

---

## Context

- `grpcmw.go`: các hằng `MetadataTenantID`, `MetadataUserID`, `MetadataRole`, `MetadataClientIP`; `TenantExtractionInterceptor` đọc bốn khoá. Comment trong file: `ChainUnary`/`StatsHandler` được cả 16 service gọi (blast radius CRITICAL theo `impact`); vì vậy task này **chỉ thêm hằng và hàm**, không đổi chữ ký hay hành vi của `TenantExtractionInterceptor`/`ChainUnary`. Việc đọc metadata thành ctx nằm trong `ActorTypeInterceptor` của `request-service` (task 04).
- `tenant.go`: khoá ctx `contextKey{name}`; `WithRole/Role`, `WithClientIP/ClientIP` là mẫu. `AttachIdentity(ctx, usecase.Identity)` ở `api-gateway/internal/adapter/grpc/dial.go:49` đọc `tenant.ClientIP(ctx)` (chú thích: "every existing call site picks it up with no signature change") và được gọi ở ~20 file `wscompat/channels_*.go`. Làm theo đúng khuôn đó cho actor type: không đổi `usecase.Identity`, không sửa 20 điểm gọi.
- `executor.go:138`: `CallTool` dựng `wscompat.Identity{TenantID, UserID, Role}` từ `mcpserver.Principal`; đây là điểm duy nhất mọi tool MCP đi qua trước `dispatch` (đọc `executor.go:126-140` trước khi sửa). Resource MCP (`resources/provider.go`) cũng gọi kênh nền: kiểm bằng `grep -rn "Dispatch(" internal/adapter/mcpserver` rằng có đường tương tự, và đánh dấu `agent` ở đó (ngoài ra là chỉ đọc nên rủi ro thấp, nhưng nhất quán).
- Mẫu token nội bộ ở gateway: `cmd/server/mcp_governance_wiring.go:27`: `os.Getenv("MCP_INTERNAL_CALLER_TOKEN")` ⇒ `grpc.WithChainUnaryInterceptor(internalcaller.ClientInterceptor(tok))` và `StreamClientInterceptor`. Tên biến cho `request-service`: **`REQUEST_INTERNAL_CALLER_TOKEN`** phía gateway (cùng quy ước `*_INTERNAL_CALLER_TOKEN`), khớp `GATEWAY_INTERNAL_TOKEN` phía `request-service` (CR 035 dùng tên này cho server).
- Chạy `gitnexus_impact` trên `AttachIdentity`, `Executor.CallTool`, `tenant.WithRole` trước khi sửa; báo mức rủi ro (CRITICAL nếu hiện ra) trong PR.

## Việc cần làm

1. `grpcmw.go`: thêm `MetadataActorType = "x-orca-actor-type"` kèm comment một dòng ("giá trị `user|agent|system`; thiếu nghĩa là `user`"). Không đổi gì khác.
2. `tenant.go`: thêm `actorTypeKey = &contextKey{"actor_type"}`, `func WithActorType(ctx, actorType string) context.Context`, `func ActorType(ctx) string` trả `"user"` khi thiếu hoặc giá trị lạ (chỉ chấp nhận `user|agent|system`; lạ ⇒ `"user"` và **không** panic). Hằng `ActorUser`, `ActorAgent`, `ActorSystem` ở gói `tenant`.
3. `dial.go` `AttachIdentity`: sau khối `ClientIP`, thêm `grpcmw.MetadataActorType, tenant.ActorType(ctx)` vào `metadata.AppendToOutgoingContext`. Mọi call site cũ tự mang `user` khi ctx không đặt (hành vi tương đương: service cũ không đọc khoá này).
4. `executor.go` `CallTool`: trước khi dựng `Identity` và gọi `dispatch`, `ctx = tenant.WithActorType(ctx, tenant.ActorAgent)`. Nếu ctx đã có `WithToolOrigin` (terminal/agent) thì vẫn `agent`.
5. Resource provider: cùng cách tại điểm vào đọc resource của MCP (đã tìm ở Context).
6. `request_wiring.go`: đọc `os.Getenv("REQUEST_INTERNAL_CALLER_TOKEN")`; nếu không rỗng thêm `internalcaller.ClientInterceptor(tok)` và `StreamClientInterceptor(tok)` vào tuỳ chọn dial của `request-service` (khuôn `mcp_governance_wiring.go`); rỗng thì log cảnh báo một dòng `REQUEST_INTERNAL_CALLER_TOKEN is empty: request-service will reject every call` (server fail closed, nên thiếu token là lỗi cấu hình rõ ràng, không phải lỗ hổng). Đưa tên biến vào `docs`? **Không** tạo tài liệu mới: ghi vào comment đầu file và README của `api-gateway` (một dòng ở mục cấu hình nếu có bảng biến).
7. Ghi chú nối tiếp: `deploy/dev/docker-compose.yml` cần cùng một giá trị cho `request-service` (`GATEWAY_INTERNAL_TOKEN`) và `api-gateway` (`REQUEST_INTERNAL_CALLER_TOKEN`) và một giá trị riêng `SERVICE_INTERNAL_TOKEN` cho `request-service` (task TASK-REQ-001-06 phụ trách compose; task này liệt kê biến để người đó thêm, không tự sửa compose).
8. Đường webhook (TASK-REQ-004-08) đặt `actor_type` `user` (reporter cấu hình); không đổi ở đây (câu hỏi mở 2 của solution).

## Kiểm thử

- `tenant_test.go`: `TestActorType_DefaultUser`, `TestActorType_RejectsUnknown` (`"root"` ⇒ `"user"`), `TestWithActorType_Agent`.
- `grpcmw_test.go`: hằng `MetadataActorType == "x-orca-actor-type"` (khoá hợp đồng giữa gateway và service); test cũ của `TenantExtractionInterceptor` không đổi (xanh nguyên vẹn).
- `dial_test.go` (api-gateway): `AttachIdentity` với ctx chưa đặt actor ⇒ metadata `x-orca-actor-type: user`; với `WithActorType(agent)` ⇒ `agent` (dùng `metadata.FromOutgoingContext`).
- `executor_test.go`: `TestCallTool_MarksActorAgent`: kênh giả ghi ctx nhận được, khẳng định `tenant.ActorType(ctx) == "agent"`; một lời gọi WS (không qua `CallTool`) vẫn `user`.
- `request_wiring_test.go`: có token ⇒ tuỳ chọn dial chứa interceptor (kiểm bằng `grpc.ServerOption` giả/`bufconn` gọi và đọc metadata tại server giả: `x-orca-internal-token` có mặt); rỗng ⇒ không interceptor.
- Lệnh: `cd backend-go && go test ./common/... ./services/api-gateway/internal/adapter/grpc/... ./services/api-gateway/internal/adapter/mcpserver/... ./services/api-gateway/cmd/server/...` và `go build ./...` toàn `go.work` (đảm bảo 16 service vẫn biên dịch).

## Tiêu chí hoàn thành

- [ ] Tool MCP luôn đi tới `request-service` với `x-orca-actor-type: agent`; WS/HTTP là `user`.
- [ ] Không đổi chữ ký `AttachIdentity`, `usecase.Identity`, `TenantExtractionInterceptor`, `ChainUnary`.
- [ ] Giá trị actor lạ hoặc thiếu ⇒ `user` (không bao giờ nâng quyền).
- [ ] Gateway gắn `x-orca-internal-token` cho kết nối tới `request-service` khi có token.
- [ ] `go build ./...` toàn workspace xanh.

## Ví dụ tham khảo

Luồng của một tool MCP ghi:

```
MCP client ─▶ mcpserver (Principal) ─▶ Executor.CallTool
      ctx = tenant.WithActorType(ctx, "agent")
   ─▶ wscompat channel ─▶ AttachIdentity(ctx, Identity)   (đọc actor từ ctx, như ClientIP)
   ─▶ gRPC metadata: x-orca-tenant-id, x-orca-user-id, x-orca-role, x-orca-client-ip, x-orca-actor-type=agent,
                     x-orca-internal-token (từ ClientInterceptor)
   ─▶ request-service: Guard ─▶ ActorTypeInterceptor ─▶ RequestAccessInterceptor
```

Biến cấu hình cần thêm cho người làm compose (TASK-REQ-001-06): `request-service`: `GATEWAY_INTERNAL_TOKEN`, `SERVICE_INTERNAL_TOKEN`; `api-gateway`: `REQUEST_INTERNAL_CALLER_TOKEN` (cùng giá trị với `GATEWAY_INTERNAL_TOKEN`).

## Thứ tự làm gợi ý

1. `tenant.go` và `grpcmw.go` (thêm hằng, hàm), chạy `go build ./...` toàn `go.work`.
2. `AttachIdentity` rồi test `dial_test.go`.
3. `Executor.CallTool` và resource provider.
4. `request_wiring.go` (token) sau cùng, khi TASK-REQ-016-01 đã dial `request-service`.

## Rủi ro và lưu ý

- `x-orca-actor-type` do gateway đặt; đường vào khác gateway có thể giả `user` (chỉ giảm bằng `Guard`, không loại bỏ; CR mục 6).
- Nếu `Executor.CallTool` không phải đường duy nhất cho MCP (resources, prompts), actor sẽ là `user` ở đó; chấp nhận cho resource chỉ đọc nhưng phải được liệt kê trong PR.
- Nhiều replica đổi khoá token cùng lúc cần quy trình xoay vòng (Q6, chưa thiết kế).
- Thêm khoá metadata làm tăng nhẹ kích thước header mỗi cuộc gọi: không đáng kể.

## Tiến độ (2026-10-07)

Đã làm và kiểm chứng (`cd backend-go/common && go test ./grpcmw/ ./tenant/`; `cd backend-go/services/api-gateway && go test ./internal/adapter/grpc/ ./internal/adapter/mcpserver/... ./cmd/server/`):
- [x] `grpcmw.MetadataActorType` + `TestMetadataActorTypeKeyIsStable`; `TestTenantExtractionInterceptor_IgnoresActorTypeMetadata` (interceptor không đổi hành vi).
- [x] `tenant.WithActorType/ActorType`, `ActorUser/Agent/System`; ba test của task (giá trị lạ về `user`).
- [x] `AttachIdentity` gắn `x-orca-actor-type` (`dial_test.go`, hai ca).
- [x] `Executor.CallTool` ép `agent` kể cả khi ctx đã mang `system` (`TestCallTool_MarksActorAgent`, kèm ca dispatch không qua `CallTool` là `user`).
- [x] Resource provider đánh `agent` ở `fetch` (`TestRead_MarksActorAgent`). Đường khác: `pty_session_reaper`/`pty_session_registry` gọi `Dispatch` nội bộ (đóng terminal khi hết phiên), không đi tới `request-service`, để nguyên.
- [x] `dialRequestService` (`cmd/server/request_wiring.go`) gắn `x-orca-internal-token` khi có `REQUEST_INTERNAL_CALLER_TOKEN`, cảnh báo khi rỗng; test dùng gRPC server thật trên 127.0.0.1 (token + `x-orca-actor-type` tới server; rỗng thì không có token).
- Người gọi đã kiểm: `AttachIdentity` (~20 file `wscompat/channels_*.go`, không đổi chữ ký), `TenantExtractionInterceptor`/`ChainUnary` (không sửa). `go build ./...` xanh ở mọi module trừ hai module cần tải gói mạng (`cmd/orca-cli`, `automation-service`, không liên quan).

Còn thiếu (vì sao chưa DONE):
- [ ] `dialRequestService` chưa được gọi từ `main.go`: gateway chưa dial `request-service` (việc của TASK-REQ-016-01, task này phụ thuộc vào nó). Cần nối một dòng khi 016-01 xong; hiện không có đường gọi từ ngoài.
- [ ] Tiêu chí "Tool MCP luôn tới `request-service` với `agent`" chỉ kiểm chứng tới tầng dispatch/metadata, chưa tới RPC `request-service` thật.
- [ ] Dòng cấu hình trong README `api-gateway` chưa thêm (README không có bảng biến); tên biến ghi ở comment đầu `request_wiring.go`.
- Biến cho TASK-REQ-001-06 (compose): `request-service`: `GATEWAY_INTERNAL_TOKEN`, `SERVICE_INTERNAL_TOKEN`; `api-gateway`: `REQUEST_INTERNAL_CALLER_TOKEN` (cùng giá trị `GATEWAY_INTERNAL_TOKEN`).
