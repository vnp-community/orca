# SOL-FE2E-004 — Quyết định "Share this Orca server" — Kết luận: (a), no-op

**CR:** [CR-FE2E-004](../../../../../docs/crs/v2/frontend-e2ee/CR-FE2E-004-share-link-decision.md)
**TDD Refs:** Không có mục TDD nào mô tả `RuntimeEnvironmentsPane`/`RuntimePairingUrlGenerator` — đây là 1 khoảng trống tài liệu hoá tồn tại từ trước (ngoài phạm vi CR series này để lấp), quyết định dựa hoàn toàn vào đọc code thật.
**Approach:** Investigation — trả lời câu hỏi mở bằng bằng chứng, không suy đoán.

---

## 1. Câu trả lời: **(a)** — "Share this Orca server" chỉ chạy trên Desktop, không bao giờ chạy trong bất kỳ web client nào (cả use case A lẫn B)

### Bằng chứng

**File:** `frontend/src/renderer/src/components/settings/Settings.tsx:1541-1545`
```tsx
<RuntimeEnvironmentsPane
  settings={settings}
  switchRuntimeEnvironment={...}
  canGeneratePairingUrl={!isWebClient}
  allowLocalRuntime={!isWebClient}
/>
```

**File:** `frontend/src/renderer/src/components/settings/Settings.tsx:299`
```tsx
const isWebClient = isWebClientLocation()
```

**File:** `frontend/src/renderer/src/lib/web-client-location.ts` (toàn bộ file)
```ts
export function isWebClientLocation(): boolean {
  if (typeof window === 'undefined') {
    return false
  }
  return (
    Boolean((window as unknown as { __ORCA_WEB_CLIENT__?: boolean }).__ORCA_WEB_CLIENT__) ||
    window.location.pathname.endsWith('/web-index.html')
  )
}
```

`isWebClientLocation()` là 1 check **generic** — đúng với **mọi** trang phục vụ qua `web-index.html` (tức là toàn bộ web bundle, cả use case A và B dùng chung 1 bundle này, chỉ khác nhau ở nhánh runtime bên trong `main.tsx`, không phải build riêng — xem SOL-FE2E-001 mục 4). Không có cách nào phân biệt "use case A" với "use case B" ở tầng flag này — cả hai đều có `isWebClient === true`.

→ `canGeneratePairingUrl={!isWebClient}` = `false` cho **cả 2** use case A và B. `RuntimeEnvironmentsPane.tsx:1163` (`{canGeneratePairingUrl ? (...) : null}`) ẩn hoàn toàn khối "Advertise this app as a server" / "Share this Orca server" khi `false`.

**Suy luận kiến trúc (khớp logic sản phẩm):** người **tạo** link chia sẻ luôn là 1 Desktop app (Electron) đang chạy — người **nhận** link đó mới là browser/mobile dùng nó để pair vào. Đây chính xác là ngữ cảnh "Desktop Pair Code sharing" (use case B) nhìn từ phía người NHẬN — nhưng phía TẠO link không bao giờ là web client. Vì vậy `WebConnect.tsx`'s thông báo lỗi (*"open the browser access link from Settings → Runtime Environments → Share this Orca server → New Link"*) đang hướng dẫn người dùng **quay lại ứng dụng Desktop** để tạo link, không phải làm điều đó ngay trong trình duyệt.

## 2. Áp dụng mục 3 của CR (kết luận "no-op")

- [x] Đóng CR này với kết luận: **"no-op — share-link chỉ chạy trên Desktop (Electron), không tồn tại trong bất kỳ web client nào (use case A hay B) — CR-FE2E-002/003 không cần thay đổi gì để né tránh tính năng này."**
- [x] Cập nhật comment/doc tại `RuntimeEnvironmentsPane.tsx` — xem mục 3 bên dưới (đề xuất, chưa áp dụng — thuộc phạm vi CR-FE2E-002 khi implement).

## 3. Đề xuất patch nhỏ (tuỳ chọn, không bắt buộc để đóng CR)

Thêm 1 dòng comment tại chỗ dùng `canGeneratePairingUrl` để người đọc sau không phải lần lại investigation này:

```diff
+ // Why: "Share this Orca server" only makes sense from the app that OWNS a
+ // runtime to advertise — Desktop only. isWebClient is true for both the
+ // multi-user backend path AND the bare Desktop-pair-code path (same
+ // web-index.html bundle, see docs/crs/v2/frontend-e2ee/), so this hides the
+ // section in both, not just the multi-user one. Confirmed via
+ // specs/frontend/crs/frontend-e2ee/solutions/SOL-FE2E-004.
  canGeneratePairingUrl={!isWebClient}
```

File: `frontend/src/renderer/src/components/settings/Settings.tsx`.

## 4. Tác động tới CR-FE2E-002/003

**Không có.** Vì tính năng chưa bao giờ reachable từ web client, việc bỏ `PairCodeFallback` khỏi `LoginPage` (CR-002) và code-split `WebConnect`/E2EE khỏi bundle multi-user (CR-003) **không đụng chạm** gì tới `RuntimeEnvironmentsPane`/`RuntimePairingUrlGenerator` — 2 nhóm code hoàn toàn tách biệt theo `isWebClient` flag, không phải theo use case A/B mà CR-002/003 đang thao tác.

## Acceptance Criteria — Kết quả

| # | Criteria | Kết quả |
|---|---|---|
| AC-1 | Câu hỏi mục 1 có câu trả lời bằng văn bản trước khi CR-FE2E-002 merge | ✅ (a), có bằng chứng code cụ thể |
| AC-2 | Nếu (b): kế hoạch không để CR-FE2E-003 phá share-link | N/A — kết quả là (a) |
| AC-3 | Không regression trên `AddInstanceForm`/`OrcaInstanceSwitcher` bất kể (a)/(b) | ✅ Không đụng file nào trong nhóm này — 0 rủi ro |
