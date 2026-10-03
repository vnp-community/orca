# frontend Solutions — MCP Quality & Rollout (v5)

**CR:** [docs/crs/v5/mcp-quality-rollout](../../../../../../docs/crs/v5/mcp-quality-rollout/README.md) (CR-MCP-015)
**Hợp đồng bắt buộc:** [CONTRACT-mcp-ui-api.md](../../../../../backend-go/crs/v5/CONTRACT-mcp-ui-api.md) · **Quy ước FE:** [crs/v5/README.md](../../README.md)
**Phía backend:** [BE-MCP-SOL-015](../../../../../backend-go/crs/v5/mcp-quality-rollout/solutions/BE-MCP-SOL-015-conformance-e2e-observability-rollout.md)

## Feature này có UI không?

**Không có UI mới.** CR-015 là chất lượng/vận hành. Phần FE là: (1) bộ kiểm thử cho mọi FE-MCP-SOL, (2) sự kiện telemetry UI, (3) tài liệu người dùng, (4) hành vi rollout/trạng thái rỗng của UI đã có (không thêm cờ build riêng — CONTRACT §6).

## Re-verify (khảo sát mã, 2026-10-01)

| Khẳng định (yêu cầu giao) | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| "Playwright e2e dưới `/opt/repos/orca/tests/e2e`" | `tests/playwright.config.ts` dùng `@stablyai/playwright-test`, `testDir: ./e2e`, `globalSetup` **build Electron** (`electron-vite build --mode e2e`, cần `out/main/index.js`), `projects` `electron-headless/headful` khớp `**/*.spec.ts`. `frontend/` hiện chỉ có build web (Vite, `ORCA_PLATFORM='web'`) ⇒ đặt spec web vào `tests/e2e/*.spec.ts` sẽ bị project Electron chọn và hỏng ở `globalSetup` | **Lệch** ⇒ dùng thư mục con `tests/e2e/mcp-web/` + hậu tố `*.web.e2e.ts` (không khớp `**/*.spec.ts`) + config riêng `tests/playwright.web.config.ts` |
| Vitest cho FE | `frontend/package.json`: `"test": "vitest run --config config/vitest.config.ts"`; `include` `src/**/*.test.ts(x)`; môi trường mặc định `node`, test React dùng `// @vitest-environment happy-dom` + `@testing-library/react` | Không |
| Telemetry UI | `lib/telemetry.ts#track` → `window.api.telemetryTrack`; **ở web, `telemetryTrack: () => Promise.resolve()` (no-op)** (`web-preload-api.ts:860`); sự kiện phải khai báo zod `.strict()` trong `shared/telemetry-events.ts` (`eventSchemas`) | **Telemetry UI web hiện không phát đi đâu** ⇒ thiết kế schema sẵn, chốt nguồn sự thật rollout là metric backend (BE-015) |
| Tài liệu người dùng ở `docs/guides` | `docs/guides/` có thư mục theo chủ đề (`auth/`, `task-automation/`, …), tiếng Việt | Tạo `docs/guides/mcp/` |
| Có sẵn cờ tính năng FE | Không; CONTRACT §6 cấm cờ build riêng. Cờ thật: `MCP_ENABLED` (process), `enabled` theo tenant, kill switch | Không |
| Script `verify:localization-*` | được gọi bởi `pnpm lint` nhưng không có trong `config/scripts/` | Chưa xác minh |

## Solutions

| Solution | CR | Area | Effort | Status |
|---|---|---|---|---|
| [FE-MCP-SOL-012](./FE-MCP-SOL-012-testing-telemetry-and-rollout.md) | CR-MCP-015 (phần FE) | `tests/`, `frontend/src/**/*.test.ts(x)`, `shared/mcp-telemetry-events.ts`, `docs/guides/mcp/` | Medium (rải theo các FE solution) | ✅ Implemented — see Gaps |

## Thứ tự thực thi & phụ thuộc

FE-012 chạy **cùng nhịp** các FE solution khác (test viết kèm từng solution), phần Playwright cần backend:

| Mốc | Làm khi | Việc |
|---|---|---|
| T0 | FE-001 merge | khung test (`mcp-test-fixtures.ts`), test nền |
| T1 | FE-002 | test Connect tab; docs `connect-*.md` |
| T2 | FE-003/004 + BE-005/006 | e2e consent + token (mock WS cho trang consent) |
| T3 | FE-009 + BE-013 | e2e approval (cần backend dev có seed) |
| T4 | trước GA | smoke rollout, rà soát docs, telemetry bridge (nếu có) |

**Đã chốt (2026-10-01):** client tham chiếu trong CI là MCP Python SDK chính thức (job `backend-go/ci/mcp-conformance/`, BE-015 §A.2 — API chưa xác minh); tenant mới bật mặc định (`MCP_TENANT_DEFAULT_ENABLED=true`) nên stack dev/e2e không cần bật tay, và UI vẫn phải xử lý trạng thái tenant đã bị admin tắt.
