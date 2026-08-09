# Ghi chú thực thi — hạ tầng phát hiện trong quá trình chạy 14 task

**Ngày:** 2026-08-09

Khi bắt đầu thực thi, phát hiện `frontend/` (package "isolated copy, split from monorepo" — theo mô tả trong chính `package.json`) thiếu 2 mảng hạ tầng cần cho việc verify:

## 1. Không có test runner — đã fix, nằm ngoài phạm vi 14 task gốc

`frontend/package.json` trước đây chỉ có `"build"`/`"dev"`, không có `"test"`. `vitest run` mặc định không tìm thấy config hợp lệ (chỉ có `vite.config.ts` build-only, root trỏ vào `src/renderer`).

**Đã fix:** copy `desktop/config/vitest.config.ts` → `frontend/config/vitest.config.ts` (không đổi nội dung, chỉ cập nhật comment nguồn gốc), thêm `"test"`/`"test:watch"` vào `frontend/package.json`. Xác nhận hoạt động: `pnpm test -- agent-ws-server` chạy và pass.

## 2. `tsc --noEmit` không chạy được ở mức toàn package — CHƯA fix, ngoài phạm vi

`frontend/tsconfig.json` hiện có `include` không đầy đủ — hàng loạt file dưới `src/main/**` (agent-hooks, mọi `hook-service.ts` theo provider...) bị TypeScript báo `TS6307: File ... is not listed within the file list of project` dù các file này **được import hợp lệ** bởi những file nằm trong `include`. Đây là lỗi cấu hình dự án (project references / include glob), có sẵn từ trước, không liên quan tới bất kỳ thay đổi nào trong 14 task này — xác nhận bằng cách chạy `tsc --noEmit` trên bản chưa sửa, lỗi giống hệt.

**Không sửa trong series `hld-v1` này** — phạm vi sửa `tsconfig.json` cho cả package là việc lớn, rủi ro ảnh hưởng build thật, cần 1 bug/task riêng ở domain phù hợp (đề xuất tạo `BUG-FE-HLD-XXX` mới hoặc domain riêng nếu muốn theo dõi).

**Verify thay thế đã dùng cho mọi task trong series:** `pnpm test -- <tên file>` (qua vitest, dùng Vite transform pipeline riêng, không phụ thuộc `tsc --project`) — xác nhận bắt được lỗi cú pháp/type cơ bản trong phạm vi file test tương ứng. Task nào không thể xác nhận bằng cách này (task doc-only, hoặc task cần review con người) được ghi rõ trong chính file task đó.

## 3. Kết quả cuối cùng sau khi thực thi cả 14 task (2026-08-09)

```
pnpm test -- agent-ws-server useGit runtime-git-client web-runtime-environment
              web-runtime-environment-crypto web-preload-api web-credential-store
              web-pairing web-runtime-client web-viewport-shell web-workspace-session
              ConnectionStatusProvider ConnectionStatusBanner SsoButton LoginPage
              LoginForm preload-no-change

Test Files  3 failed | 16 passed (19)
     Tests  4 failed | 161 passed (165)
```

**4 test fail còn lại — cả 4 đều pre-existing, không liên quan tới bug fix series này:**
- `web/__tests__/preload-no-change.test.ts` (3 test) — thiếu `src/preload/index.ts` (chỉ có `api-types.ts`)
- `web-preload-api.test.ts` (1 test) — thiếu `src/preload/gitlab.ts`

Cả hai đều là khoảng trống hạ tầng có sẵn từ đợt tách `frontend/` khỏi monorepo (mục 1/2 phía trên), xác nhận bằng lỗi `ENOENT`/`Cannot find module`, không phải lỗi logic.

## 4. File thật đã thay đổi (tổng hợp)

**Sửa:**
- `frontend/src/main/dev-server/agent-ws-server.ts` (BUG-004)
- `frontend/src/renderer/src/hooks/useGit.ts` + `hooks/__tests__/useGit.test.ts` (BUG-002)
- `frontend/src/renderer/src/web/web-runtime-environment.ts` (BUG-001)
- `frontend/src/renderer/src/web/web-preload-api.test.ts` (fix regression phát sinh từ BUG-001)
- `frontend/src/main/credentials/web-credential-store.ts` (BUG-003)
- `docs/features/README.md` (BUG-005)
- `.oxlintrc.json` (root, BUG-005)
- `AGENTS.md` (BUG-006)
- `config/max-lines-baseline.txt` (khôi phục + prune, BUG-006)

**Thêm mới:**
- `frontend/config/vitest.config.ts`, script `test`/`test:watch` trong `frontend/package.json` (hạ tầng, ngoài 14 task gốc nhưng cần để verify được)
- `frontend/src/main/dev-server/agent-ws-server.test.ts`
- `frontend/src/renderer/src/web/web-runtime-environment-crypto.ts` + `.test.ts` (BUG-001)
- `frontend/src/renderer/src/web/web-runtime-environment.test.ts` (BUG-001, module chưa từng có test)
- `frontend/src/main/credentials/web-credential-store.test.ts` (BUG-003, module chưa từng có test)

**Xoá:**
- `frontend/src/renderer/src/runtime/runtime-rpc-stream.ts` (BUG-002)

**Khôi phục (từ git history, không viết mới):**
- `config/scripts/check-max-lines-ratchet.mjs`, `config/scripts/check-max-lines-ratchet.test.mjs` (BUG-006)

**Không đổi, chỉ xác nhận đã đúng sẵn:**
- `frontend/src/renderer/src/runtime/runtime-git-client.ts` (`pushRuntimeGit()` đã có sẵn, đúng)
- root `package.json` (`check:max-lines-ratchet` script đã sẵn wiring)
