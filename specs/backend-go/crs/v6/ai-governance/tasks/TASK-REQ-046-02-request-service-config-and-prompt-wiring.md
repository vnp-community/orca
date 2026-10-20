# TASK-REQ-046-02: `REQUEST_DEFAULT_OUTPUT_LANGUAGE` trong `request-service`: đọc cấu hình, dừng khi sai, truyền ngôn ngữ vào sáu nơi dựng prompt

**From Solution:** [BE-REQ-SOL-046](../solutions/BE-REQ-SOL-046-ai-output-language.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/config/config.go` (sửa), `cmd/server/output_language_wiring.go` (+ `_test.go`, mới), `cmd/server/{main.go,wire_solution.go,wire_solution_test.go}` (sửa), `internal/usecase/{analysis_settings,solution_prompt,plan_prompt,openspec_plan_prompt,run_solution_generation,run_agent_readonly_analysis,run_plan_generation,engine_openspec,engine_native}.go` (sửa), test cùng gói
**Depends on:** TASK-REQ-046-01
**Status:** [ ] TODO

---

## Context

- `AnalysisSettings` (`usecase/analysis_settings.go`) đã đến `RunSolutionGeneration`, `RunAgentReadonlyAnalysis`, `RunPlanGeneration` và `OpenSpecEngineDeps.Settings` (`engine_openspec.go:29`); `cmd/server/wire_solution.go:65` dựng một bản dùng chung (`analysisSettings(cfg)`, `:186`). `DefaultAnalysisSettings()`/`withDefaults()` giữ ngôn ngữ rỗng để test và golden hiện có không đổi.
- Sáu nơi dựng đầu vào/gọi render: `run_solution_generation.go:50,59`; `engine_native.go:47,52` (chỉ test gọi `NewNativeEngine`); `run_agent_readonly_analysis.go:109,125` và lần dựng lại không đồ thị `:141`; `run_plan_generation.go:71,80`; `engine_openspec.go:225`.
- `config.Load()` không trả lỗi và `EngineDefaultsFromEnv` (`domain/engine_settings.go:76`) chỉ cảnh báo khi giá trị sai. Biến này theo quyết định của người dùng phải **dừng service** khi sai (CR-REQ-046 L2); `main()` đã thoát mã 1 khi `run()` trả lỗi (`main.go:44-48`).

## Việc cần làm

1. `config.Config.OutputLanguage string`: `Load()` đọc `REQUEST_DEFAULT_OUTPUT_LANGUAGE` (cắt khoảng trắng, chưa phân tích).
2. `cmd/server/output_language_wiring.go`: `parseOutputLanguage(raw string) (output_language.Language, error)`: rỗng → `Vietnamese`; còn lại qua `output_language.Parse`; lỗi: `REQUEST_DEFAULT_OUTPUT_LANGUAGE "<raw>" is not supported; use one of: vi, en`. `run()` gọi nó **trước** `tracing.Init` và `openStores`, trả lỗi để `main` thoát mã 1; ghi `log.Info("output language", slog.String("code", …))`. Lệnh `healthcheck` không đi qua đây.
3. `analysisSettings` nhận ngôn ngữ đã phân tích (chữ ký `analysisSettings(cfg config.Config, lang output_language.Language)` hoặc tương đương) và đặt `OutputLanguage`; cập nhật `wire_solution_test.go:100-117`.
4. `AnalysisSettings.OutputLanguage` + `OutputLanguageFor(domain.Request) output_language.Language` (chỉ trả giá trị cấu hình; chú thích một dòng: chỗ cắm cho ghi đè theo dự án).
5. `SolutionPromptInput`, `PlanPromptInput`, `OpenSpecPlanPromptInput` thêm `Language output_language.Language` và chuyển tiếp sang `analysis.Input`, `plan_native.Input`, `plan_openspec.Input`.
6. Đặt `Language: <settings>.OutputLanguageFor(req)` ở sáu nơi trên, ngay khi dựng đầu vào (trước vòng thử lại, nên lần dựng lại ở `:141` cũng mang ngôn ngữ). `nativeEngine` thêm `WithOutputLanguage(l)`; `engine_openspec.go:225` dùng `e.d.Settings`.
7. Không ghi nội dung Request hay phân tích vào log; chỉ mã ngôn ngữ.

## Kiểm thử

- `parseOutputLanguage`: rỗng, `vi`, `VI `, `en`, `fr`, `vi-VN`, `vietnamese` (bảng).
- Cấu hình: `Load()` với và không có biến; khởi động với giá trị sai thoát có lỗi, **trước** khi mở kho dữ liệu (theo mẫu `TestRun_UnknownDSNExitsWithDialectError`, `cmd/server/startup_test.go:36`), thông báo nêu biến và giá trị.
- Mỗi nơi gọi: dùng fake đã có của test cùng gói để lấy prompt gửi đi; với `OutputLanguage` đặt thì prompt chứa `Output language: Vietnamese.`, không đặt thì không chứa; riêng `run_agent_readonly_analysis` kiểm cả lần dựng lại không đồ thị (`:141`); `RunPlanGeneration` và `OpenSpecEngine.runAgent` mang ngôn ngữ; `nativeEngine` với `WithOutputLanguage`.
- `go test ./... -race` trong `request-service`; `bash backend-go/ci/check-request-service-wiring.sh` xanh.
- `bash backend-go/ci/check-prompt-golden.sh` vẫn xanh (golden dùng ngôn ngữ rỗng).

## Tiêu chí hoàn thành

- [ ] Không biến: `vi`; `en`: English; giá trị sai: service thoát mã 1 trước khi mở kho, thông báo rõ.
- [ ] Cả sáu nơi gọi mang ngôn ngữ cấu hình; test chứng minh từng nơi.
- [ ] `DefaultAnalysisSettings()`/`withDefaults()` không đổi; mọi test và golden hiện có xanh.
- [ ] Mọi nơi lấy ngôn ngữ qua `OutputLanguageFor`, không đọc `OutputLanguage` trực tiếp ngoài hàm đó.

## Rủi ro và lưu ý

- Blast radius: `RenderSolutionPrompt` (năm nơi gọi, nhiều test), `AnalysisSettings` (bốn worker): chạy `gitnexus_impact` và ghi vào PR.
- Đổi chữ ký `analysisSettings` kéo theo test; không để tham số mặc định ẩn làm bật ngôn ngữ trong test khác.
- Đây là biến `REQUEST_DEFAULT_*` đầu tiên làm service dừng khi sai; ghi rõ trong tài liệu (task 06).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
