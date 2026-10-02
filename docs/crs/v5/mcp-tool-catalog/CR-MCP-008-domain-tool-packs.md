# CR-MCP-008 — Tool pack theo domain: agent làm được những gì UI làm được

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-008 |
| **Tên** | Khai báo `ToolSpec` cho các domain chức năng của UI, theo đợt (đọc trước, ghi sau, phá huỷ cuối) |
| **Loại** | Feature (khối lượng lớn, chia đợt) |
| **Priority** | 🔴 P0 (đợt 1–2), 🟠 P1 (đợt 3–4) |
| **Effort** | XL — ước lượng 4–6 tuần cho 4 đợt; mỗi đợt là một PR/series riêng |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Thống kê `Register*("ns.method")` trong `wscompat/channels*.go` |
| **Phụ thuộc** | [CR-MCP-007](./CR-MCP-007-registry-introspection-and-descriptors.md); đợt ghi/phá huỷ cần [CR-MCP-012](../mcp-governance-safety/CR-MCP-012-tool-policy-and-annotations.md) + [CR-MCP-013](../mcp-governance-safety/CR-MCP-013-approvals-audit-killswitch.md) |

---

## Bối cảnh

Thống kê grep literal (số lần đăng ký theo namespace, top): `git` 44, `github` 32, `task` 20, `repo` 19, `linear` 19, `jira` 19, `terminal` 18, `worktree` 15, `devServer` 14, `admin` 14, `workflow` 13, `starNag` 12, `ephemeralVm` 11, `project` 10, `profile` 10, `onboarding` 10 … tổng 417 trên 58 namespace. Số này là **cận trên thô** (chưa tính channel đăng ký trong vòng lặp, và gồm channel không nên phơi ra). CR-007 sinh số chính xác.

Mục tiêu "agent như người dùng" không có nghĩa là phơi **cả** 417. Có nhiều channel là chi tiết UI (onboarding, starNag, crashReports, telemetry, clientState, emulator…) vô nghĩa hoặc nguy hiểm với agent.

## Giải pháp đề xuất — phân đợt theo giá trị & rủi ro

### Đợt 1 — Khám phá & đọc (annotation `readOnlyHint:true`, scope `orca:read`)

| Domain | Việc agent làm được |
|--------|---------------------|
| `project`, `projectGroup`, `orcaProjects`, `repo` | Liệt kê/tìm project, repo, nhóm |
| `worktree` (list/status) | Xem worktree và trạng thái |
| `git` (status/log/diff/branch list) | Đọc lịch sử, diff |
| `task` (list/get), `annotation` (read) | Xem task/đồ thị task |
| `github`/`gitlab`/`hostedReview` (đọc PR/issue/checks) | Theo dõi review/CI (lưu ý rate limit `gh`, AGENTS.md) |
| `jira`/`linear` (đọc issue) | Đọc ticket |
| `workflow`/`automation` (list/get/run history) | Xem tự động hoá |
| `files` (đọc/grep/glob, giới hạn kích thước) | Đọc mã nguồn |
| `aiProvider` (list, **không** trả key) | Biết provider khả dụng |

### Đợt 2 — Ghi có thể hoàn tác (scope `orca:write`)

Tạo/sửa/đóng `task`, `annotation`; tạo `worktree`; `git` commit/branch/stage cục bộ; bình luận PR/issue; tạo/sửa `workflow`/`automation` (ở trạng thái *draft*); đổi trạng thái issue. Mỗi tool ghi nêu rõ cách hoàn tác (nếu có).

### Đợt 3 — Thực thi (scope `orca:exec`; **bắt buộc** qua approval CR-013 ở mức mặc định)

`terminal` (tạo/ghi/đọc output — chi tiết CR-009), `agent` (spawn/gửi input/dừng), `workflow`/`automation` chạy, `git` push/fetch có mạng, `devServer`, `ephemeralVm`. Đây là nhóm nguy hiểm nhất: một lệnh shell = toàn quyền trên máy dev của người dùng.

### Đợt 4 — Quản trị & phá huỷ (scope `orca:admin`; mặc định **tắt**, bật theo tenant)

`admin` (users/policies/sessions/audit), xoá worktree/project, `git` force-push/reset, xoá workflow, merge PR. Chỉ mở khi policy tenant cho phép **và** có phê duyệt người dùng mỗi lần.

### Danh sách loại trừ mặc định (ghi vào `excluded_channels.yaml` kèm lý do)

- Lộ/chuyển secret: `credentials.*` (đọc giá trị), `auth` token minting, `devServerAgentTokens`, `admin` force-revoke/tạo user.
- Gắn với thiết bị/UI: `mobile.*` (yêu cầu `DeviceID`, `registry.go:22-28`), `clientState`, `starNag`, `onboarding`, `telemetry`, `crashReports`, `emulator`, `browser` screencast.
- Stream nhị phân: `terminal.multiplex` (thay bằng thiết kế CR-009).

## Quy ước cho mọi tool trong pack

1. Mô tả nêu rõ tác dụng phụ & điều kiện tiên quyết; tên động từ rõ nghĩa (`worktree_create`, không `worktree_do`).
2. `Annotations` đúng sự thật (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) — **chỉ là gợi ý cho client**, không phải cơ chế bảo mật; enforcement thật ở CR-012/013.
3. Không nhận `tenant_id`/`user_id` từ tham số; luôn lấy từ `Identity`.
4. Kết quả cắt kích thước + phân trang; không trả secret, đường dẫn tuyệt đối nhạy cảm của máy khác, hay token trong URL remote git (che `https://user:token@…`).
5. Hỗ trợ SSH (AGENTS.md): tool không giả định thực thi cục bộ — đi qua cùng đường như UI (`infra-fleet`/relay).
6. Tương thích Git ≥ 2.25 và provider ngoài GitHub (GitLab…) — dùng tên chung (`hostedReview`), không đặt tên theo GitHub.

## Acceptance Criteria

- [ ] Mỗi đợt có bảng `channel → tool/loại trừ` được duyệt; parity test (CR-007) xanh.
- [ ] Kịch bản e2e đợt 1: agent (MCP Inspector/Claude Code) liệt kê project → chọn worktree → đọc diff → đọc PR mà không cần UI.
- [ ] Kịch bản e2e đợt 2: tạo task + worktree + commit trên repo thử nghiệm.
- [ ] Tool đợt 3/4 mặc định ẩn nếu CR-012/013 chưa bật (test cấu hình).
- [ ] Không tool nào nhận/trả secret (test quét schema + golden output).

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** quá nhiều tool làm LLM chọn sai/tốn ngữ cảnh ⇒ gom nhóm, ưu tiên 60–100 tool giá trị cao; cân nhắc tool "meta" tìm kiếm tool nếu danh sách vẫn lớn.
- **Rủi ro:** lệch hành vi so với UI ⇒ parity test + e2e so sánh kết quả cùng một thao tác qua UI và MCP.
- **Ngoài phạm vi:** UI cho người dùng cấu hình pack nào bật (frontend).
