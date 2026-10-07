# BE-CV-TASK-040-07: Catalog 46 kênh, `registerCodeIntelChannels` và placeholder `CODEINTEL_UNAVAILABLE`

**From Solution:** BE-CV-SOL-040-codeintel-channel-foundation
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_catalog.go` (mới), `channels_codeintel_register.go` (mới, thay bản rỗng của 040-01), `channels_codeintel_catalog_test.go` (mới), `channels_codeintel_register_test.go` (mới)
**Depends on:** TASK-040-01 (ChannelDeps), TASK-040-06 (runner dùng kiểu spec)
**Status:** [x] DONE

---

## Context

UI-API §3 liệt kê 46 kênh (26 `codeIntel.*` + 20 `codeIntel.quality.*`; 45 unary + `codeIntel.subscribe`). G3 (proto-and-data-map §7.1) đòi đăng ký **vô điều kiện** để `parity_test.go:13-19` (inventory với `ChannelDeps{TaskActivityEnabled:true}`) thấy đủ kênh. `Registry.Channels()` (`registry_channels.go`) cho ảnh chụp tên đã đăng ký; đăng ký sau ghi đè trước.

## Việc cần làm

1. `channels_codeintel_catalog.go`: `codeIntelChannelSpec` (SOL-040-foundation 2.4) và `var codeIntelChannelCatalog = []codeIntelChannelSpec{...}` đủ 46 dòng theo bảng "Kênh -> solution -> task" ở [tasks/README](./README.md): `Name`, `Timeout` (8 s: `status, reindex, reindexStatus, dismissFinding, reviewState.*, c4.*, bindRepo, settings.*, quality.start/cancel/run/runs/waive/gate/profile.*/trend/turn.record/turns/turn`; 20 s: các kênh view còn lại, `quality.findings/coverage/trace/report/ci`; 24 s: `quality.summary`), `MaxArgsBytes` (mặc định `16<<10`; `reviewState.save` `256<<10`; `c4.save` `96<<10`; `quality.profile.save` `96<<10`; `quality.trace.confirm|link` `8<<10`), `AllowDevice` (chỉ `settings.get`), `Quality`, `MaxResponse` (`symbol` `320<<10`), `RPC`. Hằng `codeIntelReadTimeout=20s`, `codeIntelStateTimeout=8s`, `codeIntelSummaryTimeout=24s`. `func mustCatalogSpec(name string) codeIntelChannelSpec` panic nếu thiếu (lỗi lập trình, phát hiện bởi test).
2. `channels_codeintel_register.go`: `registerCodeIntelChannels(r, d)`: tạo `codeIntelDeps` (áp mặc định `MaxResponseBytes` 2 MiB, `MaxStreams` 500 khi 0); gọi các hàm đăng ký nhóm (ba solution sau thêm vào danh sách này: `registerCodeIntelViewChannels`, `registerCodeIntelStateChannels`, `registerCodeIntelSubscribe`, `registerCodeIntelQualityChannels`; ở task này danh sách rỗng); rồi `registerCodeIntelPlaceholders(r, d)`: với mỗi tên catalog **chưa** có trong `r.Channels()` đăng ký placeholder (unary: trả `errCodeIntelUnavailable` sau kiểm `DeviceID`/cỡ như runner? **không**, chỉ trả `CODEINTEL_UNAVAILABLE: channel not wired`; stream: `RegisterStream` trả cùng lỗi).
3. Lỗi `errCodeIntelUnavailable`/`NotFound`/`NotAuthorized` đã có ở TASK-040-04/06; không định nghĩa lại.

## Kiểm thử

- `TestCodeIntelCatalog_MatchesContract`: liệt kê cứng 46 tên của UI-API §3 (copy nguyên, kèm chú thích nguồn dòng); catalog có đúng 46, không trùng, 45 unary + 1 stream; mọi `Timeout` thuộc {8,20,24 s} cho unary; `MaxArgsBytes` đúng bảng; chỉ `settings.get` có `AllowDevice`; `Quality` đúng 20 kênh `codeIntel.quality.*`.
- `TestCodeIntelChannelInventory`: registry mới + `registerCodeIntelChannels(r, ChannelDeps{})` => `Channels()` chứa đúng 46 tên, 45 `ChannelUnary` + 1 `ChannelStream`.
- Gọi từng kênh qua `Registry.Dispatch` (client nil) => `CODEINTEL_UNAVAILABLE`.
- `TestCodeIntelPlaceholders_OnlyFillMissing`: đăng ký trước một kênh giả `codeIntel.status`, placeholder không ghi đè.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntel(Catalog|ChannelInventory|Placeholders)'`.

## Tiêu chí hoàn thành

- [x] 46 tên đúng hợp đồng, test cứng chống lệch.
- [x] `ChannelDeps{}` rỗng đăng ký đủ (G3).
- [x] Timeout/cỡ theo bảng UI-API 2.4.

## Rủi ro và lưu ý

- Tên kênh sai chính tả chỉ lộ ở FE; test liệt kê cứng là lưới duy nhất, đừng sinh danh sách từ catalog.
- `quality.summary` 24 s là ngoại lệ (xem solution quality); không đặt 120 s như cột T/o hợp đồng.
