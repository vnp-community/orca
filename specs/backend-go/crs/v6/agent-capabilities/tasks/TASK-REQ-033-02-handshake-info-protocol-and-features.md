# TASK-REQ-033-02: `HandshakeInfo` mang `Capabilities`, `Features`, `ProtocolVersion`, `BuildVersion` ở cả ba đường kết nối

**From Solution:** [BE-REQ-SOL-033](../solutions/BE-REQ-SOL-033-dev-server-capability-profile-and-agent-client.md) mục 2.F
**Priority:** P1
**Service/Area:** `infra-fleet-service` / usecase port, adapter `devserveragent`, `agentwsserver`, `sshrelay`
**File:** `internal/usecase/ports.go` (sửa), `internal/adapter/devserveragent/session.go` (sửa), `internal/adapter/devserveragent/client.go` (sửa), `internal/adapter/agentwsserver/server.go` (sửa), `internal/adapter/sshrelay/provisioner.go` (sửa), và các `_test.go` cạnh chúng
**Depends on:** không (độc lập với 01; làm song song được)
**Status:** [ ] TODO

---

## Context

Đã đọc ngày 2026-10-06:
- `usecase.HandshakeInfo` ở `internal/usecase/ports.go` dòng 85 đến 90 chỉ có `Platform, Arch, NodeVersion, AgentVersion`. Comment ngay trên đó giải thích đây là bản sao có chủ ý của `devserveragent.HandshakeInfo` để `usecase` không import adapter (chu trình import).
- `devserveragent.HandshakeInfo` (`session.go` dòng 47 đến 58) có thêm `SessionID`, `Capabilities []string`, `SockPath` (`json:"-"`).
- `Client.LastHandshakeInfo` (`client.go` dòng 473 đến 492) chuyển sang bản `usecase` **chỉ bốn trường**: nếu không sửa, mọi trường mới rơi ở đây.
- `agentwsserver/server.go`: `inboundHandshakeParams` dòng 73 đến 81 (`Capabilities []string`), dựng `HandshakeInfo` ở dòng 212 đến 219; `json.Unmarshal` thường, bỏ lỗi (`_ =`), nên trường lạ vô hại. Có hàm `firstNonEmpty`.
- `sshrelay/provisioner.go`: struct handshake có `Capabilities` (dòng 64) và gán ở dòng 255; `sshrelay/bulk_provision.go` dòng 43 dựng `usecase.HandshakeInfo{...}` thủ công (bốn trường).
- Đường `relay-websocket` (Orca là bên khởi tạo): `session.go:267 runInitiatorHandshake` đọc kết quả handshake của agent thành `HandshakeInfo`. Cần đọc đúng chỗ giải mã (đọc lại hàm này trước khi sửa, chưa mở hết thân hàm ở lần soạn).
- `devserveragent.HandshakeInfo` có người dùng khác: `HandshakeInfoFor` (`client.go:373`), `handshakeInfoSnapshot`, `AttachTransport` (`:301`), `AttachInboundSession` (`:323`); test `last_handshake_info_test.go`.
- Agent mới gửi thêm `protocolVersion` (số nguyên; vắng nghĩa là 1), `buildVersion`, `features[]` trong handshake (CR-REQ-033 mục 2.6; phía agent do `AG-REQ-SOL-033-*`). `agentVersion` vẫn `5.0.0` và **không** đổi (`ResumeAgentSession`, `MinAgentVersion`/`isBelowMinimumVersion` so sánh nó).

## Việc cần làm

1. `usecase.HandshakeInfo` thêm bốn trường: `Capabilities []string`, `Features []string`, `ProtocolVersion int`, `BuildVersion string`. Cập nhật comment: bản sao vẫn có chủ ý.
2. `devserveragent.HandshakeInfo` thêm `Features []string \`json:"features"\``, `ProtocolVersion int \`json:"protocolVersion"\``, `BuildVersion string \`json:"buildVersion"\``; `Capabilities` đã có.
3. Thêm hàm `(h HandshakeInfo) EffectiveProtocolVersion() int` trả `1` khi `ProtocolVersion <= 0`. Đặt ở `devserveragent` và một bản `usecase.HandshakeInfo.EffectiveProtocolVersion()` cùng logic (hai bản có chủ ý, test chéo bằng bảng giống nhau).
4. `Client.LastHandshakeInfo`: chuyển đủ tám trường; sao chép slice (`append([]string(nil), ...)`) để tránh chia sẻ bộ nhớ với phiên.
5. `agentwsserver.inboundHandshakeParams` thêm `Features`, `ProtocolVersion`, `BuildVersion` và điền vào `HandshakeInfo` (dòng 212); `Features` cắt tối đa 64 phần tử, mỗi phần tử tối đa 64 ký tự, bỏ phần tử vượt (agent đáng ngờ không được phình bộ nhớ phiên).
6. `sshrelay/provisioner.go`: thêm ba trường vào struct handshake và gán ở chỗ dòng 255; `bulk_provision.go:43` chuyển thêm `Capabilities`, `Features`, `ProtocolVersion`, `BuildVersion` nếu `Provision` có sẵn (nếu hàm trả bản `devserveragent`, chuyển đủ).
7. `runInitiatorHandshake` (relay-websocket): đọc `protocolVersion`, `buildVersion`, `features` từ kết quả; thiếu thì để zero-value.
8. Hàm kiểm giới hạn dùng chung ba nơi: `domain.SanitizeAgentFeatures(in []string) []string` (đặt ở `internal/domain/agent_features.go` của task 01 nếu đã có, nếu chưa thì tạo file này trước và task 01 dùng lại), áp dụng ở cả ba đường.
9. Không đổi `AgentVersion`, `Capabilities`, `ResumeAgentSession`, `isBelowMinimumVersion`.

## Kiểm thử

- `TestLastHandshakeInfo_CarriesFeaturesAndProtocol` (mở rộng `last_handshake_info_test.go`): handshake có đủ trường mới thì `LastHandshakeInfo` trả đủ.
- `TestLastHandshakeInfo_LegacyHandshake`: handshake cũ (không có trường mới) thành công, `Features` nil, `EffectiveProtocolVersion()==1`.
- `TestInboundHandshake_AcceptsUnknownFields` và `TestInboundHandshake_FeaturesCapped` (`agentwsserver/server_test.go`, dùng `httptest.NewServer` như các test hiện có): trường lạ không làm hỏng; 100 features bị cắt còn 64.
- `TestProvisioner_HandshakeCarriesFeatures` (`sshrelay`, qua `export_test.go` có sẵn).
- `TestInitiatorHandshake_ReadsFeatures` (relay-websocket) với transport giả.
- `TestSanitizeAgentFeatures`: bỏ chuỗi rỗng, quá dài, trùng, không phải ASCII in được.
- Hồi quy: `go test ./services/infra-fleet-service/internal/adapter/devserveragent/... ./services/infra-fleet-service/internal/adapter/agentwsserver/... ./services/infra-fleet-service/internal/adapter/sshrelay/...` phải xanh như trước khi sửa.

## Tiêu chí hoàn thành

- [ ] Trường mới có mặt ở cả ba đường kết nối và ở `LastHandshakeInfo`.
- [ ] Backend bắt tay thành công với agent cũ (không có trường mới) và với agent gửi trường lạ.
- [ ] `AgentVersion` và `Capabilities` giữ nguyên giá trị như trước.
- [ ] Không có `DisallowUnknownFields` được thêm vào đâu.
- [ ] `gitnexus_impact` đã chạy cho `HandshakeInfo` và `LastHandshakeInfo` trước khi sửa (quy ước repo) và kết quả ghi vào PR.

## Thứ tự thực hiện gợi ý

1. Viết test hồi quy trước (handshake cũ vẫn thành công) để có điểm tựa; chạy xanh trên mã chưa sửa.
2. Thêm trường vào hai struct `HandshakeInfo` và hàm `EffectiveProtocolVersion`; sửa `LastHandshakeInfo`; chạy lại test cũ.
3. Sửa từng đường kết nối một, mỗi đường một commit kèm test của nó: `agentwsserver` (inbound), `sshrelay` (provisioner và bulk provision), `runInitiatorHandshake` (relay-websocket).
4. Cuối cùng thêm `SanitizeAgentFeatures` và áp dụng ở ba đường; chạy `grep -rn "HandshakeInfo{" internal` để chắc không còn nơi dựng struct nào bỏ sót.

## Kiểm tra thủ công sau khi xong

- Với agent cũ trên dev server thật: kết nối thành công như trước, `LastHandshakeInfo` trả `Features` rỗng.
- Với agent mới (khi có): thấy `features`, `protocolVersion=2`, `buildVersion` trong log cấp debug của handshake (không log token).

## Rủi ro và lưu ý

- Sót một đường dựng `HandshakeInfo` thì hồ sơ của dev server kết nối bằng đường đó luôn `handshake_only` mà không báo lỗi. Cách bắt: test bảng "đường kết nối x trường" ở trên, và `grep -rn "HandshakeInfo{" internal` trong review.
- `usecase.HandshakeInfo` được `UpdateProvisionResult` (repository, `postgres/repository.go:688`, `mysql/repository.go:713`) nhận vào: thêm trường không đổi chữ ký, nhưng nếu có fake dùng struct literal vị trí (không có tên trường) thì biên dịch lỗi; sửa fake.
- Agent giả mạo `features` để làm backend tin có chế độ chỉ đọc: `features` chỉ là gợi ý để chọn đường; không dùng làm bằng chứng an toàn (xem SOL-033 mục 6).
