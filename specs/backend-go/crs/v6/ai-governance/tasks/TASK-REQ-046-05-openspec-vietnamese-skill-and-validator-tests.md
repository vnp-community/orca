# TASK-REQ-046-05: OpenSpec bằng tiếng Việt: mục Language trong kỹ năng và bằng chứng qua parser, bộ kiểm gói và `openspec validate`

**From Solution:** [BE-REQ-SOL-046](../solutions/BE-REQ-SOL-046-ai-output-language.md)
**Priority:** P1
**Service:** `request-service`, `skills/`
**File:** `skills/tdd-openspec-author/SKILL.md` (sửa, repo gốc), `backend-go/services/request-service/internal/domain/testdata/openspec_vietnamese/` (mới: `proposal.md`, `specs/dang-nhap/spec.md`, `tasks.md`), `internal/domain/openspec_vietnamese_test.go` (mới)
**Depends on:** TASK-REQ-046-01
**Status:** [x] DONE (2026-10-20)

---

## Context

- Đã chạy `openspec validate <id> --strict --json` (1.3.0) trên thay đổi tiếng Việt, ngày 2026-10-20 (CR-REQ-046 mục 1.5): hợp lệ khi có `SHALL`/`MUST`, tiêu đề `## ADDED Requirements` và `#### Scenario:`; không hợp lệ khi câu yêu cầu viết "phải" thay `SHALL`/`MUST` hoặc tiêu đề delta bị dịch; `#### Kịch bản:` qua `openspec` nhưng Orca không ghép được tiêu chí chấp nhận (`scenarioNameRe`, `domain/openspec_bundle.go:57`). Không cần `openspec init`; thư mục có `openspec/changes/<id>/` (và `git init`) là đủ.
- Orca đọc ngữ pháp `tasks.md` bằng parser chặt (`tasks_md_parser.go:16-21`, tiêu đề ≤ 200 rune ở `:154`) và `CheckBundle` đòi bốn dòng `Serves:`, `Code:`, `Verify:`, `Done when:` (`openspec_bundle.go` `descriptionKeys`), từ `modify`/`create` trong `Code:`, và `#### Scenario:` bắt đầu bằng id tiêu chí (`anyScenarioStartsWith`, ranh giới tính theo byte đầu của phần còn lại, an toàn với khoảng trắng và dấu `:`).
- Kỹ năng `tdd-openspec-author` được cài từ nhánh mặc định của `vnp-community/orca` bằng `npx skills add` (`docs/guides/request/knowledge-hub-setup.md:19-24`); sửa `SKILL.md` chỉ có hiệu lực sau khi hợp nhất và `npx skills update tdd-openspec-author --global` trên từng dev server. Prompt (`_language_rule_openspec.tmpl`, task 01) là nguồn chính.

## Việc cần làm

1. `skills/tdd-openspec-author/SKILL.md`: thêm mục `## Language` sau `## Inputs you are given` (khoảng 6 dòng): văn xuôi (proposal, design, câu yêu cầu, bước kịch bản, tiêu đề và văn bản `Serves`/`Verify`/`Done when`) viết bằng ngôn ngữ mà prompt nêu ("Output language: …"); không có chỉ thị thì theo ngôn ngữ của Request; luôn giữ tiếng Anh các tiêu đề ngữ pháp (`## Why`, `## What Changes`, `## ADDED|MODIFIED|REMOVED|RENAMED Requirements`, `### Requirement:`, `#### Scenario:`, `## PH-n`, `## TASKS`), `SHALL`/`MUST`, `WHEN`/`THEN`/`AND`, nhãn dòng task, thuộc tính, `modify`/`create`, id, đường dẫn, mã và khối kết thúc (`Change:`, `Phases:`, `Validation:`...). Không đổi các kỹ năng khác. Ghi vào PR: hiệu lực cần hợp nhất và `npx skills update`; `SkillHash` của thay đổi tạo từ nay khác.
2. Dữ liệu mẫu `testdata/openspec_vietnamese/`: một thay đổi hoàn chỉnh bằng tiếng Việt có dấu (proposal, một spec với hai requirement mỗi cái có `SHALL`/`MUST` và `#### Scenario: AC-1 …`/`AC-2 …`, `tasks.md` với hai phase, bốn task đủ bốn dòng mô tả, thuộc tính `[ac=AC-1]`, `[depends=…]`).
3. `openspec_vietnamese_test.go`: (a) `ParsePlanRegion` đọc `tasks.md` mẫu không vi phạm, tiêu đề giữ nguyên dấu; (b) `CheckBundle` với `ACIDs` `AC-1`, `AC-2` và `PathProbe` giả: không vi phạm; tên kịch bản "AC-1 Đăng nhập…" khớp, "AC-10 …" không khớp `AC-1`; (c) ca âm: bản dịch `#### Kịch bản:` làm `BundleACNotCovered`; thiếu nhãn `Done when:` (viết "Xong khi:") làm vi phạm; `[nhãn=…]` thuộc tính lạ làm `TASKSMD_UNKNOWN_ATTR`; (d) khi `exec.LookPath("openspec")` thành công: chép mẫu vào `t.TempDir()/openspec/changes/<id>/`, chạy `openspec validate <id> --strict --json`, khẳng định `valid: true`; thêm hai ca âm (viết "phải" thay `SHALL`/`MUST`; dịch tiêu đề delta) khẳng định `valid: false` với đúng thông báo; thiếu CLI thì `t.Skip` có thông báo (CI hiện có thể không cài `openspec`).
4. Kiểm tay (ghi vào PR, không phải test tự động): đọc `plan_openspec/_language_rule_openspec.tmpl` đã render (golden `phases_medium_knowledge_language_vi`) đối chiếu với bảng 1.5 của CR và với `SKILL.md`; không mâu thuẫn.

## Kiểm thử

- `cd backend-go/services/request-service && go test ./internal/domain/ -run 'OpenSpecVietnamese' -race -count=1` xanh (ca CLI chạy nếu có `openspec`; ghi vào PR máy có CLI hay bỏ qua).
- `grep -n '^## Language' skills/tdd-openspec-author/SKILL.md` có đúng một mục.
- Run OpenSpec thật (7 đến 13 USD) **không** nằm trong task này.

## Tiêu chí hoàn thành

- [x] Gói mẫu tiếng Việt qua `ParsePlanRegion`, `CheckBundle` và (khi có CLI) `openspec validate --strict`. (`TestOpenSpecVietnamese_TasksMdParsesWithAccentedTitles`, `_BundlePassesCheckBundle`, `_OpenSpecValidateAcceptsTheBundleAndRefusesTranslatedGrammar`: xanh trên máy có `openspec` 1.3.0)
- [x] Ca âm chứng minh phần nào phải giữ tiếng Anh: `SHALL`/`MUST`, tiêu đề delta, `#### Scenario:`, nhãn dòng task, thuộc tính. (`TestOpenSpecVietnamese_TranslatedGrammarIsRejected`: 7 ca; hai ca `openspec validate` âm: "phải" thay `SHALL`/`MUST` và tiêu đề delta dịch; ca `#### Kịch bản:` qua `openspec` nhưng Orca báo `REQUEST_SPEC_AC_NOT_COVERED`)
- [x] `SKILL.md` có mục Language nhất quán với chỉ thị trong prompt; PR nêu việc phân phối kỹ năng. (`grep -c '^## Language' skills/tdd-openspec-author/SKILL.md` = 1; đối chiếu tay với `_language_rule_openspec.tmpl`: cùng danh sách phải giữ tiếng Anh. Việc phân phối: chỉ có hiệu lực sau khi hợp nhất và `npx skills update tdd-openspec-author --global` trên từng dev server; `SkillHash` của thay đổi tạo từ nay khác: nêu trong PR.)

## Rủi ro và lưu ý

- Hành vi khi kỹ năng đã cài (không có mục Language) bất đồng với prompt chưa kiểm; prompt vẫn nêu đủ.
- Phiên bản `openspec` trên dev server có thể khác 1.3.0 (`REQUEST_OPENSPEC_MIN_VERSION`); ca CLI chỉ chứng minh cho phiên bản chạy test.
- Trần byte của gói (tệp ≤ 256 KiB, tổng ≤ 1 MiB, `tasks.md` ≤ 1 MiB) co lại khoảng 1,5 lần theo ký tự với tiếng Việt; chưa đo trên thay đổi thật.

## Ghi chú triển khai (2026-10-20)

**Tệp:** `skills/tdd-openspec-author/SKILL.md` (mục `## Language` sau "Inputs you are given", một đoạn), `backend-go/services/request-service/internal/domain/testdata/openspec_vietnamese/{proposal.md,specs/dang-nhap/spec.md,tasks.md}` (REQ-901, 2 phase, 4 task đủ bốn dòng mô tả, hai AC), `internal/domain/openspec_vietnamese_test.go`.

**Lệnh và kết quả:** `cd backend-go/services/request-service && go test ./internal/domain/ -run 'OpenSpecVietnamese' -race -count=1`: ok (5,5 s; ca CLI chạy thật vì máy có `openspec` 1.3.0, ~4 s). `go vet ./internal/domain/`, `gofmt -l`: sạch.

**Phát hiện khi viết ca âm (đưa vào test, không phải lỗi sản phẩm):**
- Khoá thuộc tính dịch có dấu (`[tiêu_chí=AC-1]`) KHÔNG làm `TASKSMD_UNKNOWN_ATTR`: ngữ pháp thuộc tính của parser là ASCII (`[a-zA-Z0-9_]+`) nên cả cụm rơi vào tiêu đề task, và hệ quả là `REQUEST_SPEC_AC_NO_TASK` ở `CheckBundle`. Chỉ khoá ASCII lạ (`[nhan=...]`) mới ra `TASKSMD_UNKNOWN_ATTR`. Bản đặc tả task ghi `[nhãn=…]` ra `TASKSMD_UNKNOWN_ATTR`: thực tế là như trên; test ghi đúng hành vi thật.
- Tên kịch bản "AC-1 đăng nhập" (chữ Việt thường ngay sau id) vẫn khớp `AC-1`: ranh giới chỉ tính byte đầu của phần còn lại.

**Blast radius:** chỉ thêm tệp test, testdata và một mục trong `SKILL.md`; không đổi mã sản xuất. **Chưa kiểm:** bản `openspec` khác 1.3.0; hành vi khi kỹ năng đã cài (chưa có mục Language) bất đồng với prompt; trần byte của gói với tiếng Việt trên thay đổi thật; không chạy OpenSpec thật (7 đến 13 USD).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
