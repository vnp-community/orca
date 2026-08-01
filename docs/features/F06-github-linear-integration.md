# F06 — GitHub & Linear Integration

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F06 |
| **Tên** | GitHub & Linear Integration |
| **Ưu tiên** | P1 — Should Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.5 |
| **Tham chiếu URD** | UR-040, UR-041, UR-042, UR-043 |
| **Tham chiếu SRS** | FR-6.1, FR-6.2, FR-6.3, FR-6.4 |
| **ADR References** | — |
| **HLD References** | C3.9 |

---

## Mô tả

Duyệt PR, issues, và project board ngay trong Orca — không cần context switch sang browser. Tạo worktree từ issue/task, annotate AI diff, và auto-generate PR/commit message bằng AI.

---

## Vấn đề cần giải quyết

Developers mất nhiều thời gian chuyển đổi giữa Orca (IDE), GitHub (review PR), và Linear (task management). Mỗi lần switch context làm gián đoạn flow làm việc. Ngoài ra, sau khi agent tạo code, việc review diff và gửi feedback phải làm thủ công qua nhiều bước.

---

## Providers được hỗ trợ

| Provider | Tính năng |
|---------|----------|
| **GitHub** | PR, Issues, Checks, Projects, Actions |
| **GitLab** | MR, Issues, Pipelines |
| **Linear** | Issues, Projects, Cycles, Teams |
| **Gitea** | Repos, Issues, PRs |
| **Bitbucket** | PRs, Issues |
| **Azure DevOps** | Work Items, PRs |
| **Jira** | Issues (read) |

---

## Tính năng chi tiết

### GitHub Integration

**Pull Requests:**
- Danh sách PR với filter (open/closed, assignee, label, reviewer)
- PR detail: title, body, CI status, reviews
- Xem diff per file với syntax highlighting
- Comment trên PR (line comment, review comment)
- Merge PR với các phương thức: merge commit, squash, rebase
- Auto-merge settings

**Issues:**
- Browse và filter issues
- Issue detail với comments
- Tạo worktree từ issue (auto-create branch)

**GitHub Projects:**
- Xem project board (Kanban/Table view)
- Update issue status từ trong Orca

**Authentication:**
- OAuth flow
- Personal Access Token (PAT)
- GitHub App

### Linear Integration

**Issues:**
- List issues với filter (assignee, status, priority, cycle)
- Issue detail với description, comments, attachments
- Tạo worktree từ issue
- Update issue status: In Progress → In Review → Done
- Inline media rendering

**Projects & Cycles:**
- Xem project list và cycle progress

**Agent Access:**
- Linear context được cung cấp cho agent khi tạo worktree từ issue

### Annotate AI Diff

- Xem diff của thay đổi agent tạo ra
- Click vào bất kỳ dòng nào để thêm inline comment
- Comment được format với context (file, line number, original code)
- Gửi tất cả comment về agent terminal
- Agent điều chỉnh code dựa trên feedback

### AI Commit Message Generation

- Thu thập staged changes (git diff --staged)
- Thu thập context: recent commits, branch name, PR description
- Build prompt và stream response từ AI model
- Người dùng review và chỉnh sửa trước khi commit
- Tuân thủ commit convention của dự án (nếu có)

### AI Pull Request Generation

- Auto-generate PR title và description từ commits
- Include summary của thay đổi
- Suggest reviewers dựa trên code ownership

---

## Luồng người dùng

```
[Tạo worktree từ Linear issue]
1. Mở Linear panel trong Orca
2. Click vào issue "Fix login bug"
3. Click "Create Worktree" → Orca tạo branch + worktree
4. Agent được khởi động với issue description làm context
5. Agent làm việc → issue status tự động → In Progress

[Review và annotate AI diff]
6. Agent hoàn thành → xem diff trong Orca
7. Click vào dòng có vấn đề → nhập comment "Cần handle null case"
8. Click "Send to Agent"
9. Agent nhận feedback và sửa code

[Tạo PR]
10. Review final → click "Create PR"
11. AI generate title và description
12. Submit PR lên GitHub
```

---

## Tiêu chí chấp nhận

- [ ] Xem và review PR mà không cần rời Orca
- [ ] Annotate diff và gửi feedback về agent hoạt động
- [ ] Commit message generation hoàn thành trong < 10 giây
- [ ] Tạo worktree từ GitHub issue tạo đúng branch name
- [ ] GitLab MR được hỗ trợ tương đương GitHub PR

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **GitHub client** | `src/main/github/client.ts` (~162K bytes) |
| **GitLab client** | `src/main/gitlab/` |
| **Linear SDK** | `@linear/sdk` v82.1.0 |
| **Commit message** | `src/shared/commit-message-agent-spec.ts` |
| **PR generation** | `src/shared/pull-request-generation.ts` |
| **Diff comments** | `src/shared/diff-comments-format.ts` |
| **Hosted review** | `src/shared/hosted-review.ts` |
| **GitHub rate limit** | `src/main/github/rate-limit.ts` |

---

## Metrics

| KPI | Target |
|----|-------|
| Commit message generation | < 10 giây |
| PR load time | < 2 giây |
| GitHub API rate limit handling | Auto-retry với backoff |
