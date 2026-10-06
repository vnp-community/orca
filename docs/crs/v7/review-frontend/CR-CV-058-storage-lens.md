# CR-CV-058 — Lens Lưu trữ: StorageMap chỉ đọc (service → kho → topic → secret), đánh dấu thành phần bị đổi, che secret

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-058 |
| **Tên** | Lens Lưu trữ trong màn Review: sơ đồ chỉ đọc service → kho dữ liệu → topic → secret, đánh dấu thành phần bị thay đổi, che secret ngay ở UI |
| **Loại** | Feature |
| **Priority** | ⚪ P2 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-050 (kiểu `StorageMap`, `codeIntelClient.call`/`useCodeIntelQuery` cho kênh `codeIntel.storage`, slice, i18n, `useCodeIntelSupport`), CR-CV-051 (khung, trạng thái/lỗi), CR-CV-057 (nút chuyển sang ERD, mẫu bố cục/xyflow); backend CR-CV-035 (dựng StorageMap, không lộ secret), CR-CV-036 (`changedFiles`), CR-CV-040 (kênh `codeIntel.storage`), CR-CV-072 (kiểm thử che secret) |
| **Mở khoá** | Không |
| **Tác động** | `frontend/src/renderer/src/components/review-map/storage/` (mới), `components/review-map/ReviewLensTabs.tsx` (đăng ký lens, CR-CV-051 sở hữu), `store/slices/code-intel.ts` (khoá `storage`), `i18n/locales/*.json` |

---

## 1. Bối cảnh và vấn đề

Agent thường sửa cấu hình mà người review dễ bỏ sót: thêm topic event bus, đổi DSN, thêm biến môi trường secret, thêm adapter `postgres`/`mysql` cho một service. Nghiên cứu [08 §6](../../../research/view-code/08-views-and-review-models.md) đề xuất `StorageMap {stores, bindings, topics}` ghép từ compose, cấu hình mỗi service, thư mục `adapter/{postgres,mysql,eventbus}` và migration. Lens này chỉ **vẽ** kết quả đó, chỉ đọc.

Hiện trạng frontend (đã đọc code):

- Chưa có lens/biểu đồ lưu trữ nào. Khung `components/review-map/` do CR-CV-050/051 tạo; xyflow đã có trong repo (`components/task/TaskDAGView.tsx`).
- `components/ui/` có `badge`, `toggle-group`, `tooltip`, `popover`, `skeleton`, `scroll-area` (đã đọc thư mục).
- Không có thành phần UI nào hiện tại hiển thị DSN/secret của backend; quy tắc che dữ liệu nhạy cảm là hợp đồng series: README v7 mục 6 ("Không đưa secret vào kết quả/cache/log: chỉ lưu tên khoá, đường dẫn Vault; che giá trị DSN, token, khoá API").
- 08 §6 nêu rõ độ tự động **thấp–trung bình** và `deploy/prod/docker-compose.yml` chỉ có một service `orca-server`; topology thật còn phải xác định (09). UI không được vẽ như thể đó là sự thật đã xác nhận.

## 2. Giải pháp đề xuất

### 2.1 Dữ liệu vào và chỗ hợp đồng cần bổ sung

`StorageMap` theo 08 §6: `Store {id, kind: postgres|mysql|redis|object|volume|vault|queue|other, name, env: dev|prod, owner?, schemas?}`, `Binding {service, store, access: rw|ro, via, configKey?}`, `Topic {name, publisher, subscribers[], payload?}`, trong `CodeIntelResult<StorageMap>`.

Các điểm **chưa có** trong schema 08 mà lens cần (ghi ở "Điều chỉnh hợp đồng", chốt với CR-CV-035/036):

| Cần | Đề xuất | Lý do |
|---|---|---|
| Đánh dấu thay đổi | `sourceFiles: string[]` trên `Store`, `Binding`, `Topic` (đường dẫn tương đối gốc repo), và/hoặc `change?: 'added'\|'removed'\|'modified'` do backend tính | UI giao `sourceFiles` với `ChangeOverlay.changedFiles` được, nhưng không có `sourceFiles` thì không thể biết thành phần nào bị đổi |
| Secret | Tầng secret biểu diễn bằng `Store.kind='vault'` cùng `Binding.configKey` (tên khoá) và đường dẫn Vault; **không** thêm trường giá trị | Không đổi hợp đồng, đủ cho "service → secret" |
| Độ tin cậy | `origin: 'declared' \| 'inferred'` trên mỗi phần tử | 08 §6: phần lớn là suy luận; UI phải gắn nhãn "suy luận" (README v7 mục 7) |
| Môi trường | `Store.env` đã có | Chuyển dev/prod |

Hook (mới) `components/review-map/storage/use-code-intel-storage.ts` bọc `useCodeIntelQuery(worktreeId, 'storage', { env })` (CR-CV-050 2.7): `{ status, stage?, result, error, reload }`; khoá cache theo `queryKey = "storage|<scopeKey>|<paramsHash>"` do slice lo; bỏ qua phản hồi cũ khi đổi `env`. Lens đăng ký `id: 'storage'` trong `REVIEW_LENS_DEFINITIONS` và nhận `ReviewLensProps`.

### 2.2 Cây component và bố cục

```
StorageLens                              (storage/StorageLens.tsx)
├─ StorageToolbar                        (storage/StorageToolbar.tsx)
│    ├─ EnvToggle                        (ui/toggle-group: dev | prod)
│    ├─ ChangedOnlyToggle                ("Chỉ thành phần bị đổi + liên quan")
│    └─ StorageKindFilter                (Badge bật/tắt theo kind)
├─ StorageInferenceNotice                (nhãn "suy luận từ cấu hình" + liên kết nguồn; không phải toast)
├─ StorageCanvas                         (storage/StorageCanvas.tsx; ReactFlow, chỉ đọc)
│    nodeTypes: storageService | storageStore | storageTopic | storageSecret
├─ StorageLegend
└─ (cột phải khung Review) StorageNodeDetail  (storage/StorageNodeDetail.tsx)
```

Bố cục bốn làn cố định (không cần thuật toán, không thêm thư viện, theo O5):

```
 Service            Kho dữ liệu                Topic                  Secret
┌───────────┐ rw  ┌────────────────────┐   ┌────────────────┐    ┌──────────────────┐
│ infra-    │────▶│ ▣ postgres (infra) │   │ ✉ orca.infra…  │    │ 🔑 vault_ssh_role │
│ fleet     │ ro  ├────────────────────┤   └────────────────┘    │   (tên khoá)      │
│           │────▶│ ▣ mysql   (infra)  │      ▲ publish/sub      └──────────────────┘
└───────────┘     └────────────────────┘
```

`storage-layout.ts` (hàm thuần): cột x cố định theo làn; trong mỗi làn sắp theo `name` (tất định) rồi giảm cắt cạnh bằng một lượt barycenter theo hàng xóm ở làn trái. Cạnh: `Binding` (service→store; nhãn `rw/ro`), `Topic` (publisher→topic, topic→subscriber; mũi tên khác loại), `Binding.configKey` (service→secret, nét đứt). Số service không lớn (17 service Go + `agent`, `frontend`), nên không cần ảo hoá; vẫn bật `onlyRenderVisibleElements`.

### 2.3 Đánh dấu thành phần bị đổi

`storage-change-marks.ts` (hàm thuần): `computeStorageChangeMarks(map, changedFiles)` trả `Map<nodeId, 'added'|'modified'|'removed'|'related'>` bằng cách (1) dùng `change` của backend nếu có; (2) nếu không, giao `sourceFiles` với `changedFiles`; (3) đánh `related` cho nút nối trực tiếp với nút bị đổi. Ký hiệu `+ ~ −` kèm nhãn chữ (không chỉ màu). Màu dùng token `main.css` (không hex); thêm/xoá/đổi theo cùng quy ước `--git-decoration-*` như lens ERD (CR-CV-057), `related` dùng viền muted. Nếu thiếu cả hai nguồn: hiện banner "Chưa xác định thành phần nào bị đổi (backend chưa cung cấp tệp nguồn)", không tô gì.

Bộ lọc "Chỉ thành phần bị đổi + liên quan" mặc định bật khi có ít nhất một nút bị đổi; nếu không có nút nào bị đổi hiện thông báo trung tính "Thay đổi này không chạm cấu hình lưu trữ trong phạm vi index", **không** khẳng định "an toàn" (10 §7).

### 2.4 Che secret ở UI (phòng thủ nhiều lớp)

Nguồn chính là backend (CR-CV-035/072: chỉ trả tên khoá, đường dẫn Vault). UI cài thêm một lớp phòng thủ vì dữ liệu đi qua cache và gateway:

- `storage-secret-masking.ts`: hàm thuần `maskSensitiveText(text)` áp lên **mọi** chuỗi tự do từ `StorageMap` trước khi render (`name`, `configKey`, `payload`, `via`, chuỗi chi tiết): che phần userinfo của URL/DSN (`scheme://user:pass@host` → `scheme://•••@host`), các cặp `key=value` với khoá thuộc danh sách nhạy cảm (`password`, `passwd`, `secret`, `token`, `apikey`, `api_key`, `dsn`, `private_key`, `credential`), chuỗi dài giống token (base64/hex ≥ 32 ký tự không có khoảng trắng), khối `-----BEGIN … PRIVATE KEY-----`. Danh sách là đề xuất, phải thống nhất với bộ che của backend (CR-CV-072) để không lệch.
- `SecretNode` chỉ hiển thị **tên khoá** và **đường dẫn Vault** (ví dụ `vault_ssh_role`), nhãn "giá trị không bao giờ hiển thị". **Không có** nút "hiện giá trị" và không có "sao chép giá trị"; chỉ "Sao chép tên khoá".
- Khi `maskSensitiveText` đã thay đổi một chuỗi, hiện biểu tượng `ShieldAlert` nhỏ kèm tooltip "Giá trị nhạy cảm đã bị che ở giao diện — báo cho người triển khai backend" (tín hiệu rằng backend lẽ ra phải che), và ghi `console.warn` không kèm nội dung. Không gửi nội dung gốc đi đâu.
- Không đưa nội dung StorageMap vào `localStorage`, tên tab, hay tiêu đề cửa sổ; cache chỉ ở slice trong bộ nhớ, bị dọn khi xoá worktree.
- Gửi cho agent (CR-CV-060): ghi chú gắn vào nút lưu trữ chỉ chèn tên/id nút đã che, không chèn `payload` thô.

### 2.5 Chi tiết nút và liên kết sang lens khác

`StorageNodeDetail` (cột phải, cắm vào panel chi tiết chung của CR-CV-053): tên, `kind`, `env`, `owner`, `schemas`, danh sách binding vào/ra (`rw/ro`, `via`), publisher/subscriber của topic, nhãn `declared`/`inferred`, `sourceFiles` (mỗi tệp có "Xem diff" nếu thuộc `changedFiles`, theo luồng mở diff của CR-CV-053; "Mở tệp" thường). Với kho `postgres`/`mysql` có `schemas`: nút "Mở ERD của service" gọi `setReviewLens(worktreeId, 'erd')` + `setErdService(owner)` (CR-CV-057). Với topic: liệt kê service publish/subscribe; bấm service → chọn nút service trong canvas.

### 2.6 Trạng thái, lỗi, phím tắt, i18n

| Tình huống | Hiển thị |
|---|---|
| Đang tải | Theo ngưỡng (STYLEGUIDE UX rule 1): <100 ms không đổi; 100 ms–1 s khoá điều khiển; ≥1 s nhãn + `Loader2`; ≥3 s nêu giai đoạn nếu có `stage`; qua SSH trì hoãn hiển thị ~200 ms, khoá nút ngay |
| Không tìm thấy compose/cấu hình | Rỗng inline: "Không tìm thấy cấu hình lưu trữ trong repo này" + liên kết tài liệu; không là lỗi |
| `env=prod` không đủ dữ liệu | Notice "Topology prod chưa xác định đầy đủ (chỉ thấy một service `orca-server` trong compose)" — nội dung thật lấy từ `warnings`/`origin`, không cứng trong UI |
| `stale` | Banner chung của khung Review, vẫn hiển thị |
| Lỗi mã chuẩn `CODEINTEL_*` | Persistent inline, có "Thử lại" |
| Dev server offline | Dùng trạng thái `offline` của khung Review (CR-CV-051); không dùng `ConnectionStatusBanner` |

Phím tắt (khi tiêu điểm trong lens, ngoài ô nhập): `f` vừa khung, `Esc` bỏ chọn; không có phím sửa đổi. Chuỗi qua `translate()`, khoá tiền tố `auto.components.review.map.storage.` đủ 5 locale `en/es/ja/ko/zh`. Chỉ biến CSS, reduced-motion tắt animation khung nhìn. Chỉ đi qua `codeIntelClient`/`useCodeIntelQuery` nên chạy ở Electron và web (Electron local còn phụ thuộc preload của CR-CV-050).

## 3. Quyết định thiết kế

- **Chỉ đọc hoàn toàn**: không có thao tác ghi cấu hình nào từ lens (ngoài phạm vi và rủi ro với secret).
- **Bốn làn cố định** thay vì thuật toán đồ thị: đơn giản, ổn định, đủ cho vài chục nút; tránh thêm thư viện (O5).
- **Che hai lớp**: backend là chính, UI là phòng thủ; không có đường "hiện giá trị".
- **Gắn nhãn suy luận** thay vì vẽ như sự thật: StorageMap có độ tự động thấp–trung bình (08 §6).
- **Không khẳng định an toàn** khi không thấy thay đổi.

## 4. Tiêu chí chấp nhận

- [ ] Lens Lưu trữ có trong `ReviewLensTabs` khi `useCodeIntelSupport().state === 'enabled'`, hoạt động ở Electron và web.
- [ ] Hiển thị bốn làn service → kho → topic → secret với cạnh `rw/ro`, publish/subscribe, và khoá secret.
- [ ] Chuyển được dev/prod; kho có nhãn kind (icon lucide + chữ) và `schemas` nếu có.
- [ ] Thành phần bị đổi được đánh dấu `+ ~ −` kèm nhãn chữ, không chỉ màu; có bộ lọc "chỉ thành phần bị đổi + liên quan"; khi không có dữ liệu `sourceFiles`/`change` hiện banner nêu rõ.
- [ ] Phần tử `origin='inferred'` có nhãn "suy luận"; không có câu nào khẳng định "an toàn/không thay đổi" ngoài "không phát hiện trong phạm vi index".
- [ ] Không nơi nào hiển thị giá trị secret; không có nút hiện/sao chép giá trị; chuỗi giống DSN/token/khoá riêng bị che bởi `maskSensitiveText` với icon cảnh báo nhỏ; test chứng minh với dữ liệu giả có secret.
- [ ] Kho `postgres/mysql` có nút mở ERD của service sở hữu (chuyển lens + service); topic liệt kê publisher/subscriber bấm được.
- [ ] Trạng thái tải/rỗng/lỗi/offline/stale đúng bảng 2.6; lỗi dạng inline có "Thử lại", không toast.
- [ ] Không hex cứng; chuỗi `translate()` đủ 5 locale; không dùng `components/code-review/*`; không thêm thư viện; không thêm `max-lines` disable.

## 5. Kiểm thử

Vitest (+ Testing Library cho component, `renderToStaticMarkup` cho tĩnh):

- Hàm thuần: `storage-layout` (tất định, làn đúng, không chồng nút), `storage-change-marks` (ưu tiên `change` backend; giao `sourceFiles`∩`changedFiles`; `related`; thiếu dữ liệu → rỗng và cờ "không xác định"), `storage-secret-masking` (bảng test: DSN có mật khẩu, `password=…`, token dài, khối khoá riêng, chuỗi vô hại không bị che nhầm như UUID/đường dẫn Vault, idempotent khi che hai lần).
- Component: `StorageSecretNode` không render bất cứ giá trị nào ngoài tên khoá/đường dẫn (kể cả khi dữ liệu giả nhét giá trị vào `payload`); `StorageNodeDetail` (nút mở ERD, mở diff chỉ khi tệp đổi); `StorageLens` (rỗng, `prod` thiếu dữ liệu, lỗi + thử lại).
- Hook: bỏ qua phản hồi cũ khi đổi `env`.
- Bảo mật (phối hợp CR-CV-072): fixture có secret giả đi qua toàn bộ đường UI không xuất hiện trong DOM, `localStorage`, hay log (`console` spy). **Chưa chạy**, đây là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- Schema `StorageMap` thiếu `sourceFiles`/`change`/`origin` (2.1); không có chúng, tính năng "đánh dấu bị đổi" suy giảm. Cần chốt với CR-CV-035.
- Danh sách khoá/mẫu che là đề xuất; che thừa gây khó đọc, che thiếu gây lộ. Cần thống nhất và kiểm thử cùng CR-CV-072.
- Chưa đọc cấu hình thật của các service và `deploy/*/docker-compose.yml` để biết hình dạng dữ liệu StorageMap thực; mọi ví dụ ở trên lấy từ 08 §6.
- Topology prod chưa xác định (08 §6/09); lens có thể gần như trống ở `env=prod`.
- Số nút chưa đo; giả định vài chục nút, không cần ảo hoá.
- Phụ thuộc cả CR-CV-035 (P2) nên lens này đến muộn (đợt 6); khi chưa có backend thì lens ẩn khỏi `ReviewLensTabs` (kiểm tra capability từ CR-CV-050), không hiện tab chết.

## 7. Câu hỏi mở

1. Backend trả `change`/`sourceFiles` hay UI tự giao? (Khuyến nghị trả `sourceFiles` và để UI giao với phạm vi hiện tại, vì phạm vi review đổi theo người dùng.)
2. Danh sách mẫu che chung nằm ở đâu để hai phía dùng cùng một nguồn (proto, hay mỗi phía một bản có test đối chiếu)?
3. Có cần hiển thị `env=prod` ở MVP hay chỉ dev khi topology prod chưa rõ?
4. Có cho "Sao chép tên khoá" và đường dẫn Vault không (đường dẫn Vault tự nó có thể coi là thông tin nhạy cảm theo chính sách nào)?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 6 che secret, mục 7, O5, O8)
- `/opt/repos/orca/docs/research/view-code/08-views-and-review-models.md` (§6 `StorageMap`)
- `/opt/repos/orca/docs/research/view-code/09-external-inputs-required.md`
- `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§5, §7)
- `/opt/repos/orca/guides/STYLEGUIDE.md`
- `/opt/repos/orca/frontend/src/renderer/src/components/task/TaskDAGView.tsx` (xyflow)
- `/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-050-review-frontend-foundation.md`, `CR-CV-051-review-workspace-shell.md` (`REVIEW_LENS_DEFINITIONS`, `ReviewLensProps`, trạng thái chung)
- `/opt/repos/orca/frontend/src/renderer/src/i18n/no-top-level-translate.test.ts`
- Mới: `components/review-map/storage/{StorageLens,StorageToolbar,StorageCanvas,StorageNodeDetail,StorageLegend}.tsx`, `storage-layout.ts`, `storage-change-marks.ts`, `storage-secret-masking.ts`, `use-code-intel-storage.ts`
