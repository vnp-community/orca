# Orca CLI (F09) — Change Requests (v4)

> **Bối cảnh:** Yêu cầu "thực thi đầy đủ F09 — Orca CLI ở lớp `backend-go`", xuất phát từ
> `docs/roadmap/feature-completion-matrix.md` dòng F09 ("Không có bằng chứng rõ ràng ở cả 3 —
> CLI binary có thể ở package khác chưa audit hoặc chưa migrate") và gap #6 ("Xác minh vị trí
> thực tế... trước khi kết luận là 'chưa làm'"). Khảo sát trực tiếp mã nguồn (không chỉ
> `frontend/`, `backend-go/`, `agent/` như audit gốc quét) cho kết quả **khác hẳn giả định ban
> đầu**: F09 **không hề "chưa làm"** — nó đã hoàn thành đầy đủ, đang chạy hàng ngày, chỉ là ở một
> package thứ 4 mà audit 3-way trước đó chưa từng quét tới: `desktop/`.

## Kết luận vị trí CLI thật

| Câu hỏi | Trả lời | Bằng chứng |
|---|---|---|
| CLI binary (`orca`) nằm ở đâu? | `desktop/` — gói Electron main+preload tách ra từ `backend/` cũ (`desktop/package.json`: *"Orca Desktop... isolated copy, split from monorepo"*) | `desktop/src/cli/` (119 file: `dispatch.ts`, `handlers/`, `runtime/`), `desktop/package.json`'s `"bin": {"orca": "./out/cli/index.js"}` khớp root `package.json:7-10` |
| CLI có thật sự hoạt động không? | Có — tài liệu hoá đầy đủ, dùng hàng ngày bởi agent (kể cả trong chính môi trường chạy audit này) | `skills/orca-cli/SKILL.md` (287 dòng, mọi lệnh `orca worktree/terminal/automations/browser/emulator ...`) |
| CLI có nói chuyện với `backend-go` không? | **Không, ở bất kỳ transport nào hiện có** — cả 2 transport (Unix socket cục bộ, WS+E2EE remote pairing) đều chỉ nói chuyện với tiến trình Electron main, không có transport nào gọi `api-gateway` | Grep trực tiếp `desktop/src/cli/`: 0 tham chiếu `api-gateway`/`:8080`; `CR-TRACE-010` (2026-08-01) xác nhận cùng kết luận, vẫn khớp code hiện tại |
| `backend-go` có gì tên "cli" không? | Có, nhưng là **1 concern khác hoàn toàn**: `channels_cli.go`'s `cli.install`/`cli.remove` chỉ cài đặt binary `orca` lên 1 dev server từ xa — không phải logic thực thi lệnh CLI | `backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli.go`, relay qua `infrafleetv1.InfraFleetServiceClient` |
| Vậy gap thật ở lớp backend-go là gì? | F09's lời hứa "headless mode cho Linux server không có display / CI-CD" (`docs/features/F09-orca-cli.md` dòng 57-61, 89-96) **chưa được hiện thực đúng** — `orca serve` hiện chỉ respawn lại chính Electron app, không dùng `backend-go` dù `backend-go` đã có sẵn hầu hết API cần thiết (xây cho F22 Web Server Mode) | `serveOrcaApp()` (`desktop/src/cli/runtime/launch.ts:66`); coverage 1:1 `worktree.*`/`terminal.*`/`git.*` giữa CLI và `wscompat` channels — xem CR-CLI-001 |

## Bộ CR

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-CLI-001](./CR-CLI-001-headless-transport-to-backend-go.md) | CLI không có transport nào nói chuyện với `backend-go`; `orca serve` headless thực chất vẫn respawn Electron | 🟡 P1 | Large | 🔲 Chưa triển khai |
| [CR-CLI-002](./CR-CLI-002-headless-credential-for-cli.md) | Không có credential không-tương-tác (API token/service-account) ở `auth-service` cho CLI chạy CI/CD | 🟡 P1 | Medium | 🔲 Chưa triển khai — cần security review |
| [CR-CLI-003](./CR-CLI-003-disambiguate-cli-installer-channel-and-fix-audit-docs.md) | Tên `cli.*` ở `backend-go` trùng với F09, gây audit sai; `feature-completion-matrix.md` thiếu `desktop/` trong phạm vi | 🟢 P3 | Small | 🔲 Chưa triển khai |

## Thứ tự thực thi

```
CR-CLI-003 (đổi tên + sửa audit doc) ── độc lập, làm bất kỳ lúc nào, nên làm sớm để
                                          audit doc không tiếp tục gây nhầm lẫn

CR-CLI-001 (transport backend-go)    ── cần chốt kiến trúc trước khi code (xem CR's
        │                                "Giải pháp đề xuất"); có thể triển khai và
        │                                dùng ngay ở chế độ "user đã đăng nhập tương tác"
        │                                (không cần CR-CLI-002) cho use case SSH/remote thủ công
        ▼
CR-CLI-002 (credential headless)     ── chặn CỨNG riêng use case CI/CD không tương tác
                                          của F09 (GitHub Actions); cần security review
                                          scope/expiry/revocation trước khi triển khai
```

CR-CLI-001 và CR-CLI-002 tách rời có chủ đích: CR-CLI-001 tự nó đã mở khoá phần lớn giá trị
(SSH/remote CLI không cần Electron đầy đủ, miễn có phiên đăng nhập trước) mà không cần chờ
quyết định bảo mật về credential dài hạn của CR-CLI-002.

## Impact analysis (gitnexus, chạy trước khi sửa — bắt buộc theo CLAUDE.md)

| Symbol sửa | Risk | Impacted count | CR |
|---|---|---|---|
| `sendRequest` (`desktop/src/cli/runtime/transport.ts`) | HIGH | 32 (2 direct; flows: `worktree create`, `automations edit`, `automations create`) | CR-CLI-001 (tham khảo — không sửa trực tiếp, chỉ thêm hàm chị em) |
| `RuntimeClient` (`desktop/src/cli/runtime/client.ts:23`) | LOW | 5 (3 direct) | CR-CLI-001 |
| `serveOrcaApp` (`desktop/src/cli/runtime/launch.ts:66`) | LOW | 1 direct | CR-CLI-001 |
| `registerCliChannels` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli.go`) | LOW | 3 (1 direct) | CR-CLI-003 |

CR-CLI-002 đề xuất toàn bộ symbol mới (chưa tồn tại) nên chưa có gì để chạy `impact()` — sẽ
chạy lên `SessionValidator.ValidateToken`/`AuthServiceClient` ngay trước khi implement. Không
CR nào trong bộ này được thực thi (code) trong lần khảo sát này — cả 3 file trên là tài liệu
đặc tả, chưa có thay đổi code nào.

## Việc chưa làm ngoài bộ CR này

- Quét lại toàn bộ 42 feature trong `feature-completion-matrix.md` với `desktop/` được thêm vào
  phạm vi audit (CR-CLI-003 chỉ sửa đúng dòng F09 liên quan tới bộ CR này) — các feature khác có
  thể có cùng vấn đề "sống ở `desktop/`, bị bỏ sót khỏi audit 3-way", cần một lượt audit riêng
  nếu team muốn phủ hết.
- Thêm channel `orchestration.run`/`orchestration.dispatch`/`orchestration.send` và
  `automation.runNow` vào `wscompat` — gap có sẵn ở chính `backend-go`, thuộc track F14/F36
  đang 🚧, CR-CLI-001 chỉ định tuyến transport, không tự ý implement các channel này.
- Quyết định cuối cùng về thời hạn/scope/revocation của CLI token (CR-CLI-002) — chờ security
  review, tương tự cách CR-RBAC-007 (SAML) chờ xác nhận business trước khi triển khai.
