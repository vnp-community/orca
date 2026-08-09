# BUG-FE-HLD-005 — `IPlatformServices` chỉ có adapter web; 72 file `src/main` import `electron` trực tiếp

**Mức độ:** 🟠 High
**Status:** 🔴 Open
**Module:** `frontend/src/platform/`, `frontend/src/main/**`
**Phát hiện:** 2026-08-08 (audit `frontend/` code vs thiết kế — `audit/frontend/02-platform-abstraction-and-coding-standards.md` §1)

---

## Mô tả

Chuẩn #3 trong "Coding Standards cho v5.0" (`docs/features/README.md`) ghi: *"IPlatformServices: không import electron trực tiếp"*.

`find frontend/src/platform -type f` cho thấy layer abstraction chỉ có **1 adapter**:

```
src/platform/adapters/web/rpc-client.ts   ← duy nhất
src/platform/app-interface.ts
src/platform/context.ts
src/platform/ipc-interface.ts
src/platform/rpc-client-interface.ts
src/platform/storage-interface.ts
src/platform/system-interface.ts
src/platform/types.ts
src/platform/window-interface.ts
```

Không tồn tại `src/platform/adapters/electron/` (hay tương đương). Trong khi đó, grep `from 'electron'` (loại test) trên `src/main` trả về **72 file** import electron trực tiếp — ví dụ `src/main/browser/browser-manager.ts`, `src/main/ipc/browser.ts`, `src/main/window/clipboard-image-temp-file.ts`, `src/main/claude-accounts/oauth-refresh.ts`, `src/main/persistence.ts`, `src/main/runtime/orca-runtime.ts`.

## Hậu quả

`IPlatformServices` hiện chỉ thực sự phục vụ target web — code Electron/desktop main-process bỏ qua abstraction hoàn toàn. Nếu đọc đúng nghĩa đen chuẩn #3 (không giới hạn "chỉ web"), đây là khoảng trống conformance có thật: bất kỳ nỗ lực tương lai nào muốn chạy phần main-process logic trên 1 platform thứ 3 (không phải Electron, không phải web/Node server) sẽ phải viết lại 72 file này thay vì chỉ thêm 1 adapter mới.

**Lưu ý quan trọng trước khi coi đây là "phải sửa ngay":** chuẩn #3 nằm trong đoạn dành riêng cho *"Tất cả v5.0 features"* (Profile/Project/AI Provider/Workflow/Task — các module này thực tế **0 hit** electron, xem audit). Rất có thể ý định ban đầu chỉ áp cho code mới thuộc nhóm F33-F39, không phải toàn bộ `src/main`. Nguyên tắc "Additive only, không sửa code Electron gốc" trong [restructure_v1 CR series](../../../../docs/crs/v1/restructure_v1/README.md) càng củng cố khả năng đây là chủ ý, không phải sai sót.

## Bằng chứng

```
find frontend/src/platform -type f  → chỉ 1 adapter (web/rpc-client.ts)
grep -rl "from 'electron'" frontend/src/main (loại test) → 72 file
src/main/profile,project,task,workflow → 0 hit electron (các module v5.0 F33-F39 thật sự sạch)
```

## Đề xuất fix

1. **Trước tiên: làm rõ phạm vi thật của chuẩn #3** với người viết `docs/features/README.md` — nếu ý định là v5.0-only, sửa lại câu chữ cho khỏi mơ hồ (đây là fix rẻ nhất, không đụng code).
2. Nếu ý định là toàn bộ `src/main`, đây là 1 hạng mục lớn (72 file) — không nên làm gấp, cần roadmap riêng, ưu tiên thấp hơn các bug bảo mật (FE-HLD-001/002/003).

## Tham khảo

- Audit: `audit/frontend/02-platform-abstraction-and-coding-standards.md` §1
- Doc gốc: `docs/features/README.md` ("Coding Standards cho v5.0"), `docs/hld/v1/C2-containers.md` ("Container Boundaries và Isolation")
- Liên quan: [restructure_v1 CR series](../../../../docs/crs/v1/restructure_v1/README.md) (nguồn gốc của layer `IPlatformServices`)
