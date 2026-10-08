# FE-CV-SOL-062: Mobile: màn tóm tắt Review chỉ đọc (kèm host method desktop `codeIntel.reviewSummary`)

> 🚧 In Progress (rà soát 2026-10-07): 4/6 task DONE (062-01, 02, 05, 06), 2 PARTIAL (062-03 hook chưa test, 062-04 UI chưa typecheck/chạy thiết bị). Priority P2 (đợt 6). Port mặc định `unavailable` tới khi O-4 chốt.

**CR:** [CR-CV-062](../../../../../../docs/crs/v7/review-frontend/CR-CV-062-mobile-review-summary.md)
**Area:** mobile (`mobile/`) + host runtime desktop (`desktop/src/main/runtime/` — **ngoài `frontend/`, cần chủ sở hữu desktop duyệt**)
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (**§8 `MobileReviewSummary`**, U8 phiên thiết bị bị từ chối ở mọi kênh `codeIntel.*` trừ `settings.get`; §4.3 `RiskAssessment`/`Finding`; §3.1 kênh mà host gọi), [CONTRACT-codeintel-proto-and-data-map.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (**PQ-36, PQ-06, PQ-32**; §7 đợt 6; §8.1 dòng chạm `desktop/`; §8.2; §9 **O-4**; §10 dòng `MOBILE_RPC_METHOD_ALLOWLIST`). **Tham chiếu:** [api/mobile-rpc-catalog.md](../../../../api/mobile-rpc-catalog.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md).

## 1. Trạng thái hiện tại (re-verify)

**Đã xác minh trong phiên này:**
- `desktop/src/main/runtime/runtime-rpc.ts`: `const MOBILE_RPC_METHOD_ALLOWLIST = new Set([...])` ở dòng 155 (không export), kiểm tra `device.scope === 'mobile' && !…has(request.method)` ở dòng 1086; `rpc/methods/` có nhiều file `defineMethod` (mẫu `host-capabilities.ts`: `defineMethod({name, params, handler})`, mảng `*_METHODS: RpcMethod[]`), `methods/index.ts` và `mobile.test.ts` tồn tại; chưa có method `codeIntel.*`.
- `mobile/src/source-control/mobile-git-status.ts:93` `isMobileGitUnavailable(code, message)`; tồn tại `mobile/app/h/[hostId]/review/[worktreeId].tsx`, `mobile/src/components/MobileDiffReviewScreenView.tsx`, `mobile/src/session/mobile-diff-review-{loaders,rpc}.ts`; `specs/frontend/api/mobile-rpc-catalog.md`.
**Theo CR-CV-062 (chưa kiểm lại):** `RpcClient.sendRequest(method, params?)`, `useHostClient`, `ReviewScreenState` union, mobile không có i18n, Vitest môi trường `node` chỉ `src/**/*.test.ts`, `buildMobileReviewFileRoute`, `MobileDiffReviewDrawers.tsx`, `MobileSourceControlBranchCard.tsx`.

### Lệch giữa CR và hợp đồng (hợp đồng thắng)

| # | CR-062 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | `findings.bySeverity {high, medium, low, info}` | `{error, warning, info}` (+ `high|medium|low` tuỳ chọn; PQ-06/UI-API §8) | Mobile chỉ đọc `error|warning|info`; bỏ qua `high|medium|low` |
| 2 | `items[].origin: introduced_in_scope|preexisting|unknown` | `introduced|touched|preexisting|unknown` | Bốn giá trị; "Chỉ do thay đổi này" = `introduced` |
| 3 | `risk.level` gồm `UNKNOWN`; `reasons: string[]` | Giữ (chữ HOA, PQ-32); `reasons` là chuỗi do host dựng | Giữ; enum lạ ép về `UNKNOWN` |
| 4 | `reason` ∈ `flag_off|no_binding|index_missing|tool_unavailable`, `available:false` | Giữ + `reason` **tuỳ chọn** | `available:false` không `reason` ⇒ "Tóm tắt chưa khả dụng" chung |
| 5 | Host gom `changeOverlay`+`findings`+`status` | Đường host → gateway **chưa chốt (O-4)**; `title`/`summary` do host dựng từ `titleKey`+`params`; ≤ 50 mục; không `evidence`, không ghi | Host method dựng qua **cổng (port) tiêm được**; mặc định `unavailable` tới khi O-4 chốt |
| 6 | Phiên thiết bị mobile gọi gateway? | U8: `DeviceID≠""` bị `CODEINTEL_NOT_AUTHORIZED` ở mọi kênh trừ `settings.get` | Mobile **không** đi qua gateway; host dùng thông tin xác thực của người dùng desktop (O-4), không dùng phiên thiết bị |
| 7 | Mobile "không i18n" | Giữ | Chuỗi tiếng Anh cố định (nợ chung); ngoài `translate()` |
| 8 | Chữ "high/medium" ở UI ví dụ | — | Nhãn UI: Error/Warning/Info |

## 2. Giải pháp

### 2.1 Cây file

```
mobile/app/h/[hostId]/review-summary/[worktreeId].tsx          (mới) route
mobile/src/session/mobile-review-summary-rpc.ts                (mới) readMobileReviewSummaryResult (parser thủ công)
mobile/src/session/mobile-review-summary-loaders.ts            (mới) loadMobileReviewSummary
mobile/src/session/mobile-review-summary-model.ts              (mới) lọc/sắp xếp/nhãn (hàm thuần)
mobile/src/session/use-mobile-review-summary-controller.ts     (mới)
mobile/src/components/{MobileReviewSummaryScreenView,MobileReviewSummaryHeader,MobileReviewMetricGrid,MobileReviewFindingRow}.tsx   (mới)
mobile/src/components/mobile-review-summary-styles.ts          (mới)
mobile/src/components/MobileDiffReviewDrawers.tsx, source-control/MobileSourceControlBranchCard.tsx   (sửa nhỏ: điểm vào)
desktop/src/main/runtime/rpc/methods/code-intel.ts (+ .test.ts)  (mới; ngoài frontend/, cần chủ sở hữu desktop duyệt)
desktop/src/main/runtime/rpc/methods/index.ts                   (sửa: đăng ký)
desktop/src/main/runtime/runtime-rpc.ts                         (sửa: thêm 'codeIntel.reviewSummary' vào MOBILE_RPC_METHOD_ALLOWLIST)
mobile/scripts/mock-server-rpc-handlers.ts                      (sửa tuỳ chọn: trả reviewSummary giả; chưa đọc kỹ)
```

### 2.2 Host method `codeIntel.reviewSummary`

`defineMethod({name:'codeIntel.reviewSummary', params: {worktree: string (dạng 'id:<worktreeId>'), scope?: 'branch'}, handler})`. Handler: tách `worktreeId`; gọi `CodeIntelSummaryPort` (giao diện tiêm được, mới) để lấy `ChangeOverlay(detail:'full')`, `findings {limit:50, scope:'changed'}`, `status`; ánh xạ sang `MobileReviewSummary` (UI-API §8): `counts` từ `limits.totalCounts` + độ dài mảng, `risk` từ `RiskAssessment` (`reasons` = chuỗi dựng từ `messageKey`+`params`), `findings.items` ≤ 50 với `title`/`summary` dựng từ `titleKey`+`params` bằng bảng mẫu tiếng Anh (khoá lạ ⇒ dùng `rule`), `inChangedFiles` từ `changedFiles`, không `evidence`, không ghi. Che chuỗi tự do bằng bộ che ở host (cùng mẫu với `maskSensitiveText`; nơi đặt O-16).
- **Port mặc định** (`NoGatewayCodeIntelPort`) trả `{available:false}` — đúng PQ-36: "trước khi chốt O-4, mobile ở trạng thái `unavailable`". Khi O-4 chốt (desktop main tới được `api-gateway` bằng credential nào), thay port bằng bản gọi WS gateway; không đổi handler/shape. Cờ tắt (`CODEINTEL_DISABLED`) ⇒ `{available:false, reason:'flag_off'}`; `NO_BINDING` family ⇒ `no_binding`; `INDEX_MISSING` ⇒ `index_missing`; `TOOL_UNAVAILABLE` ⇒ `tool_unavailable`; lỗi khác ⇒ ném lỗi RPC thường (mobile hiện `error`).
- **Allowlist:** thêm đúng một chuỗi `'codeIntel.reviewSummary'` vào `MOBILE_RPC_METHOD_ALLOWLIST` (giữ thứ tự chữ cái); không thêm bất kỳ method `codeIntel.*` nào khác (mobile không được gọi `reindex`, `reviewState.save`…). Không đổi `MOBILE_PROTOCOL_VERSION`: host cũ trả `forbidden`/`method_not_found` ⇒ mobile hiện `unavailable`.
- Trước khi sửa `runtime-rpc.ts`/`methods/index.ts`: chạy `gitnexus_impact` (báo blast radius), `detect_changes` trước commit.

### 2.3 Mobile: parser, loader, controller

- `readMobileReviewSummaryResult(value: unknown): MobileReviewSummary | null` bằng `readString/readNumber/readBoolean` (mẫu `mobile-diff-review-rpc.ts`, không zod): bỏ qua trường lạ, ép enum về giá trị đã biết (`risk.level` lạ ⇒ `UNKNOWN`; `severity` lạ ⇒ `info`; `origin` lạ ⇒ `unknown`), cắt chuỗi (title ≤ 200, summary ≤ 400), tối đa 50 mục, không ném với `null`/kiểu sai.
- `loadMobileReviewSummary(client, worktreeId)`: `client.sendRequest('codeIntel.reviewSummary', {worktree: \`id:${worktreeId}\`})`; `ok` ⇒ parse ⇒ `ready` hoặc `unavailable` theo `available/reason`; `forbidden`/`method_not_found`/"not available to mobile clients" (qua `isMobileGitUnavailable`) ⇒ `unavailable` "Host này chưa hỗ trợ tóm tắt review"; lỗi khác ⇒ `error` + Thử lại.
- `useMobileReviewSummaryController`: tải khi vào màn và khi kéo-làm-mới; **không polling/subscribe**; bộ đếm thế hệ chống phản hồi cũ (mẫu `loadGenerationRef`); mất kết nối dùng `connState` + `onReconnect`.
- `mobile-review-summary-model.ts`: lọc (`error|warning|info`, "chỉ do thay đổi này"), sắp (severity rồi origin), nhãn rủi ro, "x phút trước".

### 2.4 Màn hình (chỉ đọc, không đồ thị)

```
┌ ‹  Review · feat/x ─────────── ↻ ┐
│ Index d819812 · 2 phút trước     │
│ Rủi ro: MEDIUM · chạm 3 luồng    │
│ ┌────┬────┬────┐                 │
│ │ 12 │ 38 │  3 │ files/symbols/flows
│ │  2 │  1 │  5 │ tables/contracts/untested
│ └────┴────┴────┘                 │
│ Phát hiện (5) [Tất cả][Error][Warning][Chỉ do thay đổi này]
│ ⛔ Error · usecase→adapter        │
│    adapter/…/repo.go:88 · do thay đổi này
└──────────────────────────────────┘
```
Trạng thái theo `ReviewScreenState`: `loading` (từ ≥ 3 s nhãn giai đoạn nếu host gửi), `ready`, `unavailable {message}` (mỗi `reason` một thông điệp có hướng xử lý; không viết "an toàn"), `error` + "Thử lại"; `stale` ⇒ banner nhưng vẫn hiển thị; `truncated` ⇒ "Đang hiển thị X / Y". Bấm mục ⇒ mở rộng tại chỗ; "Xem diff tệp" gọi `router.push(buildMobileReviewFileRoute({..., area:'branch'}))` chỉ khi `inChangedFiles`. Rộng (`isWideLayout`) hai cột. Màu từ `colors` (`mobile-theme.ts`), không hex; severity luôn có icon + nhãn chữ. Không hiển thị `evidence`; không lưu dữ liệu vào bộ nhớ bền. Điểm vào: hành động "Review summary" ở overflow của `MobileDiffReviewDrawers.tsx` (luôn hiện) và chip ở thẻ nhánh Source Control (chỉ sau khi biết khả dụng) — vị trí chính xác cần xác nhận khi làm.

## 3. Quyết định thiết kế

- **Một method host gọn** thay vì mobile gọi nhiều kênh (độ trễ mạng di động, parser thủ công).
- **Chỉ đọc, không đồ thị**; thao tác ghi ở desktop (SOL-059/060).
- **Cổng tiêm được cho đường host → gateway** vì O-4 chưa chốt; mặc định `unavailable` (đúng PQ-36).
- **`unavailable` là trạng thái hợp lệ**, không phải lỗi.
- **Không `evidence`, không lưu bền**; che chuỗi ở host.
- **Theo mẫu diff review** để người bảo trì mobile quen thuộc; không i18n mới.

## 4. Phụ thuộc chéo khu vực

| Cần | Solution đối ứng | Ghi chú |
|---|---|---|
| Kênh mà host gọi (`changeOverlay`, `findings`, `status`) | `BE-CV-SOL-040-codeintel-view-channels` (cổng nền **G3**: `BE-CV-SOL-040-codeintel-channel-foundation`); `BE-CV-SOL-036-change-overlay-pipeline`; `BE-CV-SOL-037-structure-findings-and-dismissals` | |
| Quyền/phiên thiết bị bị từ chối | `BE-CV-SOL-013-authorization-flags-and-audit` (U8) | Host không dùng phiên thiết bị |
| Credential desktop main → gateway | **O-4 (chưa chốt)** | Chặn bản thật; fake port để test |
| Hình dạng `Finding`, `RiskAssessment` | `FE-CV-SOL-059-contract-lens-and-findings`, `FE-CV-SOL-050-types-and-runtime-bridge` | Mobile khai báo kiểu cục bộ (không dùng `vendor-shared`) |
| Cờ tenant | `BE-CV-SOL-073-settings-flag-and-rollout` | `flag_off` |
| Bản đầu dùng fake: mock-server mobile | `FE-CV-SOL-073-flag-gating-and-web-e2e` (fake backend G4 chỉ cho web) | Mobile có mock-server riêng |
| Agent | `AG-*`: — | |

Thứ tự: 050 (kiểu) → 059 (Finding) → **062** (đợt 6). Phía host chặn bởi O-4.

## 5. Tiêu chí chấp nhận

- [ ] Từ điểm vào mở đúng màn của worktree; nút Back; hẹp và rộng.
- [ ] Hiển thị chip index, rủi ro + lý do, lưới số liệu; danh sách phát hiện lọc theo `error|warning|info` và "chỉ do thay đổi này"; mở rộng tại chỗ; không thao tác ghi.
- [ ] "Xem diff tệp" mở màn diff review hiện có (`area='branch'`) chỉ khi `inChangedFiles`.
- [ ] `loading`, `unavailable` (host cũ, cờ tắt, chưa binding, chưa index, công cụ thiếu, port mặc định), `error` + thử lại, mất kết nối, `stale`, `truncated` đều có UI; không "an toàn".
- [ ] Host cũ (`method_not_found`/`forbidden`) ⇒ "chưa hỗ trợ", không văng lỗi; không đổi `MOBILE_PROTOCOL_VERSION`.
- [ ] Parser không ném với dữ liệu sai hình dạng; enum lạ ép đúng; chuỗi/mảng bị cắt.
- [ ] Không tải `evidence`/mã nguồn; không lưu bền; không hex; không đồ thị; không thêm thư viện.
- [ ] Host: chỉ **một** method mới vào `MOBILE_RPC_METHOD_ALLOWLIST`; test chứng minh; port mặc định trả `available:false`; không có `codeIntel.*` nào khác từ mobile.

## 6. Kiểm thử

Mobile (Vitest môi trường `node`, chỉ `src/**/*.test.ts`): `mobile-review-summary-rpc.test.ts` (đúng/thiếu trường/enum lạ/mảng dài/chuỗi dài/mỗi `reason`/`null`), `mobile-review-summary-loaders.test.ts` với `RpcClient` giả (mẫu `session/github-pr-rpc.test.ts`: ok, forbidden, method_not_found, lỗi khác, phản hồi cũ), `mobile-review-summary-model.test.ts`. Không có test component RN (hạ tầng không hỗ trợ); kiểm tay trên thiết bị/giả lập. Host (`desktop/src/main/runtime/rpc/methods/code-intel.test.ts`, mẫu `mobile.test.ts`): ánh xạ overlay/findings → `MobileReviewSummary` với port giả, cắt 50, không `evidence`, mã lỗi → `reason`, port mặc định; test allowlist (`'codeIntel.reviewSummary'` ∈ tập, các `codeIntel.*` khác ∉). **Chưa chạy**; kế hoạch.

## 7. Rủi ro và điểm chưa kiểm chứng

- **O-4 chưa chốt**: nếu desktop main không tới được gateway thì tính năng không khả dụng trên mobile dù UI sẵn sàng.
- Hình dạng payload/tên method là đề xuất hợp đồng nhưng chưa có host cài đặt; bảng mẫu `titleKey` → tiếng Anh ở host cần đồng bộ với `titleKey` backend (không đóng).
- Đồng bộ `vendor-shared`: tránh bằng kiểu cục bộ ở mobile.
- Mobile không có i18n: người dùng ngôn ngữ khác thấy tiếng Anh.
- Điểm vào `MobileDiffReviewDrawers`/`MobileSourceControlBranchCard` mới đọc đầu file (theo CR).
- Độ trễ qua mạng di động và chi phí overlay phía host chưa đo.
- Chỉnh `desktop/` cần chủ sở hữu desktop duyệt; che dữ liệu nhạy cảm là trách nhiệm backend + host.

## 8. Câu hỏi mở

1. (O-4) Desktop main tới `api-gateway` bằng credential nào?
2. Có muốn nút Review ở hàng agent xong trên mobile (ngoài MVP)?
3. Mobile có cần bỏ qua phát hiện (ghi) về sau, hay vĩnh viễn chỉ đọc?
4. Có thêm hạ tầng i18n cho mobile trước màn này?
5. Điểm vào Source Control hay chỉ diff review?

## 9. Tham chiếu

`/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-062-mobile-review-summary.md`, `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§9), `/opt/repos/orca/desktop/src/main/runtime/runtime-rpc.ts`, `/opt/repos/orca/desktop/src/main/runtime/rpc/methods/host-capabilities.ts`, `/opt/repos/orca/mobile/src/session/mobile-diff-review-loaders.ts`, `/opt/repos/orca/mobile/src/source-control/mobile-git-status.ts`, `/opt/repos/orca/specs/frontend/api/mobile-rpc-catalog.md`.
