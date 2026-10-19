# FE-REQ-TASK-040-02: Tách `RepoRefPicker` dùng chung, giữ `settings/BaseRefPicker`

**From Solution:** [FE-REQ-SOL-040](../solutions/FE-REQ-SOL-040-request-base-branch-picker.md)
**Priority:** P1
**Area:** frontend / request
**File:** `components/git-ref-picker/{RepoRefPicker.tsx,use-repo-ref-search.ts}` (mới), `components/settings/BaseRefPicker.tsx` (sửa thành lớp mỏng), test
**Depends on:** Không
**Status:** [x] DONE (2026-10-19)

---

## Context

- `BaseRefPicker` có logic tìm kiếm (debounce, giới hạn truy vấn), mặc định, đếm remote; props công khai `{repoId, currentBaseRef?, onSelect, onUsePrimary?}`.
- Dùng ở Settings và `SourceControl.tsx`; không được đổi hành vi của hai nơi này.

## Việc cần làm

1. Tách logic tìm ref/mặc định/đếm remote thành `use-repo-ref-search.ts` (hook) và phần trình bày thành `RepoRefPicker` với props `{repoId, value?, onSelect, onUseDefault?, placeholder?, disabled?}`; trạng thái `loading|empty|error|offline`.
2. `settings/BaseRefPicker` giữ chữ ký, gọi `RepoRefPicker`; test hiện có của nó xanh không sửa kỳ vọng.
3. Hỗ trợ bàn phím (mũi tên, Enter, Esc) và nhãn truy cập (`role=listbox`, `aria-activedescendant`).
4. Ref remote `a/b/c` hiển thị đầy đủ, cắt giữa nếu dài, có tooltip; không dùng màu làm tín hiệu duy nhất.
5. Tên file theo nội dung (không `helpers`/`utils`).

## Kiểm thử

- Test hook: debounce, huỷ yêu cầu cũ, giới hạn độ dài truy vấn, lỗi mạng, mặc định không bịa.
- Test hiện có của `BaseRefPicker` và `SourceControl` liên quan ref xanh.
- Test bàn phím và truy cập.

## Tiêu chí hoàn thành

- [x] `RepoRefPicker` dùng được độc lập; Settings và SourceControl không đổi hành vi.
- [x] Đủ trạng thái và truy cập bàn phím.

## Rủi ro và lưu ý

- `SourceControl.tsx` lớn: chỉ chạm qua `BaseRefPicker`, không sửa trực tiếp.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.

## Ghi chú triển khai (2026-10-19)

**File đổi/tạo** (dưới `frontend/src/renderer/src/`):
- `components/git-ref-picker/use-repo-ref-search.ts` (mới): hook chia sẻ. Gắn nhãn mọi trạng thái theo `repo + runtime owner` nên đổi repo không bao giờ hiện mặc định/kết quả/truy vấn của repo trước; debounce 200 ms; bỏ truy vấn < 2 ký tự và quá 2 KiB (hiện "không có kết quả" như cũ, không gửi); bỏ phản hồi cũ (IPC không huỷ được nên dùng cờ `stale`); mặc định không bịa (`null` khi chưa biết/không có/lỗi); `remoteCount` 0 khi lỗi (không hiện gợi ý sai); trạng thái tìm `idle|searching|results|empty|error|offline` (offline = `navigator.onLine === false` hoặc mã `runtime_unavailable|remote_runtime_unavailable|connection_refused|timeout`); `retry()`.
- `components/git-ref-picker/RepoRefPicker.tsx` (mới): props `{repoId, value?, onSelect, onUseDefault?, placeholder?, disabled?}` cộng hai prop tuỳ chọn `renderHeader` (Settings đặt phần tóm tắt + gợi ý nhiều remote vào đúng chỗ cũ) và `className`. Combobox + `role=listbox`/`option`, `aria-activedescendant`, `aria-selected`, vùng `role=status` `aria-live=polite`; mũi tên (vòng), Enter, Esc (xoá truy vấn trước, `stopPropagation`), bỏ qua phím khi đang gõ IME, Enter không submit form bao quanh; dòng "Default branch (<tên>)" khi có `onUseDefault`; hàng được chọn có chữ "Current" + biểu tượng tick (không chỉ dựa vào màu); `aria-label` đầy đủ của ref cho mỗi hàng (nhãn cắt đôi sẽ bị đọc thành hai từ).
- `components/git-ref-picker/MiddleTruncatedRef.tsx` (mới): ref `a/b/c` hiển thị đủ khi vừa, cắt giữa bằng CSS khi dài (đuôi = đoạn cuối), tooltip chứa tên đầy đủ.
- `components/settings/BaseRefPicker.tsx`: còn là lớp mỏng (chữ ký không đổi), chỉ giữ phần tóm tắt và nút "Use Primary" bằng các khoá cũ.
- i18n: nhóm `auto.components.gitRefPicker.RepoRefPicker.*` (5 locale). 9 khoá `auto.components.settings.BaseRefPicker.*` đã chuyển sang nhóm mới (giữ nguyên bản dịch, sửa ví dụ `upstream`, `upstream/main` thành không dịch ở ja/zh vì là tên người dùng gõ) và bị xoá khỏi 5 file; không còn nơi nào tham chiếu chúng (đã grep). 8 khoá mới có bản dịch tay cho es/ja/ko/zh.
- Test mới: `use-repo-ref-search.test.tsx` (22), `RepoRefPicker.test.tsx` (36), `settings/BaseRefPicker.test.tsx` (12), `repo-ref-picker-locale-coverage.test.ts` + helper `test-support/translate-call-locale-assertions.ts` (đọc các lời gọi `translate('khoá','dự phòng')` từ mã nguồn rồi kiểm 5 catalog).

**"Test hiện có của BaseRefPicker"**: `BaseRefPicker` không có test riêng trước đây; `RepositoryWorktreeDefaultsSection.test.tsx` mock nó. Nên đã viết `BaseRefPicker.test.tsx` mô tả hành vi cũ (tóm tắt 3 trạng thái, Use Primary, tìm kiếm, chọn, đánh dấu hiện hành). Không sửa kỳ vọng của test nào có sẵn.

**Khác với hành vi cũ (có chủ đích)**: lỗi tìm kiếm trước đây bị nuốt và hiện "No matching branches found."; nay hiện lỗi + nút Retry (đúng yêu cầu trạng thái `error|offline`), và vẫn `console.error`. Ngoài ra bàn phím/ARIA mới, nhãn "Current" có thêm tick.

**Blast radius**: `BaseRefPicker` chỉ được import ở `RepositoryWorktreeDefaultsSection.tsx` (Settings) và `right-sidebar/SourceControl.tsx` (hộp thoại chọn base ref; chữ ký giữ nguyên nên không sửa file này). `tsc` không có lỗi mới ở cả hai nơi.

**Kiểm chứng đã chạy**
- `npx vitest run --config config/vitest.config.ts src/renderer/src/components/git-ref-picker src/renderer/src/components/settings/BaseRefPicker.test.tsx src/renderer/src/components/settings/RepositoryWorktreeDefaultsSection.test.tsx` xanh (70 + 5).
- `npx vitest run ... src/renderer/src/components/right-sidebar/SourceControl` : 16 file, 132 test xanh.
- Kiểm kiểu: `cd frontend && npx tsc --noEmit -p tsconfig.json --composite false --incremental false` có 114 lỗi có sẵn (admin/fleet, store/ui, test mock `AppState`...); sau thay đổi còn 113, không lỗi mới (bớt 1 lỗi `beforeEach` không dùng ở `request.test.ts`). Dùng `--incremental false` để không ghi đè `frontend/tsconfig.tsbuildinfo`.
- Lint: `npx oxlint <các file đổi>` sạch. `pnpm run lint` nguyên bản không chạy được ở gốc repo vì repo đã tách (`config/scripts/verify-localization-catalog.mjs`, `config/oxlint-switch-exhaustiveness.json` nằm ở `desktop/`), và `check:max-lines-ratchet` báo lỗi có sẵn ở các file `desktop/`/`tests/` không thuộc task này.
- i18n: `verify-localization-catalog` và `audit-localization-coverage --check` hiện **thất bại sẵn trên HEAD** (284 tham chiếu khoá thiếu trong `en.json`, 1227 chuỗi cứng; chủ yếu `web/`, `DepartmentGate`...). Đã chạy hai script `desktop/config/scripts/*` trên bản sao `src/renderer/src` ở scratchpad (script đòi `<root>/src/{main,renderer/src}`) và so sánh đầu ra trước/sau: **giống hệt**, tức không thêm khoá thiếu hay chuỗi cứng. Thêm kiểm ngang hàng: không locale nào thiếu/thừa khoá hay lệch placeholder so với `en` do thay đổi này (631 khoá thiếu ở es/ja/ko/zh là có sẵn trên HEAD).

**Chưa kiểm / lưu ý**
- Chưa xem trực quan trong ứng dụng (happy-dom không có bố cục): độ cắt giữa của nhãn, tooltip trong hộp thoại SourceControl, và việc cuộn bằng bánh xe trong Radix Dialog (giữ nguyên bộ xử lý wheel cũ) cần mắt người.
- Esc: Radix Dialog/Popover lắng nghe Escape ở pha capture trên `document` nên **vẫn đóng** trước khi picker kịp xoá truy vấn; picker chỉ chặn Esc lan trong cây React. Nơi dùng trong popover (task 03) nên truyền `onEscapeKeyDown` khi truy vấn không rỗng nếu muốn "Esc xoá trước".
- Cần `TooltipProvider` ở gốc (đã có ở `App.tsx`); test phải bọc `TooltipProvider`.
- Nhập tay ref khi offline/lỗi (SOL-040 mục 2.3) chưa nằm trong picker: dùng `validateBaseRefSyntax` ở task 03.
- Hiệu năng tìm ref trên host SSH chưa đo.
