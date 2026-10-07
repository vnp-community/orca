# BE-CV-TASK-040-06: Phong bì kết quả `CodeIntelEnvelope` và runner kênh unary generic

**From Solution:** BE-CV-SOL-040-codeintel-channel-foundation
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_envelope.go` (mới), `channels_codeintel_runner.go` (mới), `channels_codeintel_envelope_test.go` (mới), `channels_codeintel_runner_test.go` (mới)
**Depends on:** TASK-040-03 (args), TASK-040-04 (lỗi), TASK-040-05 (encoder); catalog (TASK-040-07) cung cấp `mustCatalogSpec` (hai task thống nhất kiểu `codeIntelChannelSpec`; làm 06 và 07 cùng PR hoặc 07 trước phần kiểu)
**Status:** [x] DONE

---

## Context

- UI-API 2.2: phong bì phẳng `{repo?, worktreeId, view, sources[], headCommit, stale, truncated, totalCount, etag, fromCache, generatedAt, notModified?, nextPageToken?, data?}`; PQ-12: `ifNoneMatch` khớp thì trả meta **không có `data`**.
- Runner: thứ tự guard ở SOL-040-foundation 2.4 (client nil, Identity, `DeviceID`, cỡ + giải mã, timeout, gọi, ánh xạ lỗi, trần phản hồi). `Registry.Dispatch` gắn `AttachIdentity` (có `Role`) và trần 60 s trước handler (`registry.go:172-201`); runner không gắn lại.
- `normalizeNilSlices` không đụng `json.RawMessage` không-nil.

## Việc cần làm

1. `channels_codeintel_envelope.go`: `func encodeEnvelope(meta *codeintelv1.ResultMeta, data proto.Message, nextPageToken string) (json.RawMessage, error)`: dựng phong bì từ `ResultMeta` (trừ `dev_server_id`) bằng encoder (TASK-040-05) cho phần `data`; `view` = tên view từ `ViewKind` (`structure`, `architecture`, ... theo enum); `sources[].indexedAt`/`commit` rỗng => `null`, `lineBase` ra khi > 0; `headCommit` rỗng => `null`; `totalCount` luôn ra (0 = không biết); khi `meta.NotModified` => bỏ `data`, `notModified:true`; `nextPageToken` chỉ ra khi khác rỗng; `etag` nguyên văn (có nháy). `status` (IndexStatus phẳng) **không** dùng hàm này.
2. `channels_codeintel_runner.go`:
   - `type codeIntelDeps struct{core codeintelv1.CodeIntelServiceClient; quality codeintelv1.QualityGateServiceClient; limits CodeIntelLimits}`, `configured(spec)` chọn client theo `spec.Quality`;
   - `type codeIntelCaller struct{Core codeintelv1.CodeIntelServiceClient; Quality codeintelv1.QualityGateServiceClient}`;
   - `registerCodeIntelUnary[A codeIntelArgs](...)` đúng như SOL-040-foundation 2.4;
   - hàm `finishCodeIntelResponse(spec, resp proto.Message, encode func() (json.RawMessage, error)) (any, error)`: `proto.Size(resp) > limit` => `errCodeIntelResponseTooLarge(size, limit)` (`CODEINTEL_RESPONSE_TOO_LARGE | {"bytes":N,"limit":L}`) **trước** khi mã hoá; `limit` = `spec.MaxResponse` nếu > 0, ngược lại `limits.MaxResponseBytes` (mặc định 2 MiB).
   - Lỗi từ `call` đi qua `codeIntelChannelError`; lỗi do gateway sinh (đã đúng định dạng) trả nguyên.
   - Không retry, không cache.
3. Ghi nhận ở comment ngắn "Why": không log `args`/kết quả (CR 2.10; `channel_args_redaction.go` ghi hiện không log args); không dùng `send` (U2) — gateway không thể chặn, nhắc ở README.

## Kiểm thử

- Envelope: `meta` đủ trường; `commit=""` => `null`; `NotModified` không có khoá `data`; `ResultMeta.dev_server_id` không ra; `generatedAt` RFC 3339; `totalCount:0` vẫn ra.
- Runner với fake client `codeintelv1.CodeIntelServiceClient` (nhúng interface, mẫu `fakeOrchestrationClient`, `channels_orchestration_test.go:15`):
  - client nil => `CODEINTEL_UNAVAILABLE` ngay cả khi args hỏng;
  - `Identity{}` rỗng => `CODEINTEL_NOT_FOUND`; `DeviceID` != "" => `CODEINTEL_NOT_AUTHORIZED` (trừ kênh `AllowDevice`);
  - ctx của lời gọi có deadline đúng `spec.Timeout` (kiểm `ctx.Deadline()` trong fake, sai số ≤ 1 s);
  - fake trả `status.Error(Unimplemented, ...)` => `CODEINTEL_UNAVAILABLE`; trả `FailedPrecondition "CODEINTEL_DISABLED: ..."` => `CODEINTEL_DISABLED`;
  - phản hồi lớn: message có `proto.Size` = limit+1 => `RESPONSE_TOO_LARGE`, bằng limit => qua; `symbol` limit 320 KiB;
  - gửi `tenantId` giả => `INVALID_PARAMS` và fake **không** được gọi; metadata gRPC (qua interceptor/`metadata.FromOutgoingContext`) mang `TenantID`/`UserID`/`Role` từ `Identity`.
  - ghi đè: kênh hai dialect — kết quả `json.RawMessage` ra đúng ở native (`result`) và session-client (`result`), lỗi ở `message`.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntel(Envelope|Runner)'`.

## Tiêu chí hoàn thành

- [x] Một runner phục vụ 45 kênh unary; không kênh nào tự lặp guard.
- [x] Trần phản hồi tính trên `proto.Size`, không cắt ngầm.
- [x] Không kênh nhận được danh tính từ `args`.

## Rủi ro và lưu ý

- `Identity{}` rỗng không thể xảy ra trên đường WS đã xác thực (`resolveIdentity`, `handler.go:120`); giữ như phòng thủ với thông điệp trung tính (không thành oracle).
- Generics + closures: giữ mỗi kênh một hàm `call` ngắn để file kênh dễ đọc; không dùng reflection ngoài encoder.
- `proto.Size` tính tốn CPU trên phản hồi 2 MiB: đo ở CR-071.
