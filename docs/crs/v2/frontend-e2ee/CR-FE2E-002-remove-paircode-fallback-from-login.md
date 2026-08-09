# CR-FE2E-002 — Bỏ PairCodeFallback khỏi LoginPage (multi-user path)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-FE2E-002 |
| **Tên** | Loại bỏ entry point E2EE pairing khỏi luồng login multi-user |
| **Loại** | Cleanup / Simplification |
| **Priority** | P0 |
| **Phiên bản** | v5.1 |
| **Ngày tạo** | 2026-08-08 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-FE2E-001 |
| **Tác động HLD** | web-server-architecture.md §5.2 (đánh dấu "legacy pairing mode" giới hạn lại use case B), §7.1 Auth Flow |

---

## 1. Vì sao an toàn

`PairCodeFallback` chỉ render bên trong `LoginPage`, và `LoginPage` chỉ render khi `sessionUser === null` ([main-web-bootstrap.tsx:158, 182-193](../../../../frontend/src/renderer/src/web/main-web-bootstrap.tsx#L158)):

```tsx
if (sessionUser !== null) {
  // ... render <App/> thẳng, KHÔNG bao giờ qua LoginPage
}
if (!hasEnvironment) {
  return <LoginPage ... />   // ← PairCodeFallback chỉ ở đây
}
```

Trong multi-user mode (F23), local login + SSO **luôn khả dụng** — không có trạng thái nào mà user cần pairing code để vào được Orca nhưng lại không login được. Code hiện tại tự gọi đây là *"PairCode backward-compat section"* ([LoginPage.tsx:59](../../../../frontend/src/renderer/src/web/login/LoginPage.tsx#L59)) — nghĩa là chính tác giả gốc đã coi nó là fallback tạm, không phải đường chính.

## 2. Thay đổi

### 2.1 `LoginPage.tsx` — bỏ `PairCodeFallback`

```diff
- import { PairCodeFallback } from './PairCodeFallback'
  ...
-        {/* PairCode backward-compat section */}
-        <div className="login-divider" aria-hidden="true">
-          or
-        </div>
-        <PairCodeFallback />
      </main>
```

### 2.2 `main-web-bootstrap.tsx` — đơn giản hoá `installAuthFailedRedirect`

Comment hiện tại giả định vẫn có thể có "E2EE-paired environments" bên cạnh session-auth:

```ts
// Guards: only runs once (redirected flag), only for session-auth environments
// (E2EE-paired environments should reconnect, not logout).
```

Sau CR này, trong nhánh `bootstrapWebApp()` (use case A), **environment luôn là `session-auth`** — không còn cách nào tạo `StoredWebRuntimeEnvironment` kiểu pairing từ trong luồng multi-user nữa (không còn `PairCodeFallback`/`WebConnect` gọi tới). Giữ nguyên logic check `env?.id !== 'session-auth'` (vẫn đúng, chỉ không còn nhánh nào khác có thể xảy ra) — **không cần sửa hàm này**, chỉ cập nhật comment cho khớp thực tế:

```diff
- * Guards: only runs once (redirected flag), only for session-auth environments
- * (E2EE-paired environments should reconnect, not logout).
+ * Guards: only runs once (redirected flag), only for session-auth environments.
+ * (E2EE pairing is no longer reachable from the multi-user bootstrap path —
+ * see CR-FE2E-002 — this check is now a defensive no-op for that path, kept
+ * because bootstrapWebApp() and main.tsx's WebRoot still share this file's
+ * exported helpers with tests.)
```

### 2.3 KHÔNG đụng tới

- `main.tsx` (`renderOriginalPairCodeApp` branch) — nguyên vẹn.
- `WebConnect.tsx`, `AddInstanceForm.tsx`, `OrcaInstanceSwitcher.tsx` — nguyên vẹn (dùng bởi use case B, và tuỳ kết quả CR-FE2E-004).
- `web-runtime-client.ts`, `web-e2ee.ts`, `web-pairing.ts`, `web-runtime-environment.ts` — nguyên vẹn, chỉ không còn được `import` từ `LoginPage.tsx` nữa.
- `web-preload-api.ts`'s `getRuntimeClientForEnvironment` — **không sửa ở CR này**. Vẫn cần trả về `WebRuntimeClient` cho use case B. (Việc bundle nó có bị tải trong use case A hay không là phạm vi CR-FE2E-003.)

### 2.4 Xoá test đã hết ý nghĩa / cập nhật test còn lại

- [ ] `login/__tests__/*` — xoá test case cho `PairCodeFallback` render trong `LoginPage`.
- [ ] `main-web-bootstrap.test.ts` — xoá/điều chỉnh test case nào giả lập user nhập pairing code từ `LoginPage` (nếu có); giữ nguyên test cho `sessionUser !== null` → App, `sessionUser === null` → LoginPage.
- [ ] KHÔNG xoá `web-pairing.test.ts`, `web-e2ee.test.ts` (dùng bởi use case B).

## 3. Acceptance Criteria

- [ ] Multi-user login page không còn hiển thị "Pairing URL or Code".
- [ ] `main.tsx`'s `renderOriginalPairCodeApp()` path (test bằng cách mock `/auth/config` trả 404) vẫn hiển thị `WebConnect` y như trước — không thay đổi hành vi.
- [ ] Không còn `import { PairCodeFallback }` ở bất kỳ đâu ngoài chính file `PairCodeFallback.tsx` và test của nó (giữ file để không phá TypeScript project reference nếu còn nơi khác dùng — xác nhận qua grep trước khi merge).
- [ ] `pnpm --filter frontend test` xanh toàn bộ, không giảm coverage phần `web/login`.
