# TASK-BE-CLI-007: `gitnexus rename` — `channels_cli.go` → `channels_cli_installer.go`

**Solution:** [BE-CLI-SOL-003](../solutions/BE-CLI-SOL-003-rename-cli-installer-channels.md) | **CR:** CR-CLI-003
**Service:** `api-gateway`
**Depends on:** Không
**Status:** ✅ DONE

> **Kết quả thực tế:** Dùng `mcp__gitnexus__rename` (KHÔNG find-and-replace) cho
> cả 7 symbol. 6/7 rename áp dụng đúng cả declaration + mọi call site
> (`cliStatusHandler`→`cliInstallerStatusHandler`, `cliMutationHandler`→
> `cliInstallerMutationHandler`, `cliWslStatusHandler`→
> `cliInstallerWslStatusHandler`, `cliWslMutationHandler`→
> `cliInstallerWslMutationHandler`, `relayCliMethod`→
> `relayCliInstallerMethod`, `cliNotConnectedStatus`→
> `cliInstallerNotConnectedStatus`). **1 phát hiện lệch với kỳ vọng**:
> `registerCliChannels`→`registerCliInstallerChannels` — tool chỉ đổi được
> mọi call site (`channels.go`, `channels_cli_installer_test.go`), KHÔNG đổi
> declaration của chính hàm đó trong file (`go build` fail ngay với
> `undefined: registerCliInstallerChannels`, phát hiện bằng build thật, không
> phải đoán); gọi lại `rename` (kể cả sau khi đã `git mv`) trả lỗi "Symbol not
> found" — index đã coi symbol này đã đổi tên xong dù disk chưa khớp. Đã sửa
> thủ công đúng 1 dòng declaration (`func registerCliChannels(...)` →
> `func registerCliInstallerChannels(...)`) để khớp với mọi call site đã đổi
> — đây là sửa 1 dòng xác định, không phải find-and-replace hàng loạt, và là
> phần còn thiếu của chính thao tác rename đã chạy qua tool. Ghi nhận lại đây
> vì task yêu cầu không bịa kết quả.
>
> `git mv channels_cli.go channels_cli_installer.go` +
> `git mv channels_cli_test.go channels_cli_installer_test.go`. Thêm đúng 1
> câu vào cuối đoạn đầu doc comment. Cập nhật `docs/roadmap/feature-completion-matrix.md`
> đủ cả 3 điểm: thêm dòng `desktop/` vào bảng Codebase §0, đổi cột Backend-go
> của F09 từ 🟡 sang ❌ kèm ghi chú CR-CLI-001/002/003, gạch bỏ gap #6 ở §4
> với ghi chú "Đã xác minh".
>
> **Build/test thật**: `go build ./...` sạch. `go test
> ./internal/adapter/wscompat/... -run TestCli -v` — toàn bộ test có sẵn
> trong file đổi tên PASS NGUYÊN VẸN, không sửa assertion nào (bao gồm
> `TestCliChannels_RelaySuccess`, `TestCliChannels_MissingDevServerID_...`,
> `TestCliStatusChannels_NotConnected_...`,
> `TestCliMutationChannels_NotConnected_...`). `gofmt -l` sạch.
> `git diff --stat ... | grep -i cli` xác nhận đúng 2 file đổi tên (không có
> file nào khác bị "cli" chạm ngoài dự kiến). `git diff ... | grep -E
> '^[+-].*"cli\.'` xác nhận 0 dòng string-literal channel name bị đổi (chỉ
> tên hàm bao quanh literal thay đổi).
>
> **Ghi chú môi trường** (không thuộc phạm vi task): `git status` cho thấy
> `channels_dev_server_access_control.go` và `channels_onboarding.go` cũng bị
> đổi (thêm comment "TASK-BE-013") — xác nhận đây KHÔNG phải do task này gây
> ra (không đụng tới 2 file đó), rất có thể do 1 agent khác chạy song song
> trong cùng working directory không cô lập worktree — không sửa/không revert.

---

## Mục tiêu

Đổi tên nội bộ 7 symbol + 2 file trong `wscompat` để tránh nhầm lẫn với F09 Orca CLI
thật (ở `desktop/`) — KHÔNG đổi bất kỳ chuỗi channel wire nào.

## Files cần sửa

1. `backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli.go` → đổi tên thành `channels_cli_installer.go`
2. `backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli_test.go` → đổi tên thành `channels_cli_installer_test.go`
3. `docs/roadmap/feature-completion-matrix.md` (MODIFY — 3 điểm, xem CR-CLI-003 §"Giải pháp đề xuất B")

## Danh sách symbol cần `gitnexus rename` (đã re-verify khớp code hiện tại — xem BE-CLI-SOL-003 mục 1)

| Tên cũ | Tên mới đề xuất |
|---|---|
| `registerCliChannels` | `registerCliInstallerChannels` |
| `cliStatusHandler` | `cliInstallerStatusHandler` |
| `cliMutationHandler` | `cliInstallerMutationHandler` |
| `cliWslStatusHandler` | `cliInstallerWslStatusHandler` |
| `cliWslMutationHandler` | `cliInstallerWslMutationHandler` |
| `relayCliMethod` | `relayCliInstallerMethod` |
| `cliNotConnectedStatus` | `cliInstallerNotConnectedStatus` |

**Chuỗi channel wire giữ nguyên 100%** — `"cli.getInstallStatus"`, `"cli.install"`,
`"cli.remove"`, `"cli.getWslInstallStatus"`, `"cli.installWsl"`, `"cli.removeWsl"`
(các literal string ở `registerCliInstallerChannels`'s body) — `gitnexus rename`
chỉ đổi tên symbol Go, không đụng string literal, nhưng vẫn PHẢI xác nhận lại sau
khi rename bằng `git diff` rằng không dòng string literal nào bị chạm.

## Nội dung comment cần bổ sung (đầu file, sau khi đổi tên file)

Doc comment hiện có (dòng 1-26 của file cũ) giữ nguyên gần như toàn bộ — chỉ thêm 1
câu vào cuối đoạn đầu tiên:

```go
// ...(giữ nguyên nội dung hiện có)...
//
// Logic thực thi lệnh Orca CLI thật (worktree/terminal/orchestration/...) nằm ở
// desktop/src/cli/ — không liên quan tới file này. Xem CR-CLI-001.
package wscompat
```

## Test cases cần cover

- Không viết test mới — **mọi test hiện có trong `channels_cli_test.go` (đổi tên
  thành `channels_cli_installer_test.go`) phải PASS NGUYÊN VẸN**, không sửa
  assertion nào bên trong (đây chính là guard xác nhận wire protocol không đổi).

## Verify

```bash
cd backend-go/services/api-gateway
go build ./...
go test ./internal/adapter/wscompat/... -run TestCli -v   # hoặc tên test thật sau khi rename file
gofmt -l internal/adapter/wscompat/channels_cli_installer.go
git diff --stat internal/adapter/wscompat/ | grep -i cli   # xác nhận chỉ 2 file đổi tên, không file nào khác bị chạm ngoài dự kiến
git diff internal/adapter/wscompat/channels_cli_installer.go | grep -E '^\+.*"cli\.' # xác nhận 0 thay đổi ở dòng string literal channel name
```

## gitnexus

- **Bắt buộc dùng `gitnexus rename`, KHÔNG find-and-replace thủ công** (yêu cầu
  tuyệt đối của CLAUDE.md/AGENTS.md cho toàn bộ 7 symbol ở bảng trên).
- `impact({target: "registerCliChannels", direction: "upstream"})` trước khi rename
  — xác nhận đúng như CR-CLI-003 đã ghi (LOW risk, 3 impacted, 1 direct caller ở
  `channels.go`'s `run`).
- `detect_changes()` sau khi rename, trước khi commit — xác nhận thay đổi CHỈ chạm
  đúng 2 file đổi tên + `feature-completion-matrix.md`, không lan sang symbol/file
  nào khác ngoài dự kiến.

## Blocking

Không có task nào phụ thuộc — độc lập hoàn toàn với TASK-BE-CLI-001 đến 006.
