# 05 — Module Inventory (Recap phiên audit trước)

Phiên làm việc trước đã audit toàn bộ module `frontend/src/main/**` và `frontend/src/renderer/src/components/**` đối chiếu với `docs/`, tìm ra **8 module có code thật nhưng 0 tài liệu ở bất kỳ đâu trong `docs/`** (grep đệ quy toàn bộ `docs/features`, `docs/business`, `docs/flows`, `docs/crs`, `docs/logic`, `docs/adrs`). Ghi lại ở đây để audit này có đầy đủ lịch sử, không lặp lại công việc.

## Đã tìm thấy & đã xử lý (bổ sung docs)

| Module | Vấn đề gốc | Đã xử lý |
|---|---|---|
| `main/kimi/` | Provider Kimi Code, 0 mention trong docs | ✅ Thêm vào bảng agent first-class ở [F04](../../docs/features/F04-ai-agent-support.md) |
| `main/minimax/` | Provider MiniMax | ✅ Thêm vào F04 |
| `main/openclaude/` | Provider OpenClaude | ✅ Thêm vào F04 |
| `main/command-code/` | Provider Command Code | ✅ Thêm vào F04 |
| `main/droid/` | Provider Factory Droid | ✅ Thêm vào F04 |
| `renderer/.../sparse/` | Sparse checkout preset UI, không doc | ✅ Thêm mục "Sparse Checkout Presets" vào [F01](../../docs/features/F01-parallel-worktrees.md) |
| `renderer/.../pet/` | Desktop pet/mascot overlay, 0 mention ở PRD/URD/SRS/features/CRs | ✅ Viết doc mới [F41-desktop-pet-companion.md](../../docs/features/F41-desktop-pet-companion.md) |
| `renderer/.../contextual-tours/` | Engine tour onboarding theo ngữ cảnh, 0 mention | ✅ Viết doc mới [F42-contextual-onboarding-tours.md](../../docs/features/F42-contextual-onboarding-tours.md) |

`docs/features/README.md` đã cập nhật registry 40→42 feature, thêm Group 6.

## Chưa xử lý — cần theo dõi

- **native-chat/** — không tính là module thừa (hạ tầng transcript dùng chung cho Claude/Codex/Grok đã document), nhưng chưa có mục riêng mô tả kiến trúc của nó trong bất kỳ HLD nào — nên cân nhắc thêm 1 đoạn ngắn vào C3/C4 nếu có thời gian.
- **Test coverage cho 6/8 module trên = 0** — xem chi tiết [04-code-health-and-standards.md §3](./04-code-health-and-standards.md#3-test-coverage-gap-ở-5-module-provider-mới).
- **`backend/`** đã được audit riêng (kết luận: sạch, superset hợp lý của `frontend/src/main`, không có module thừa nào ngoài 5 provider đã nêu — cùng lỗi, cùng cách xử lý vì docs dùng chung).

## Phát hiện phụ đáng chú ý (ngoài phạm vi "module thừa")

Trong lúc audit, phát hiện `frontend/`, `desktop/`, `backend/`, `agent/` (4 package trong `pnpm-workspace.yaml`) đều **chưa được `git add`** (toàn bộ nằm ở mục `??` trong `git status`), song song với ~7,700 file bị xoá (`D`) so với `HEAD` — dấu hiệu repo đang giữa đợt tái cấu trúc lớn (từ 1 app Electron monolith tại root sang monorepo `frontend`/`backend`/`agent`/`desktop`/`mobile`/`native`). Đây không phải lỗi thiết kế nhưng là rủi ro vận hành (chưa commit = chưa có lịch sử, chưa có CI baseline) đáng nêu cho người quản lý repo, không thuộc phạm vi kỹ thuật của audit này.
