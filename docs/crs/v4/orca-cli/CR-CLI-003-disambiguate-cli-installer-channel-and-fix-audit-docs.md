# CR-CLI-003 — Tách nhãn "CLI installer" (`cli.*` wscompat) khỏi khái niệm Orca CLI (F09) + sửa audit doc

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CLI-003 |
| **Tên** | Đổi tên nội bộ `channels_cli.go`/`registerCliChannels` → rõ nghĩa "CLI installer trên dev server"; cập nhật `feature-completion-matrix.md` |
| **Loại** | Chore / Documentation |
| **Priority** | 🟢 P3 (không chặn chức năng, chỉ tránh nhầm lẫn khi audit/bảo trì tiếp theo) |
| **Effort** | Small |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu "thực thi đầy đủ F09 ở lớp backend-go" |
| **Tác động HLD** | — |
| **Tác động Features** | F09 (Orca CLI) — chỉ gián tiếp, qua việc audit doc chính xác hơn |
| **Phụ thuộc** | Không — độc lập với CR-CLI-001/002 |

---

## Bối cảnh & Vấn đề

Khi audit F09 ở lớp `backend-go`, symbol duy nhất khớp từ khoá "cli" là `backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli.go` (`registerCliChannels`, `cliStatusHandler`, `cliMutationHandler`, `cliWslStatusHandler`, `cliWslMutationHandler`, `relayCliMethod`) với các channel `cli.getInstallStatus`, `cli.install`, `cli.remove`, `cli.getWslInstallStatus`, `cli.installWsl`, `cli.removeWsl`.

Đây **không phải** logic thực thi lệnh CLI (F09) — đọc kỹ `channels_cli.go` và commit gần nhất chạm tới nó (`1d14647b4`, "feat(frontend): thread devServerId through cli.\* so Orchestration Install works on web") xác nhận: các channel này relay qua `infrafleetv1.InfraFleetServiceClient` tới **agent của 1 dev server**, mục đích là **cài đặt/gỡ chính binary `orca` (hoặc bản WSL của nó) lên một dev server từ xa** — để agent skill (`skills/orca-cli/SKILL.md`) có `orca`/`orca-ide` sẵn sàng trong terminal trên host đó. Đây là 1 concern hoàn toàn khác F09 (Orca CLI tự thân) — gần với "CLI prerequisite/bootstrap" hơn.

Sự trùng tên `cli.*` này là nguồn gây nhầm lẫn trực tiếp dẫn tới kết luận sai ở `docs/roadmap/feature-completion-matrix.md` dòng F09 ("Không có bằng chứng rõ ràng ở cả 3 layer") — vì `channels_cli.go` là thứ duy nhất backend-go có tên "cli", nhưng nó không liên quan gì tới việc F09 đã hoàn thành ở `desktop/` (xem CR-CLI-001). Ngoài ra, `feature-completion-matrix.md`'s bảng "Codebase" ở §0 liệt kê `backend/`, `frontend/`, `backend-go/`, `agent/`, `emulator/`, `packages/dev-agent-transport/` — **thiếu hẳn `desktop/`**, khiến 3 sub-agent audit gốc không có cơ hội tìm ra `desktop/src/cli/`.

## Giải pháp đề xuất

### A. Đổi tên nội bộ (không đổi wire protocol)

Dùng `gitnexus rename` (bắt buộc theo CLAUDE.md — "NEVER rename symbols with find-and-replace") để đổi:

- File `channels_cli.go` → `channels_cli_installer.go` (và `channels_cli_test.go` tương ứng).
- `registerCliChannels` → `registerCliInstallerChannels`.
- `cliStatusHandler`/`cliMutationHandler`/`cliWslStatusHandler`/`cliWslMutationHandler`/`relayCliMethod`/`cliNotConnectedStatus` → thêm hậu tố `Installer` (vd. `cliInstallerStatusHandler`).

**Giữ nguyên 100% chuỗi channel trên wire** (`"cli.getInstallStatus"`, `"cli.install"`, ...) — đây là giao thức đã có client thật (`frontend/src/renderer/src/web/web-preload-api.ts`'s `createCliApi()`) đang dùng, đổi wire string là breaking change không cần thiết cho mục tiêu của CR này (chỉ là chore đặt tên nội bộ).

Thêm comment package-level ở đầu file mới, nêu rõ (theo đúng tinh thần AGENTS.md "Document the Why"): *"Đây là kênh cài đặt/gỡ binary `orca` lên dev server — không phải logic thực thi lệnh CLI. Logic thực thi lệnh Orca CLI (worktree/terminal/orchestration/...) nằm ở `desktop/src/cli/`, xem CR-CLI-001."*

### B. Sửa `docs/roadmap/feature-completion-matrix.md`

1. Bảng "Codebase" ở §0: thêm dòng `desktop/` — mô tả đúng vai trò ("Electron main + preload — tách từ `backend/` cũ, chứa CLI thật `src/cli/` + runtime RPC `src/main/runtime/`").
2. Dòng F09 ở bảng ma trận §1: đổi cột Backend-go từ 🟡 sang ❌ (đúng thực tế: backend-go có 0 logic thực thi lệnh CLI, chỉ có `cli.*` installer không liên quan), thêm ghi chú trỏ tới CR-CLI-001/002/003; cột Frontend/Agent giữ nguyên hoặc note rằng CLI thật nằm ở `desktop/` (ngoài phạm vi 3 cột hiện có của bảng, cần bổ sung ghi chú thay vì đổi cột).
3. §4 gap #6: cập nhật hành động đã hoàn tất ("Đã xác minh — CLI thật ở `desktop/src/cli/`, không ở `backend-go`; xem CR-CLI-001/002/003"), xoá khỏi danh sách "chưa rõ".

## Không thuộc phạm vi CR này

- Đổi wire protocol `cli.*` — xem "Giải pháp đề xuất A".
- Bất kỳ thay đổi hành vi runtime nào — CR này thuần tuý đổi tên/comment/doc.
- Thêm `desktop/` vào phạm vi audit đầy đủ (quét lại toàn bộ 42 feature theo `desktop/`) — ngoài phạm vi, chỉ sửa dòng F09 liên quan trực tiếp tới CR set này.

## Tiêu chí chấp nhận

- [ ] `go build`/test của `api-gateway` pass sau khi đổi tên (không đổi hành vi).
- [ ] Không có chuỗi channel wire nào (`cli.install`, `cli.remove`, ...) bị đổi — test tích hợp `channels_cli_installer_test.go` (đổi tên từ `channels_cli_test.go`) vẫn pass nguyên vẹn.
- [ ] `feature-completion-matrix.md` có dòng `desktop/` trong bảng Codebase, dòng F09 phản ánh đúng thực tế, gap #6 đánh dấu đã xác minh.

## Impact analysis (gitnexus)

| Symbol | Direction | Risk | Impacted |
|---|---|---|---|
| `registerCliChannels` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli.go`) | upstream | LOW | 3 (1 direct: `run` ở `api-gateway/cmd/server/main.go`; modules `V1`, `Wscompat`) |

Blast radius nhỏ — rename an toàn, nhưng vẫn phải dùng `gitnexus rename` (không find-and-replace thủ công) và chạy `detect_changes()` trước khi commit theo đúng CLAUDE.md.

## Liên quan

- CR-CLI-001 (nơi logic CLI thật được mô tả chi tiết)
- `docs/roadmap/feature-completion-matrix.md` (gap #6, bảng Codebase §0)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli.go`
- `frontend/src/renderer/src/web/web-preload-api.ts` (`createCliApi()`), `frontend/src/renderer/src/lib/agent-skill-cli-prerequisite.ts`
- Commit `1d14647b4` (bối cảnh gần nhất chạm `cli.*`)
