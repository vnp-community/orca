# TASK-BE-014.2: Instrument `GitProviderCredentialService` (BL-INT-01, phần Main) với tracing

**Phase:** 2
**SOL Ref:** [SOL-BE-TRACE-014](../solutions/SOL-BE-TRACE-014-remote-integration.md) §2.2
**CR Ref:** [CR-TRACE-014](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-014-remote-integration.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-014.1
**Status:** ✅ Done (2026-08-04) — `getGitHubPAT`/`getGitLabPAT` instrumented with try/catch span per doc. Drift: real code had no try/catch and `getGitLabPAT`'s `projectId` param was already `_projectId` (unused) — kept as-is, only added tracing. `pnpm run typecheck:node` clean for this file.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "GitProviderCredentialService.getGitHubPAT"
codegraph explore "GitProviderCredentialService.getGitLabPAT"
```

Cả 2 là method đã tồn tại (MODIFY case), xử lý credential nhạy cảm. Chạy:

```
gitnexus_impact({ target: "GitProviderCredentialService.getGitHubPAT", direction: "upstream" })
gitnexus_impact({ target: "GitProviderCredentialService.getGitLabPAT", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa — đặc biệt xác nhận không có caller nào phụ thuộc việc method này KHÔNG throw (nếu có, giữ nguyên hành vi throw). Tuyệt đối không đưa token/PAT giải mã vào bất kỳ trace field nào. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc `getGitHubPAT()`/`getGitLabPAT()` trong `src/main/project/GitProviderCredentialService.ts` bằng `Tracers.remoteIntegrationCredentialDecryptFlow`, với `step('decrypt')` bao quanh lời gọi `store.getToken(...)`. **KHÔNG sửa `WebCredentialStore.getToken()` chính nó** — chỉ instrument ở call site để tránh đổi shared low-level API. Lưu ý cả `getGitHubPAT`/`getGitLabPAT` đều "mượn" slot credential — `github` dùng slot `'bitbucket'`, `gitlab` dùng slot `'gitea'` (do `WebCredentialStore` chỉ hỗ trợ enum cố định, xem `setGitHubPAT`/`setGitLabPAT` không đổi trong file này) — giữ nguyên hành vi mượn slot này, chỉ thêm tracer.

## File: `src/main/project/GitProviderCredentialService.ts` [MODIFY]

Dùng bản có `try/catch` (khuyến nghị dùng khi implement thật — bọc cả decrypt fail lẫn found/not-found trong 1 span hoàn chỉnh):

```typescript
// src/main/project/GitProviderCredentialService.ts
import type { WebCredentialStore } from '../credentials/web-credential-store'
import { Tracers } from '../../shared/trace/tracers'

export class GitProviderCredentialService {
  constructor(
    private readonly getUserStore: (userId: string) => WebCredentialStore
  ) {}

  // ── GitHub ──────────────────────────────────────────────────────────────────

  async setGitHubPAT(userId: string, token: string): Promise<void> {
    const store = this.getUserStore(userId)
    await store.setToken('bitbucket', token, { provider: 'github', userId })
    // Note: reusing 'bitbucket' slot for github since WebCredentialStore is per-userId
  }

  async getGitHubPAT(userId: string): Promise<string | null> {
    const span = Tracers.remoteIntegrationCredentialDecryptFlow.start({ provider: 'github', userId })
    const store = this.getUserStore(userId)
    span.step('decrypt', { provider: 'github' })
    // FIX-note: slot 'bitbucket' tái dùng cho github (xem setGitHubPAT ở trên) —
    // KHÔNG đưa giá trị `token` trả về vào bất kỳ field nào của span.
    try {
      const token = await store.getToken('bitbucket')
      span.ok({ provider: 'github', found: token !== null })
      return token
    } catch (err) {
      span.fail(err, { provider: 'github' })
      throw err
    }
  }

  async deleteGitHubPAT(userId: string): Promise<void> {
    const store = this.getUserStore(userId)
    await store.deleteToken('bitbucket')
  }

  // ── GitLab ──────────────────────────────────────────────────────────────────

  async setGitLabPAT(userId: string, projectId: string, token: string): Promise<void> {
    const store = this.getUserStore(userId)
    await store.setToken('gitea', token, { provider: 'gitlab', userId, projectId })
  }

  async getGitLabPAT(userId: string, projectId: string): Promise<string | null> {
    const span = Tracers.remoteIntegrationCredentialDecryptFlow.start({ provider: 'gitlab', userId })
    const store = this.getUserStore(userId)
    span.step('decrypt', { provider: 'gitlab' })
    try {
      const token = await store.getToken('gitea')
      span.ok({ provider: 'gitlab', found: token !== null })
      return token
    } catch (err) {
      span.fail(err, { provider: 'gitlab' })
      throw err
    }
  }

  async deleteGitLabPAT(userId: string, projectId: string): Promise<void> {
    const store = this.getUserStore(userId)
    await store.deleteToken('gitea')
  }
}
```

**Ràng buộc bắt buộc (bảo mật, tuyệt đối):** giá trị token/PAT đã giải mã trả về từ `store.getToken(...)` **KHÔNG BAO GIỜ** được đưa vào bất kỳ `TraceFields` nào của `remoteIntegrationCredentialDecryptFlow`. `TraceFields` hiển thị plaintext trong console/TracePanel không có redaction tự động — đưa token vào field là lộ secret trực tiếp. Chỉ các field sau được phép: `provider` (`'github'|'gitlab'`), `userId`, `found` (boolean). Không field nào khác.

- Không sửa `setGitHubPAT`/`deleteGitHubPAT`/`setGitLabPAT`/`deleteGitLabPAT` — chỉ `getGitHubPAT`/`getGitLabPAT` được instrument (theo gap analysis của SOL-BE-TRACE-014 §1.2, chỉ 2 method "đọc" này nằm trong scope BL-INT-01).
- Không sửa `WebCredentialStore.getToken()` (`src/main/credentials/web-credential-store.ts`).

## Verification

```bash
pnpm run typecheck:node
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `getGitHubPAT()` bọc bằng `Tracers.remoteIntegrationCredentialDecryptFlow` với `provider:'github'`, có `step('decrypt')` trước khi gọi `store.getToken('bitbucket')`
- [ ] `getGitLabPAT()` bọc tương tự với `provider:'gitlab'`, `store.getToken('gitea')`
- [ ] `remoteIntegration:credentialDecrypt` không bao giờ chứa giá trị token/PAT plaintext trong bất kỳ field nào — chỉ `provider`/`userId`/`found`
- [ ] Khi `store.getToken()` throw, `span.fail(err, {provider})` được gọi trước khi re-throw (lỗi không bị nuốt)
- [ ] `setGitHubPAT`/`deleteGitHubPAT`/`setGitLabPAT`/`deleteGitLabPAT` không bị sửa
- [ ] `WebCredentialStore.getToken()` không bị sửa
- [ ] `pnpm run typecheck:node` pass, không lỗi mới
