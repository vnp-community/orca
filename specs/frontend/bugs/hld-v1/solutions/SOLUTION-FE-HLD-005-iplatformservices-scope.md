# SOLUTION: BUG-FE-HLD-005 — `IPlatformServices` chỉ có adapter web

**Source-verified:** ✅ Dựa trên source code thực tế
**TDD tham chiếu:** [tdd/v5/03-runtime-client-layer.md](../../../tdd/v5/03-runtime-client-layer.md) §"restructure_v1 Addendum" — nguyên văn: *"Từ restructure_v1, có thêm một interface layer trong `src/platform/`"*, và liệt kê **duy nhất** `platform/adapters/web/rpc-client.ts` là implementation cụ thể. TDD **không** đề cập bất kỳ `platform/adapters/electron/` nào — xác nhận layer này từ đầu được thiết kế **chỉ cho web target**, không phải toàn bộ `src/main`.

Điều này, cộng với nguyên tắc "Additive only, không sửa code Electron gốc" trong [restructure_v1 CR series](../../../../../docs/crs/v1/restructure_v1/README.md), cho kết luận: **đây không phải bug thiếu implementation — đây là câu chữ trong `docs/features/README.md` chuẩn #3 bị phát biểu quá rộng so với phạm vi thật của thiết kế.**

---

## Fix — sửa doc, không sửa 72 file code

### Bước 1: Sửa `docs/features/README.md`

```diff
  Tất cả v5.0 features phải tuân thủ:
  1. **Zero Mock**: không dùng mock data trong implementation
  2. **Zero Hardcode**: config qua env vars hoặc DB settings
- 3. **IPlatformServices**: không import electron trực tiếp
+ 3. **IPlatformServices**: code v5.0 mới (Profile/Project/AI Provider/Workflow/Task —
+    `src/main/profile`, `project`, `ai-providers`, `workflow`, `task`) không import
+    electron trực tiếp. Áp dụng cho code **mới** thuộc các domain này; không áp dụng
+    hồi tố cho code Electron/desktop main-process đã tồn tại trước restructure_v1
+    (xem `specs/frontend/crs/v1/restructure_v1/README.md` — nguyên tắc "Additive only").
  4. **IConnectionPool**: không access DB dialect trực tiếp
  5. **relay.call()**: tất cả remote operations qua relay RPC
```

### Bước 2: Verify các module v5.0 vẫn sạch (đã xác nhận qua audit — không cần sửa code)

`src/main/profile/`, `src/main/project/`, `src/main/task/`, `src/main/workflow/` — 0 hit `from 'electron'`, giữ nguyên. Thêm 1 lint rule (`no-restricted-imports` cho `electron` trong `eslintrc`/`oxlintrc` scoped riêng tới 4 thư mục này) để tự động giữ conformance, tránh phải audit thủ công lần sau:

```jsonc
// .oxlintrc.json (hoặc eslint override tương ứng)
{
  "overrides": [
    {
      "files": ["src/main/{profile,project,ai-providers,workflow,task}/**/*.ts"],
      "rules": { "no-restricted-imports": ["error", { "paths": ["electron"] }] }
    }
  ]
}
```

### Bước 3 (tuỳ chọn, không khẩn cấp): nếu muốn mở rộng phạm vi thật sự trong tương lai

Nếu về sau có nhu cầu chạy `src/main` trên 1 platform thứ 3 (không Electron, không Node web server), tạo `src/platform/adapters/electron/` mới — đây là việc lớn (72 file), không nằm trong phạm vi bug này, cần CR riêng nếu được ưu tiên.

## Tóm tắt thay đổi

| File | Thay đổi |
|---|---|
| `docs/features/README.md` | Làm rõ phạm vi chuẩn #3 — chỉ áp cho 5 module v5.0, không áp hồi tố |
| `.oxlintrc.json`/eslint override | Thêm rule chặn `import 'electron'` scoped riêng 5 thư mục v5.0 để giữ conformance tự động |
| 72 file `src/main` hiện tại | **Không đổi** — nằm ngoài phạm vi chuẩn theo kết luận ở trên |
