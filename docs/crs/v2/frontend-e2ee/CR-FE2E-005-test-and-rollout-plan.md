# CR-FE2E-005 — Test Matrix & Rollout Plan

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-FE2E-005 |
| **Tên** | Test matrix đảm bảo 0 regression trên cả 3 use case + kế hoạch rollout |
| **Loại** | QA / Release Process |
| **Priority** | P0 |
| **Phiên bản** | v5.1 |
| **Ngày tạo** | 2026-08-08 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-FE2E-002, CR-FE2E-003, CR-FE2E-004 |
| **Tác động HLD** | — |

---

## 1. Test Matrix

| # | Kịch bản | Use case | Trước CR | Kỳ vọng sau CR | Cách test |
|---|---|---|---|---|---|
| 1 | Login local (email/password) vào Orca Web Server | A | OK | OK, không đổi | `web/login/__tests__` |
| 2 | Login SSO | A | OK | OK, không đổi | `web/login/__tests__` |
| 3 | Nhập pairing code trên trang login multi-user | A | Kết nối được (fallback) | **Không còn ô nhập** — UI đã bỏ | Playwright: assert `pair-code-fallback` không tồn tại trong DOM |
| 4 | Session cookie hết hạn giữa phiên (mã đóng WS 4401) | A | Redirect `/login`, xoá state | Không đổi | `main-web-bootstrap.test.ts` — event `orca:auth-failed` |
| 5 | Mở `https://orca-server/?pairing=<runtime-scope-offer>` do backend tự phát (nếu CR-FE2E-004 kết luận (b)) | A | Auto-save + vào thẳng App | Không đổi *(chỉ nếu CR-FE2E-004 = (b); nếu (a) — case này không tồn tại, bỏ qua)* | Manual + Playwright |
| 6 | Mở browser trỏ thẳng vào Desktop app (không qua backend), `/auth/config` 404 | B | Hiện `WebConnect`, pairing hoạt động | **Y hệt trước** | e2e: mock `/auth/config` → 404, assert `WebConnect` render + connect thành công |
| 7 | Quét QR pairing từ Desktop app trong `AddInstanceForm` | B | OK | Không đổi | Test hiện có của `AddInstanceForm` |
| 8 | Chuyển giữa nhiều Orca instance đã pair (`OrcaInstanceSwitcher`) | B | OK | Không đổi | Test hiện có |
| 9 | Mobile Companion app pair vào `backend` qua QR | C | OK | Không đổi (0 file backend đổi) | Test hiện có của `mobile/` — không nằm trong scope sửa nhưng chạy lại để xác nhận |
| 10 | Bundle size khi vào qua multi-user (use case A) | A | Chứa TweetNaCl + pairing UI | Giảm (theo số đo CR-FE2E-003) | `pnpm --filter frontend build:web` + size report |

## 2. Thứ tự merge đề xuất

```
CR-FE2E-001 (audit, không code)
  │
  ├── CR-FE2E-004 câu hỏi mục 1 PHẢI trả lời trước
  │
  ▼
CR-FE2E-002 (bỏ PairCodeFallback khỏi LoginPage)
  │
  ▼
CR-FE2E-003 (code-split — điều chỉnh ranh giới theo kết quả CR-FE2E-004 nếu là (b))
  │
  ▼
CR-FE2E-005 test matrix đầy đủ chạy xanh → merge
```

## 3. Rollout

- [ ] Deploy CR-FE2E-002 riêng trước (rủi ro thấp nhất, dễ rollback), theo dõi lỗi login 1-2 ngày trên staging.
- [ ] Deploy CR-FE2E-003 sau, theo dõi bundle size + lỗi tải chunk (Sentry breadcrumb `renderer_bootstrap_started` đã có sẵn — thêm breadcrumb cho `pair-code-app-entry` chunk load fail nếu chưa có).
- [ ] Không cần thay đổi gì ở `backend/` hoặc `mobile/` — không cần phối hợp release giữa các package.
- [ ] Cập nhật [web-server-architecture.md](../../../hld/web-server-architecture.md) §5.2 sau khi merge: đổi mô tả `WebRuntimeClient` từ "legacy pairing mode" thành mô tả rõ 2 use case (A đã bỏ, B vẫn còn) như README của CR series này.

## 4. Acceptance Criteria (đóng toàn bộ series)

- [ ] Tất cả 10 kịch bản ở mục 1 pass.
- [ ] `git diff --stat` cho toàn series chỉ đụng tới file trong `frontend/src/renderer/src/web/**` (+ file mới `pair-code-app-entry.tsx`) và file test tương ứng — 0 file trong `backend/`, `mobile/`, `desktop/`.
- [ ] `docs/hld/web-server-architecture.md` được cập nhật phản ánh đúng trạng thái mới.
- [ ] CR-FE2E-004 có kết luận bằng văn bản (a) hoặc (b), không còn để ngỏ.
