# FE-CV-SOL-073: Gating cờ `code_intel_enabled`/`quality_gate_enabled` và e2e Playwright web (fake backend G4)

> 🟡 Partial (2026-10-07): 073-02, 073-06 DONE (W6: thẻ admin đã nối Settings + i18n); 073-01/03/04/07 PARTIAL; 073-05 TODO (xem tasks/README.md). Priority P0 (gate trước khi bật cho người dùng). Viết ngày 2026-10-06; chưa chạy test hay ứng dụng.

**CR:** [CR-CV-073](../../../../../../docs/crs/v7/quality-rollout/CR-CV-073-e2e-feature-flag-rollout-runbook.md) (**chỉ phần frontend**: gating cờ, fake backend, T2 web, Electron smoke tối thiểu; service/T1/T3/T4, runbook, script wiring là việc của `BE-CV-SOL-073-settings-flag-and-rollout`)
**Area:** frontend + `tests/` (Playwright web) + thẻ cài đặt admin
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (U7, U8; **§6 cờ và phát hiện tính năng**; §3.1 `settings.get|set`, `subscribe`; §4.1 `Settings`; §5 push + `codeIntelResyncCounter`; §2.3 mã lỗi), [CONTRACT-codeintel-proto-and-data-map.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (**PQ-01, PQ-02, PQ-23, PQ-24, PQ-27, PQ-36**; **§7.1 G3/G4**; §7.2; §8.2; §8.3; §9 O-8). **TDD:** [v5/06-web-client](../../../../tdd/v5/06-web-client.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md); mẫu kiểm thử MCP: `tests/e2e/mcp-web/`.

## 1. Trạng thái hiện tại (re-verify)

**Đã xác minh trong phiên này:**
- `tests/playwright.web.config.ts`: `testDir: './e2e/mcp-web'`, `testMatch: '**/*.web.e2e.ts'`, `timeout 60_000`, `use.baseURL = MCP_E2E_BASE_URL ?? http://127.0.0.1:5174`, `MCP_E2E_CHROMIUM_PATH`, một project `chromium`, `webServer` Vite cổng 5174 (`cwd ../frontend`, url `/web-index.html`) khi không có BASE. `package.json` gốc dòng 83: `"test:e2e:mcp-web": "npx playwright test --config tests/playwright.web.config.ts"`; `@playwright/test ^1.59.1`.
- `tests/e2e/mcp-web/` có `approval|consent|rollout|token.web.e2e.ts` và `support/{mcp-app-navigation,mcp-dev-backend,mock-orca-boot-channels,mock-orca-ws}.ts`; `rollout.web.e2e.ts` có mẫu "MCP disabled: no MCP entry … `expect(backend.streamCount()).toBe(0)`" và "kill switch banner appears live"; `mock-orca-ws.ts` nhập `createFakeMcpBackend` từ `frontend/src/renderer/src/test-support/mcp-fake-backend.ts` và trả lời dialect session-client `{id, authToken, method, params}` → `{id, ok, result|error, _meta}`.
- `test-support/mcp-fake-backend.ts`: bộ xử lý theo kênh, `failures` map, `calls[]`, `listeners`, `emit`; lỗi dạng `new Error(\`\${code}: \${msg}\`)`.
- Mẫu thẻ cài đặt bật/tắt theo tenant: `components/settings/mcp/McpPane.tsx`, `McpTenantSettingsForm.tsx`; vai trò admin đọc bằng `useAppStore((s) => s.currentUser?.role === 'admin')` (`Settings.tsx:286`, `McpExternalServersTab.tsx:29`).
- FE-CV-TASK-085-01 (`useQualityFeatureFlags`) đã được soạn bởi nhóm quality-gate: hook gộp cờ `codeIntel/quality/ai` từ slice `settings` của SOL-050. Không có `frontend/src/renderer/src/test-support/code-intel-*` và không có `tests/e2e/code-intel-web/`.
**Theo CR-CV-073 (chưa kiểm lại):** `tests/e2e/AGENTS.md` ("khẳng định trên DOM, không trên store"), `tests/playwright.config.ts` Electron, workflow CI.

### Lệch giữa CR và hợp đồng (hợp đồng thắng)

| # | CR-073 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | Biến `CODE_INTEL_ENABLED` | PQ-23: `CODEINTEL_ENABLED` (service); gateway `CODE_INTEL_SERVICE_ADDR` | Chỉ ảnh hưởng tài liệu/dev-stack; frontend không đọc biến môi trường |
| 2 | `settings.set { enabled }` | `settings.set { codeIntelEnabled?, qualityGateEnabled?, qualitySecurityScanEnabled?, … }` (≥ 1 trường, admin) → `Settings` | Fake backend và thẻ admin dùng tên hợp đồng |
| 3 | Cờ đơn `code_intel_enabled` | Có thêm `quality_gate_enabled`, `quality_security_scan_enabled`, `aiReviewEnabled` ở `effective` (§4.1) | Gating ba cấp: code-intel ⊃ quality ⊃ AI; fake backend và spec kiểm đủ |
| 4 | Lỗi `CODEINTEL_DISABLED` ở mọi RPC ngoài `GetSettings` | UI-API U7/§2.3: ba mã (`DISABLED`, `QUALITY_GATE_DISABLED`, `AI_REVIEW_DISABLED`) → `kind` `disabled|quality-disabled|ai-disabled`; `PROFILE_UNKNOWN` **không** ẩn tính năng (PQ-01) | Fake backend phát đúng ba mã; spec kiểm hành vi ẩn riêng từng mức |
| 5 | Tắt cờ "trực tiếp (push)" làm lối vào biến mất | Hợp đồng **không có** push cờ; có `settings.get` mỗi 60 s khi tab Review mở và `CODEINTEL_DISABLED` ở lần gọi kế tiếp (§6, U7) | Spec dùng `page.clock` tua 60 s **và** ca "lần gọi kế tiếp trả `CODEINTEL_DISABLED`"; không có ca push |
| 6 | Kênh push `codeIntel.changed`, `codeIntel.reindexProgress` | §5: một `subscribe`; khung có `event` (`changed|reindexProgress|quality.progress|quality.finished|quality.gateChanged`); session-client mất tên kênh | Fake backend phát khung có `event`, dạng streaming của session-client |
| 7 | Fake backend "dữ liệu từ tệp vàng CR-070" | G1: `testdata/agent-results/*.json` ở backend-go; dạng đã chuẩn hoá của agent, không phải dạng WS | Fixture frontend `code-intel-fixtures.ts` dựng thủ công theo kiểu §4 và đối chiếu bằng test với tệp vàng khi có (task 073-02); không sao chép quá mức |
| 8 | Kênh `settings.get` "mọi thành viên tenant" | U8: phiên thiết bị chỉ được `settings.get` | Không ảnh hưởng web; ghi để mobile không gọi |
| 9 | Fake backend thuộc CR-050 hoặc 073? | README review-frontend ghi CR-050 liệt kê file; hợp đồng §7.1 G4 gán "(CR-073 T2)" | **073-02 sở hữu**; nếu SOL-050 đã dựng khung rỗng thì 073-02 mở rộng, không tạo bản thứ hai (câu hỏi mở 1) |
| 10 | Biến e2e `MCP_E2E_BASE_URL` hay mới | CR để mở | `CODE_INTEL_E2E_BASE_URL` riêng (không dùng chung stack MCP); ca `@dev-stack` bị bỏ qua khi thiếu |

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/renderer/src/test-support/
  code-intel-fake-backend.ts          (mới/mở rộng) createFakeCodeIntelBackend — G4
  code-intel-fixtures.ts              (mới) makeSettings, makeIndexStatus, makeChangeOverlay, makeErdModel, makeFindings…
  code-intel-integration/code-intel-flag-gating.integration.test.tsx   (mới) ma trận cờ × lối vào (073-01)
frontend/src/renderer/src/components/settings/code-intel/CodeIntelSettingsCard.tsx (+ test)   (mới; 073-06)
tests/playwright.web.config.ts                    (sửa) projects: mcp-web + code-intel-web
tests/e2e/code-intel-web/
  flag.web.e2e.ts, review-summary.web.e2e.ts, states.web.e2e.ts, reindex.web.e2e.ts,
  lenses.web.e2e.ts, review-notes.web.e2e.ts, dev-stack.web.e2e.ts
  support/mock-code-intel-ws.ts, code-intel-app-navigation.ts, code-intel-dev-backend.ts
tests/e2e/code-intel-electron.spec.ts             (mới, tối thiểu; 073-07)
package.json (gốc)                                (sửa) script test:e2e:code-intel-web
```
Phần chung giữa `mcp-web` và `code-intel-web` (đặt trạng thái WS giả, boot channels): tái dùng `mockOrcaApp`/`bootApp`; nếu cần di chuyển, đặt vào thư mục **đặt tên theo nội dung** (không `helpers`/`utils`/`common`).

### 2.2 Gating cờ ở frontend

Nguồn chuẩn `codeIntel.settings.get` (§6), đã do SOL-050-store-and-query-hooks (`useCodeIntelSupport`: gọi khi khởi động và mỗi 60 s **chỉ khi** có tab Review mở) và FE-CV-TASK-085-01 (`useQualityFeatureFlags`: `codeIntel ⊃ quality ⊃ ai`) hiện thực. Solution này **không tạo hook cờ thứ hai**; nó (a) kiểm hợp đồng gating bằng ma trận test, (b) thêm thẻ cài đặt admin, (c) thêm e2e. Quy tắc phải giữ ở mọi lối vào (kiểm bằng 073-01 và e2e):
1. `effective.codeIntelEnabled=false` ⇒ **không** lối vào (hàng agent, Source Control, Cmd+K, tab sidebar, tab Review), **không** kênh nào ngoài `settings.get`, **không** `subscribe` (`backend.streamCount()==0`).
2. `CODEINTEL_DISABLED` ở bất kỳ lần gọi (`kind:'disabled'`) ⇒ ẩn như trên, không toast; `CODEINTEL_UNAVAILABLE` ⇒ `unsupported` (ẩn, không toast).
3. `effective.qualityGateEnabled=false` hoặc `CODEINTEL_QUALITY_GATE_DISABLED` ⇒ chỉ ẩn phần chất lượng; `CODEINTEL_AI_REVIEW_DISABLED` ⇒ chỉ ẩn AI; `CODEINTEL_PROFILE_UNKNOWN` **không** ẩn gì (PQ-01).
4. Đọc `settings.get` thất bại (mạng) ⇒ giữ trạng thái trước nếu có, nếu không `unknown` ⇒ fail closed (không hiển thị lối vào).
5. Phiên thiết bị: web không dùng; mobile không gọi kênh nào (đi qua host, SOL-062).

### 2.3 Thẻ cài đặt admin

`CodeIntelSettingsCard` (mẫu `McpTenantSettingsForm.tsx`): hiển thị `effective` (bốn cờ), `tenant` hiện tại, chênh lệch ("công tắc của máy chủ đang tắt" khi `tenant.codeIntelEnabled=true` mà `effective=false`), hai công tắc `codeIntelEnabled` và `qualityGateEnabled` (admin: `currentUser?.role==='admin'`; người thường không thấy nút), gọi `settings.set` với **đúng trường thay đổi**, khoá nút ngay + hiển thị trạng thái sau ~200 ms, lỗi `CODEINTEL_NOT_AUTHORIZED` ⇒ thông điệp inline; sau thành công làm mới `settings.get`. Cấu hình khác (`aiReviewLevel`, `indexPolicy`, quét bảo mật…) thuộc solution của CR sở hữu chúng; thẻ để điểm mở rộng. Vị trí trong Settings (pane/mục) chưa kiểm chứng (câu hỏi mở 2). Lưu ý hợp đồng: `settings.get` **không** trả vai trò người dùng; vai trò lấy từ `currentUser`, quyết định cuối ở service (`admin`).

### 2.4 Fake backend G4 (`createFakeCodeIntelBackend`)

Theo mẫu `createFakeMcpBackend`: bộ xử lý theo kênh có kiểu (`CodeIntelRpcContract` của SOL-050), thao tác kịch bản:

```ts
type FakeCodeIntelBackend = {
  setSettings(p: Partial<Settings['effective']> & { tenant?: Partial<Settings['tenant']> }): void  // phát CODEINTEL_DISABLED khi tắt
  setIndex(p: { overall?: IndexOverall; stale?: boolean; indexedAt?: string; headCommit?: string }): void
  setOverlay(o: Partial<ChangeOverlay>): void; setErd(m: ErdModel | Record<string, ErdModel>): void
  setStorage(m: StorageMap): void; setFindings(f: Finding[]): void; setContractDiff(d: ContractDiff): void
  setReviewState(s: Partial<ReviewState>): void; setQuality(p: …): void                 // cho nhóm quality
  failNext(method: CodeIntelMethod, codeWithSuffix: string): void                         // 'CODEINTEL_TIMEOUT: … | {"inProgress":true,"retryAfterMs":3000}'
  setRole(r: 'admin' | 'user'): void
  pushChanged(p?: Partial<PushChanged>): void; pushProgress(p: Partial<PushReindexProgress>): void
  pushQuality(p: PushQualityProgress | PushQualityFinished | PushGateChanged): void
  dropStream(): void                       // phát changed {resync:true} rồi đóng
  streamCount(): number; calls: { method: string; params: unknown }[]; reset(): void
}
```
Hợp đồng cần tuân (kiểm bằng test bộ xử lý): `args[0]` một object, khoá lạ ⇒ `CODEINTEL_INVALID_PARAMS` (`DisallowUnknownFields`), `len(args)>1` ⇒ lỗi; `tenantId|userId|deviceId|role|devServerId|workspaceRoot|repo|args|command|cypher` bị từ chối (U3); mọi kênh view yêu cầu `projectId`+`worktreeId`; lỗi dạng `CODEINTEL_X: mô tả[ | {json}]` trên `message`, `error.code` luôn `internal` ở dialect session-client; kết quả view là phong bì phẳng (`etag`, `fromCache`, `generatedAt`, `sources`…); `settings.get` luôn trả; cờ tắt ⇒ mọi kênh khác `CODEINTEL_DISABLED`; `subscribe`: ack `null` rồi khung có `event` (native `{type:'push',channel,args:[obj]}`; session-client `streaming:true` không tên kênh, kết thúc `{type:'end'}`); kênh chưa cài ⇒ lỗi như gateway thật ("is not yet implemented"). Dữ liệu fixture dựng theo kiểu §4; test đối chiếu với `backend-go/services/code-intel-service/testdata/agent-results/` khi tệp vàng (CR-070/G1) có, nếu chưa thì ghi "chưa đối chiếu" (D10 của CR: một nguồn sự thật).

### 2.5 Playwright (T2) và Electron smoke

- `tests/playwright.web.config.ts`: `projects: [{name:'mcp-web', testDir:'./e2e/mcp-web', use: chromium}, {name:'code-intel-web', testDir:'./e2e/code-intel-web', use: chromium}]`, giữ `testMatch '**/*.web.e2e.ts'` (không lẫn project Electron `**/*.spec.ts`), một `webServer` Vite 5174 dùng chung; `baseURL` cho project code-intel ưu tiên `CODE_INTEL_E2E_BASE_URL`; script `test:e2e:code-intel-web` (`npx playwright test --config tests/playwright.web.config.ts --project code-intel-web`); giữ `test:e2e:mcp-web` hoạt động nguyên (dùng `--project mcp-web`, **kiểm hồi quy** vì đổi cấu hình dùng chung).
- Specs (khẳng định trên DOM theo `tests/e2e/AGENTS.md`): `flag.web.e2e.ts` (2.2 + thẻ admin: cờ tắt ẩn mọi lối + `streamCount()==0` + `calls` chỉ có `settings.get`; cờ bật có; admin bật được, người thường không thấy nút; tắt giữa chừng bằng `page.clock` tua 60 s và bằng `failNext(..,'CODEINTEL_DISABLED')`; ba mức cờ độc lập); `review-summary.web.e2e.ts` (mở tab `review` từ Source Control, thanh tóm tắt, chip index `stale`/`indexedAt`, thứ tự đọc/tiến độ, bàn phím, không lộ khoá i18n thô); `states.web.e2e.ts` (`TOOL_UNAVAILABLE`, `INDEX_MISSING`, `TIMEOUT` có `inProgress` tự thử lại, `AMBIGUOUS_SYMBOL` + `candidates`, `truncated`, `NOT_AUTHORIZED` trung tính, mất kết nối không treo); `reindex.web.e2e.ts` (tiến trình qua `reindexProgress` có `percent:null`, lần hai `REINDEX_IN_PROGRESS` gắn vào job, `REINDEX_COOLDOWN`, xong thì chip `stale` biến mất nhờ `changed`; `dropStream()` ⇒ `resync` và tải lại); `lenses.web.e2e.ts` (Ảnh hưởng, ERD, Lưu trữ, Hợp đồng/Phát hiện từ fixture; nhãn chứa HTML/`<script>`/Mermaid hiển thị như văn bản — U9; canary DSN không có trong DOM); `review-notes.web.e2e.ts` (SOL-060); `dev-stack.web.e2e.ts` (`@dev-stack`, chỉ khi có `CODE_INTEL_E2E_BASE_URL`: đăng nhập, bật cờ, mở Review, thấy dữ liệu thật; **không chặn PR**, T3).
- Electron smoke (`tests/e2e/code-intel-electron.spec.ts`): `window.api.codeIntel` tồn tại ở preload (chỉ khi `desktop/src/preload/index.ts` đã có — **ngoài `frontend/`, cần chủ sở hữu desktop duyệt**), cờ tắt ẩn lối vào, khẳng định DOM, build `--mode e2e`; **không chặn ở v7**; nếu preload chưa có thì spec `test.skip` có lý do.
- CI: chặn PR (T2) khi PR đụng `frontend/src/renderer/src/**` liên quan code-intel (bộ lọc đường dẫn do workflow quyết; việc workflow thuộc `BE-CV-SOL-073`); frontend chỉ cung cấp lệnh. Chưa kiểm chứng Vite + Chromium chạy được trong runner.

## 3. Quyết định thiết kế

- **Không hook cờ thứ hai**: tái dùng `useCodeIntelSupport` (050) và `useQualityFeatureFlags` (085-01); 073 là lưới kiểm.
- **Fail closed** khi `unknown`.
- **Fake backend bám hợp đồng** (kể cả từ chối khoá lạ, mã lỗi trên `message`) để bắt lệch sớm; `@dev-stack` và T3 là lưới bù.
- **Tắt cờ giữa chừng kiểm bằng hai đường hợp đồng cho phép** (refresh 60 s, `CODEINTEL_DISABLED` ở lần gọi sau), không giả lập push cờ không có trong hợp đồng.
- **`code-intel-web` là project riêng** của cấu hình dùng chung; không phá `test:e2e:mcp-web`.
- **Electron chỉ smoke**.

## 4. Phụ thuộc chéo khu vực

| Cần | Solution đối ứng | Ghi chú |
|---|---|---|
| Cờ, `settings.get|set`, `tenant_settings`, interceptor `FeatureGate`, cache 5 s (PQ-24) | `BE-CV-SOL-073-settings-flag-and-rollout` (sau `BE-CV-SOL-013-authorization-flags-and-audit`) | Frontend dùng fake backend G4 trước; thật sau G3 |
| Đăng ký 46 kênh, giải mã chặt, `codeIntelChannelError`, `SetReadLimit` | `BE-CV-SOL-040-codeintel-channel-foundation` (**G3**) và các solution 040 khác | |
| Tệp vàng `testdata/agent-results/*` (G1) | `BE-CV-SOL-070-collector-golden-contract`; `AG-CV-SOL-070-golden-fixtures-and-parsers` | Đối chiếu fixture frontend |
| Kill-switch agent | `AG-CV-SOL-073-agent-kill-switch` | Không ảnh hưởng frontend |
| Bridge, store, `useCodeIntelSupport`, bộ phân loại lỗi | `FE-CV-SOL-050-types-and-runtime-bridge`, `FE-CV-SOL-050-store-and-query-hooks`, `FE-CV-SOL-050-review-tab-wiring` | G4 bắt đầu cùng 050 |
| Cờ chất lượng/AI | `FE-CV-SOL-085-source-control-quality-notice` (task 085-01) | Dùng lại |
| Lối vào, lens, ghi chú | `FE-CV-SOL-061-…`, `FE-CV-SOL-051..060` | Spec kiểm |
| Preload Electron `codeIntel` | `desktop/src/preload/index.ts` (SOL-050; ngoài `frontend/`) | Cần chủ sở hữu desktop duyệt |
| Agent (`AG-*`) | — | Chỉ kill-switch phía agent |

Thứ tự (§7.1/§7.2): **073-02 (fake backend) làm cùng 050** để mọi lens dùng; 073-01/03/04 sau 050-store; 073-05 theo từng lens; 073-06 sau `BE-CV-SOL-073` (hoặc fake); 073-07 cuối.

## 5. Tiêu chí chấp nhận

- [ ] Ma trận test: cờ tắt/unknown ⇒ không lối vào và **không** gọi kênh ngoài `settings.get`, không `subscribe`; ba mức cờ độc lập; `PROFILE_UNKNOWN` không ẩn.
- [ ] `createFakeCodeIntelBackend` từ chối khoá lạ/`args[1+]`/tham số danh tính, trả lỗi `CODEINTEL_X: …` trên `message`, phát khung push có `event`; test bộ xử lý xanh.
- [ ] `tests/playwright.web.config.ts` có hai project; `test:e2e:code-intel-web` chạy; `test:e2e:mcp-web` không hồi quy.
- [ ] `flag.web.e2e.ts`: cờ tắt không lỗi trang, `streamCount()==0`, `calls` chỉ `settings.get`; bật có lối; admin bật được, người thường không thấy nút; tắt giữa chừng (tua 60 s và `CODEINTEL_DISABLED`).
- [ ] Các spec còn lại xanh trên fake backend; `@dev-stack` bị bỏ qua khi thiếu biến.
- [ ] Thẻ admin: gửi đúng trường thay đổi; lỗi quyền inline; làm mới sau khi lưu.
- [ ] Electron smoke có hoặc `skip` có lý do.
- [ ] Không hex trong component; chuỗi `translate()` đủ 5 locale cho thẻ; không thêm thư viện; không `max-lines` disable; không `components/code-review/*`.

## 6. Kiểm thử

Tự thân: test bộ xử lý fake backend (hợp đồng), ma trận gating (Vitest, `happy-dom`), `CodeIntelSettingsCard` (Testing Library), `code-intel-fixtures` đối chiếu tệp vàng (khi có). E2E Playwright như 2.5 — **chưa chạy** (chưa kiểm chứng Vite/Chromium trong CI). Lệnh: `pnpm --filter orca-frontend test -- src/renderer/src/test-support src/renderer/src/components/settings/code-intel`; `pnpm run test:e2e:code-intel-web`; hồi quy `pnpm run test:e2e:mcp-web`.

## 7. Rủi ro và điểm chưa kiểm chứng

- Fake backend có thể lệch backend thật; `@dev-stack`/T3 là lưới bù; T2 chặn PR nhưng không chứng minh tích hợp.
- Chưa đọc kỹ `mock-orca-ws.ts` (gắn cứng `FakeMcpBackend`?): có thể cần tách giao diện chung hoặc bản mỏng riêng.
- `page.clock` (Playwright ≥ 1.45; repo `^1.59.1`) và tương tác với timer ứng dụng chưa thử.
- Đổi `playwright.web.config.ts` dùng chung có thể làm hỏng `test:e2e:mcp-web`.
- `settings.get` không trả vai trò; hiển thị theo `currentUser.role` có thể lệch quyền thật (service quyết định).
- Vị trí thẻ cài đặt chưa xác định; một số cờ phụ chưa có UI.
- Tính khả thi stack `deploy/dev` trong CI chưa kiểm chứng (thuộc BE).
- Phát hiện cờ bằng `typeof window.api.codeIntel` **cấm** (web bọc Proxy `withFallback`); spec phải bắt hồi quy này.

## 8. Câu hỏi mở

1. Fake backend do SOL-050 dựng khung hay 073-02 tạo hoàn toàn? (Đề xuất: 073-02 sở hữu, 050 chỉ dùng.)
2. Thẻ cài đặt admin đặt ở pane nào của Settings?
3. Có cần push/capability để tắt cờ "trực tiếp" thay vì chờ 60 s/`DISABLED`? (Hợp đồng hiện không có.)
4. Dùng chung `MCP_E2E_BASE_URL` hay `CODE_INTEL_E2E_BASE_URL` (đề xuất: riêng)?
5. Có đưa Electron smoke vào PR sau khi preload có `codeIntel`?

## 9. Tham chiếu

`/opt/repos/orca/docs/crs/v7/quality-rollout/CR-CV-073-e2e-feature-flag-rollout-runbook.md`, `/opt/repos/orca/docs/crs/v7/README.md` (O8, O9, mục 6), `/opt/repos/orca/tests/playwright.web.config.ts`, `/opt/repos/orca/tests/e2e/mcp-web/`, `/opt/repos/orca/tests/e2e/AGENTS.md`, `/opt/repos/orca/frontend/src/renderer/src/test-support/mcp-fake-backend.ts`, `/opt/repos/orca/frontend/src/renderer/src/components/settings/mcp/McpTenantSettingsForm.tsx`, `/opt/repos/orca/package.json`.
