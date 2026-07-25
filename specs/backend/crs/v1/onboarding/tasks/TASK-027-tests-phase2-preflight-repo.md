# TASK-027: Viết Unit Tests — Phase 2 (Preflight + Remote Repo)

**Phase:** 2 — Verification  
**Solution:** [SOL-004-005-006](../solutions/SOL-004-005-006-platform-preflight-repo.md) Tests section  
**Depends on:** TASK-022, TASK-025  
**Blocks:** (không — verification task)

---

## Mục tiêu

Viết unit tests cho các handlers Phase 2: preflight status, git identity, và remote repo operations.

---

## Files cần tạo

1. `src/main/ipc/__tests__/onboarding-preflight.test.ts`
2. `src/main/ipc/__tests__/repo-remote.test.ts`

---

## Test cases cần implement

### `onboarding-preflight.test.ts`

```typescript
describe('onboarding.getPreflightStatus', () => {
  it('cache miss → gọi relay, lưu cache')
  it('cache hit (<30s) → không gọi relay')
  it('force: true → bỏ qua cache, gọi relay lại')
  it('relay không kết nối → throw Error')
  it('gh installed + authenticated → { installed: true, authenticated: true }')
  it('gh installed + not authenticated → { installed: true, authenticated: false }')
  it('gh not installed → { installed: false, authenticated: false }')
  it('git installed, có identity → { installed: true, hasUserName: true, hasUserEmail: true }')
  it('git installed, chưa có email → { hasUserEmail: false }')
})

describe('onboarding.setGitIdentity', () => {
  it('gọi preflight.setGitIdentity trên relay với đúng name + email')
  it('invalidate preflight cache sau khi set thành công')
  it('relay không connected → throw Error')
})

describe('onboarding.detectGhosttyConfig', () => {
  it('forward đến relay, trả về configPath + themeDir')
  it('relay không connected → throw Error')
})
```

### `repo-remote.test.ts`

```typescript
describe('repo.listRemoteDirectory', () => {
  it('trả về directories trên dev server path')
  it('includeGitStatus = true → đánh dấu git repos')
  it('path không tồn tại → throw Error')
  it('relay không connected → throw Error')
})

describe('repo.cloneRemote', () => {
  it('gọi git.clone trên relay với url + targetPath')
  it('add repo vào store sau khi clone thành công')
  it('targetDir mặc định = devServer.workspaceDir + repoName')
  it('targetDir được cung cấp → dùng targetDir')
})

describe('repo.scanRemote', () => {
  it('chỉ trả về entries có isGitRepo = true')
  it('0 git repos → trả về []')
})
```

---

## Acceptance Criteria

- [x] Tất cả test cases được implement (không để empty)
- [x] Mock relay calls không cần SSH thật
- [x] Tests cho cache behavior dùng `jest.useFakeTimers()`
- [x] Tất cả tests pass
