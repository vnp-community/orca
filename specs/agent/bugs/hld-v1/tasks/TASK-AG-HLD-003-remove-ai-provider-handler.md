# TASK-AG-HLD-003 — Xoá `ai-provider-handler.ts` (Dead Code, Comment Sai Về Mã Hoá)

**Solution:** [SOL-AG-HLD-012](../solutions/SOL-AG-HLD-012-remove-dead-ai-provider-handler.md)
**Bug:** [BUG-AG-HLD-012](../BUG-AG-HLD-012-ai-provider-handler-dead-code-false-encryption-claim.md)
**File:** `agent/src/relay/ai-provider-handler.ts` (xoá), `agent/src/relay/__tests__/ai-provider-handler.test.ts` (xoá)
**Phụ thuộc:** —
**Estimated:** 60 phút (1 giờ theo SOL-AG-HLD-012)
**Status:** ✅ DONE — 2026-08-09 (code + typecheck verified; vitest không chạy được trong môi trường này — xem ghi chú cuối file)

---

## Mục Tiêu

Xoá hẳn `ai-provider-handler.ts` (0 caller, comment sai tuyên bố mã hoá AES-256-GCM nhưng thực chất không mã hoá gì) và test file tương ứng — bản đúng đang chạy thật là `agent-credential-store.ts`.

---

## Context

Đọc trước:
- `agent/src/relay/ai-provider-handler.ts` — toàn bộ file (dead code cần xoá)
- `agent/src/relay/agent-rpc-dispatch.ts` — xác nhận các case `ai.provider.writeCredential`/`readCredential`/`healthCheck`/`deleteCredential` chỉ dùng dynamic `import('./agent-credential-store')`, không import `./ai-provider-handler`
- `agent/src/relay/agent-credential-store.ts` — bản đúng, đang chạy thật, có test coverage riêng (`agent-credential-store.test.ts`), KHÔNG đụng tới
- `agent/src/shared/ai-credential-contract.ts` — type contract dùng chung, cần rà soát để đảm bảo không import từ `ai-provider-handler.ts`

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

Kỳ vọng: **không có kết quả nào** trong `agent/` (file `desktop/src/relay/ai-provider-handler.ts` là một bản riêng ở package `desktop/`, ngoài scope task này — KHÔNG đụng tới).

Nếu dùng GitNexus, chạy lại để chắc chắn index hiện hành khớp:

```
impact({target:"aiProviderHandlers", file_path:"agent/src/relay/ai-provider-handler.ts", direction:"upstream", repo:"orca"})
→ phải vẫn ra impactedCount: 0
```

> [!IMPORTANT]
> Nếu có bất kỳ caller nào xuất hiện (index đã đổi kể từ audit), **DỪNG LẠI** — không xoá, quay lại điều tra thay vì làm theo kế hoạch này.

### Bước 2 — Xoá file chính

```bash
git rm agent/src/relay/ai-provider-handler.ts
```

### Bước 3 — Xoá test file tương ứng (chỉ test dead code, không còn giá trị sau khi xoá handler)

```bash
git rm agent/src/relay/__tests__/ai-provider-handler.test.ts
```

### Bước 4 — Rà soát file liên quan không nên đụng tới

`agent/src/shared/ai-credential-contract.ts` là **type contract** dùng chung (payload shape của `encryptedBlob`/`iv`/`algorithm`), không phải implementation. Kiểm tra xem nó có import gì từ `ai-provider-handler.ts` không:

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

Nếu build/typecheck fail vì thiếu import `ai-provider-handler`, đó là bằng chứng có caller ẩn mà Bước 1 bỏ sót — điều tra lại trước khi tiếp tục, KHÔNG suppress lỗi.

---

## Verify

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
  (khớp với impactedCount:0 đã xác nhận ở Bước 1)
```

---

## Definition of Done

- [ ] Bước 1: xác nhận lại 0 caller ngoài chính file + test file (grep + GitNexus impact nếu có)
- [ ] `agent/src/relay/ai-provider-handler.ts` đã bị `git rm`
- [ ] `agent/src/relay/__tests__/ai-provider-handler.test.ts` đã bị `git rm`
- [ ] `agent/src/shared/ai-credential-contract.ts` không import từ `ai-provider-handler.ts` → giữ nguyên, không xoá
- [ ] `desktop/src/relay/ai-provider-handler.ts` (package `desktop/`, ngoài scope) KHÔNG bị đụng tới
- [ ] `npm run typecheck` (trong `agent/`) pass, không còn import chết nào tới `ai-provider-handler`
- [ ] `npx oxlint src/relay` không báo lỗi liên quan
- [ ] `npx vitest run src/relay/__tests__/agent-credential-store.test.ts` pass
- [ ] `npx vitest run` (toàn bộ suite `agent/`) pass
- [ ] `git status --porcelain` show cả 2 file ở trạng thái `D` (deleted)
- [ ] `detect_changes({scope: "compare", base_ref: "main"})` chỉ báo 2 file bị xoá, 0 symbol/execution-flow khác bị ảnh hưởng

---

## Kết Quả Thực Thi (2026-08-09)

Đã xoá `agent/src/relay/ai-provider-handler.ts` và `agent/src/relay/__tests__/ai-provider-handler.test.ts` (dùng `rm` vì `agent/` hiện không được git track trong repo này — trạng thái tái cấu trúc monorepo dở dang từ trước phiên làm việc). Grep xác nhận 0 caller còn lại trong `agent/`; `ai-credential-contract.ts` và bản riêng ở `desktop/` không bị đụng.

**Phương pháp verify dùng thực tế:** `npx tsc --noEmit -p agent/tsconfig.json` (so sánh delta lỗi trước/sau mỗi thay đổi — baseline 98 lỗi pre-existing không đổi qua toàn bộ 16 task) + grep xác nhận đoạn code khớp thật trước khi sửa. `pnpm test`/`npx vitest` **không chạy được** trong môi trường này vì `config/vitest.config.ts` không tồn tại (thiếu hạ tầng test, không phải lỗi do thay đổi này gây ra) — các checkbox liên quan tới vitest trong "Definition of Done" ở trên chưa được xác nhận bằng test tự động, chỉ bằng đọc code + typecheck.
