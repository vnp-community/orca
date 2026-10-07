# AG-REQ-TASK-033-13: Tăng `AGENT_VERSION`, cập nhật triển khai và kiểm tương thích hai chiều

**From Solution:** [AG-REQ-SOL-033-capability-report-handshake-and-ai-complete](../solutions/AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md) mục 2.4 và 3.5
**Priority:** P1
**Area:** `agent/` (Dev Server Agent) và `deploy/agent/`
**File:** `agent/build.mjs` (sửa), `agent/src/relay/agent-entry.ts` (sửa, hai chuỗi log), `deploy/agent/package.json` (sửa), `deploy/agent/README.md` (sửa), `agent/src/relay/agent-compat-matrix.test.ts` (mới)
**Depends on:** [12](./AG-REQ-TASK-033-12-handshake-protocol-version-and-features.md) (và gián tiếp 01 đến 11)
**Status:** [x] DONE

## Context

CR-033 mục 2.7 viết rằng với `relay-ssh` "sshrelay tự đẩy khi `AGENT_VERSION` đổi", nên phải tăng `AGENT_VERSION` trong `agent/build.mjs` (đề xuất `2.2.0`) cùng `deploy/agent/package.json`. Đã kiểm lại phía Go và phát hiện CR chưa chính xác: `sshrelay/provisioner.go:141` bỏ qua đẩy bundle khi `version == p.cfg.OrcaVersion`, với `version` là `AGENT_VERSION` của bundle ở xa (`version_check.go`, `remoteVersionAndPresence` chạy `node -e "require(...).AGENT_VERSION"`); `OrcaVersion` là biến `ORCA_VERSION` của backend (`sshrelay/config.go:20-34`). Tức là không có "phiên bản agent mong muốn" nào được so; tăng `AGENT_VERSION` lên `2.2.0` chỉ kích hoạt đẩy lại khi `ORCA_VERSION` không trùng chuỗi của bundle xa. Giá trị `ORCA_VERSION` ở môi trường thật chưa kiểm chứng. Nếu `ORCA_VERSION` đang trùng `2.1.0`, bundle cũ trên máy xa sẽ không bao giờ được thay (dù bản mới ở `agent/out/agent.js`). Task này vì vậy KHÔNG chỉ tăng số, mà phải ghi quy trình triển khai an toàn và để người vận hành xác nhận.

Số phiên bản có nhiều nguồn (đã xác minh): `5.0.0` ở handshake (không đổi), `AGENT_VERSION = '2.1.0'` ở `agent/build.mjs:22`, chuỗi log cứng `Orca Dev Agent v2.1.0` ở `agent-entry.ts` dòng 83 (chế độ stdio) và 115, `1.4.138-rc.6` ở `agent/package.json` (gói tách, không đổi), `2.1.0` ở `deploy/agent/package.json` và hai ví dụ trong `deploy/agent/README.md` (dòng 61 và 84). `build.mjs` còn ghi `.agent-version` dạng `2.1.0+<hash>` (dòng 70), không dùng cho so sánh `sshrelay`.

Hai bản agent: `agent/` là nguồn thật cho dev server (`deploy/agent/README.md` dùng `scp agent/out/agent.js`; `sshrelay.Config.BundlePath` là "agent/out/agent.js"). `desktop/src/relay/` (154 file) không có `execPrompt` (đã `grep -rn execPrompt desktop/src/relay`, không ra); `desktop/config/scripts/build-agent-only.mjs` build bản riêng ra `desktop/out/relay/agent.js`. Kết luận CR đúng: chỉ sửa `agent/`; bundle build từ `desktop/` rơi vào đường degradation (trả `-32601` cho mọi method mới và không có `features`).

## Việc cần làm

1. `agent/build.mjs:22`: `const AGENT_VERSION = '2.2.0'`.
2. `agent/src/relay/agent-entry.ts` dòng 83 và 115: đổi `v2.1.0` thành chuỗi dùng `AGENT_VERSION` (`\`Orca Dev Agent v${AGENT_VERSION} (stdio mode)\``) để khỏi cứng lần nữa; `AGENT_VERSION` đã là hằng xuất ở đầu file.
3. `deploy/agent/package.json`: `"version": "2.2.0"`.
4. `deploy/agent/README.md`: cập nhật hai ví dụ `2.1.0` thành `2.2.0`; thêm mục "Nâng cấp lên 2.2.0 (CR-REQ-033)" gồm: (a) thứ tự BẮT BUỘC agent trước, backend sau (backend mới chỉ gọi method mới khi `features` có, nên không có cửa sổ lỗi); (b) lệnh thủ công `scp agent/out/agent.js` rồi `systemctl restart orca-agent` (luôn đúng); (c) với `relay-ssh`: nêu rõ cảnh báo về `provisioner.go:141` và yêu cầu người vận hành kiểm `ORCA_VERSION` của backend, nếu muốn buộc đẩy lại hãy đặt `ORCA_VERSION` khác chuỗi `AGENT_VERSION` của bundle cũ hoặc xoá bundle xa; (d) cách xác nhận sau nâng cấp: kết nối lại rồi gọi `agent.capabilities` (kỳ vọng `agent.buildVersion == "2.2.0"`, `protocolVersion == 2`) hoặc xem log handshake.
5. Tạo `agent-compat-matrix.test.ts` (mới): một bảng test cấu trúc cho ma trận tương thích ở solution mục 3.5, KHÔNG chạy mạng:
   - agent mới + tham số cũ: `handleAgentExecPrompt` không tham số mới cho kết quả đúng khung cũ (dùng cùng khung `FakeChild` như `agent-print-mode-exec.test.ts`; không sao chép nhiều, chỉ một ca đại diện).
   - agent mới trả `applied.accessMode === 'readonly'` khi gửi `readonly` (tín hiệu để backend phát hiện agent cũ).
   - handshake chứa đủ `protocolVersion`, `buildVersion`, `features`, và `agentVersion === '5.0.0'`.
   - `ai.complete` không `maxTokens` vẫn trả `content` và `model`.
   - đường mô phỏng "agent cũ": cung cấp một hàm `legacyExecPromptEcho(params)` trong test mô tả hành vi agent cũ (bỏ qua khoá lạ, không `applied`) để đối chiếu với kỳ vọng của backend (tài liệu hoá hợp đồng dạng test; backend dùng khung này để viết test của mình).
6. Cập nhật tài liệu TDD? Không sửa `specs/agent/tdd/`; ghi vào báo cáo cuối task danh sách TDD cần cập nhật (`v5/04-handshake-session.md`, `v5/07-jsonrpc-dispatch.md`, `v5/08-deployment.md`, `v5/09-ai-credential-relay.md`) cho người điều phối.
7. Chạy kiểm tra tay trên dev server thử (mục Kiểm thử, phần "tay").

## Kiểm thử

Tự động (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/agent-compat-matrix.test.ts`; rồi `pnpm test` (toàn gói). Build: `pnpm build` (script `build` là `node build.mjs`) rồi kiểm `out/.agent-version` bắt đầu bằng `2.2.0+`; chạy `node -e "console.log(require('./out/agent.js').AGENT_VERSION)"` trong `agent/` in `2.2.0` (đây chính xác là cách `version_check.go` đọc phiên bản). Chưa chạy.

Tay (CHƯA CHẠY, bắt buộc trước khi bật cờ `request_flow_enabled` ở CR-REQ-025):
1. Một dev server chạy bundle cũ (`2.1.0`) cạnh một dev server chạy bundle mới (`2.2.0`), cùng backend mới.
2. Với server cũ: gọi `agent.capabilities` phải trả `-32601`; backend ghi hồ sơ `handshake_only`, `degraded=true`.
3. Với server mới: `agent.capabilities` có `agent.protocolVersion: 2`; handshake có `features`.
4. Với từng chế độ kết nối (direct-websocket, relay-websocket, relay-ssh): xác nhận `features` tới `HandshakeInfo` (xem Rủi ro của task 12).
5. Với server mới và `claude` thật: chạy thử nghiệm đối kháng ở task 04 và ghi kết quả.
6. Bundle build từ `desktop/` (nếu ai đó triển khai nhầm): xác nhận hành vi như agent cũ.

## Tiêu chí hoàn thành

- [x] `AGENT_VERSION` là `2.2.0` ở `build.mjs`, `deploy/agent/package.json`, hai chuỗi log và hai ví dụ README; `agent/package.json` không đổi.
- [x] `node -e "require('./out/agent.js').AGENT_VERSION"` sau build in `2.2.0`.
- [x] README triển khai nêu thứ tự agent trước backend sau, lệnh thủ công, cảnh báo `provisioner.go:141`, và cách xác nhận.
- [x] `agent-compat-matrix.test.ts` xanh; toàn gói `pnpm test` xanh.
- [x] Kết quả thử trên dev server thật (hoặc ghi rõ "chưa chạy") được ghi lại.

## Rủi ro và lưu ý

- Điểm backend phải khớp: `sshrelay` không so với phiên bản mong muốn. Chọn một trong: đặt `ORCA_VERSION` bằng `AGENT_VERSION` mong muốn (khi đó bundle `2.1.0` xa bị coi cũ và được đẩy lại), hoặc sửa `provisioner.go` so với hằng số (ngoài `agent/`). Câu hỏi mở 3 của solution.
- Tăng số phiên bản không đổi `agentVersion: '5.0.0'` của handshake, nên `ORCA_AGENT_MIN_VERSION` vẫn chưa chặn được gì; việc hợp nhất ba nguồn số phiên bản thuộc CR dọn dẹp riêng (câu hỏi mở 4 của CR).
- Đổi chuỗi log có thể làm vỡ test hoặc giám sát khớp chuỗi `v2.1.0` (chưa grep toàn repo; chạy `grep -rn "Orca Dev Agent v" /opt/repos/orca --include=*.ts --include=*.go --include=*.sh` trước khi đổi).
- Triển khai sai thứ tự (backend trước, agent sau) an toàn vì backend chỉ gọi method mới khi `features` có; nhưng người vận hành sẽ thấy hồ sơ `handshake_only` tới khi agent được nâng cấp.
- Bản build từ `desktop/` vẫn có thể bị triển khai nhầm: không có cảnh báo chủ động (rủi ro đã nêu ở CR); chỉ có đường degradation.

## Không làm trong task này

- Không sửa `desktop/config/scripts/build-agent-only.mjs` hay `desktop/src/relay/`.
- Không đổi `agent/package.json` (`1.4.138-rc.6`).
- Không sửa `sshrelay/provisioner.go` (thuộc backend; chỉ nêu trong README triển khai).

## Ghi chú thực thi (điền khi làm)

- Giá trị `ORCA_VERSION` thật của backend ở môi trường triển khai: chưa biết.
- Kết quả chạy tay mục "Tay" bước 1 đến 6: chưa chạy.
