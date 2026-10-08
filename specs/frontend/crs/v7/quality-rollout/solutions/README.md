# Solutions: quality-rollout (frontend, v7)

> 🟡 **Partial.** 2026-10-07: 073-02 fake backend và 073-06 thẻ admin DONE; 073-01/03/04/07 PARTIAL; 073-05 TODO. Soạn 2026-10-06 từ [docs/crs/v7/quality-rollout](../../../../../../docs/crs/v7/quality-rollout/README.md).

## Bảng CR → Solution

| CR | Solution | Nội dung | Trạng thái |
|---|---|---|---|
| CR-CV-073 | [FE-CV-SOL-073](./FE-CV-SOL-073-flag-gating-and-web-e2e.md) | Gating cờ `codeIntelEnabled`/`qualityGateEnabled`/AI ở frontend (ma trận kiểm), thẻ cài đặt admin, fake backend G4, e2e Playwright web (project `code-intel-web`), Electron smoke tối thiểu | [~] PARTIAL |

CR-070, 071, 072 không có việc frontend (§8.2). Phần BE/AG của CR-073: `BE-CV-SOL-073-settings-flag-and-rollout`, `AG-CV-SOL-073-agent-kill-switch`.

## Thứ tự và phụ thuộc

```
FE-CV-SOL-050 (bridge, store, useCodeIntelSupport) ─▶ FE-CV-SOL-073: 073-02 (fake backend G4, làm cùng 050) ─▶ mọi lens frontend
073-02 ─▶ 073-03 (Playwright) ─▶ 073-04 (flag spec), 073-05 (specs lens/trạng thái) ─▶ 073-07
050-store + 085-01 ─▶ 073-01 (ma trận gating);  BE-CV-SOL-073 (hoặc fake) ─▶ 073-06 (thẻ admin)
```
Cổng: frontend dùng kênh thật chỉ sau **G3** (`BE-CV-SOL-040-codeintel-channel-foundation`); trước đó dùng fake backend.

## Quyết định chung

- Nguồn cờ chuẩn là `codeIntel.settings.get` (§6); không dò bằng `typeof window.api.codeIntel` (web bọc Proxy `withFallback`). Không tạo hook cờ thứ hai (dùng `useCodeIntelSupport` của SOL-050 và `useQualityFeatureFlags` của FE-CV-TASK-085-01).
- Fail closed khi cờ `unknown`; cờ tắt ⇒ không kênh nào ngoài `settings.get`, không `subscribe`.
- Tắt cờ giữa chừng chỉ qua hai đường hợp đồng cho phép: refresh `settings.get` 60 s (tab Review mở) và `CODEINTEL_DISABLED` ở lần gọi kế; không có push cờ.
- e2e khẳng định trên DOM (`tests/e2e/AGENTS.md`); T2 (fake backend) chặn PR, `@dev-stack`/Electron không chặn.
- Không hex, `translate()` + 5 locale, không thêm thư viện, không `max-lines` disable, không `components/code-review/*`.

## Lệch giữa CR-073 và hợp đồng (tóm tắt; chi tiết ở SOL-073 mục 1)

Biến `CODE_INTEL_ENABLED` → `CODEINTEL_ENABLED` (PQ-23); `settings.set {enabled}` → `{codeIntelEnabled?, qualityGateEnabled?, …}`; cờ ba cấp; không push cờ; push là một `subscribe` với khung có `event`.

## Cách chạy

- Vitest: `pnpm --filter orca-frontend test -- <đường dẫn>`.
- Playwright web: `pnpm run test:e2e:code-intel-web` (script do 073-03 thêm; hồi quy `pnpm run test:e2e:mcp-web`). **Chưa chạy** mọi lệnh trên; chưa kiểm chứng Vite + Chromium trong CI.
