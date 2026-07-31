# TASK-03: Mở rộng orca-relay `preflight.check` thêm `glab` support

**Status:** ✅ DONE — 2026-07-25 (AC verified 2026-07-25)  
**Phase:** 2 — Relay Extension  
**Priority:** 🔴 Critical  
**Depends on:** Không có (relay code độc lập)  
**Solution:** SOL-01-CLI-Preflight.md  
**CRs:** CR-GH-001, CR-INT-001  
**Estimated effort:** ~30 phút

---

## Mục tiêu

`preflight.check` trên relay (`src/relay/preflight-handler.ts`) **đã** kiểm tra `gh` và `git`. Cần **bổ sung `glab`** vào kết quả để Orca Server có thể biết status GitLab CLI khi proxy qua relay.

---

## Hiện trạng code

**File:** `src/relay/preflight-handler.ts` — `checkFullPreflight()` (line 197–211):

```typescript
private async checkFullPreflight(): Promise<{
  platform: NodeJS.Platform
  gh: { installed: boolean; authenticated: boolean; version?: string }
  git: { installed: boolean; version?: string; hasUserName: boolean; hasUserEmail: boolean }
}> {
  const [ghResult, gitResult] = await Promise.all([
    this.checkGhCli(),
    this.checkGitCli()
  ])
  return {
    platform: process.platform,
    gh: ghResult,
    git: gitResult
  }
}
```

---

## Các bước thực thi

### Bước 1: Thêm `checkGlabCli()` method vào `PreflightHandler`

Thêm method mới sau `checkGhCli()` (sau line 229):

```typescript
private async checkGlabCli(): Promise<{
  installed: boolean
  authenticated: boolean
  version?: string
}> {
  try {
    const { stdout: version } = await execFileAsync('glab', ['--version'])
    try {
      await execFileAsync('glab', ['auth', 'status'])
      return { installed: true, authenticated: true, version: version.trim() }
    } catch {
      return { installed: true, authenticated: false, version: version.trim() }
    }
  } catch {
    return { installed: false, authenticated: false }
  }
}
```

### Bước 2: Sửa `checkFullPreflight()` để include `glab` result

Sửa method `checkFullPreflight()` (line 197–211):

```typescript
private async checkFullPreflight(): Promise<{
  platform: NodeJS.Platform
  gh: { installed: boolean; authenticated: boolean; version?: string }
  glab: { installed: boolean; authenticated: boolean; version?: string }  // THÊM
  git: { installed: boolean; version?: string; hasUserName: boolean; hasUserEmail: boolean }
}> {
  const [ghResult, glabResult, gitResult] = await Promise.all([  // THÊM glabResult
    this.checkGhCli(),
    this.checkGlabCli(),  // THÊM
    this.checkGitCli()
  ])
  return {
    platform: process.platform,
    gh: ghResult,
    glab: glabResult,  // THÊM
    git: gitResult
  }
}
```

---

## Tests cần thêm: `src/relay/preflight-handler.test.ts`

Tìm test file hiện tại và thêm test cases cho `glab`:

```typescript
describe('glab CLI detection', () => {
  it('reports glab installed and authenticated when glab auth status succeeds')
  it('reports glab installed but not authenticated when auth status fails')
  it('reports glab not installed when glab binary not found')
})
```

---

## Acceptance Criteria

1. ✅ `preflight.check` call tới relay trả về object có field `glab: { installed, authenticated }`
2. ✅ Test passes với mock cho `glab` binary (3 test cases added: authenticated/not-auth/not-installed)
3. ✅ Khi `glab` không được cài trên Dev Server: `glab.installed = false`, không throw error
4. ✅ Build relay thành công: `pnpm build:relay`

---

## Files cần sửa

- `src/relay/preflight-handler.ts` — thêm `checkGlabCli()` + sửa `checkFullPreflight()`
- `src/relay/preflight-handler.test.ts` — thêm test cases cho `glab`

---

## Lưu ý quan trọng

- Relay đã có `preflight.check` handler. **KHÔNG** tạo endpoint mới `preflight.check.cli` như đề xuất trong SOL-01 — relay code hiện tại đã đủ tốt, chỉ cần bổ sung `glab`.
- Pattern `checkGhCli()` và `checkGlabCli()` hoàn toàn đối xứng nhau → copy-paste và đổi binary name.
