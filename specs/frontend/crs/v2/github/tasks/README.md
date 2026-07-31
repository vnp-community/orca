# Frontend Tasks — CR v2/github (Web Server Mode)

> **Phiên bản:** 1.0 | **Ngày:** 2026-07-25  
> **Solutions:** `specs/frontend/crs/v2/github/solutions/`  
> **TDD tham chiếu:** `specs/frontend/tdd/`

---

## Task Index

| Task | Solution | File | Status | Mô tả |
|------|----------|------|--------|-------|
| [FE-TASK-01](./FE-TASK-01-preload-api-types.md) | FE-SOL-02, FE-SOL-03 | `api-types.ts` | ✅ DONE & 🧪 AC Verified | Thêm `credentials`, `github`, `gitlab` vào `PreloadApi` |
| [FE-TASK-02](./FE-TASK-02-web-preload-github-gitlab.md) | FE-SOL-02 | `web-preload-api.ts` | ✅ DONE & 🧪 AC Verified | Expose `github.*` và `gitlab.*` auth methods |
| [FE-TASK-03](./FE-TASK-03-webmode-cli-auth-section.md) | FE-SOL-02 | `WebModeCliAuthSection.tsx` | ✅ DONE & 🧪 AC Verified | Component PTY auth login cho GitHub/GitLab |
| [FE-TASK-04](./FE-TASK-04-cli-cards-webmode-branch.md) | FE-SOL-02 | `cli-source-control-integration-cards.tsx` | ✅ DONE & 🧪 AC Verified | GitHub/GitLab cards: phân nhánh web mode |
| [FE-TASK-05](./FE-TASK-05-credential-input-form.md) | FE-SOL-03 | `CredentialInputForm.tsx` | ✅ DONE & 🧪 AC Verified | Form nhập credentials + `useCredentialManager` hook |
| [FE-TASK-06](./FE-TASK-06-token-cards-credential-form.md) | FE-SOL-03 | `token-source-control-integration-cards.tsx` | ✅ DONE & 🧪 AC Verified | Bitbucket/AzureDevOps/Gitea cards: web mode form |
| [FE-TASK-07](./FE-TASK-07-task-tracker-credential-form.md) | FE-SOL-03 | `task-tracker-integration-cards.tsx`, `jira-integration-card.tsx` | ✅ DONE & 🧪 AC Verified | Linear/Jira cards: web mode credential form |
| [FE-TASK-08](./FE-TASK-08-remote-preflight-type.md) | FE-SOL-04 | `dev-server-types.ts` | ✅ DONE & 🧪 AC Verified | Mở rộng `RemotePreflightStatus` thêm `glab` field |
| [FE-TASK-09](./FE-TASK-09-preflight-card-merge.md) | FE-SOL-04 | `source-control-preflight-card-status.ts` | ✅ DONE & 🧪 AC Verified | `mergePreflightStatuses()` — ưu tiên remote cho CLI |
| [FE-TASK-10](./FE-TASK-10-preflight-response-update.md) | FE-SOL-04 | `preflight.ts` (slice) | ✅ DONE & 🧪 AC Verified | Cập nhật `remotePreflightByServer` khi nhận preflight response |



---

## Thứ tự thực thi

```
Nhóm 1 — Infrastructure (ĐÃ DONE):
  FE-TASK-01 → PreloadApi type extension
  FE-TASK-02 → web-preload-api github/gitlab methods
  FE-TASK-03 → WebModeCliAuthSection component
  FE-TASK-04 → CLI integration cards web mode branch
  FE-TASK-05 → CredentialInputForm + useCredentialManager

Nhóm 2 — Credential UI (CẦN LÀM):
  FE-TASK-06 → Bitbucket/AzureDevOps/Gitea credential forms
  FE-TASK-07 → Linear/Jira credential forms

Nhóm 3 — Remote Preflight (CẦN LÀM):
  FE-TASK-08 → RemotePreflightStatus.glab field
  FE-TASK-09 → mergePreflightStatuses hook
  FE-TASK-10 → Preflight response → remotePreflightByServer
```

---

## Acceptance Criteria Tổng hợp

- [ ] GitHub card trong Web mode hiển thị "Login with GitHub CLI" khi `not-authenticated`
- [ ] GitLab card trong Web mode hiển thị "Login with GitLab CLI" khi `not-authenticated`
- [ ] Bitbucket card trong Web mode hiển thị form nhập token/email khi `not-configured`
- [ ] Azure DevOps card trong Web mode hiển thị form nhập PAT/baseUrl
- [ ] Gitea card trong Web mode hiển thị form nhập token/baseUrl
- [ ] Linear card trong Web mode hiển thị form nhập API key
- [ ] Jira card trong Web mode hiển thị form nhập token/email/baseUrl
- [ ] Relay preflight response cập nhật `remotePreflightByServer[devServerId]`
- [ ] GitHub/GitLab cards đọc `gh`/`glab` status từ relay (không phải local Orca Server)
- [ ] TypeScript 0 lỗi mới trong tất cả files
