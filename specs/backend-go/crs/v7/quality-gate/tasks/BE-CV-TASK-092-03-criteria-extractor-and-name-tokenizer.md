# BE-CV-TASK-092-03: Domain rút tiêu chí từ `Task.Description` và tokenizer tên

**From Solution:** BE-CV-SOL-092-requirement-trace
**Priority:** P2
**Service:** `code-intel-service`
**File:** `internal/domain/requirement_criteria_extractor.go`, `requirement_name_tokenizer.go` (mới)
**Depends on:** BE-CV-TASK-092-02
**Status:** [x] DONE

## Việc cần làm
1. Extractor: tiêu đề song ngữ (`acceptance criteria`, `tiêu chí (chấp nhận|hoàn thành)`, `definition of done`, `yêu cầu`) → danh sách; không có thì `- [x]/- [x]`; không có nữa ⇒ một `title_only`. ≤ 20 mục, ≤ 300 ký tự; `[x]` không là bằng chứng; **không** đọc `AIContext`/`AIPlanJSON`.
2. `key="task:<task_id>#<sha256(chuẩn hoá NFKC, thường, bỏ dấu)[:8]>"`.
3. Tokenizer: tách camelCase/snake/kebab, bỏ dấu, từ dừng; Jaccard cho `name_overlap` (ngưỡng 0,5 = giá trị khởi điểm chưa hiệu chỉnh; ≤ 5 gợi ý/yêu cầu).

## Kiểm thử
- Bảng: Markdown tiếng Việt/Anh, danh sách lồng, tick, tiêu đề lạ, rỗng, 100 mục; tokenizer ca "xấu"; fuzz không panic.

## Tiêu chí hoàn thành
- [x] 3 dòng "Acceptance criteria" ⇒ 3 yêu cầu `checklist_heuristic`; [ ] không tiêu chí ⇒ `title_only` + `no_structured_criteria`.

## Rủi ro
- Phần lớn mô tả thật có thể chỉ cho `title_only` (chưa đo).
