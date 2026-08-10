# TASK-BIGFILE-036 — Move (composition): Automation domain

**Loại:** Move — composition pattern (KHÔNG phải barrel, class có `this`)
· **Effort:** S · **Phụ thuộc:** TASK-BIGFILE-008, 009
**Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3) · Sinh ra từ `TASK-BIGFILE-035`

**Vì sao làm task này TRƯỚC (không theo thứ tự số)**: nhỏ nhất (~191 dòng,
8 method), field lõi ít nhất (chỉ `automationService`, span 75 dòng) —
dùng để xác nhận pattern composition hoạt động đúng (typecheck, forward
call, không phá caller ngoài) trước khi làm TASK-037 (lớn, rủi ro cao hơn).

## Input

- File nguồn: `frontend/src/main/runtime/orca-runtime.ts`
- Đọc **đúng dòng 2,624–2,830** (đọc rộng hơn ước tính 1 chút để thấy đúng
  ranh giới method liền trước/sau — KHÔNG đọc phần khác của file).
- Method cần chuyển (8 method, xác nhận lại khi đọc):
  `listAutomations`, `listAutomationRuns`, `showAutomation`,
  `createAutomation`, `updateAutomation`, `deleteAutomation`,
  `runAutomationNow`, `resolveAutomationTarget`
- Field private cần: `automationService` (đã có setter
  `setAutomationService` — giữ nguyên ở `OrcaRuntimeService`, KHÔNG chuyển
  setter), có thể cần đọc thêm `_orchestrationDb`/`OrchestrationDb` (dùng
  chéo — xác nhận qua `grep -n "_orchestrationDb\|OrchestrationDb"` trong
  dải dòng trên).
- Type liên quan (đã tách sẵn ở TASK-009):
  `RuntimeAutomationCreateInput`, `RuntimeAutomationUpdateInput` từ
  `./orca-runtime-types`.

## Output

- File mới: `frontend/src/main/runtime/orca-runtime-automation.ts` — class
  mới (ví dụ `AutomationDomain`) nhận dependency qua constructor (tối
  thiểu: `automationService`, các field/method dùng chéo nếu có).
- `orca-runtime.ts`: `OrcaRuntimeService` thêm 1 field
  `private automation = new AutomationDomain({ ... })`, 8 public method cũ
  đổi thành forward call 1 dòng (`return this.automation.listAutomations()`
  v.v.) — **GIỮ NGUYÊN chữ ký cũ** (tham số, kiểu trả về) để không phá bất
  kỳ caller nào (IPC handlers, RPC methods).

## Các bước

1. `gitnexus impact({target: "createAutomation", direction: "upstream"})`
   và `updateAutomation` — dừng nếu risk HIGH/CRITICAL, báo cáo trước khi
   tiếp tục.
2. Đọc dòng 2,624–2,830, xác nhận đúng 8 method + field dùng.
3. Tạo `orca-runtime-automation.ts`, class mới nhận dependency qua
   constructor (copy logic nguyên văn, đổi `this.automationService` →
   `this.deps.automationService` theo pattern trong solution doc).
4. Sửa `orca-runtime.ts`: thêm field `automation`, thay 8 method bằng
   forward call.
5. `setAutomationService` ở `OrcaRuntimeService` cần cập nhật cả field cũ
   (nếu còn nơi khác dùng trực tiếp) lẫn truyền vào domain object mới —
   kiểm tra kỹ thứ tự khởi tạo (constructor `OrcaRuntimeService` gọi
   `setAutomationService` SAU khi tạo `this.automation` hay TRƯỚC — dependency
   injection phải nhận giá trị mới nhất, có thể cần domain object giữ tham
   chiếu tới 1 getter thay vì copy giá trị tại thời điểm construct).

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `orca-runtime.ts` giảm
      ~150-190 dòng (forward call ngắn hơn code gốc, không giảm hết ~191
      vì còn field + forward call ở lại)
- [ ] Test automation liên quan (nếu có) pass; chạy thêm test PTY/terminal
      tổng quát (class này trung tâm nhiều flow) dù domain automation ít
      liên quan trực tiếp

## Rollback

```
git checkout -- frontend/src/main/runtime/orca-runtime.ts
rm frontend/src/main/runtime/orca-runtime-automation.ts
```
