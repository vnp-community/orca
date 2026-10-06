# Code Intel Graph Pipeline — Change Requests (v7)

> Đường ống dữ liệu từ agent tới mô hình đồ thị chuẩn: định nghĩa schema, thu thập qua `infra-fleet-service`, cache snapshot, vận chuyển thông báo, phân phối sự kiện tới gateway. Bối cảnh, quyết định D1–D7, mặc định O1–O8 và hợp đồng chung ở [README v7](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-CV-020](./CR-CV-020-canonical-graph-model.md) | Chưa có proto/domain cho đồ thị chuẩn; hai công cụ lệch kind, tên đủ, dòng (0/1-based), hậu tố `#arity` | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-021](./CR-CV-021-agent-collector.md) | Chưa có thành phần gọi `codeintel.*`; mã lỗi agent bị mất ở infra-fleet; không có hàng đợi chờ kết nối lại ở Go | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-022](./CR-CV-022-snapshot-cache.md) | Mỗi lần mở view tốn ~1,8 s CLI; chưa có định nghĩa `stale`, singleflight, ETag, dung lượng | 🟠 P1 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-CV-023](./CR-CV-023-infra-fleet-codeintel-transport.md) | Timeout 30 s, thông báo `codeintel.*` bị bỏ, `tools[]` handshake không tới Go, chưa có RPC stream | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-CV-024](./CR-CV-024-event-distribution.md) | Chưa có đường từ `indexChanged` tới huỷ cache, outbox và push UI; chưa chống bão sự kiện | 🟠 P1 | Medium | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-CV-020 (mô hình) ──┐
CR-CV-023 (infra-fleet)┼─▶ CR-CV-021 (collector) ─▶ CR-CV-022 (cache) ─▶ CR-CV-024 (sự kiện)
                       └──────────────────────────────────────────────▶ (CR-CV-023 cũng là nền của 024)
```

CR-CV-020 và CR-CV-023 độc lập nhau, làm song song (đợt 1 của README v7). CR-CV-021 cần cả hai: mô hình để giải mã, và mã lỗi/timeout từ infra-fleet. CR-CV-022 bọc CR-CV-021; CR-CV-024 cần `StreamCodeIntelEvents` (CR-CV-023) và `InvalidateBinding` (CR-CV-022).

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| F1 | Chuẩn hoá ở backend (CR-CV-020), agent trả dữ liệu theo từng công cụ kèm thẻ `tool` | Quy tắc hợp nhất sửa một chỗ; cần chốt với CR-CV-002 (Q1 của CR-CV-020) |
| F2 | Khoá `SymbolRef` bỏ hậu tố `#arity`, dòng quy về 1-based, `::` → `.` | Đã đo lệch thật giữa GitNexus và CodeGraph trên repo Orca |
| F3 | Collector tự chờ dev server kết nối lại (20 s), không sửa `RelayByDevServer` | `RelayByDevServer` dùng chung cho nhiều service |
| F4 | Mã lỗi `CODEINTEL_*` đi qua infra-fleet bằng tiền tố message + trailer `x-orca-agent-error-data-bin` | `apperrors.ToGRPCStatus` chỉ gửi `code: message` |
| F5 | Cache khoá theo HEAD; `stale` không kích hoạt thu thập lại; huỷ bằng xoá | O3 (người dùng chủ động làm mới) |
| F6 | `StreamCodeIntelEvents` (gRPC) thay vì event bus để lấy thông báo từ agent; gateway nhận push bằng gRPC stream | Khớp mẫu hiện có và README; event bus buộc tra tenant trong đường nóng |
| F7 | Giữa các replica dùng outbox + NATS (consumer tạm); `processed_events` chỉ ở giai đoạn ingest | Mỗi replica phải phát cho luồng gateway của mình |

## Ranh giới sở hữu với feature khác

- `code-intel-sources` sở hữu `C4ComponentView`, `DataFlow`, `ErdModel`, `StorageMap` trong `graph_sources.proto`; CR-CV-020 không tạo file đó, chỉ cam kết dùng chung `graph_common.proto`.
- `ChangeOverlay` có dải số field dành riêng cho CR-CV-036 (10–19), 037 (20–29), 038 (30–39).
- Bảng `graph_snapshots` thuộc CR-CV-011; CR-CV-022 đề nghị thêm cột `etag`, `total_count` và chỉ mục (mục 2.8 của CR đó).

## Điểm lệch README v7 và code, đã phát hiện khi viết feature này

- README v7 mục 3.3 thiếu ba mã do collector/infra-fleet sinh: `CODEINTEL_DEV_SERVER_OFFLINE`, `CODEINTEL_AGENT_UNSUPPORTED`, `CODEINTEL_RESULT_INVALID`.
- README v7 mục 7 và D2 coi "xếp hàng ~20 s (`RECONNECT_WAIT_MS`)" là hành vi sẵn có; thực tế chỉ có ở cầu nối TypeScript cũ, Go trả lỗi ngay (`relay_by_dev_server.go:55-57`).
- README v7 mục 1 ("Go lưu `tools[]` qua `LastHandshakeInfo`") lệch code: `agentwsserver` không đọc `tools`, `HandshakeInfo` không có trường này.
- `RelayByDevServer` làm mất `error.data.code` của agent; cần CR-CV-023 trước khi mã lỗi `CODEINTEL_*` tới được `code-intel-service`.
- Không có `MaxCallRecvMsgSize` ở bất kỳ đâu: trần gRPC 4 MiB, thấp hơn khung agent 16 MiB.
- `StreamFileChanges` (mẫu cho `StreamCodeIntelEvents`) không gắn tenant từ metadata stream như các stream khác; chưa chạy xác nhận.
- Research 05 §3 thiếu kind `import`, `enum_member`, `namespace`, `Enum`, `Constructor`, `Property`, `Variable`, và không nhắc lệch dòng 0/1-based.
