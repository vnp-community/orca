# Solutions — CR series `frontend-e2ee` (Bỏ E2EE Pairing khỏi Browser Multi-user Path)

**Nguồn:** [docs/crs/v2/frontend-e2ee/](../../../../../docs/crs/v2/frontend-e2ee/) (5 CR) đối chiếu với [specs/frontend/tdd/v4](../../../tdd/v4/) và [specs/frontend/tdd/v5](../../../tdd/v5/).
**Trạng thái:** Proposed — solution đã đầy đủ để implement, **chưa thực thi code** (khác với series `specs/frontend/bugs/hld-v1/` đã được thực thi thật). Nếu muốn thực thi, làm theo đúng thứ tự ở [SOL-FE2E-005 mục 2](./SOL-FE2E-005-test-and-rollout-plan.md#2-thứ-tự-triển-khai--đã-hoàn-tất-bước-auditdecision-cập-nhật-thứ-tự-còn-lại).

---

## Danh sách Solutions

| CR | Solution | Loại | Kết quả chính |
|---|---|---|---|
| [CR-FE2E-001](../../../../../docs/crs/v2/frontend-e2ee/CR-FE2E-001-scope-and-discovery-audit.md) | [SOL-FE2E-001](./SOL-FE2E-001-scope-and-discovery-audit.md) | Investigation | ✅ Audit hoàn tất — `/auth/config` và `/auth/local` share 1 mount guard (không thể lệch nhau), inventory khớp 100%, phát hiện thêm: TDD-FE-06 mô tả sai luồng bootstrap |
| [CR-FE2E-002](../../../../../docs/crs/v2/frontend-e2ee/CR-FE2E-002-remove-paircode-fallback-from-login.md) | [SOL-FE2E-002](./SOL-FE2E-002-remove-paircode-fallback-from-login.md) | Implementation plan | Diff của CR xác nhận đúng 100%; bổ sung bằng chứng TDD (TDD-FE-02 §10 cũng mô tả sai, càng củng cố lý do CR tồn tại) |
| [CR-FE2E-003](../../../../../docs/crs/v2/frontend-e2ee/CR-FE2E-003-lazy-split-pairing-bundle.md) | [SOL-FE2E-003](./SOL-FE2E-003-lazy-split-pairing-bundle.md) | Implementation plan | **Điều chỉnh:** dùng `lazyWithRetry` có sẵn (đã dùng cho `WebConnect` trong `main-web-bootstrap.tsx`) thay vì tự viết cơ chế retry mới |
| [CR-FE2E-004](../../../../../docs/crs/v2/frontend-e2ee/CR-FE2E-004-share-link-decision.md) | [SOL-FE2E-004](./SOL-FE2E-004-share-link-decision.md) | Decision (đã trả lời) | **Kết luận: (a)** — "Share this Orca server" chỉ chạy trên Desktop (`canGeneratePairingUrl={!isWebClient}`), không bao giờ reachable từ web client (cả use case A và B) — no-op, không chặn CR-002/003 |
| [CR-FE2E-005](../../../../../docs/crs/v2/frontend-e2ee/CR-FE2E-005-test-and-rollout-plan.md) | [SOL-FE2E-005](./SOL-FE2E-005-test-and-rollout-plan.md) | Test plan | Test matrix cập nhật theo kết quả CR-004 (bỏ kịch bản #5 điều kiện, thêm #11 bảo vệ phát hiện CR-004); thứ tự triển khai đơn giản hoá (không còn phụ thuộc quyết định đang mở) |

---

## Phát hiện xuyên suốt khi đối chiếu với TDD + code thật

1. **CR-FE2E-004's câu hỏi mở đã được trả lời dứt khoát bằng code, không cần hỏi product owner** như CR gốc dự trù — `Settings.tsx:1544` (`canGeneratePairingUrl={!isWebClient}`) + `isWebClientLocation()` cho câu trả lời rõ ràng: (a), no-op. Khác với `BUG-FE-HLD-007`/`ConflictPanel` (series `hld-v1`) — nơi câu hỏi thực sự cần người quyết định vì code không để lại dấu vết nào — ở đây code TỰ trả lời được.
2. **TDD (cả v4 lẫn v5) mô tả sai lệch đáng kể so với kiến trúc bootstrap thật của `main.tsx`** — không chỉ HLD (đã biết từ `audit/frontend/03-hld-doc-drift.md`) mà cả TDD cũng lệch. TDD-FE-06 mô tả 1 hàm `bootstrapWebApp()` tự quyết định no-auth-mode bên trong nó; thực tế quyết định đó nằm ở `main.tsx`, TRƯỚC khi `bootstrapWebApp()` được gọi. TDD-FE-02 mô tả `PairCodeFallback` có điều kiện hiển thị riêng; thực tế nó luôn hiển thị vô điều kiện bên trong `LoginPage`. Cả 2 phát hiện này càng củng cố lý do CR-FE2E-002 cần tồn tại (fallback không có lý do hiển thị hợp lý, dù theo TDD hay theo hành vi thật).
3. **CR-FE2E-003 có thể tận dụng tiện ích có sẵn** (`lazyWithRetry`) thay vì tự xây cơ chế resilience mới — giảm code cần review, tăng nhất quán với `main-web-bootstrap.tsx` (cùng thư mục, cùng loại vấn đề).

## Việc CÒN CẦN LÀM nếu muốn thực thi (không nằm trong solution — cần quyết định con người)

- Review + merge theo thứ tự ở [SOL-FE2E-005](./SOL-FE2E-005-test-and-rollout-plan.md).
- Cập nhật `docs/hld/web-server-architecture.md` §5.2 VÀ `specs/frontend/tdd/v4/06-web-entry.md` sau khi merge (2 tài liệu, không chỉ 1 — phát hiện mới so với kế hoạch gốc chỉ nhắc HLD).
