# Tasks: quality-rollout (frontend, v7)

> 🟡 **Partial (2026-10-07).** 073-02, 073-06 DONE (fake backend + 10 test; thẻ admin đã nối vào Settings); 073-01/03/04/07 PARTIAL; 073-05 TODO. E2E Playwright chưa chạy được (không có Chromium); vitest xanh.

## Bảng Solution → Task

| Task | Mô tả | Priority | Phụ thuộc | Status |
|---|---|---|---|---|
| [FE-CV-TASK-073-01](./FE-CV-TASK-073-01-flag-gating-matrix-tests.md) | Ma trận gating cờ × lối vào (không gọi kênh khi cờ tắt) | P0 | 050-store, 085-01, 073-02 | [~] PARTIAL |
| [FE-CV-TASK-073-02](./FE-CV-TASK-073-02-code-intel-fake-backend.md) | Fake backend `createFakeCodeIntelBackend` (G4) + fixture | P0 | 050-types | [x] DONE |
| [FE-CV-TASK-073-03](./FE-CV-TASK-073-03-playwright-code-intel-web-project.md) | Project Playwright `code-intel-web`, WS giả, script | P0 | 073-02 | [~] PARTIAL |
| [FE-CV-TASK-073-04](./FE-CV-TASK-073-04-flag-web-e2e-spec.md) | Spec `flag.web.e2e.ts` | P0 | 073-02, 073-03, 061 | [~] PARTIAL |
| [FE-CV-TASK-073-05](./FE-CV-TASK-073-05-review-web-e2e-specs.md) | Specs tóm tắt/trạng thái/reindex/lens | P1 | 073-02, 073-03, lens 050–061 | [ ] TODO |
| [FE-CV-TASK-073-06](./FE-CV-TASK-073-06-code-intel-settings-admin-card.md) | Thẻ cài đặt admin (`settings.set`) | P1 | 050-store, 073-02 | [x] DONE |
| [FE-CV-TASK-073-07](./FE-CV-TASK-073-07-dev-stack-electron-smoke-and-ci-entry.md) | `@dev-stack`, Electron smoke, lệnh CI | P2 | 073-03..05, preload desktop | [~] PARTIAL |

## Thứ tự

```
073-02 ─┬▶ 073-03 ─┬▶ 073-04 ─┐
        │          └▶ 073-05 ─┼▶ 073-07
        ├▶ 073-01 ───────────┘
        └▶ 073-06
```
073-02 là cổng G4 nên làm cùng FE-CV-SOL-050; các task lens của `review-frontend` tham chiếu 073-02/073-03 cho e2e của chúng.

## Lưu ý

- Task chạm `desktop/` (Electron smoke cần preload `codeIntel`) ghi "ngoài `frontend/`, cần chủ sở hữu desktop duyệt".
- Mọi lệnh test/e2e **chưa chạy**; `tests/playwright.web.config.ts` là cấu hình dùng chung với MCP: bắt buộc hồi quy `test:e2e:mcp-web` khi sửa.
