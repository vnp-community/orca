# F41 — Desktop Pet Companion

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F41 |
| **Tên** | Desktop Pet Companion |
| **Ưu tiên** | P3 — Nice to Have |
| **Trạng thái** | ✅ Đã phát hành (chưa có trong PRD gốc — bổ sung từ code thực tế 2026-08-08) |
| **Tham chiếu PRD** | — |
| **Tham chiếu URD** | — |
| **Tham chiếu SRS** | — |
| **ADR References** | — |
| **HLD References** | C3.1 |

---

## Mô tả

Một overlay "pet" ảo (mặc định: **Claudino**, ngoài ra có **OpenCode** và **Gremlin**) nổi trên toàn bộ cửa sổ Orca, phản ứng theo trạng thái agent đang chạy — chạy khi agent `working`, đứng chờ khi agent `blocked`/`waiting`, "xem lại" khi agent `done`. Người dùng có thể kéo-thả đổi vị trí, đổi kích thước, và upload pet tuỳ chỉnh (ảnh tĩnh hoặc sprite sheet animation).

> Trong code, tên nội bộ trước đây là "Sidekick" (`sidekick-overlay-position`, `sidekickVisible` vẫn được đọc như legacy alias) — UI hiện tại gọi là "Pet".

---

## Vấn đề cần giải quyết

Đây là tính năng gamification/engagement — không giải quyết painpoint nghiệp vụ cụ thể nào trong bộ actor (Alex/Maya/Carlos/Sam/QA/DevOps). Mục tiêu là tăng gắn kết cảm xúc với sản phẩm khi làm việc dài với agent (tương tự "desktop pet" ở nhiều dev tool khác), đồng thời cho một tín hiệu trạng thái agent ở dạng ambient (không cần nhìn vào status bar).

---

## Tính năng chi tiết

### Animation theo trạng thái Agent
- `selectPetAnimationName()` map trạng thái agent (từ `agentStatusByPaneKey` trong store) sang 1 trong 5 animation: `idle`, `running`, `waiting`, `review`, `jumping`
- `waiting` ưu tiên cao nhất nếu có bất kỳ pane nào `blocked`/`waiting`; `running` nếu có pane `working`; `review` nếu có pane `done` hoặc còn agent "retained" (đã xong nhưng chưa dismiss); `jumping` khi người dùng đang kéo pet
- Chỉ tính các entry còn "fresh" (`isExplicitAgentStatusFresh`, ngưỡng `AGENT_STATUS_STALE_AFTER_MS`) — agent status cũ không còn ảnh hưởng animation

### Sprite Rendering
- Hỗ trợ 2 nguồn: sprite sheet có manifest khai báo sẵn (`sprite.animations`, cell theo `row`/`frames`/`fps`) render bằng CSS `steps()` keyframes, hoặc sprite tự động dò khung hình (`DetectedSpriteFrame`) render bằng `<canvas>` + `requestAnimationFrame` khi ảnh không có manifest
- Ảnh tĩnh (không phải sprite) fallback về `<img>` đơn giản

### Tuỳ chỉnh
- 3 pet dựng sẵn: Claudino (mặc định), OpenCode, Gremlin (`pet-models.ts`)
- Upload pet tuỳ chỉnh (ảnh hoặc sprite sheet); blob URL cache qua IPC (`pet-blob-cache.ts`)
- Đổi kích thước qua `petSize` trong store (menu status bar)
- Vị trí lưu tại `localStorage` (`pet-overlay-position`), tự động clamp về trong viewport khi resize cửa sổ hoặc đổi size pet

### Accessibility & Performance
- Tôn trọng `prefers-reduced-motion` — dừng animation khi bật
- Tạm dừng animation khi document không visible (tab/app ở background)
- Toàn bộ overlay `pointer-events-none` trừ vùng grab của pet, để không chặn tương tác với UI bên dưới
- Component lazy-load (`lazy(() => import(...))`), chỉ mount sau khi persisted UI đã hydrate

### Bật/Tắt
- Cờ `petVisible` trong persisted UI settings (mặc định `true`), toggle qua store action `setPetVisible`

---

## Tiêu chí chấp nhận

- [ ] Animation đổi đúng theo trạng thái agent trong vòng 1 tick của status freshness scheduler
- [ ] Kéo-thả không rời khỏi viewport, giữ đúng vị trí sau khi resize cửa sổ
- [ ] Tắt `prefers-reduced-motion` dừng mọi animation (bao gồm sprite steps và bob float)
- [ ] Ẩn/hiện qua setting không ảnh hưởng phần còn lại của UI

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Overlay chính** | `src/renderer/src/components/pet/PetOverlay.tsx` |
| **Animation state** | `src/renderer/src/components/pet/pet-agent-state.ts` |
| **Pet đóng gói sẵn** | `src/renderer/src/components/pet/pet-models.ts` |
| **Blob cache (pet tuỳ chỉnh)** | `src/renderer/src/components/pet/pet-blob-cache.ts` |
| **URL resolver** | `src/renderer/src/components/pet/usePetUrl.ts` |
| **Sprite frame auto-detect** | `src/renderer/src/components/pet/sprite-frame-detection.ts` |
| **Visibility gate** | `src/renderer/src/components/pet/pet-overlay-visibility.ts` |
| **Persisted state** | `petVisible`, `petId`, `petSize`, `customPets` (store slice `ui.ts`) |

---

## Notes

- Không có ghi nhận trong PRD/URD/SRS/CR gốc; đề xuất bổ sung vào PRD §3.x nếu team muốn giữ tính năng chính thức, hoặc đánh dấu rõ "experimental/easter egg" nếu không nằm trong roadmap chính thức.
