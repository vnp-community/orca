# 02 — Platform Abstraction & Coding Standards (v5.0)

Đối chiếu với 5 nguyên tắc trong `docs/features/README.md` ("Coding Standards cho v5.0") và `docs/hld/v1/C2-containers.md` ("Container Boundaries và Isolation").

---

## 1. `IPlatformServices` chỉ tồn tại cho web

**Mức độ: 🟠 High**

Chuẩn #3 ghi: *"IPlatformServices: không import electron trực tiếp"*.

`find frontend/src/platform -type f` cho thấy layer abstraction chỉ có:

```
src/platform/adapters/web/rpc-client.ts   ← DUY NHẤT adapter tồn tại
src/platform/app-interface.ts
src/platform/context.ts
src/platform/ipc-interface.ts
src/platform/rpc-client-interface.ts
src/platform/storage-interface.ts
src/platform/system-interface.ts
src/platform/types.ts
src/platform/window-interface.ts
```

**Không có `src/platform/adapters/electron/`** (hay tương đương). Trong khi đó, grep `from 'electron'` (loại test) trên toàn bộ `src/main` trả về **72 file** — ví dụ `src/main/browser/browser-manager.ts`, `src/main/ipc/browser.ts`, `src/main/window/clipboard-image-temp-file.ts`, `src/main/claude-accounts/oauth-refresh.ts`, `src/main/persistence.ts`, `src/main/runtime/orca-runtime.ts`.

**Kết luận:** layer `IPlatformServices` hiện chỉ phục vụ target web (đúng tinh thần [restructure_v1 CR series](../../docs/crs/v1/restructure_v1/README.md) — "Additive only", không sửa code Electron gốc); code Electron/desktop main-process bỏ qua abstraction hoàn toàn, gọi thẳng `electron`. Nếu đọc đúng nghĩa đen chuẩn #3 ("không import electron trực tiếp" — không giới hạn phạm vi web-only), đây là một khoảng trống conformance có thật. Nhưng cần lưu ý: chuẩn #3 nằm trong mục dành riêng cho *"Tất cả v5.0 features"* — nếu ý định ban đầu chỉ áp cho code mới (Profile/Project/AI Provider/Workflow/Task, xem mục 2), thì đây không phải vi phạm mà là phạm vi bị hiểu nhầm — **cần làm rõ trong docs** để hết mơ hồ.

**Riêng module `ai-providers` được docs nhắc tên** (`src/main/ai-providers/`, theo `C2-containers.md` "Containers mới (v5.0)" mục 14) **không tồn tại** trong `frontend/src/main` — chỉ có type definitions ở `frontend/src/shared/ai-provider-types.ts` và UI components ở `frontend/src/renderer/src/components/ai-provider/`. Cần xác nhận: module này có thực sự nằm ở `backend/src/main/ai-providers/` (đã xác nhận có ở backend qua audit phiên trước) và `frontend` chỉ giữ phần UI + type — nếu vậy, đây không phải thiếu sót mà là docs mô tả sai layer, nên sửa doc.

## 2. Các module v5.0 F33-F39 khác — ✅ Sạch

`src/main/profile/`, `src/main/project/`, `src/main/task/`, `src/main/workflow/` — **0 hit** cho `from 'electron'`. Đây là các service thuần business-logic (DB, `IConnectionPool`), không cắt ngang platform boundary — chuẩn #3 không thực sự áp dụng vì các module này không cần platform services.

## 3. Zero Hardcode — ✅ Phần lớn sạch, 1 điểm nhỏ

Sample 8 vị trí có `localhost`/`127.0.0.1`/port hardcode:

- 7/8 vị trí là **legitimate**: default-with-env-override (`process.env['ORCA_HTTP_PORT'] ?? '6768'`), hoặc loopback-by-design (SSH port-forward, CDP proxy, local hook server bind `0`/OS-assigned port) — không phải vi phạm.
- **1 điểm đáng chú ý (Medium):** [src/main/dev-server/agent-ws-server.ts:103](../../frontend/src/main/dev-server/agent-ws-server.ts#L103) — thông báo hướng dẫn user hardcode `ws://<orca-host>:6768${AGENT_WS_PATH}` trong chuỗi hiển thị, **không đọc** `process.env['ORCA_HTTP_PORT']`. Nếu operator đã override port, thông báo này hiển thị sai — khác với pattern default-with-fallback đúng đắn ở các nơi khác trong cùng thư mục.

## 4. Zero Mock — ✅ Sạch

Không tìm thấy mock/fake data nào lẫn vào production code (`main/` và `renderer/src/`, loại test). Các match "fake"/"mock" tìm được đều là tên biến hợp lệ (`fakePaneId` cho tmux pane ID quản lý phần mềm, `$fakeExitCode` trong PowerShell script) hoặc static copy cho slide demo mobile-companion — không phải data giả thay cho gọi API/DB thật.

## 5. Renderer sandbox (`src/renderer/src/**`) — ✅ Sạch

39 hit cho import `electron`/`node:fs`/`node:child_process`/`node:net`/`node:os` — **100% nằm trong file test** (Vitest đọc fixture/spawn helper process). Không có leak nào vào production renderer code.

## 6. `.orig`/`.rej` merge artifacts — ✅ Không có

`find frontend/src -iname "*.orig" -o -iname "*.rej"` — 0 kết quả.

---

## Tổng kết mục này

| Hạng mục | Trạng thái |
|---|---|
| IPlatformServices (web) | ✅ |
| IPlatformServices (electron/desktop) | 🟠 Chưa có adapter, 72 file bypass |
| `ai-providers` module location | 🟡 Docs trỏ sai layer (frontend vs backend) — cần sửa doc |
| Zero Hardcode | ✅ (1 điểm nhỏ 🟡) |
| Zero Mock | ✅ |
| Renderer sandbox | ✅ |
| Merge artifacts | ✅ |
