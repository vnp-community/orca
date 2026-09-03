# Hướng dẫn sử dụng AI Agent để thực thi phát triển code

**Cập nhật:** 2026-09-02

> Tổng hợp từ [F04 — AI Agent Support](../../features/F04-ai-agent-support.md),
> [F01 — Parallel Worktrees](../../features/F01-parallel-worktrees.md),
> [F08 — Annotate AI Diffs](../../features/F08-annotate-ai-diffs.md),
> [F37 — Task Graph Management](../../features/F37-task-graph-management.md),
> [F14 — Automations](../../features/F14-automations.md).
> Trạng thái phát hành theo từng phần được ghi rõ ở mỗi mục — F04/F01/F08 ✅ đã phát hành,
> F37/F14 🚧 đang phát triển (F37 backend đã chạy thật, xem
> [task-automation-orchestration-integration.md](../task-automation/task-automation-orchestration-integration.md)
> để đối chiếu trạng thái thật vs. doc gốc).

---

## 1. Khởi động agent trong một worktree (luồng cơ bản) — ✅

1. Orca tự động scan PATH và phát hiện các agent CLI đã cài (Claude Code, Codex, Gemini,
   Cursor, OpenCode, v.v. — 30+ agent, xem danh sách đầy đủ trong F04) và hiển thị trong UI.
2. Tạo **worktree** mới từ một branch/commit — mỗi worktree có terminal và file explorer
   riêng, cô lập hoàn toàn với các worktree khác.
3. Chọn agent (vd. Claude Code) → Orca khởi động agent đó trong PTY (terminal thật), agent
   chạy trực tiếp như khi gõ lệnh CLI.
4. Orca parse trạng thái agent tự động (qua OSC + hook lifecycle) để biết agent đang
   idle/running/cần permission.
5. Nếu agent chạm rate limit → Orca cảnh báo → có thể **hot-swap** sang agent/tài khoản khác
   (account switcher).
6. Đóng app rồi mở lại → **session resume** (`--resume <session-id>` với Claude Code, tương
   đương với Codex/OpenCode) tiếp tục đúng phiên làm việc cũ.

**Trust presets** kiểm soát quyền của agent khi chạy:

| Preset | Quyền |
|--------|-------|
| `minimal` | Chỉ đọc, không chạy lệnh |
| `standard` | Đọc/ghi file, chạy lệnh an toàn |
| `trusted` | Full quyền |

Remote agent (SSH host) có trust presets riêng biệt.

---

## 2. Fan-out — chạy nhiều agent song song để so sánh kết quả — ✅

Cách dùng mạnh nhất khi chưa chắc cách tiếp cận nào tốt nhất:

```
1. Nhập một prompt duy nhất
2. Chọn "Fan-out" → chọn số lượng (1–10 agent)
3. Orca tạo N worktree mới từ cùng base branch
4. Mỗi worktree tự khởi động agent + inject cùng prompt, chạy độc lập song song
5. Theo dõi tiến trình từng agent real-time
6. So sánh diff giữa các worktree → chọn worktree tốt nhất
7. Merge worktree thắng vào nhánh chính, cleanup các worktree còn lại
```

Với monorepo lớn, dùng **sparse checkout preset** (lưu theo repo, chọn lại khi tạo worktree
tiếp theo) để giới hạn worktree chỉ checkout một tập thư mục con — agent chạy nhanh hơn và
tránh nhiễu context.

---

## 3. Review và feedback code do agent tạo (không rời khỏi Orca) — ✅

1. Click "Review Changes" → mở diff viewer (unified/split, syntax highlight 50+ ngôn ngữ,
   collapse hunk lớn cho diff dài).
2. Click trực tiếp vào dòng code cần góp ý (single line hoặc multi-line) → nhập comment
   (hỗ trợ markdown).
3. Review xong nhiều dòng → click "Send to Agent": Orca gom tất cả comment thành một prompt
   có cấu trúc (file path, line number, code gốc, nội dung comment) và inject thẳng vào PTY
   của agent.
4. Agent xử lý, tạo diff mới → diff viewer tự refresh.
5. Hài lòng → "Commit" (AI có thể tự generate commit message) → review → submit. Có thể tích
   hợp gửi comment lên GitHub PR review luôn.

---

## 4. Quản lý công việc có cấu trúc bằng Task Graph — 🚧

Khi task phức tạp hơn một lần chạy agent đơn lẻ:

- Tạo task dạng cây (Epic → Story → Task → Subtask) với quan hệ cha-con và depends-on (DAG).
- Mỗi task có field **prompt riêng** (`promptTemplate` + `aiContext`) gắn liền context của
  task đó.
- Click **"AI: Plan this task"** → AI tự phân rã task lớn thành subtask kèm ước lượng và
  dependency, người dùng review/accept/sửa trước khi tạo vào graph.
- Từ Task Detail Panel: **"Run Agent"** spawn agent trên dev server của project, agent đọc
  `promptTemplate + aiContext + description` và thực thi, log stream về Activity feed của
  task.
- Có thể chia sẻ task/tree theo scope company/team/user với 5 mức quyền
  (view/comment/edit/execute/manage).

> Backend của hệ Task (Plan) — `OrcaTask` — đã chạy thật với 18 RPC method (không chỉ là
> spec), xem đối chiếu chi tiết ở
> [task-automation-orchestration-integration.md](../task-automation/task-automation-orchestration-integration.md).

---

## 5. Tự động hóa lặp lại bằng Automations — 🚧

Cho các tác vụ lặp lại định kỳ, định nghĩa automation dạng YAML gồm trigger
(cron/manual/event) + actions (create_worktree → run_agent → commit → create_pr → notify):

```yaml
name: "Morning Code Review"
trigger:
  cron: "0 9 * * 1-5"
actions:
  - type: create_worktree
    base: main
  - type: run_agent
    agent: claude
    prompt: "Review all TODOs and suggest fixes"
  - type: commit
    message: "chore: automated TODO review"
  - type: create_pr
    title: "Weekly TODO cleanup"
```

---

## Tóm tắt luồng thực tế đề xuất

```
Worktree đơn lẻ + 1 agent          → task nhỏ/rõ ràng
Fan-out nhiều agent                → chưa chắc cách tiếp cận nào tốt nhất
Annotate Diffs                     → vòng lặp review/sửa với agent
Task Graph                         → việc lớn cần phân rã, giao nhiều người/nhiều agent
Automations                        → việc lặp lại định kỳ
```
