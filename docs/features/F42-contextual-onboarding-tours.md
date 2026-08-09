# F42 — Contextual Onboarding Tours

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F42 |
| **Tên** | Contextual Onboarding Tours |
| **Ưu tiên** | P2 — Should Have |
| **Trạng thái** | ✅ Đã phát hành (chưa có trong PRD gốc — bổ sung từ code thực tế 2026-08-08) |
| **Tham chiếu PRD** | — |
| **Tham chiếu URD** | — |
| **Tham chiếu SRS** | — |
| **ADR References** | — |
| **HLD References** | C3.4 |

---

## Mô tả

Hệ thống tour hướng dẫn theo ngữ cảnh (spotlight/tooltip overlay) tự động xuất hiện lần đầu người dùng chạm tới một khu vực UI cụ thể (vd. mở Workspace Board lần đầu), khác với F28 (Dev Server Onboarding Wizard — luồng setup dev server) ở chỗ đây là các tour nhỏ, per-feature, kích hoạt tại đúng thời điểm dùng tính năng đó thay vì một wizard tuyến tính lúc setup ban đầu.

---

## Vấn đề cần giải quyết

Người dùng mới không khám phá hết các tính năng nằm sâu trong UI (split terminal, board theo lane, browser pane, automations...). Một wizard onboarding một lần khi mở app không đủ vì người dùng quên hoặc chưa sẵn sàng tiếp nhận hết thông tin. Contextual tour giải quyết bằng cách dạy đúng lúc, đúng chỗ — khi người dùng vừa mở tính năng đó lần đầu.

---

## Danh sách Tour (7 tour)

| Tour ID | Khu vực UI | Trigger source |
|---------|-----------|-----------------|
| `workspace-board` | Workspace Board (xem theo lane thay vì theo project) | `workspace_board_visible` |
| `workspace-agent-sessions` | Terminal pane của agent trong workspace (vd. gợi ý split pane) | `workspace_agent_sessions_visible` |
| `browser` | Browser pane (Design Mode) | `browser_visible` |
| `tasks` | Task Graph | `tasks_open` |
| `automations` | Automations | `automations_open` |
| `floating-workspace` | Floating terminal/workspace | `floating_workspace_visible` |
| `workspace-creation` | Tạo workspace mới | `workspace_creation_visible` |

---

## Tính năng chi tiết

### Điều kiện kích hoạt (Gate)
Một tour chỉ tự động bắt đầu (`getContextualTourRequestDecision`) khi **đồng thời** thoả:
- Persisted UI đã hydrate xong (`persistedUIReady`)
- Cờ `contextualToursAutoEligible` bật (người dùng chưa tắt tour trong Settings)
- Không đang hiển thị onboarding wizard chính, không có blocking surface (dialog/sheet chặn) khác
- Tour chưa từng "seen" (`contextualToursSeenIds`) và chưa hiển thị tour nào khác trong session hiện tại
- Modal đang mở (nếu có) nằm trong `allowedActiveModals` của tour
- Target DOM element của step bắt buộc (`requiredForStart: true`) đã render và **visible** (không `display:none`, không trong `[hidden]`/`[aria-hidden]`/`[inert]`, có kích thước đo được)

### Target Detection
- Mỗi step trỏ tới 1 `targetSelector` (đánh dấu bằng `data-contextual-tour-target="..."` trong JSX)
- Dùng `MutationObserver` trên `document.body` + poll tối đa 20 lần / 500ms để bắt kịp UI hydrate async (dialog mở chậm, virtualized list...)
- Step không có target hiện tại bị bỏ qua khi tính progress (`getVisibleContextualTourStepIndexes`), không chặn tour

### Feature Interaction Tracking
- Khi 1 tour `enabled`, ghi nhận "đã tương tác với tính năng" (`recordFeatureInteraction`) — dùng để tour không tự bật lại nếu người dùng đã tự khám phá tính năng trước khi tour kịp render

### Step Controls & Actions
- Step có thể nhúng control tương tác trực tiếp trong tour, vd. `auto-rename-branch-from-work` (bật/tắt setting "Auto-name workspace từ tin nhắn đầu tiên" ngay trong tour)
- Step action (`primaryAction`/`secondaryAction`) hỗ trợ các kind: `next`, `complete`, `split-terminal-pane`, `create-worktree`, `show-worktrees`, `open-tasks`, `open-getting-started`
- `advanceOnFeatureInteraction`: step tự next khi người dùng thực hiện đúng hành động được dạy (không cần bấm nút)

### Vòng đời
- Tour bị suppress (kết thúc dạng "cancelled") khi source tắt (`enabled=false`) trong lúc đang active, hoặc khi component unmount đột ngột (sheet/dialog đóng)
- Sau khi xem hết hoặc suppress, tour ID được thêm vào `contextualToursSeenIds` (persisted) — không tự động hiện lại

---

## Tiêu chí chấp nhận

- [ ] Tour không tự bật khi onboarding wizard chính đang mở
- [ ] Tour không bật lại sau khi đã "seen" hoặc đã bị suppress
- [ ] Step ẩn do target không tồn tại không làm tour bị kẹt (progress tính đúng trên các step visible)
- [ ] Tắt tour giữa chừng (đóng sheet/dialog chứa target) tạo outcome "cancelled", không để lại trạng thái treo

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Định nghĩa tour** | `src/shared/contextual-tours.ts` |
| **Hook kích hoạt tour** | `src/renderer/src/components/contextual-tours/use-contextual-tour.ts` |
| **Logic gate/target/progress** | `src/renderer/src/components/contextual-tours/contextual-tour-gate.ts` |
| **Overlay UI** | `src/renderer/src/components/contextual-tours/ContextualTourOverlay.tsx`, `ContextualTourOverlaySurface.tsx`, `ContextualTourArrow.tsx`, `ContextualTourProgressDots.tsx` |
| **Step control** | `src/renderer/src/components/contextual-tours/ContextualTourControl.tsx` |
| **Vị trí floating** | `src/renderer/src/components/contextual-tours/contextual-tour-floating-position.ts` |
| **Feature interaction tracking** | `src/shared/feature-interactions.ts` |

---

## Notes

- Không có ghi nhận trong PRD/URD/SRS/CR gốc; đề xuất bổ sung vào PRD nếu đây là roadmap chính thức cho onboarding UX.
- Liên quan nhưng độc lập với F28 (Dev Server Onboarding Wizard) — nên tham chiếu chéo trong PRD nếu gộp chung nhóm "Onboarding".
