# CR-CV-062 — Mobile: màn tóm tắt Review chỉ đọc (số liệu + danh sách phát hiện)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-062 |
| **Tên** | Màn tóm tắt Review chỉ đọc trên mobile (số liệu thay đổi, rủi ro, danh sách phát hiện), theo mẫu `MobileDiffReviewScreenView`; không vẽ đồ thị |
| **Loại** | Feature |
| **Priority** | ⚪ P2 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-059 (hình dạng `Finding`), CR-CV-036/037 (dữ liệu tóm tắt và phát hiện), CR-CV-040 (kênh `codeIntel.changeOverlay`, `codeIntel.findings`, `codeIntel.status`), CR-CV-072 (che secret); **phía host của mobile**: một method RPC runtime mới (xem 2.2, chưa có trong README v7) |
| **Mở khoá** | Không |
| **Tác động** | `mobile/app/h/[hostId]/review-summary/[worktreeId].tsx` (mới), `mobile/src/components/MobileReviewSummaryScreenView.tsx` và các component con (mới), `mobile/src/session/` (controller, loader, parser, model — mới), điểm vào trong `MobileDiffReviewDrawers.tsx` và `mobile/src/source-control/` (sửa nhỏ); phía host: `desktop/src/main/runtime/rpc/methods/` và `MOBILE_RPC_METHOD_ALLOWLIST` trong `desktop/src/main/runtime/runtime-rpc.ts` |

---

## 1. Bối cảnh và vấn đề

Sau khi agent code, người dùng thường muốn biết nhanh "agent vừa làm gì, có gì đáng lo không" khi đang xa máy. [10 §9](../../../research/view-code/10-frontend-review-ux.md) chốt: giai đoạn đầu mobile chỉ cần **màn tóm tắt chỉ đọc (số liệu + danh sách phát hiện), không vẽ đồ thị**, vì đã có `MobileDiffReviewScreenView` cho review diff.

### 1.1 Mobile gọi backend như thế nào (đã đọc code)

- Mobile **không** nói chuyện với `api-gateway` (kênh `wscompat` `/ws`) mà với **runtime của máy chủ đã ghép đôi** (desktop) qua WebSocket mã hoá đầu-cuối: `mobile/src/transport/rpc-client.ts` xuất `RpcClient` với `sendRequest(method, params?, options?) => Promise<RpcResponse>` và `subscribe(...)`; màn hình lấy client bằng `useHostClient(hostId)` (`transport/client-context.tsx`) cùng `connState` và `useForceReconnect()`. Ví dụ thật ở `mobile/src/session/mobile-diff-review-loaders.ts`: `client.sendRequest('git.branchCompare', { worktree: \`id:${worktreeId}\`, baseRef })`, đọc `response.ok`, `response.error?.code/message`, rồi ép kiểu bằng bộ đọc thủ công (`mobile-diff-review-rpc.ts`: `readString/readNumber/readBoolean`, không dùng zod trên mobile).
- Phía host, `desktop/src/main/runtime/runtime-rpc.ts` chặn mọi method **không có** trong `MOBILE_RPC_METHOD_ALLOWLIST` (dòng ~155) với lỗi `forbidden` ("Method '…' is not available to mobile clients"); method mới phải được định nghĩa bằng `defineMethod` trong `desktop/src/main/runtime/rpc/methods/*.ts` (đăng ký ở `methods/index.ts`) **và** thêm vào allowlist. Hiện **không có** method `codeIntel.*` nào ở host runtime (liệt kê thư mục `rpc/methods`).
- Mobile đã có quy ước xử lý method vắng: `isMobileGitUnavailable(code, message)` (`mobile/src/source-control/mobile-git-status.ts`) coi `forbidden`/`method_not_found`/"not available to mobile clients" là "không khả dụng" và hiện thông báo thay vì lỗi (cũng thấy ở `dictation/mobile-dictation-setup.ts`).
- Phiên bản giao thức: `mobile/src/transport/protocol-version.ts` và `protocol-compat.ts` (`evaluateCompat` có thể chặn bằng `ProtocolBlockScreen`); thêm method mới không buộc nâng `MOBILE_PROTOCOL_VERSION` nếu mobile xử lý `method_not_found` một cách duyên dáng.

**Khoảng trống quan trọng:** phần Go (`code-intel-service`) và kênh `codeIntel.*` nằm sau `api-gateway`; nhưng đường mobile → desktop runtime không đi qua gateway. Chưa kiểm chứng desktop main có kết nối được tới `api-gateway` (ở chế độ runtime `environment`/đa người dùng, renderer dùng `callRuntimeRpc` với target `environment`; main process thì chưa đọc). Vì vậy CR này **không thể chốt đường dữ liệu**; 2.2 nêu lựa chọn và điều kiện tiên quyết.

### 1.2 Mẫu giao diện đã có

- Route: `mobile/app/h/[hostId]/review/[worktreeId].tsx` dùng `useLocalSearchParams`, `useHostClient`, `useForceReconnect`, dựng controller `useMobileDiffReviewController({client, connState, hostId, worktreeId, name, …, onReconnect})` rồi render `MobileDiffReviewScreenView controller onBack`. `MobileDiffReviewScreenView` (152 dòng) dùng `SafeAreaView`, `useResponsiveLayout()` (`isWideLayout`), header riêng (`MobileDiffReviewHeader`), `MobileDiffReviewBody`, ngăn kéo (`MobileDiffReviewDrawers`, `ActionSheetModal`, `BottomDrawer`, `RightDrawer`).
- Trạng thái màn hình là union phân biệt (`ReviewScreenState` ở `session/mobile-diff-review-screen-model.ts`): `loading | ready | unavailable {message} | error {message}`.
- Style tách file (`mobile-diff-review-screen-styles.ts`), màu qua `colors` ở `mobile/src/theme/mobile-theme.ts` (không hex trong component), icon `lucide-react-native`.
- **Mobile không có i18n**: chuỗi hiện là chữ Anh viết thẳng (ví dụ "Review Actions", "Send Notes", "Back"); `translate()` của desktop không có ở mobile (grep `translate(` trong `mobile/src` chỉ khớp thuộc tính SVG; các file như `source-control/hosted-review-copy.ts` ghi rõ là bản sao của desktop "minus i18n").
- Test mobile: Vitest môi trường `node`, chỉ `src/**/*.test.ts` (`mobile/vitest.config.ts`); không có test component React Native; các test hiện có kiểm tra hàm thuần và bộ đọc RPC (`session/*.test.ts`, ví dụ `github-pr-rpc.test.ts` dùng client giả).
- Đường sang diff từng tệp đã có: `buildMobileReviewFileRoute({hostId, worktreeId, worktreeName, filePath, area})` (`source-control/mobile-review-route.ts`) mở `/h/<host>/review/<worktree>?scope=all&file=…&area=…` với `area` có thể là `'branch'`.
- Hàng agent trên mobile: `components/WorktreeAgentRow.tsx` (dùng `RuntimeWorktreeAgentRow` từ `vendor-shared`; có `stateStartedAt`), chưa có hành động theo agent.

## 2. Giải pháp đề xuất

### 2.1 Màn hình

Route mới `mobile/app/h/[hostId]/review-summary/[worktreeId].tsx`, tham số `hostId`, `worktreeId`, `name?`. Thành phần (đều trong `mobile/src/components/`, tách theo mẫu diff review):

```
MobileReviewSummaryScreenView        (SafeAreaView; header + ScrollView/FlatList; RefreshControl kéo để làm mới)
├─ MobileReviewSummaryHeader         (Back, tiêu đề "Review", tên worktree, nút làm mới)
├─ MobileReviewIndexBanner           (chip index: commit, "x phút trước", stale; lỗi/thiếu index)
├─ MobileReviewMetricGrid            (lưới số: file · symbol · luồng · bảng · hợp đồng · chưa có test; Rủi ro + lý do)
├─ MobileReviewFindingsHeader        (đếm theo mức độ; bộ lọc: Tất cả | Cao | Vừa | Chỉ do thay đổi này)
├─ MobileReviewFindingRow × N        (FlatList; icon kind, tiêu đề, mức độ, tệp:dòng, nhãn nguồn gốc; bấm mở rộng)
└─ MobileReviewSummaryStates         (loading / unavailable / error / rỗng)
```

Bố cục điện thoại:

```
┌ ‹  Review · feat/x ─────────── ↻ ┐
│ Index d819812 · 2 phút trước ✓    │
│ Rủi ro: MEDIUM · chạm 3 luồng     │
│ ┌────┬────┬────┐                  │
│ │ 12 │ 38 │  3 │  file/symbol/luồng│
│ │  2 │  1 │  5 │  bảng/hợp đồng/chưa test │
│ └────┴────┴────┘                  │
│ Phát hiện (5)  [Tất cả][Cao][Vừa] │
│ ⛔ Vi phạm lớp  usecase→adapter    │
│    adapter/…/repo.go:88  · do thay đổi này │
│ ⚠ Thiếu tenant_id  …              │
└───────────────────────────────────┘
```

- **Chỉ đọc**: không bỏ qua/đánh dấu xử lý phát hiện, không ghi chú, không gửi cho agent, không mở đồ thị. Những thao tác đó vẫn ở desktop (CR-CV-059/060).
- Bấm một phát hiện: mở rộng tại chỗ (tóm tắt, vị trí). Nút "Xem diff tệp" gọi `router.push(buildMobileReviewFileRoute({ ..., area: 'branch' }))` khi tệp nằm trong thay đổi; nếu không, ẩn nút (không mở tệp ngoài thay đổi ở MVP).
- Rộng (`isWideLayout`): hai cột (số liệu bên trái cố định, danh sách bên phải) dùng cùng `useResponsiveLayout()` như `MobileDiffReviewScreenView`; hẹp: một cột như hình.
- Trạng thái (theo mẫu `ReviewScreenState`): `loading` (chỉ chỉ báo tải; từ ≥3 s hiện nhãn giai đoạn nếu host gửi `stage`), `ready`, `unavailable {message}` (method vắng/cờ tắt/chưa gắn dev server/chưa có index: mỗi trường hợp một thông điệp riêng, có hướng xử lý; không ghi "an toàn"), `error {message}` có nút "Thử lại"; mất kết nối dùng cơ chế `connState` + `onReconnect` như màn diff review. Nếu host trả `stale`, hiện banner nhưng vẫn hiển thị dữ liệu; nếu `truncated`, hiện "Đang hiển thị X / Y".
- Không hiển thị `evidence` (đoạn mã/SQL) trên mobile, chỉ tiêu đề, tóm tắt ngắn và vị trí, để giảm kích thước và rủi ro lộ dữ liệu nhạy cảm qua đường di động.

### 2.2 Đường dữ liệu và method host (điều kiện tiên quyết)

Đề xuất **một method host mới** `codeIntel.reviewSummary` (cho mobile) thay vì để mobile gọi nhiều kênh: host gom `codeIntel.changeOverlay` + `codeIntel.findings` + `codeIntel.status` thành một payload nhỏ. Lý do: mobile hiện chỉ biết RPC của host; mỗi lượt gọi qua mạng di động có độ trễ; payload gọn dễ đọc bằng bộ đọc thủ công.

```ts
// params
{ worktree: `id:${worktreeId}`, scope?: 'branch' }                    // O7: so với merge-base
// result (đề xuất, chưa có trong hợp đồng)
{ available: boolean; reason?: 'flag_off' | 'no_binding' | 'index_missing' | 'tool_unavailable'
  index?: { state: 'missing'|'building'|'ready'|'stale'; indexedCommit?: string; headCommit?: string; indexedAt?: string }
  counts?: { files: number; symbols: number; flows: number; tables: number; contracts: number; uncovered: number }
  risk?: { level: 'LOW'|'MEDIUM'|'HIGH'|'CRITICAL'|'UNKNOWN'; reasons: string[] }
  findings?: { totalOpen: number; bySeverity: { high: number; medium: number; low: number; info: number };
               items: { key: string; kind: string; severity: string; title: string; summary: string;
                        origin: 'introduced_in_scope'|'preexisting'|'unknown'; filePath?: string; startLine?: number;
                        inChangedFiles: boolean }[]; truncated: boolean }   // tối đa 50 mục
  stale?: boolean; truncated?: boolean; headCommit?: string }
```

Điều kiện tiên quyết cần chốt **ngoài CR này**:

1. Host runtime (desktop main) phải có đường tới `code-intel-service`: hoặc qua `api-gateway` bằng thông tin xác thực của người dùng, hoặc nơi khác. **Chưa kiểm chứng**; nếu host chạy local không có backend thì method trả `available:false, reason:'flag_off'` hoặc `no_binding` (không lỗi).
2. Định nghĩa method bằng `defineMethod` trong file host mới (đề xuất `desktop/src/main/runtime/rpc/methods/code-intel.ts`), đăng ký trong `methods/index.ts`, thêm `'codeIntel.reviewSummary'` vào `MOBILE_RPC_METHOD_ALLOWLIST`; kiểm tra quyền/tenant ở backend (CR-CV-013); che dữ liệu nhạy cảm ở backend/host (CR-CV-072).
3. Không có thay đổi `MOBILE_PROTOCOL_VERSION`: mobile cũ không biết method, mobile mới gặp host cũ nhận `method_not_found` và hiện "Host này chưa hỗ trợ tóm tắt review".

Phương án khác, **không khuyến nghị**: mobile mở WebSocket trực tiếp tới `api-gateway` (cần xác thực, vận chuyển và quản lý khoá mới; vượt phạm vi P2).

Nếu điều kiện 1 không thoả, CR này dừng ở mức UI + loader + test với client giả (không phát hành điểm vào), chờ CR phía host.

### 2.3 Mã mobile (tên file cụ thể, mẫu theo diff review)

| File (mới, dưới `mobile/src/`) | Vai trò |
|---|---|
| `session/mobile-review-summary-rpc.ts` | `readMobileReviewSummaryResult(value: unknown): MobileReviewSummary \| null` bằng bộ đọc thủ công (theo `mobile-diff-review-rpc.ts`); bỏ qua trường lạ, giới hạn độ dài chuỗi hiển thị, ép enum về giá trị đã biết (mức lạ → `info`, kind lạ → hiển thị nguyên văn đã cắt) |
| `session/mobile-review-summary-loaders.ts` | `loadMobileReviewSummary(client, worktreeId): Promise<MobileReviewSummaryScreenState>` gọi `client.sendRequest('codeIntel.reviewSummary', { worktree: \`id:${worktreeId}\` })`; ánh xạ `isMobileGitUnavailable`-kiểu: `forbidden`/`method_not_found` → `unavailable`; lỗi khác → `error`; `available:false` → `unavailable` theo `reason` |
| `session/mobile-review-summary-model.ts` | Hàm thuần: bộ lọc mức độ/nguồn gốc, sắp xếp (mức độ rồi `origin`), nhãn rủi ro, định dạng "x phút trước" (có thể dùng `worktree/agent-row-display.ts` `formatTimeAgo`) |
| `session/use-mobile-review-summary-controller.ts` | `useMobileReviewSummaryController({client, connState, hostId, worktreeId, name, onReconnect})` → `{screenState, filter, setFilter, expandedKey, toggleExpanded, refresh, openFileDiff}`; chống phản hồi cũ bằng bộ đếm thế hệ (như `loadGenerationRef` ở controller diff review); tải khi vào màn và khi kéo-làm-mới, **không** polling, không subscribe |
| `components/MobileReviewSummaryScreenView.tsx` (+ `MobileReviewMetricGrid.tsx`, `MobileReviewFindingRow.tsx`, `MobileReviewSummaryHeader.tsx`) | Giao diện như 2.1 |
| `components/mobile-review-summary-styles.ts` | `StyleSheet`, màu từ `colors`; mức độ dùng `statusRed`/`statusAmber`/`textSecondary` kèm biểu tượng và nhãn chữ (không chỉ màu) |
| `app/h/[hostId]/review-summary/[worktreeId].tsx` | Route (mẫu `review/[worktreeId].tsx`) |

Điểm vào (nhỏ, đề xuất, chưa đọc kỹ file đích): thêm một hành động "Review summary" vào `useOverflowActions` của `MobileDiffReviewDrawers.tsx` (menu "Review Actions"), và một chip "Review summary" ở thẻ nhánh của Source Control (`source-control/MobileSourceControlBranchCard.tsx`) khi có thay đổi so với base. Chỉ hiển thị sau khi lần gọi đầu thành công (không phải `unavailable`) hoặc luôn hiện và màn hình giải thích nếu không khả dụng; **đề xuất**: luôn hiện ở overflow (rẻ), ẩn chip ở thẻ nhánh cho tới khi biết khả dụng.

### 2.4 Chỉ báo "agent xong" trên mobile (tuỳ chọn, ngoài MVP)

`RuntimeWorktreeAgentRow` có `stateStartedAt` và trạng thái; có thể thêm sau nút "Review" ở hàng agent đã xong. Không làm trong CR này để tránh mở rộng danh sách hành động mobile.

## 3. Quyết định thiết kế

- **Một method host gọn** (`codeIntel.reviewSummary`) thay vì nhiều kênh: phù hợp mô hình mobile ↔ host runtime và mạng di động.
- **Chỉ đọc, không đồ thị** (10 §9): tránh lặp tính năng phức tạp; thao tác ghi vẫn ở desktop.
- **Không đưa `evidence` xuống mobile**: giảm rủi ro lộ dữ liệu nhạy cảm và kích thước.
- **`unavailable` là trạng thái hợp lệ**, không phải lỗi: đúng quy ước `isMobileGitUnavailable`.
- **Theo mẫu diff review** (controller + loader + parser + view tách file, trạng thái union) để người bảo trì mobile quen thuộc.
- **Không i18n mới**: mobile hiện không có hạ tầng i18n; thêm chuỗi tiếng Anh theo quy ước hiện tại, ghi nhận như nợ chung.
- **Không polling/subscribe**: tránh tải thêm lên pin, mạng và backend; làm mới bằng thao tác của người dùng.

## 4. Tiêu chí chấp nhận

- [ ] Mở từ điểm vào hiển thị màn Review summary của đúng worktree; có nút quay lại; chạy ở bố cục hẹp và rộng.
- [ ] Hiển thị chip index (commit, thời gian, stale), nhãn rủi ro kèm lý do, và lưới số liệu (file, symbol, luồng, bảng, hợp đồng, chưa có test).
- [ ] Danh sách phát hiện lọc được theo mức độ và "chỉ do thay đổi này"; sắp xếp mức độ rồi nguồn gốc; mở rộng tại chỗ; không có thao tác ghi nào.
- [ ] Bấm "Xem diff tệp" mở đúng tệp ở màn diff review hiện có (`area='branch'`) chỉ khi tệp thuộc thay đổi.
- [ ] Trạng thái `loading`, `unavailable` (method vắng, cờ tắt, chưa gắn dev server, chưa có index, mỗi loại một thông điệp), `error` có "Thử lại", mất kết nối, `stale`, `truncated` đều có UI riêng; không khẳng định "an toàn".
- [ ] Host cũ (`method_not_found`/`forbidden`) hiện "chưa hỗ trợ", không văng lỗi; không đổi `MOBILE_PROTOCOL_VERSION`.
- [ ] Bộ đọc `readMobileReviewSummaryResult` từ chối/bỏ qua dữ liệu sai hình dạng mà không ném; chuỗi dài bị cắt.
- [ ] Không tải `evidence`/mã nguồn; không lưu dữ liệu tóm tắt vào bộ nhớ bền của thiết bị.
- [ ] Không có hex cứng trong component (dùng `colors`); mức độ không chỉ phân biệt bằng màu; không đồ thị; không thêm thư viện mới.
- [ ] Phía host (khi thực hiện): method mới nằm trong `MOBILE_RPC_METHOD_ALLOWLIST`, có test của host cho allowlist và cho kết quả khi không có backend.

## 5. Kiểm thử

Vitest môi trường `node`, `src/**/*.test.ts` (chỉ hàm thuần, như các test mobile hiện có):

- `mobile-review-summary-rpc.test.ts`: dữ liệu đúng; thiếu trường; enum lạ; mảng quá dài; chuỗi dài; `available:false` với từng `reason`; không ném với `null`/kiểu sai.
- `mobile-review-summary-loaders.test.ts` với `RpcClient` giả (mẫu `session/github-pr-rpc.test.ts`): `ok` → `ready`; `forbidden`/`method_not_found` → `unavailable`; lỗi khác → `error`; `available:false` ánh xạ đúng; huỷ/phản hồi cũ bị bỏ qua.
- `mobile-review-summary-model.test.ts`: lọc, sắp xếp, nhãn rủi ro, định dạng thời gian.
- Không có test component RN (hạ tầng không hỗ trợ); kiểm tra tay trên thiết bị/giả lập. Có thể mở rộng `mobile/scripts/mock-server-rpc-handlers.ts` để trả `codeIntel.reviewSummary` giả khi chạy mock-server (chưa đọc kỹ, đề xuất).
- Phía host: test cho `defineMethod` mới và allowlist (mẫu `desktop/src/main/runtime/rpc/methods/*.test.ts`). **Chưa chạy**, đây là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Đường dữ liệu host → backend chưa kiểm chứng** (1.1): rủi ro lớn nhất; nếu host không tới được `code-intel-service` thì tính năng không khả dụng trên mobile dù UI sẵn sàng.
- **Hình dạng payload và tên method là đề xuất**; README v7 không có method host cho mobile ("Điều chỉnh hợp đồng").
- Đồng bộ `vendor-shared` (kiểu dùng chung, ví dụ `vendor-shared/shared/types`) giữa desktop và mobile: `metro.config.js` ghi "mobile now keeps its own vendored copy" sau khi tách monorepo, nên là bản sao thủ công; chưa tìm thấy script đồng bộ; CR này tránh phụ thuộc kiểu mới ở `vendor-shared` bằng cách khai báo kiểu cục bộ trong `mobile-review-summary-rpc.ts`.
- **Mobile không có i18n**: chuỗi tiếng Anh cố định; người dùng ngôn ngữ khác thấy tiếng Anh.
- Điểm vào `MobileDiffReviewDrawers`/`MobileSourceControlBranchCard` mới đọc đầu file; vị trí chính xác phải xác nhận khi triển khai.
- Dung lượng dữ liệu và thời gian phản hồi qua mạng di động chưa đo; tóm tắt có thể mất vài giây nếu backend phải gọi agent/CLI (~1,8 s mỗi lần theo README mục 1).
- Che dữ liệu nhạy cảm là trách nhiệm backend/host; UI không có bộ che riêng như desktop CR-CV-058.

## 7. Câu hỏi mở

1. Host runtime của mobile có kết nối được `api-gateway` không, và bằng thông tin xác thực nào (người dùng đang đăng nhập ở desktop)?
2. Có muốn nút Review ở hàng agent đã xong trên mobile (2.4) trong CR này không?
3. Mobile có cần bỏ qua phát hiện (ghi) ở giai đoạn sau, hay vĩnh viễn chỉ đọc?
4. Có thêm hạ tầng i18n cho mobile (ngoài phạm vi) trước khi làm màn này không?
5. Điểm vào ở Source Control hay chỉ ở diff review? (đề xuất cả hai, chip chỉ hiện khi khả dụng.)

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (3.7 kênh, O6, O8, mục 5 đợt 6)
- `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§9 Mobile)
- `/opt/repos/orca/mobile/src/components/MobileDiffReviewScreenView.tsx`, `MobileDiffReviewHeader.tsx`, `MobileDiffReviewDrawers.tsx`, `WorktreeAgentRow.tsx`
- `/opt/repos/orca/mobile/app/h/[hostId]/review/[worktreeId].tsx`
- `/opt/repos/orca/mobile/src/session/use-mobile-diff-review-controller.ts`, `mobile-diff-review-loaders.ts`, `mobile-diff-review-rpc.ts`, `mobile-diff-review-screen-model.ts`, `github-pr-rpc.test.ts`
- `/opt/repos/orca/mobile/src/source-control/mobile-review-route.ts`, `mobile-git-status.ts` (`isMobileGitUnavailable`), `MobileSourceControlBranchCard.tsx`
- `/opt/repos/orca/mobile/src/transport/rpc-client.ts`, `client-context.tsx`, `protocol-version.ts`, `protocol-compat.ts`
- `/opt/repos/orca/mobile/src/theme/mobile-theme.ts`, `mobile/vitest.config.ts`
- `/opt/repos/orca/desktop/src/main/runtime/runtime-rpc.ts` (`MOBILE_RPC_METHOD_ALLOWLIST`, kiểm tra `forbidden`), `desktop/src/main/runtime/rpc/methods/index.ts`, `methods/host-capabilities.ts` (mẫu `defineMethod`)
- Mới: `mobile/app/h/[hostId]/review-summary/[worktreeId].tsx`; `mobile/src/components/{MobileReviewSummaryScreenView,MobileReviewMetricGrid,MobileReviewFindingRow,MobileReviewSummaryHeader}.tsx`, `mobile-review-summary-styles.ts`; `mobile/src/session/{mobile-review-summary-rpc,mobile-review-summary-loaders,mobile-review-summary-model,use-mobile-review-summary-controller}.ts`; `desktop/src/main/runtime/rpc/methods/code-intel.ts`
