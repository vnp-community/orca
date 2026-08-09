# SOL-AG-HLD-012 — Xoá `ai-provider-handler.ts` (Dead Code, Comment Sai Về Mã Hoá)

**Fixes:** [BUG-AG-HLD-012](../BUG-AG-HLD-012-ai-provider-handler-dead-code-false-encryption-claim.md)
**TDD Ref:** TDD-AG-09 (toàn bộ — implementation đúng, đang chạy thật, là `agent-credential-store.ts`); TDD-AG-09 "v2.1 Integration Note" xác nhận source file chính thức là `src/relay/agent-credential-store.ts`, không phải `ai-provider-handler.ts`
**File:** `agent/src/relay/ai-provider-handler.ts` (xoá), `agent/src/relay/__tests__/ai-provider-handler.test.ts` (xoá)
**Effort:** 1 giờ
**Status:** 🔴 TODO

---

## Phân Tích

`ai-provider-handler.ts` export `aiProviderHandlers` — một implementation credential-store song song với `agent-credential-store.ts` (bản đang chạy thật, được wire vào `agent-rpc-dispatch.ts` cho toàn bộ 4 case `ai.provider.writeCredential`/`readCredential`/`healthCheck`/`deleteCredential`).

Xác nhận bằng GitNexus trước khi đề xuất xoá:

```
impact({target:"aiProviderHandlers", file_path:"agent/src/relay/ai-provider-handler.ts", direction:"upstream"})
→ impactedCount: 0, risk: LOW, byDepth: {}

grep "ai-provider-handler" toàn repo (ngoài chính nó + test file):
  → 0 kết quả trong agent/src, desktop/src, backend/src
```

Khớp đúng với bằng chứng trong bug report: **0 caller**. `agent-rpc-dispatch.ts` dùng dynamic `import('./agent-credential-store')` cho mọi case `ai.provider.*` (dòng 356-408) — không có bất kỳ case nào import `./ai-provider-handler`. An toàn để xoá hoàn toàn, không có blast radius.

**Vấn đề nghiêm trọng của việc giữ lại file này ở dạng hiện tại (kể cả khi dead code):** comment đầu file (dòng 7) tuyên bố *"Credentials are stored AES-256-GCM encrypted in ~/.orca/ai-providers/<accountId>.enc"*, nhưng thân hàm `'ai.provider.writeCredential'` (dòng 30-43) chỉ `JSON.stringify({ encryptedBlob, iv, updatedAt })` rồi `writeFile(...)` thẳng — **không hề gọi bất kỳ hàm mã hoá nào ở tầng agent** (khác hẳn `agent-credential-store.ts` thật, có `encryptPayload()`/scrypt/AES-256-GCM double-encryption đúng chuẩn — xem `agent-credential-store.ts:64-80`). Path `~/.orca/ai-providers/` cũng trùng khớp với tài liệu cũ (F35/BL-AIP-01), dễ khiến người review tưởng nhầm đây là bản đúng nếu sau này bị wire lại.

**Quyết định:** Xoá hẳn, không hợp nhất/sửa comment — vì `agent-credential-store.ts` đã là bản đúng, đang chạy thật, và đã có test coverage riêng (`agent-credential-store.test.ts`). Giữ lại một file dead-code chỉ để "sửa comment" vẫn để lại rủi ro bị wire nhầm trong tương lai; xoá triệt để loại bỏ rủi ro đó thay vì giảm thiểu nó.

---

## Thay Đổi Cần Thực Hiện

### Bước 1 — Xác nhận lại 0 caller ngay trước khi xoá (double-check, tránh false negative do index cũ)

```bash
cd /opt/repos/orca
grep -rn "ai-provider-handler\|aiProviderHandlers\|AIProviderHandlerName" \
  --include="*.ts" --include="*.tsx" agent/ desktop/ backend/ frontend/ \
  | grep -v "agent/src/relay/ai-provider-handler.ts" \
  | grep -v "agent/src/relay/__tests__/ai-provider-handler.test.ts"
```

Kỳ vọng: **không có kết quả nào** trong `agent/` (file `desktop/src/relay/ai-provider-handler.ts` là một bản riêng ở package `desktop/`, ngoài scope bug này — không đụng tới).

Chạy lại GitNexus để chắc chắn index hiện hành khớp:

```
impact({target:"aiProviderHandlers", file_path:"agent/src/relay/ai-provider-handler.ts", direction:"upstream", repo:"orca"})
→ phải vẫn ra impactedCount: 0
```

Nếu có bất kỳ caller nào xuất hiện (index đã đổi kể từ audit), **dừng lại** — không xoá, quay lại điều tra thay vì làm theo kế hoạch này.

### Bước 2 — Xoá file chính

```bash
git rm agent/src/relay/ai-provider-handler.ts
```

### Bước 3 — Xoá test file tương ứng (chỉ test dead code, không còn giá trị sau khi xoá handler)

```bash
git rm agent/src/relay/__tests__/ai-provider-handler.test.ts
```

### Bước 4 — Rà soát file liên quan không nên đụng tới

`agent/src/shared/ai-credential-contract.ts` — đây là **type contract** dùng chung (payload shape của `encryptedBlob`/`iv`/`algorithm`), không phải implementation. Kiểm tra xem nó có import gì từ `ai-provider-handler.ts` không:

```bash
grep -n "ai-provider-handler" agent/src/shared/ai-credential-contract.ts
```

Nếu không có (dự kiến — vì đây là type-only contract), **giữ nguyên file này**, không xoá.

### Bước 5 — Build/lint để bắt import chết sót lại

```bash
cd agent
npm run typecheck    # hoặc lệnh build/tsc tương ứng của package agent/
npx oxlint src/relay  # bắt unused-import nếu còn sót đâu đó
```

Nếu build/typecheck fail vì thiếu import `ai-provider-handler`, đó là bằng chứng có caller ẩn mà bước 1 bỏ sót — điều tra lại trước khi tiếp tục, không suppress lỗi.

---

## Verification

```bash
cd /opt/repos/orca/agent
npx vitest run src/relay/__tests__/agent-credential-store.test.ts   # bản đúng vẫn pass nguyên vẹn
npx vitest run                                                       # toàn bộ suite agent/ — không còn tham chiếu file đã xoá
npm run typecheck
```

```bash
cd /opt/repos/orca
git status --porcelain agent/src/relay/ai-provider-handler.ts agent/src/relay/__tests__/ai-provider-handler.test.ts
# kỳ vọng: cả 2 hiện "D " (deleted)
```

GitNexus sau khi xoá (bắt buộc theo AGENTS.md — chạy trước khi commit):

```
detect_changes({scope: "compare", base_ref: "main"})
→ kỳ vọng: chỉ 2 file bị xoá, 0 symbol/execution-flow nào khác bị ảnh hưởng
  (khớp với impactedCount:0 đã xác nhận ở Phân Tích)
```

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `agent/src/relay/ai-provider-handler.ts` | Xoá — dead code, comment sai về mã hoá |
| `agent/src/relay/__tests__/ai-provider-handler.test.ts` | Xoá — test của dead code |
| `agent/src/relay/agent-credential-store.ts` | Bản đúng, đang chạy thật — không đổi |
| `agent/src/relay/agent-rpc-dispatch.ts` | Xác nhận không import `ai-provider-handler` ở bất kỳ case nào — không đổi |
| `agent/src/shared/ai-credential-contract.ts` | Type contract dùng chung — giữ nguyên, không phải implementation |
| `desktop/src/relay/ai-provider-handler.ts` | Bản riêng ở package `desktop/` — ngoài scope, không đụng tới trong fix này |
