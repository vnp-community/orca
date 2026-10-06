# BE-CV-TASK-070-02: Collector chuẩn hoá tệp vàng (một và hai nguồn, không giữ `perf`)

**From Solution:** BE-CV-SOL-070
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/collector_golden_test.go` (mới); có thể thêm hàm giải mã nhỏ ở `.../internal/usecase/agent_result_decode.go` nếu CR-021 chưa có (mới, tên đề xuất)
**Depends on:** BE-CV-TASK-070-01, BE-CV-SOL-021 (collector), BE-CV-SOL-020 (`SymbolRef` domain)
**Status:** `[ ] TODO`

---

## Context

- Kết quả `Client.Exec` là `map[string]any` (`infra-fleet-service/.../devserveragent/client.go`); collector giải mã từ đó (agent-rpc §2.2).
- PQ-20: agent chuẩn hoá `SymbolRef` (dòng 1-based, `key`, `lineBase:1`); backend kiểm lại `key`, `NormalizeRepoPath`, và hợp nhất hai nguồn.
- PQ-12 và CR-CV-071: `perf` chỉ phục vụ metric/span, **không** vào cache snapshot và **không** ra gateway.

## Việc cần làm

1. `TestCollectorNormalizesAgentGolden` (bảng, theo từng tệp vàng một nguồn): fake cổng `AgentRelay` trả đúng nội dung tệp; khẳng định `SymbolRef.key` ổn định giữa hai lần chạy, `truncated`/`totalCount` giữ nguyên, `kind` thuộc tập chuẩn (ui-api §4.1), đường dẫn tương đối (không `/`, không `\`), `startLine ≥ 1`.
2. `TestCollectorMergesTwoSourcesGolden`: `merged/symbol-two-sources.json` → một `SymbolRef` với cả `gitnexusId` và `codegraphId`, `sources` có hai mục.
3. `TestCollectorDropsPerfBlock`: đầu ra chuẩn hoá (dạng sẽ lưu snapshot và sẽ trả gateway) không có khoá `perf`; khối `perf` được chuyển sang kênh quan sát riêng (task BE-CV-TASK-071-03 gắn metric; ở đây chỉ kiểm không rò).
4. Kiểm giới hạn: kết quả > 2 MiB sau chuẩn hoá bị cắt hoặc từ chối theo PQ-14 (tuỳ CR-021); dùng tệp tổng hợp trong test, không lưu tệp lớn vào `testdata`.
5. Không đọc mạng, không DB.

## Kiểm thử

- Các test ở mục 1 đến 4; `go test ./internal/usecase/... -run Golden`.
- Không cần ma trận hai dialect (không chạm DB); ghi rõ trong PR.

## Tiêu chí hoàn thành

- [ ] Mọi tệp vàng thành công trong `gitnexus-*/`, `codegraph-*/`, `merged/` đi qua collector.
- [ ] `perf` không còn trong đầu ra chuẩn hoá.
- [ ] Thêm tệp vàng mới tự được test bao phủ (bảng sinh từ thư mục).

## Rủi ro và lưu ý

- Tên hàm và cổng của collector là của CR-021; nếu đổi, sửa test này theo, không sửa tệp vàng.
- Collector chưa tồn tại; đừng viết test ở dạng "xanh giả" (thiếu `t.Fatal` khi thư mục rỗng).
