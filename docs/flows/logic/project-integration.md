# Luồng Dữ liệu — Project Integration

**Domain:** Project Integration (GitHub/GitLab/Linear)  
**Nghiệp vụ:** BL-PI-01 → BL-PI-04  
**Kiến trúc tham chiếu:** HLD v1 — C3.9, C4.6, F30 Remote Integrations, ADR-006

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Renderer (React UI) | UI | Issues list, task import, PR review panel |
| Main Process | Business Logic | GitHubService, GitLabService, LinearService |
| GitHub REST/GraphQL API | External | Issues, PRs, reviews |
| GitLab REST API | External | Issues, MRs |
| Linear REST API | External | Tasks, projects |
| WebCredentialStore | Secrets | AES-256-GCM token storage |
| SQLite Database | Persistence | Cached issues, PR records |

---

## BL-PI-01 — Import GitHub/GitLab Issues

```
Người dùng (Maya/Alex)
    │
    ▼
[Renderer] mở "Issues" panel → click "Sync Issues"
    │ contextBridge.invoke('issues.sync', { provider: 'github', repoUrl })
    ▼
[Main Process — GitHubService.fetchIssues()]
    ├─ Load token: WebCredentialStore.get('github', userId)  ← AES-256-GCM decrypt
    ├─ GitHub GraphQL: query { repository { issues(states: OPEN) { nodes { id, title, body, labels } } } }
    │   Headers: Authorization: Bearer <token>
    ├─ Transform response → Issue[] objects
    ├─ UPSERT orca_issues (cache)   ← SQLite
    └─ emit: issues:synced { count }
    │
    ▼
[Renderer] issues list render với filter/search

GitLab variant:
    Main → GitLab REST: GET /projects/:id/issues?state=opened
         → parse → UPSERT SQLite

Linear variant:
    Main → Linear GraphQL: query issues(filter: { state: { type: { eq: "started" } } })
         → parse → UPSERT SQLite

Luồng:
User → Renderer → IPC → Main → WebCredentialStore (load token)
                              → GitHub/GitLab/Linear API (fetch)
                              → SQLite (UPSERT cache)
                              → Renderer (list render)
```

---

## BL-PI-02 — Tạo Worktree từ Issue/Task

```
Người dùng (Maya/Alex)
    │
    ▼
[Renderer] click "Create Worktree" trên issue card
    │ contextBridge.invoke('worktree.createFromIssue', { issueId, provider })
    ▼
[Main Process — ProjectIntegrationService.createWorktreeFromIssue()]
    ├─ Load issue data: SELECT FROM orca_issues WHERE id=?   ← SQLite
    ├─ Generate branch name: "feature/issue-123-fix-login-bug"
    ├─ BL-WT-01: git worktree add (với branch mới)           ← Git CLI + SQLite
    ├─ UPDATE orca_worktrees SET issueId=?                   ← SQLite
    ├─ BL-AG-01: spawn agent với issue context:
    │   inject prompt: "Working on issue #123: <title>\n<body>"
    └─ emit: worktree:createdFromIssue { worktreeId, issueId }
    │
    ▼
[Renderer] worktree card với issue reference badge

Luồng:
User → Renderer → IPC → Main → SQLite (load issue)
                              → Git CLI (create branch + worktree)
                              → SQLite (link issue to worktree)
                              → Daemon → PTY → Agent (inject issue context)
```

---

## BL-PI-03 — Cập nhật Trạng thái Issue

```
Người dùng (Maya) — sau khi worktree có PR
    │
    ▼
[Renderer] click "Update Issue Status" → chọn status mới
    │ contextBridge.invoke('issue.updateStatus', { issueId, status, provider })
    ▼
[Main Process — GitHubService.updateIssueStatus()]

GitHub:
    ├─ REST API: PATCH /repos/{owner}/{repo}/issues/{number}
    │   Body: { state: 'closed' } hoặc { labels: ['in-review'] }
    └─ UPDATE orca_issues SET status=?   ← SQLite

Linear:
    ├─ GraphQL mutation: issueUpdate(id, { stateId: <new_state_id> })
    └─ UPDATE orca_issues SET status=?   ← SQLite
    │
    ▼
[Renderer] issue card status cập nhật

Luồng:
User → Renderer → IPC → Main → GitHub/Linear API (PATCH/mutation)
                              → SQLite (UPDATE status cache)
                              → Renderer (status update)
```

---

## BL-PI-04 — Submit PR Review lên GitHub

```
Người dùng (Maya)
    │
    ▼
[Renderer] click "Submit Review" sau khi annotate (BL-CR-02)
    │ contextBridge.invoke('pr.submitReview', { prNumber, annotations[], verdict })
    ▼
[Main Process — GitHubService.submitReview()]
    ├─ Load GitHub token: WebCredentialStore.get('github', userId)
    ├─ Build review body từ annotations:
    │   { body: "Overall feedback...", event: 'APPROVE'|'REQUEST_CHANGES'|'COMMENT',
    │     comments: [{ path, line, body }] }
    ├─ GitHub REST: POST /repos/{owner}/{repo}/pulls/{number}/reviews
    └─ emit: review:submitted { prNumber, verdict }
    │
    ▼
[Renderer] confirmation toast + PR status cập nhật

Luồng:
User → Renderer → IPC → Main → WebCredentialStore (load token)
                              → GitHub REST API (POST /reviews)
                              → SQLite (UPDATE pr status)
                              → Renderer (success toast)
```

---

## Sơ đồ tổng quan — Project Integration

```
┌─────────────┐   IPC   ┌──────────────────────────────────────┐
│  Renderer   │◄───────►│  Main Process                        │
│  Issues list│         │  GitHubService                       │
│  PR panel   │         │  GitLabService                       │
│  Status     │         │  LinearService                       │
└─────────────┘         └───┬──────────────┬───────────────────┘
                             │              │
                    ┌────────▼──┐  ┌────────▼────────────────────┐
                    │  SQLite   │  │  External APIs               │
                    │  issues   │  │  GitHub REST/GraphQL         │
                    │  worktree │  │  GitLab REST                 │
                    │  pr cache │  │  Linear GraphQL              │
                    └─────┬─────┘  └─────────────────────────────┘
                          │                    ▲
                    ┌─────▼──────┐             │ Bearer token
                    │ WebCredential│←──────────┘
                    │ Store       │  (AES-256-GCM)
                    └────────────┘
```
