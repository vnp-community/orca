# SOL-FE2E-002 — Bỏ PairCodeFallback khỏi LoginPage — Giải pháp implement

**CR:** [CR-FE2E-002](../../../../../docs/crs/v2/frontend-e2ee/CR-FE2E-002-remove-paircode-fallback-from-login.md)
**TDD Refs:** [TDD-FE-02 §5, §10](../../../tdd/v4/02-auth-flow.md) (LoginPage, PairCodeFallback — v4 mô tả `PairCodeFallback` là *"Backward Compat... Chỉ show nếu /auth/me trả về 404 (endpoint không tồn tại = no-auth mode)"*), [TDD-FE-06 §1](../../../tdd/v4/06-web-entry.md)
**Approach:** Test-Driven — viết test trước theo Acceptance Criteria của CR, sau đó áp diff.

---

## 1. Đối chiếu TDD vs code thật — phát hiện quan trọng trước khi code

TDD-FE-02 §10 mô tả điều kiện hiển thị `PairCodeFallback` là *"chỉ show nếu `/auth/me` trả 404"* — tức TDD hình dung nó có logic điều kiện RIÊNG. **Code thật không có điều kiện này** — `LoginPage.tsx` render `<PairCodeFallback />` **vô điều kiện** mỗi khi `LoginPage` được render (tức mỗi khi `sessionUser === null`), không tự kiểm tra 404 gì cả:

```tsx
// LoginPage.tsx (thật, dòng 59-63)
{/* PairCode backward-compat section */}
<div className="login-divider" aria-hidden="true">or</div>
<PairCodeFallback />
```

Điều kiện "chỉ hiện khi no-auth mode" mà TDD mô tả thực chất được quyết định ở **tầng ngoài `LoginPage`** — bởi chính nhánh `/auth/config` trong `main.tsx` (xem SOL-FE2E-001 mục 4): nếu server không có `/auth/config` (404), toàn bộ `bootstrapWebApp()`/`LoginPage` không bao giờ được gọi tới, thay vào đó `renderOriginalPairCodeApp()` chạy — nghĩa là `LoginPage`/`PairCodeFallback` **chỉ tồn tại trong nhánh ĐÃ CÓ auth** (use case A), nơi TDD lại nói nó "chỉ hiện khi KHÔNG có auth" — mâu thuẫn hoàn toàn với thực tế.

**Kết luận:** TDD-FE-02 §10 mô tả sai — không phải lỗi trong code, mà lỗi trong tài liệu (được viết trước khi kiến trúc `/auth/config` probe hai nhánh ra đời, hoặc chưa cập nhật theo kịp). Đây chính là bằng chứng TDD ủng hộ cho lý do CR-FE2E-002 tồn tại: `PairCodeFallback` trong `LoginPage` **chưa bao giờ** có điều kiện hiển thị hợp lý — nó luôn hiện, trong đúng nhánh (multi-user) mà nó không cần thiết. Xác nhận diff của CR là đúng hướng.

## 2. Diff — áp dụng đúng như CR đã đặc tả

CR-FE2E-002 mục 2.1/2.2 đã có diff chính xác (đã đối chiếu lại với code thật hiện tại — line number và nội dung khớp 100%, không có thay đổi nào giữa lúc viết CR và lúc viết solution này). Không cần điều chỉnh gì thêm — copy nguyên diff của CR khi implement.

**Riêng 1 điểm cần làm rõ thêm cho `main-web-bootstrap.tsx`:** đối chiếu với TASK-FE-HLD-011 (đã thực thi ở series `hld-v1`, không thuộc CR series này) — hàm `installAuthFailedRedirect()` đọc `readStoredWebRuntimeEnvironment()` để lấy `env?.id`. Hàm này **giờ trả về giá trị đã unwrap** (deviceToken plaintext trong bộ nhớ) do BUG-FE-HLD-001 fix, nhưng **chữ ký hàm và logic `env?.id !== 'session-auth'` không đổi** — diff của CR-FE2E-002 cho file này (chỉ sửa comment) vẫn áp dụng nguyên vẹn, không xung đột với thay đổi đó.

## 3. Test Specifications

### 3.1 `LoginPage.test.tsx` — xoá test case cũ, thêm test khẳng định không còn PairCodeFallback

```tsx
// frontend/src/renderer/src/web/login/__tests__/LoginPage.test.tsx
// Xoá mọi test case render/tương tác PairCodeFallback bên trong LoginPage (nếu có).
// Thêm:
it('does not render the pairing code fallback', () => {
  render(<LoginPage availableProviders={[]} onLoginSuccess={vi.fn()} />)
  expect(screen.queryByLabelText(/pairing url or code/i)).not.toBeInTheDocument()
  expect(screen.queryByText(/pairing url or code/i)).not.toBeInTheDocument()
})
```

### 3.2 `main-web-bootstrap.test.ts` — xác nhận `renderOriginalPairCodeApp` không bị đụng

```ts
// frontend/src/renderer/src/web/__tests__/main-web-bootstrap.test.ts (hoặc file test hiện có)
// Test hiện có cho sessionUser !== null → App, sessionUser === null → LoginPage
// giữ nguyên. Thêm xác nhận: LoginPage được render không nhận prop nào liên quan
// pairing (đảm bảo không có API contract ngầm bị phá).
it('renders LoginPage without any pairing-related props when unauthenticated', () => {
  // sessionUser: null, hasEnvironment: false → LoginPage
  // (chi tiết setup theo pattern test hiện có của file)
})
```

### 3.3 Regression cho use case B — KHÔNG viết mới, chỉ chạy lại

Không có test hiện có riêng cho `renderOriginalPairCodeApp()`/`WebConnect` bị ảnh hưởng bởi diff này (diff không chạm `main.tsx`'s nhánh 404) — chạy lại toàn bộ `web-pairing.test.ts`, `web-runtime-client.test.ts` để xác nhận 0 regression (đúng theo Acceptance Criteria #2 của CR).

## 4. Acceptance Criteria — Kế hoạch verify

| # | Criteria (từ CR) | Cách verify cụ thể |
|---|---|---|
| AC-1 | Login page không còn hiển thị "Pairing URL or Code" | Test 3.1 |
| AC-2 | `renderOriginalPairCodeApp()` không đổi hành vi | Chạy lại `web-pairing.test.ts` + `WebConnect` (chưa có test riêng — xem CR-FE2E-005 mục "Kịch bản 6" cho e2e cần bổ sung) |
| AC-3 | Không còn `import { PairCodeFallback }` ngoài chính file nó | `grep -rn "PairCodeFallback" frontend/src` sau khi áp diff — kỳ vọng chỉ còn `PairCodeFallback.tsx` + test file của nó |
| AC-4 | `pnpm test` xanh toàn bộ | `pnpm --filter frontend test` (dùng `frontend/config/vitest.config.ts` đã thêm trong series `hld-v1` — xem `specs/frontend/bugs/hld-v1/tasks/NOTES.md`) |
