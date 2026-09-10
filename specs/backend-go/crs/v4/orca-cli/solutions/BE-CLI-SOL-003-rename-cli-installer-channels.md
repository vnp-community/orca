# BE-CLI-SOL-003: Đổi tên `channels_cli.go` → `channels_cli_installer.go`

> **🔲 Designed — chưa implement.** Độc lập hoàn toàn với BE-CLI-SOL-001/002.
>
> **Ghi chú về việc tách solution riêng cho 1 việc nhỏ**: CR-CLI-003 là chore
> (rename + doc), effort Small, không có quyết định kiến trúc nào cần "thiết kế" —
> tách file riêng chỉ để (a) giữ đúng cấu trúc 1-solution-per-CR nhất quán với 2 file
> kia trong cùng bộ CR, và (b) ghi lại chính xác danh sách symbol cần đổi tên (đã
> re-verify khớp 100% với CR) trước khi giao cho `tasks/` — bản thân nội dung dưới
> đây gần như chỉ là "task inlined", KHÔNG có mục "Giải pháp" tách riêng vì
> "Trạng thái hiện tại" đã đủ dùng làm chỉ dẫn implement.

**CR:** [CR-CLI-003](../../../../../../docs/crs/v4/orca-cli/CR-CLI-003-disambiguate-cli-installer-channel-and-fix-audit-docs.md)
**Service:** `api-gateway` (`internal/adapter/wscompat`)
**TDD tham chiếu:** không có mục riêng — thuần đổi tên nội bộ, không đổi API surface

---

## 1. Trạng thái hiện tại — xác nhận khớp 100% với CR, không phát hiện lệch

Re-verify (codegraph, đọc verbatim `channels_cli.go`) xác nhận **mọi chi tiết
CR-CLI-003 nêu đều đúng với code hiện tại**, không có gì cần sửa lại thiết kế:

- File `backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli.go`
  tồn tại đúng vị trí, đúng nội dung.
- Symbol cần đổi tên khớp chính xác danh sách CR nêu: `registerCliChannels`
  (dòng 59), `cliStatusHandler` (72), `cliMutationHandler` (97),
  `cliWslStatusHandler` (110), `cliWslMutationHandler` (134), `relayCliMethod`
  (158), `cliNotConnectedStatus` (190).
- 6 channel string trên wire (`cli.getInstallStatus`, `cli.install`, `cli.remove`,
  `cli.getWslInstallStatus`, `cli.installWsl`, `cli.removeWsl`, dòng 60-65) —
  **giữ nguyên**, đúng như CR yêu cầu.
- `registerCliChannels` có đúng 1 caller thật (`channels.go`) — impact thấp,
  khớp bảng impact analysis của CR (LOW, 3 impacted, 1 direct).
- File đã có doc comment package-level giải thích rõ đây là "CLI installer trên
  dev server" (dòng 1-26) — comment này **đã tồn tại từ trước** (không phải do CR
  này thêm mới); CR-CLI-003's đề xuất "thêm comment nêu rõ... xem CR-CLI-001" chỉ
  cần bổ sung 1 câu trỏ tới CR-CLI-001 vào comment đã có, không viết lại từ đầu.

## 2. Việc cần làm (không tách mục "Giải pháp" riêng — xem lý do ở đầu file)

1. `gitnexus rename` (bắt buộc — không find-and-replace) cho 7 symbol liệt kê ở
   mục 1, theo đúng hậu tố `Installer` CR đề xuất (`registerCliInstallerChannels`,
   `cliInstallerStatusHandler`, v.v.).
2. Đổi tên file `channels_cli.go` → `channels_cli_installer.go`,
   `channels_cli_test.go` → `channels_cli_installer_test.go`.
3. Bổ sung 1 câu vào doc comment đầu file (đã có sẵn phần lớn nội dung, chỉ thêm
   câu trỏ CR-CLI-001): *"Logic thực thi lệnh Orca CLI (worktree/terminal/
   orchestration/...) nằm ở `desktop/src/cli/`, xem CR-CLI-001."*
4. Sửa `docs/roadmap/feature-completion-matrix.md` theo đúng 3 điểm CR-CLI-003
   mục "Giải pháp đề xuất B" (thêm dòng `desktop/`, đổi cột Backend-go dòng F09,
   cập nhật gap #6) — đây là sửa doc, không phải code Go, không cần
   `gitnexus rename`.

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Đổi tên bằng find-and-replace thay vì `gitnexus rename` | Vi phạm CLAUDE.md nếu làm sai | Bắt buộc dùng `rename` MCP tool |
| Bỏ sót 1 trong 7 symbol khi rename | Thấp | Danh sách đã re-verify đủ ở mục 1, `detect_changes()` trước khi commit sẽ bắt được symbol còn sót tên cũ |
| Vô tình đổi wire string `cli.*` | Cao nếu xảy ra (breaking change cho `frontend/`'s `createCliApi()`) | Test `channels_cli_installer_test.go` (đổi tên, giữ nguyên assertion) phải PASS nguyên vẹn — đây là guard chính |

## Không thuộc phạm vi solution này

- Đổi wire protocol — xem CR-CLI-003 "Không thuộc phạm vi".
- Bất kỳ thay đổi hành vi runtime nào.
- Quét lại toàn bộ 42 feature theo `desktop/` — chỉ sửa đúng dòng F09 liên quan.

## Liên quan

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli.go` (toàn bộ, đã đọc verbatim)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (caller duy nhất của `registerCliChannels`)
- `docs/roadmap/feature-completion-matrix.md`
- CR-CLI-003 (nguồn), CR-CLI-001 (nơi comment mới trỏ tới)
