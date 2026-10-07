# BE-CV-TASK-040-05: Bộ mã hoá `proto -> JSON` camelCase (enum, int64, Timestamp, optional, nullable)

**From Solution:** BE-CV-SOL-040-codeintel-channel-foundation
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_wire_encoder.go` (mới), `channels_codeintel_wire_enums.go` (mới), `channels_codeintel_wire_encoder_test.go` (mới)
**Depends on:** cổng G0 cho test với message thật; làm trước được bằng `dynamicpb`/message trong `testdata`
**Status:** [x] DONE

---

## Context

- Hợp đồng chốt shape TS (UI-API §4) và quy ước enum (PQ-32) nhưng không nêu cách chuyển (SOL-040-foundation 2.5, Q1). `normalizeNilSlices` (`registry.go:227`) trả nguyên `proto.Message` nên trả proto thô sẽ ra snake_case/enum HOA (ghi chú BUG-023 ở `channels.go`).
- `protojson` của `executor.go`/`result_normalize.go` (MCP) chỉ dùng cho MCP, không phải đường WS.
- Quy tắc cần giữ (UI-API §1 U4, §4): camelCase; `[]` cho mảng rỗng; enum chữ thường trừ `risk.level`/`overall` (PQ-32); enum lạ => `'unknown'`; thời gian RFC 3339 UTC; số đếm là số nguyên không âm; `devServerId` không ra UI (UI-API 2.2).

## Việc cần làm

1. `func encodeCodeIntelWire(m proto.Message) (json.RawMessage, error)`: duyệt `m.ProtoReflect()` theo thứ tự `Descriptor().Fields()`, ghi vào `bytes.Buffer`, **không** qua `protojson`->`map`->`json.Marshal`.
2. Quy tắc theo kiểu:
   - khoá = `fd.JSONName()`;
   - `repeated` => luôn mảng; `map<K,V>` => luôn object (khoá chuỗi hoá bằng `fmt`); `oneof` => chỉ nhánh có giá trị;
   - `int32/uint32/int64/uint64` => số; vượt `2^53-1` => lỗi `CODEINTEL_RESULT_INVALID`;
   - `bool`, `string` nguyên văn (U9: không gỡ HTML); `bytes` => lỗi `RESULT_INVALID` (không có bytes trong hợp đồng);
   - `google.protobuf.Timestamp` => RFC 3339 UTC có `Z`; `Duration` => lỗi (không có trong hợp đồng); `Struct/Value` => JSON nguyên;
   - enum => `enumWire(fd.Enum(), number)`; **nullable**: trường nằm trong `codeIntelNullableFields` (khoá `"<FullMessageName>.<json_name>"`) mà vắng/zero-presence => `null`; trường khác vắng (message, `optional`) => bỏ khoá; proto3 scalar không-presence luôn ra (kể cả `0`/`""`/`false`) trừ khi nằm trong `codeIntelNullableFields` và rỗng => `null`.
3. `channels_codeintel_wire_enums.go`: `enumWire`: bỏ tiền tố `<TÊN_ENUM_SNAKE>_`, chữ thường; số 0 hoặc `*_UNSPECIFIED` hoặc số không có trong descriptor => `"unknown"`; bảng ghi đè `codeIntelEnumWireOverrides map[protoreflect.FullName]map[int32]string` cho enum **HOA** (`Risk` => `LOW|MEDIUM|HIGH|CRITICAL|UNKNOWN`; trường chuỗi `overall` của `IndexStatus` đã là chuỗi nên không qua đây) và giá trị có gạch ngang (`proto-rpc`, `ws-channel`) nếu proto dùng enum cho chúng (hợp đồng PQ-30 ghi chuỗi; nếu proto dùng `string` thì không cần).
4. `codeIntelNullableFields` và `codeIntelNeverOnWire` (`orca.codeintel.v1.ResultMeta.dev_server_id`, mọi trường tên `*_absolute_path`/`workspace_root` nếu tồn tại) là **dữ liệu**; mỗi mục có chú thích dòng UI-API §4 nguồn gốc (ví dụ `FlowSummary.entry: SymbolRef|null`).
5. Test tự kiểm ở thời điểm có proto: `TestCodeIntelWireTables_ReferToExistingFields` duyệt registry (`protoregistry.GlobalFiles`) xác nhận mọi khoá bảng trỏ tới trường có thật; trước khi có proto test bỏ qua có chú thích (t.Skip kèm lý do, không xoá).

## Kiểm thử

- Message tổng hợp (`dynamicpb` hoặc `.proto` nhỏ trong `testdata`) phủ: mảng rỗng, map rỗng, enum 0, enum số lạ, enum HOA qua bảng ghi đè, `optional int32 percent` vắng => `null` (khi nullable) / bỏ khoá (khi không), `int64` 2^53, `int64` 2^53+1 => lỗi, `Timestamp` có nano, `oneof`, message lồng 3 cấp, chuỗi chứa `<script>` và `"` và U+2028 (được thoát đúng), UTF-8 hợp lệ giữ nguyên.
- Quét khẳng định không khoá snake_case trong kết quả (regex `"[a-z]+_[a-z_]+":`).
- So sánh với `protojson` trên message không có enum/int64 để chắc khoá và cấu trúc khớp (kiểm chứng tương đương trên tập con).
- Benchmark `BenchmarkEncodeCodeIntelWire` trên graph 1 MiB (không ngưỡng cứng; ghi số vào PR; CR-071 dùng làm đường cơ sở).
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelWire' -bench CodeIntelWire -benchmem`.

## Tiêu chí hoàn thành

- [x] Một hàm duy nhất dùng cho mọi view; không có struct view viết tay cho proto lớn.
- [x] Mảng không bao giờ `null`; enum lạ => `"unknown"`.
- [x] Bảng nullable/enum có test đối chiếu proto thật khi có (cổng G0).
- [x] `devServerId` không xuất hiện ở bất kỳ đầu ra nào (test).

## Rủi ro và lưu ý

- Proto chưa tồn tại: mọi tên trường/enum ở bảng là **chưa kiểm chứng**; chủ sở hữu CR-020/031/033/034/035/036/037/038/082/085 phải xác nhận bảng khi họ khai proto (hợp đồng §8.3 điểm 2: PR proto và PR bảng cùng đợt).
- Encoder tự viết là bề mặt lỗi mới; đừng thêm tuỳ biến theo kênh ở đây, mọi ngoại lệ vào bảng dữ liệu.
- Nếu Q1 chốt "service trả JSON dựng sẵn", task này thu nhỏ thành chuyển tiếp `json.RawMessage` + kiểm hợp lệ; giữ lại bộ kiểm quét snake_case.
