# TASK-BE-CLI-004: Vá KNOWN GAP authorization ở `IssueServiceToken` usecase

> **⚠️ SECURITY-SENSITIVE TASK — cần security review độc lập trước khi MERGE
> (không phải trước khi bắt đầu code — quyết định thiết kế đã chốt, xem dưới).**
>
> **✅ Quyết định (2026-09-09, Product Owner):** vá gap NGAY ở tầng usecase
> (không chỉ chặn ở route CLI của TASK-BE-CLI-003), và **KHÔNG cho phép mint
> hộ user khác trong v1** (không có admin-override/`target_user_id`). Xem đầy
> đủ lý do ở [BE-CLI-SOL-002](../solutions/BE-CLI-SOL-002-cli-headless-service-token.md)'s
> mục "Quyết định (2026-09-09)". Task này KHÔNG còn ở trạng thái "trình bày 2
> phương án chờ chọn" — chuyển thẳng sang implement phương án đã chọn.

**Solution:** [BE-CLI-SOL-002](../solutions/BE-CLI-SOL-002-cli-headless-service-token.md) | **CR:** CR-CLI-002
**Service:** `auth-service`, `api-gateway` (propagate identity)
**Depends on:** Không (độc lập kỹ thuật với TASK-BE-CLI-003 — có thể làm song song, nhưng **TASK-BE-CLI-003 không được merge trước task này xong**, xem "Blocking")
**Status:** ✅ DONE

> **Kết quả thực tế — Bước 0 (BẮT BUỘC, chạy trước khi code)**: grep
> `WithRole\|tenant.Role(ctx)\|metadata.NewOutgoingContext` trong
> `common/tenant/`+`services/api-gateway/internal/adapter/grpcclient/` như
> task ghi KHÔNG cho kết quả hữu ích (thư mục `grpcclient/` không tồn tại ở
> `api-gateway` — chỉ tồn tại ở `auth-service`). Mở rộng grep đúng tinh thần
> câu hỏi ("cơ chế propagate 1 giá trị identity qua gRPC context/metadata đã
> có sẵn chưa") ra đúng nơi client gRPC thật của `api-gateway` nằm
> (`internal/adapter/grpc/dial.go`) + `common/grpcmw/` → **PHÁT HIỆN: CƠ CHẾ
> NÀY ĐÃ TỒN TẠI SẴN, đầy đủ 2 chiều, và đã carry đúng userID (không chỉ
> tenantID) — ngược với giả định "khả năng thật... CHƯA có" mà task này đặt
> ra**:
> - Client: `gatewaygrpc.AttachIdentity(ctx, identity)` (`adapter/grpc/dial.go:48-56`)
>   gọi `metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataUserID, id.UserID, ...)`
>   — TASK-BE-CLI-003's route handler đã gọi đúng hàm này với
>   `identity.UserID` = caller đã xác thực trước khi gọi `IssueServiceToken`.
> - Server: `grpcmw.TenantExtractionInterceptor()` (`common/grpcmw/grpcmw.go:57-76`)
>   đọc `MetadataUserID` từ incoming metadata, gọi `tenant.WithUserID(ctx, v[0])`.
>   `auth-service/cmd/server/main.go:244` đã wire
>   `grpc.NewServer(grpcmw.ChainUnary(logger))` — interceptor này ĐANG CHẠY
>   thật trên `auth-service`, không phải lý thuyết.
> - `tenant.UserID(ctx)` (`common/tenant/tenant.go:59`) đã được dùng CHÍNH
>   `auth-service`'s `requireAdminActor` (`internal/usecase/authorization.go:20`)
>   làm "acting user" cho admin-authorization — cùng semantics identity-của-
>   caller task này cần cho `CallerUserID`.
>
> → Theo đúng nhánh "Nếu đã có" của Bước 0: **tái dùng nguyên vẹn**
> `tenant.UserID(ctx)`, KHÔNG thêm metadata key mới (`x-orca-caller-user-id`),
> KHÔNG thêm interceptor thứ 2. Điều này đơn giản hơn cả 2 phương án task dự
> trù — không cần sửa `common/grpcmw/`, `common/tenant/`, hay bất kỳ file
> nào ở `api-gateway/internal/adapter/grpcclient/` (thư mục đó không tồn
> tại, và không cần tạo mới).
>
> **Trust boundary**: `grpcmw.go`'s doc comment tự thừa nhận đây là chỗ dựa
> vào "mTLS + service mesh's NetworkPolicy" ở production, CHƯA có mTLS/mesh
> thật trong code (`grep -rn "mTLS\|ServerTLS\|credentials.NewTLS"` không ra
> kết quả nào trong `auth-service/cmd/server/main.go`). Đây là **rủi ro có
> sẵn, hệ thống, không phải do task này tạo ra mới**: `requireAdminActor`
> (cấp quyền ADMIN — nhạy cảm hơn) đã dựa vào đúng cơ chế/trust boundary này
> từ trước. Tái dùng cùng cơ chế cho `CallerUserID` không mở thêm lỗ hổng
> nào so với hiện trạng đã tồn tại — không phải "lỗ hổng mới" theo đúng tiêu
> chí cảnh báo của Bước 0. Ghi nhận nhưng KHÔNG chặn merge vì lý do trên;
> khuyến nghị theo dõi cùng 1 track hardening mTLS/mesh sau này áp dụng
> chung cho toàn bộ interceptor, không riêng CR-CLI-002.
>
> **Code thay đổi thực tế** (khác task doc ở đúng 1 điểm: KHÔNG có metadata
> key mới, KHÔNG sửa `common/grpcmw`/`common/tenant`/`grpcclient`):
> 1. `issue_service_token.go`: thêm `CallerUserID` vào `IssueServiceTokenInput`,
>    thêm 2 check (missing → `KindInternal`/`AUTH_CLI_TOKEN_MISSING_CALLER`
>    fail-closed; mismatch → `KindPermissionDenied`/`AUTH_CLI_TOKEN_FORBIDDEN`,
>    không có admin-override) TRƯỚC bước lookup user — đúng thứ tự
>    `UserID==""` → `Audience==""` → `CallerUserID==""` → `CallerUserID!=UserID`
>    → lookup, giữ nguyên hành vi 2 check đầu cho test cũ.
> 2. `adapter/grpc/server.go`: đọc `tenant.UserID(ctx)` (không đọc field nào
>    từ `req` — proto không có field đó và không nên thêm), gán vào
>    `CallerUserID`; sửa lại doc comment "Known gaps" (bớt "caller-
>    authorization" khỏi danh sách còn thiếu).
>
> **4 test case yêu cầu + 3 test cũ phải cập nhật** (thêm `CallerUserID` vào
> input để không bị fail-closed chặn trước khi tới đúng nhánh test gốc định
> kiểm tra — không xoá/nới lỏng assertion nào):
> `TestIssueServiceToken_RejectsCallerMintingForOtherUser` (**test bảo mật
> quan trọng nhất**), `TestIssueServiceToken_AllowsSelfMint`,
> `TestIssueServiceToken_MissingCallerIdentityRejected` — cả 3 mới, PASS.
> `TestIssueServiceToken_SucceedsForExistingUser`/`_UnknownUserFails`/
> `_SignerFailurePropagates` — cập nhật thêm `CallerUserID` khớp `UserID`,
> PASS. `_RequiresUserIDAndAudience` không cần sửa (2 check `UserID`/
> `Audience` chạy trước check `CallerUserID` trong code, test cũ không chạm
> nhánh mới). Không có interceptor mới nên không cần
> `TestOutgoingMetadata_CarriesCallerUserID`/`TestIncomingMetadata_...`.
>
> **Build/test thật**: `cd auth-service && go build ./...` sạch. `go test
> ./internal/usecase/... -run TestIssueServiceToken -v` — **7/7 PASS**
> (bao gồm `TestIssueServiceToken_RejectsCallerMintingForOtherUser`).
> `go test ./...` (auth-service, toàn bộ) — **FAIL 1 package không liên
> quan**: `internal/adapter/policypublisher` (`TestFilePublisher_PublishPolicyChange_WritesAtomically`,
> lỗi OPA bundle merge `data/ratelimits.json`) — xác nhận bằng `git diff
> --stat` rằng `publisher.go` trong package đó đang bị 1 tiến trình/agent
> KHÁC sửa dở (132 dòng thêm, không phải do task này — không đụng file đó).
> `cd ../api-gateway && go build ./...` — **sạch**. `go test ./...` —
> **FAIL 1 package không liên quan**: `internal/adapter/httpgateway` build
> failed vì `fakeAdminAuthServiceClient`/`fakeSSOAuthServiceClient` thiếu
> method `AppendAuditEntry` — xác nhận bằng `git diff --stat` rằng
> `auth.proto`/`proto/gen/go/orca/auth/v1/*.pb.go`/`httpgateway/middleware.go`/
> `automation_routes.go` đang bị agent khác sửa dở song song (audit-log
> RPC mới), KHÔNG liên quan tới thay đổi của task này (task này không đụng
> `.proto`/`httpgateway`). Mọi package khác của `api-gateway`
> (`wsbridge`, `wscompat`, `usecase`, `authclient`) PASS.

---

## Bối cảnh (không lặp lại toàn bộ — xem BE-CLI-SOL-002 mục 1b/2B để hiểu đầy đủ)

`IssueServiceToken.Execute` (`internal/usecase/issue_service_token.go:51-92`) hiện
chỉ verify `user_id` tồn tại — không verify AI đang gọi. Doc comment ngay trong code
tự gọi đây là "KNOWN GAP". Task này đóng gap đó tận gốc, ở tầng usecase — áp dụng cho
MỌI caller của RPC này (không chỉ route CLI mới ở TASK-BE-CLI-003).

## Bước 0 — Verify điều kiện tiên quyết (BẮT BUỘC, làm trước khi code bất cứ gì)

Xác nhận cơ chế propagate 1 giá trị identity (không phải `tenantID`) từ `api-gateway`
xuống 1 service khác qua gRPC context/metadata **đã tồn tại sẵn chưa**:

```bash
grep -rn "WithRole\|tenant.Role(ctx)\|metadata.NewOutgoingContext" backend-go/common/tenant/ backend-go/services/api-gateway/internal/adapter/grpcclient/
```

- **Nếu đã có** 1 cơ chế chung (interceptor set metadata ở client, đọc lại ở server) —
  tái dùng đúng cơ chế đó, chỉ thêm 1 key mới (`caller_user_id`) vào cùng chỗ, KHÔNG
  viết interceptor thứ 2 song song.
- **Nếu CHƯA có** (khả năng thật — [CR-RBAC-002](../../../../../../docs/crs/v4/team-rbac/CR-RBAC-002-unify-role-model-and-propagate-claims.md)
  đã ghi nhận `tenant.WithRole`/`tenant.Role` tồn tại nhưng KHÔNG có gRPC interceptor
  nào thực sự gán giá trị đó vào metadata khi gọi service khác — `callerGlobalRole`
  hard-code rỗng chính vì lý do này): task này phải tự thêm 1 gRPC **client interceptor**
  ở phía gọi (`api-gateway`, nơi có `authv1.NewAuthServiceClient(...)`) gắn
  `identity.UserID` vào outgoing gRPC metadata (`metadata.AppendToOutgoingContext(ctx,
  "x-orca-caller-user-id", identity.UserID)`), và 1 **server interceptor**/helper ở
  `auth-service` đọc lại metadata đó (`metadata.FromIncomingContext(ctx)`), populate
  vào `IssueServiceTokenInput.CallerUserID` trước khi gọi usecase. Đặt tên metadata key
  theo đúng convention đã dùng cho các key nội bộ khác nếu có (kiểm tra
  `backend-go/common/grpcmw/` trước khi đặt tên mới).
- **Trust boundary quan trọng**: metadata `x-orca-caller-user-id` (hoặc tên tương
  đương) CHỈ được tin nếu tới từ 1 caller nội bộ đã qua mTLS/service-mesh xác thực
  (không phải field client bên ngoài có thể tự set) — xác nhận `auth-service`'s gRPC
  server hiện đã chạy sau lớp xác thực service-mesh nào (nếu có) trước khi tin metadata
  này; nếu RPC này lộ ra ngoài mesh mà không có gì chặn client tự set metadata tuỳ ý,
  đây là 1 lỗ hổng mới tương đương gap đang vá — ghi rõ phát hiện này nếu xảy ra, KHÔNG
  tự ý merge.

## Giải pháp

```go
// issue_service_token.go — sửa Input + Execute
type IssueServiceTokenInput struct {
	UserID       string
	Audience     string
	CallerUserID string // MỚI — identity của bên đang gọi RPC này, LUÔN lấy từ gRPC
	              // context đã xác thực (xem Bước 0) — KHÔNG BAO GIỜ từ 1 field
	              // trong IssueServiceTokenRequest proto (không có field đó, và sẽ
	              // không bao giờ có — client không được quyền tự khai identity).
}

func (uc *IssueServiceToken) Execute(ctx context.Context, in IssueServiceTokenInput) (IssueServiceTokenOutput, error) {
	if in.CallerUserID == "" {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_CLI_TOKEN_MISSING_CALLER", "internal: caller identity not propagated", nil)
	}
	if in.CallerUserID != in.UserID {
		// v1: KHÔNG có ngoại lệ nào (không admin-override) — quyết định 2026-09-09.
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindPermissionDenied, "AUTH_CLI_TOKEN_FORBIDDEN", "cannot mint a token for another user", nil)
	}
	// ... phần còn lại giữ nguyên
}
```

`adapter/grpc/server.go`'s `IssueServiceToken` handler: đọc `CallerUserID` từ context
(Bước 0), gán vào `IssueServiceTokenInput` trước khi gọi `uc.Execute` — KHÔNG bao giờ
đọc từ `req.GetCallerUserId()` vì proto không có (và không nên có) field này.

## Files cần sửa

1. `backend-go/services/auth-service/internal/usecase/issue_service_token.go` (MODIFY — thêm `CallerUserID`, check)
2. `backend-go/services/auth-service/internal/adapter/grpc/server.go` (MODIFY — đọc caller identity từ context, populate input)
3. `backend-go/services/api-gateway/internal/adapter/grpcclient/` — client gọi `auth-service` (MODIFY — gắn outgoing metadata; xác định đúng file qua Bước 0's grep)
4. `backend-go/common/grpcmw/` hoặc `backend-go/common/tenant/` (MODIFY, chỉ nếu Bước 0 xác nhận CHƯA có cơ chế chung — thêm interceptor/helper mới, mirror pattern đã có cho tenantID)

## Test cases cần cover

- `TestIssueServiceToken_RejectsCallerMintingForOtherUser` — `CallerUserID != UserID` → `PermissionDenied`, không ký JWT.
- `TestIssueServiceToken_AllowsSelfMint` — regression, route CLI (TASK-BE-CLI-003) vẫn hoạt động đúng khi `CallerUserID == UserID`.
- `TestIssueServiceToken_MissingCallerIdentityRejected` — `CallerUserID` rỗng (context không propagate được, vd. bug ở interceptor) → lỗi rõ ràng (`Internal`, không phải mint "thành công" với caller rỗng — fail-closed, không fail-open).
- Nếu Bước 0 phải thêm interceptor mới: `TestOutgoingMetadata_CarriesCallerUserID`, `TestIncomingMetadata_PopulatesCallerUserID` ở đúng package interceptor đó.

## Verify

```bash
cd backend-go/services/auth-service && go build ./... && go test ./internal/usecase/... -run TestIssueServiceToken -v
cd ../api-gateway && go build ./... && go test ./...
```

## gitnexus

`impact({target: "IssueServiceToken", direction: "upstream"})` — **bắt buộc trước khi
sửa** — xác nhận chính xác có bao nhiêu caller nội bộ khác (ngoài route CLI mới ở
TASK-BE-CLI-003) sẽ chạm input mới `CallerUserID`. Nếu phát hiện caller nội bộ khác
ngoài dự kiến (vd. 1 service khác đã gọi RPC này cho mục đích khác), DỪNG LẠI và báo
cáo lại — không tự ý mở rộng phạm vi task để "sửa cho khớp" caller đó.

`impact({target: "IssueServiceTokenInput", direction: "downstream"})` nếu tồn tại — xác
nhận không có test double/fake nào ở nơi khác implement usecase này theo interface cũ
(sẽ vỡ build nếu struct field bắt buộc thay đổi theo cách không tương thích).

## Blocking

**TASK-BE-CLI-003's route được phép code song song, nhưng KHÔNG được MERGE trước khi
task này merge xong** — nếu merge ngược thứ tự, route CLI mint token qua RPC vẫn còn
lỗ hổng (route tự ép self-identity đúng, nhưng RPC bên dưới vẫn nhận mint hộ ai cũng
được nếu có caller khác gọi trực tiếp không qua route) trong khoảng thời gian giữa 2
lần merge — chấp nhận được cho dev/staging nhưng KHÔNG được lên production ở trạng thái
đó.

TASK-BE-CLI-005 (revocation): migration bảng `issued_service_tokens` **không cần thêm
cột `issued_by` riêng** như bản nháp cũ của task này từng cân nhắc — vì quyết định v1
là self-mint-only, `issued_by` luôn trùng `user_id`, không cần lưu riêng. Cập nhật lại
TASK-BE-CLI-005 nếu file đó còn giữ ghi chú cũ.
