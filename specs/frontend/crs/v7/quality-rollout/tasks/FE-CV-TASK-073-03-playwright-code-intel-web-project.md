# FE-CV-TASK-073-03: Project Playwright `code-intel-web`, WS giả và script

**From Solution:** [FE-CV-SOL-073](../solutions/FE-CV-SOL-073-flag-gating-and-web-e2e.md) mục 2.5
**Priority:** P0
**Area:** tests / Playwright web
**File:** `tests/playwright.web.config.ts` (sửa), `tests/e2e/code-intel-web/support/{mock-code-intel-ws,code-intel-app-navigation,code-intel-dev-backend}.ts` (mới), `package.json` gốc (script `test:e2e:code-intel-web`)
**Depends on:** FE-CV-TASK-073-02
**Status:** [x] DONE (verified 2026-10-08)

## Context

- Đã xác minh cấu hình hiện tại: `testDir './e2e/mcp-web'`, `testMatch '**/*.web.e2e.ts'`, một project `chromium`, `webServer` Vite 5174, `MCP_E2E_BASE_URL`/`MCP_E2E_CHROMIUM_PATH`; script `test:e2e:mcp-web` ở `package.json:83`; `support/mock-orca-ws.ts` nhập `createFakeMcpBackend` (có thể gắn cứng kiểu `FakeMcpBackend` — đọc kỹ khi làm).
- Không tạo thư mục `helpers/utils/common`.

## Việc cần làm

1. Đổi cấu hình thành `projects: [{name:'mcp-web', testDir:'./e2e/mcp-web'}, {name:'code-intel-web', testDir:'./e2e/code-intel-web'}]` (mỗi project Chromium), giữ `testMatch`, `webServer` chung, `baseURL` project code-intel = `CODE_INTEL_E2E_BASE_URL ?? LOCAL`.
2. `mock-code-intel-ws.ts`: trả lời dialect session-client từ `createFakeCodeIntelBackend` (kể cả `subscribe` streaming không tên kênh + `{type:'end'}`), tái dùng `mockOrcaApp`/boot channels nếu tách được giao diện backend (nếu gắn cứng MCP thì bản mỏng riêng; nếu di chuyển phần chung, đặt thư mục tên theo nội dung).
3. `code-intel-app-navigation.ts` (boot app, mở worktree + tab Review), `code-intel-dev-backend.ts` (`devStackUrl` từ `CODE_INTEL_E2E_BASE_URL`).
4. Script: `"test:e2e:code-intel-web": "npx playwright test --config tests/playwright.web.config.ts --project code-intel-web"`; đổi `test:e2e:mcp-web` thành `--project mcp-web`.

## Kiểm thử

- Chạy một spec smoke (boot + `settings.get`) ở project mới; **hồi quy** `pnpm run test:e2e:mcp-web` (bắt buộc; chưa chạy).
- `pnpm run test:e2e:code-intel-web`.

## Tiêu chí hoàn thành

- [ ] Hai project độc lập; `mcp-web` không đổi hành vi.
- [ ] Không `*.spec.ts` lẫn project Electron.

## Rủi ro

- Đổi cấu hình dùng chung làm hỏng MCP e2e; chưa kiểm chứng Vite/Chromium trong CI.

## Ghi chú triển khai (2026-10-07)

- Biến dev-stack: `CODE_INTEL_E2E_BASE_URL` (cũng đặt BASE của config). `mock-code-intel-ws` dùng lại `BOOT_CHANNEL_STUBS` của mcp-web.
