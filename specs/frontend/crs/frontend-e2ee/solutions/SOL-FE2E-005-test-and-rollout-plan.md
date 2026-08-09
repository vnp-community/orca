# SOL-FE2E-005 — Test Matrix & Rollout — Giải pháp cập nhật theo kết quả CR-FE2E-001/004

**CR:** [CR-FE2E-005](../../../../../docs/crs/v2/frontend-e2ee/CR-FE2E-005-test-and-rollout-plan.md)
**TDD Refs:** [TDD-FE-02 §11](../../../tdd/v4/02-auth-flow.md#L202) (bảng test hiện có cho `web/login/__tests__` — 47 test, dùng làm baseline coverage tham chiếu), [TDD-FE-06](../../../tdd/v4/06-web-entry.md)
**Approach:** Cập nhật kế hoạch — kịch bản #5 trong CR gốc còn để ngỏ ("chỉ nếu CR-FE2E-004 kết luận (b)"); [SOL-FE2E-004](./SOL-FE2E-004-share-link-decision.md) đã trả lời dứt khoát là **(a)** — cập nhật bảng test cho khớp, không còn nhánh điều kiện.

---

## 1. Test Matrix — cập nhật (10 kịch bản gốc → 9, bỏ #5)

| # | Kịch bản | Use case | Trước CR | Kỳ vọng sau CR | Cách test |
|---|---|---|---|---|---|
| 1 | Login local (email/password) | A | OK | OK, không đổi | `web/login/__tests__` (baseline 47 test theo TDD-FE-02 §11) |
| 2 | Login SSO | A | OK | OK, không đổi | `web/login/__tests__` |
| 3 | Nhập pairing code trên trang login multi-user | A | Kết nối được (fallback) | **Không còn ô nhập** | Test mới ở [SOL-FE2E-002 §3.1](./SOL-FE2E-002-remove-paircode-fallback-from-login.md) |
| 4 | Session cookie hết hạn giữa phiên (đóng WS 4401) | A | Redirect `/login` | Không đổi | `main-web-bootstrap.test.ts` — event `orca:auth-failed` |
| ~~5~~ | ~~Mở `?pairing=` runtime-scope offer do backend tự phát~~ | — | — | **LOẠI BỎ** — CR-FE2E-004 kết luận (a): backend/multi-user không bao giờ phát pairing offer kiểu này qua web client, kịch bản không tồn tại trong thực tế | N/A |
| 6 | Mở browser trỏ thẳng Desktop app, `/auth/config` 404 | B | Hiện `WebConnect` | **Y hệt trước** | Test mới ở [SOL-FE2E-003 §4](./SOL-FE2E-003-lazy-split-pairing-bundle.md) (mock 404 → xác nhận `mountPairCodeApp` được gọi) |
| 7 | Quét QR pairing từ Desktop app trong `AddInstanceForm` | B | OK | Không đổi | Test hiện có |
| 8 | Chuyển giữa nhiều Orca instance đã pair (`OrcaInstanceSwitcher`) | B | OK | Không đổi | Test hiện có |
| 9 | Mobile Companion pair vào `backend` qua QR | C | OK | Không đổi (0 file backend đổi) | Test hiện có của `mobile/` |
| 10 | Bundle size use case A | A | Chứa TweetNaCl + pairing UI | Giảm | Lệnh đo cụ thể ở [SOL-FE2E-003 §3](./SOL-FE2E-003-lazy-split-pairing-bundle.md) |
| **11 (mới)** | "Share this Orca server" trong Settings (Desktop) | Desktop, không phải web | OK | Không đổi — xác nhận bằng regression test `Settings.test.tsx` cho `canGeneratePairingUrl` | [SOL-FE2E-004](./SOL-FE2E-004-share-link-decision.md) — thêm 1 test khẳng định `isWebClient` tiếp tục ẩn đúng section này (không phải test mới do CR gây ra, mà là bảo vệ phát hiện của CR-004) |

## 2. Thứ tự triển khai — đã hoàn tất bước audit/decision, cập nhật thứ tự còn lại

```
✅ CR-FE2E-001 (audit) — HOÀN TẤT, xem SOL-FE2E-001, không phát hiện rủi ro chặn
✅ CR-FE2E-004 (share-link decision) — HOÀN TẤT, kết luận (a) no-op, xem SOL-FE2E-004
  │
  ▼
CR-FE2E-002 (bỏ PairCodeFallback khỏi LoginPage) — sẵn sàng implement, xem SOL-FE2E-002
  │
  ▼
CR-FE2E-003 (code-split, dùng lazyWithRetry có sẵn) — sẵn sàng implement, xem SOL-FE2E-003
  │
  ▼
Test Matrix mục 1 (9 kịch bản, đã bỏ #5) chạy xanh → merge
```

**Khác so với CR gốc:** không còn bước "chờ CR-FE2E-004 trả lời trước khi CR-FE2E-002 merge" — đã trả lời, không blocking nữa. CR-FE2E-002 và CR-FE2E-003 có thể lên kế hoạch implement song song với review, không cần chờ tuần tự nghiêm ngặt như sơ đồ gốc (CR-003 phụ thuộc code CR-002 đã merge để tránh conflict, nhưng không phụ thuộc *quyết định* nào nữa).

## 3. Rollout — giữ nguyên kế hoạch gốc, bổ sung 1 mục

- [ ] Deploy CR-FE2E-002 riêng trước, theo dõi lỗi login 1-2 ngày trên staging.
- [ ] Deploy CR-FE2E-003 sau, theo dõi bundle size + lỗi tải chunk qua breadcrumb (đã có sẵn cơ chế `LazyChunkLoadError`/reload-once trong `lazy-with-retry.ts` — xem SOL-FE2E-003 mục 1 — **không cần thêm breadcrumb mới**, cơ chế reload đã tồn tại và đã được dùng cho `WebConnect` trong `main-web-bootstrap.tsx`).
- [ ] Không cần thay đổi `backend/`/`mobile/`.
- [ ] Cập nhật `web-server-architecture.md` §5.2 sau khi merge.
- [ ] **(Bổ sung)** Cập nhật `specs/frontend/tdd/v4/06-web-entry.md` §1/§7/§10 — mô tả `checkNoAuthMode()`/`renderPairCodeFallback()` không khớp kiến trúc thật (`/auth/config` probe ở `main.tsx`, không phải logic bên trong `bootstrapWebApp()`) — phát hiện ở [SOL-FE2E-001 mục 4](./SOL-FE2E-001-scope-and-discovery-audit.md#4-phát-hiện-thêm). Nên cập nhật cùng đợt với HLD để 2 tài liệu không tiếp tục lệch nhau.

## 4. Acceptance Criteria (đóng toàn bộ series) — cập nhật

| # | Criteria | Trạng thái |
|---|---|---|
| AC-1 | Tất cả kịch bản test matrix pass | 9 kịch bản (đã bỏ #5) + 1 kịch bản mới (#11) — chờ implement CR-002/003 |
| AC-2 | `git diff --stat` chỉ đụng `frontend/src/renderer/src/web/**` + test | Giữ nguyên |
| AC-3 | `web-server-architecture.md` cập nhật | Giữ nguyên, bổ sung TDD-FE-06 (mục 3) |
| AC-4 | ~~CR-FE2E-004 có kết luận (a)/(b)~~ | ✅ **Đã xong** — (a), xem SOL-FE2E-004 |
