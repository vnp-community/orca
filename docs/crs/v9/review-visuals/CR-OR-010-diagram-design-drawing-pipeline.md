# CR-OR-010 — Pipeline vẽ bằng `diagram-design` (bản mặc định MVP)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OR-010 |
| **Tên** | Phân phối skill `diagram-design` (bản ghim), preset `orca-default`, *draw run* chỉ vẽ phần trượt cache, kiểm tra tự động (`self_check.py` và bộ lọc phía Orca), lùi về bảng khi vẽ lỗi |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-20 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-OR-002 (mô hình sơ đồ `DiagramModel`), CR-OR-011 (kho, khoá cache); skill `diagram-design` (MIT, 2.6) |
| **Mở khoá** | CR-OR-012, 013, 021 |
| **Tác động** | `skills/diagram-design/` (mới, bản ghim + LICENSE), `request-service` (`usecase/outcome_review_draw_run.go`, `usecase/outcome_review_draw_prompt.go`, `domain/outcome_review_artifact_check.go`, `domain/outcome_review_preset.go`), `dev_server_capability.go` (kiểm skill), `docs/guides/request/` (hướng dẫn cài) |

---

## 1. Bối cảnh và vấn đề

1. **Skill** (đọc qua trang repo ngày 2026-10-20; **chưa đọc** `references/`): sinh một tệp HTML tự đủ chứa SVG tĩnh, không JS, giấy phép MIT, phiên bản 2.6. Quy trình: (1) cổng *style guide* hỏi người dùng nếu còn token mặc định, (2) chọn loại sơ đồ, (3) xác nhận kế hoạch, (4) vẽ từ template gần nhất, (5) chạy bảng kiểm "taste gate" và `scripts/self_check.py`, (6) nhập từ draw.io/Mermaid/Excalidraw (tuỳ chọn), (7) xuất SVG/PNG khi được yêu cầu (PNG cần Playwright). Màu theo **vai trò** (paper, ink, muted, accent, link) chứ không phải hex trong sơ đồ; một accent cho 1–2 phần tử trọng tâm; mọi toạ độ chia hết cho 4; chỉ Google Fonts là tài nguyên ngoài; có biến thể `minimal` sáng/tối và `full editorial`; chuyển động là tuỳ chọn.
2. Các loại phù hợp với review (theo bảng khuyến nghị của skill): *architecture* (và *architecture delta* cho thay đổi), *sequence*, *ER/data model* và *database schema* (bảng vật lý), *state machine*, *swimlane*, *treemap*, *dependency graph*, *data flow*.
3. **Cổng style guide hỏi người dùng** không dùng được trong lượt chạy không tương tác của agent (rủi ro treo hoặc hỏng).
4. Orca đã có cách phân phối skill cho dev server: bốn skill `tdd-*` nằm ở `skills/` của repo và cài bằng `npx skills add https://github.com/vnp-community/orca --skill … --global`; **chưa có** `diagram-design` trong `skills/`.
5. v7 chọn Mermaid + bộ vẽ SVG tự viết ở frontend và **không thêm thư viện bố cục** (O5). v9 chọn hướng khác có chủ đích: hình do agent vẽ một lần, lưu lại (kiến trúc và thiết kế của yêu cầu này), nên không thêm thư viện vào gói frontend.
6. AGENTS.md: không tự đặt màu/cỡ chữ mới khi style guide đã có; màu phải đến từ token trong `main.css`.

## 2. Giải pháp đề xuất

### 2.1 Phân phối skill (ghim, không tự cập nhật)

- Vendor bản skill đã duyệt vào `skills/diagram-design/` tại một **commit cố định**, kèm `LICENSE` (MIT) và `NOTICE` ghi nguồn và commit. Loại các script không dùng ở Orca khỏi bản vendor hoặc ghi rõ "không được gọi" (đặc biệt `export_png.py`/Playwright và các bộ trích xuất draw.io/Mermaid/Excalidraw).
- Cài trên dev server theo mẫu hiện có: `npx skills add https://github.com/vnp-community/orca --skill diagram-design --global` (cần xác nhận chạy được; lệnh cài của repo gốc chưa thấy trong trang đã đọc). Cập nhật là thao tác có chủ đích: đổi commit trong repo → `npx skills update diagram-design --global`.
- `skill_version` ghi vào mọi artifact = `<phiên bản trong SKILL.md>+<commit ngắn>`; đổi phiên bản thì khoá cache đổi và hình tự vẽ lại ở lần sinh kế tiếp.
- Mở rộng hồ sơ khả năng dev server: khả năng `reviewDraw` = đủ skill + đọc được `self_check.py`. Thiếu → `REQUEST_REVIEW_SKILL_MISSING` kèm gợi ý cài; **chỉ chặn draw run**, context run vẫn chạy (CR-OR-002 2.7). Hiện trong `orca engine show --check`.

### 2.2 Chạy không tương tác với hồ sơ cố định

Draw run được cấp **hồ sơ vẽ đã thực thể hoá** (từ preset, CR-OR-013) dưới dạng style guide của skill và lời nhắc cấm hỏi: "Không đặt câu hỏi; dùng hồ sơ đã cho; nếu không vẽ được, trả lỗi có mã". Cách thực thể hoá hồ sơ (tệp marker `.diagram-design`, style guide trong thư mục làm việc tạm) phụ thuộc `references/profiles.md` chưa đọc: **spike S1**.

### 2.3 Preset `orca-default`

Loại sơ đồ theo mục như README mục 3.8. **Architecture và Flow luôn ở mức container (C4 level 2)**: lời nhắc nêu rõ nút/làn là container, mỗi nút ghi `[kind: tech]` (ví dụ `[Service: Go, gRPC]`), mỗi cạnh/tin nhắn ghi giao thức, cạnh `deviation`/`missing` dùng accent duy nhất kèm mã hoá phụ (nét đứt, nhãn chữ), nút có `componentsChanged > 0` mang huy hiệu "n components changed"; cấm nút ở mức component và tự gọi. Skill có kiểu *architecture delta* và *sequence* nhưng **chưa xác nhận** chúng có ký hiệu gần C4 (tên + công nghệ trong nút, nhãn giao thức); nếu không, dùng kiểu chung và đặt quy ước C4 trong hồ sơ (spike S4). Các "núm" của skill: biến thể `minimal`; chuyển động tắt; kiểu phác thảo tay tắt; biểu tượng tắt; mức chi tiết `balanced`; đối tượng `mixed` (người duyệt vừa kỹ thuật vừa quyết định); ngôn ngữ nhãn tiếng Anh; cỡ khung theo preset "trung bình" của skill (tên preset chốt khi đọc `references/`). Ngân sách phức tạp: ≤ 12 nút, ≤ 6 làn, ≤ 8 sơ đồ mỗi review (đã được ép ở CR-OR-002 2.4).

**Màu:** nguyên tắc "một accent" của skill dành cho phần trọng tâm. Diff cần phân biệt thêm/sửa/xoá, nên preset khai báo các vai trò bổ sung `diff-add`, `diff-mod`, `diff-del` nối vào token diff đã có của `main.css` (tên token chốt khi đọc `main.css` và STYLEGUIDE) và **luôn có mã hoá phụ ngoài màu**: nét đứt + gạch ngang chữ cho "xoá", `+`/`~`/`−` cạnh nhãn, nét đậm cho "sửa". Lệch thiết kế (`flag: deviation`) dùng vai trò accent duy nhất. Trường hợp skill không cho thêm vai trò mà vẫn qua `self_check`: **spike S2**; phương án lùi: chỉ accent + mã hoá phụ.

### 2.4 Draw run

**Theo pha:** ở pha `design` draw run chỉ vẽ các sơ đồ `variant=design` (Architecture, Flow, Behavior, Data nếu KG mô tả; tối đa 4) nên rẻ và chạy được trước khi có code. Ở pha `delivered` chỉ vẽ `variant=delta` (tối đa 4); các hình `design` trúng cache (cùng `inputDigest` của mô hình thiết kế) và không vẽ lại.


1. Với mỗi `DiagramModel` của review, tra khoá `(section, inputDigest, presetDigest, skillVersion)` ở kho (CR-OR-011). Trúng → bỏ qua.
2. Gom **tất cả phần trượt** vào **một** lượt agent (một lần trả chi phí khởi động); nếu không có phần trượt, **không gọi agent**.
3. Lời nhắc mỗi sơ đồ: loại, hồ sơ, mô hình sơ đồ (JSON), yêu cầu trả `{section, html, selfCheck}`; nhãn lấy **nguyên văn** từ mô hình, không thêm nút/cạnh/chữ ngoài mô hình và chú giải cố định.
4. Hạn mức: tối đa 1 lần sửa mỗi sơ đồ khi `self_check` báo lỗi; thời gian và lượt tối đa theo cấu hình (đo ở CR-OR-030).
5. HTML trả **trong khối kết quả** (không phụ thuộc tệp trên đĩa của dev server; tránh bẫy "Path outside project root"); mỗi artifact ≤ 256 KiB. Script `self_check.py` chạy trên tệp tạm do agent tự tạo trong thư mục làm việc của nó, chỉ để lấy kết quả kiểm.
6. Hồ sơ quyền: ghi tệp tạm trong thư mục làm việc của draw run, **không** ghi vào repo; không mạng; chỉ chạy `self_check.py` (V4).

### 2.5 Kiểm tra phía Orca (xác định, trước khi lưu)

| Kiểm | Quy tắc |
|---|---|
| An toàn | Không `<script>`, không thuộc tính `on*`, không `javascript:`/`data:` ở `href`/`src` ngoài `data:image/*` nhỏ, không `<iframe>`, `<object>`, `<embed>`, `<foreignObject>`; không `@import`; không URL ngoài trong `src`, `href`, `url()` (kể cả Google Fonts: phông do hồ sơ ghi đè bằng phông hệ thống/Orca) |
| Hình dạng | XML hợp lệ, có `<svg>` với `viewBox`, có `<title>` và `<desc>`, id có tiền tố khe (`<slug>-title`) |
| Trung thực | Chữ hiển thị trong SVG phải thuộc tập `nhãn của mô hình ∪ `tech`/`protocol` của mô hình ∪ chú giải cố định`; vượt ngưỡng chữ lạ (đề xuất 5%, chưa đo) → `failed` (chống bịa thành phần và chữ bị chèn) |
| Kích thước | ≤ 256 KiB, ≤ 3 000 phần tử SVG |
| Kết quả `self_check` | Lưu nguyên văn tóm tắt; lỗi nghiêm trọng sau lần sửa cuối → `failed` |

Không vượt → artifact `state=failed` với `error_code=REQUEST_REVIEW_ARTIFACT_UNSAFE|…`, UI lùi về bảng (2.6). Nội dung vẫn được CR-OR-030 xem là không tin cậy khi hiển thị, kể cả khi đã qua bộ kiểm này.

### 2.6 Lùi về bảng

Sơ đồ lỗi → mục tương ứng hiển thị dạng bảng (đã có trong tầng mục) kèm nhãn "Hình vẽ không tạo được" và nút "Vẽ lại" (CR-OR-012). Không tự thử lại ngoài một lần sửa của draw run (V5).

### 2.7 Cấu trúc file

`usecase/outcome_review_draw_run.go`, `outcome_review_draw_prompt.go` (có `drawPromptVersion`, tham gia khoá cache qua `presetDigest`), `domain/outcome_review_artifact_check.go` (bộ kiểm 2.5), `domain/outcome_review_preset.go` (preset mặc định bất biến, `presetDigest`). Không `helpers/utils`.

## 3. Quyết định thiết kế

1. **Vẽ ở agent một lần, lưu lại**: đổi chi phí lặp lại (mỗi lần mở) lấy chi phí một lần.
2. **Mô hình sơ đồ nhỏ làm đầu vào**, không đưa mã nguồn cho bước vẽ.
3. **Bộ kiểm "chữ phải có trong mô hình"** để hình không tự bịa thành phần.
4. **Không tự động nâng cấp skill**; `skill_version` nằm trong khoá cache.
5. **Chỉ vẽ bốn loại ở MVP**; các loại khác (treemap, dependency graph, data flow, swimlane) mở bằng CR-OR-013.
6. **Không dùng `full editorial` và chuyển động** ở MVP: đắt hơn và không cần cho review.

## 4. Tiêu chí chấp nhận

- [ ] Review có mô hình sơ đồ cho 4 mục → đúng 1 lượt agent (draw) và 4 artifact `active`; lượt thứ hai cùng đầu vào không gọi agent.
- [ ] Đổi một mô hình sơ đồ → chỉ sơ đồ đó vẽ lại; ba sơ đồ còn lại tái dùng.
- [ ] Hồ sơ cố định đủ để skill chạy không hỏi (S1 đạt); nếu skill hỏi, lượt chạy trả `REQUEST_REVIEW_RESULT_INVALID`, không treo.
- [ ] Artifact chứa `<script>`, `onload=`, URL ngoài, hoặc chữ ngoài mô hình bị `failed`; UI lùi về bảng.
- [ ] Không có giá trị hex nào do agent tự chọn trong artifact lưu (chỉ vai trò/biến) khi S2 đạt; nếu không, hex đến từ hồ sơ.
- [ ] Thiếu skill → `REQUEST_REVIEW_SKILL_MISSING`, context run vẫn hoạt động.
- [ ] `self_check` lỗi → tối đa 1 lần sửa, sau đó `failed`; không vòng lặp.
- [ ] Mỗi artifact đều có `skill_version`, `preset_digest`, kết quả `self_check` ghi lại.

## 5. Kiểm thử

- **Unit**: bộ kiểm an toàn (bảng ca tốt/xấu gồm SVG chứa `foreignObject`, `style` với `@import`, `href="javascript:"`), kiểm chữ-trong-mô hình, digest preset.
- **Agent giả**: trả HTML tốt, HTML lỗi rồi sửa tốt, HTML lỗi hai lần, hỏi lại người dùng, thiếu nonce.
- **Golden**: 4 mô hình mẫu → HTML đã lưu (chạy thật một lần, chụp lại làm fixture cho test hiển thị; không dùng làm kiểm tra bit-for-bit vì LLM không xác định).
- **Chạy thật có ghi lại**: đo token và thời gian cho 1, 4 và 8 sơ đồ ≤ 12 nút (spike S3), ghi vào CR-OR-030.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Spike S1** (hồ sơ cố định, không tương tác), **S2** (vai trò màu bổ sung và điền token lúc hiển thị, D6), **S3** (chi phí thật), **S4** (đọc `references/` và xác nhận skill có ký hiệu gần C4 container cho *architecture delta*/*sequence*; về loại sơ đồ, `self_check.py`, ghi đè phông). Chưa spike nào được chạy.
- Hình do LLM vẽ không xác định; hai lần vẽ cùng mô hình có thể khác nhau (vì thế khoá cache theo mô hình và có nút sinh lại).
- Skill có script nhiều khả năng đòi Python và các phụ thuộc trên dev server (`python3 scripts/self_check.py`); chưa biết phụ thuộc.
- Lệnh cài của repo gốc không thấy trong trang đã đọc; phân phối qua `skills/` là suy từ mẫu hiện có.
- Ngưỡng "chữ lạ 5%" là phỏng đoán.

## 7. Câu hỏi mở

- **Q1.** Lưu HTML đầy đủ của skill hay chỉ `<svg>` (rồi bọc bằng khung của Orca)? Đề xuất: lưu HTML (nguyên bản) và để trình xem quyết định khung (CR-OR-021).
- **Q2.** Dùng model rẻ hơn cho draw run (mục đích cơ học) được không; phụ thuộc cấu hình `ai.complete` của dev server.
- **Q3.** Có cho tenant dùng bản skill khác (CR-OR-013 Q1).

## 8. Tham chiếu

- [README v9](../README.md) D4–D9, mục 3.8, 8
- Repo skill: <https://github.com/cathrynlavery/diagram-design> (MIT, 2.6; đọc 2026-10-20)
- [Hướng dẫn knowledge hub](../../../guides/request/knowledge-hub-setup.md) (cách cài skill)
- [README v7](../../v7/README.md) O5; [CR-CV-090](../../v7/quality-gate/CR-CV-090-exportable-review-report.md) (hướng Mermaid để so sánh)
- [AGENTS.md](../../../../AGENTS.md), [STYLEGUIDE](../../../../guides/STYLEGUIDE.md)
