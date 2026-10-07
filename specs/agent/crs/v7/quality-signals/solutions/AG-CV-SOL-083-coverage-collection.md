# AG-CV-SOL-083: Thu thập coverage Go (giai đoạn A) và TS (giai đoạn B, cần duyệt), diff coverage, `quality.coverage`

> ✅ **Đã triển khai.** Ngày triển khai 2026-10-07. Đã hoàn thành toàn bộ các task 083-01 đến 083-07, kiểm thử tự động xác nhận qua vitest, đạt 100% tiêu chí chấp nhận.

**CR:** [CR-CV-083](../../../../../../docs/crs/v7/quality-signals/CR-CV-083-coverage-and-diff-coverage.md) mục 2.2 đến 2.5, 2.8 (phía agent). `coverage_reports`, `GetCoverage`, fallback "ước lượng" (2.6, 2.7) thuộc `BE-CV-SOL-083-coverage-storage-and-diff`. Task AG-CV-TASK-083-01 đến 07. **Khu vực:** `agent/src/relay/`. **Feature:** `quality-signals`.
**TDD/Spec:** [TDD-AG-01](../../../../tdd/v5/01-architecture.md), [api/agent-rpc-catalog-git-fs.md](../../../../api/agent-rpc-catalog-git-fs.md) (git Part A).

## 1. Hợp đồng áp dụng

| Mục ([`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md)) | Áp vào |
|---|---|
| §5.6 `quality.coverage {workspaceRoot, runId}`: `available`, `report{source,language,mode,headCommit,baseCommit,dirty,totals,diff,files,truncated,totalCount,toolVersions,estimatedNote}`; run chưa xong → `RUN_IN_PROGRESS`; không có bước coverage → `{available:false, reason:"no_coverage_step"}`; thiếu `@vitest/coverage-v8` → `ENV_NOT_READY reason="coverage_provider_missing"` (không ra số 0); `source:"estimated"` chỉ do backend; Go `-covermode=set -coverprofile=<tmp>/<module>.out ./...` mỗi module từ `go.work`, không `-coverpkg=./...`, không `-tags=integration`; payload ≤ 2 000 tệp, ≤ 1 MiB; `uncoveredRanges` chỉ cho tệp đã đổi hoặc `pct < 0.5` | tasks 02-07 |
| §5.2 `quality.run` (scope `changed`: module Go có tệp đổi; `RUN_IN_PROGRESS`) | task 05 |
| §4.8 mẫu `git diff` (`-c core.quotePath=false`, `--raw -z`, `--unified=0`, `ls-files --others --exclude-standard -z`) | task 03 |
| §9.5 profile có tên; O12/PQ-37: `@vitest/coverage-v8` **cần duyệt**, không thêm mặc định | task 07 (cổng duyệt) |
| PQ-33 (`CoverageReport` theo CR-083: `source: measured|estimated`) | task 06 |
| PQ-21, §8.3 | trích |

## 2. Lệch giữa CR và hợp đồng

| # | CR-CV-083 | Hợp đồng | Solution theo |
|---|---|---|---|
| 1 | Báo cáo `estimated` do agent/đồ thị | §5.6: `estimated` từ `ChangeOverlay.uncoveredSymbols` (backend) | Agent **chỉ** trả `source:"measured"`; không sinh `estimated` |
| 2 | Profile viết bằng YAML minh hoạ (`pnpm exec vitest`) | §9.5: argv chỉ mẫu hợp lệ; `{bin:vitest}` | Theo hợp đồng; vitest qua `{bin:vitest}`, không `pnpm exec` |
| 3 | `quality.coverage` nhận `runId` | PQ-21: thêm `workspaceRoot` | Theo hợp đồng |
| 4 | `partial:true` ở cấp báo cáo khi `scope=changed` | §5.6: `partial` nằm ở `diff.partial`, kèm `diff.modules[]`, `diff.noTests[]` | Theo hợp đồng |
| 5 | Mã nguồn đọc dòng bằng `git diff <mergeBase>` (cây làm việc) | §4.8: so với merge-base, `head` vắng = cây làm việc | Theo CR, cùng mẫu `-c core.quotePath=false --no-color --no-ext-diff` |
| 6 | Khối ngoài module đếm `unmappedBlocks` | §5.6 `diff.unmappedBlocks` | Theo |
| 7 | Module không có test `noTests:true` tính 0% hoặc loại (Q4) | §5.6 `diff.noTests[]` | Mặc định: **loại khỏi tổng**; thay đổi trong module không test không được "mất": ghi vào `diff.noTests` và `excludedFiles` (không trộn vào mẫu số) |
| 8 | Giai đoạn A chạy `go test` mọi module 21 | `scope=worktree` | `scope=changed` chỉ module có tệp đổi, `diff.partial:true` |
| 9 | Hàm theo tệp bằng `go tool cover -func` | §5.6 `files[].functions[{name,line,pct}]` | Giữ; tuỳ chọn, bỏ khi quá chậm (mảng rỗng) |

## 3. Phụ thuộc chéo khu vực

| Đối ứng | Quan hệ |
|---|---|
| `AG-CV-SOL-081-quality-runner-core`, `-profile-catalog-and-preflight` | Bước `coverage-go` chạy qua executor; profile đăng ký bằng API catalog (task 12); `go-modules` strategy dùng chung (task 15) |
| `AG-CV-SOL-082` | `inScope`, đường dẫn, che; coverage **không** sinh `QualityFinding` (category `coverage` dành cho cổng ở backend); không cần parser 082 trừ `category` |
| `AG-CV-SOL-084`, `091` | Dùng chung thành phần diff (task 03): thêm văn bản dòng được thêm |
| `BE-CV-SOL-083-coverage-storage-and-diff`, `BE-CV-SOL-036-…` | Lưu `coverage_reports`; so `estimated` với `measured`; danh sách loại trừ mặc định thống nhất với CR-036 (`ChangedFile.isTest/isGenerated`) |
| `BE-CV-SOL-085` | Dùng `diffCoverage`, `source` |
| Thứ tự: `082 → 083-AG` (đợt 8) |

## 4. Re-verify (đã đọc, 2026-10-06)

Đã đọc: `backend-go/go.work` (`go 1.26.0`, 21 mục `use`: `./common ./proto ./cmd/orca-cli` + 18 service), `agent/package.json` (devDeps có `vitest ^4.1.5`; **không** có `@vitest/coverage-v8`), `agent/vitest.config.ts` (không có khối `coverage`), `agent/src/relay/agent-git-handler-extended.ts:106` (mẫu `-c core.quotePath=false`), CR-083 và hợp đồng.

| Điểm | Hiện trạng | Hệ quả |
|---|---|---|
| Coverage trong repo | Không cấu hình coverage; không `-coverprofile` trong workflow/Makefile (CR) | Mọi thứ là mới |
| `go test` trên package không có test file | Go ≥ 1.22 in `coverage: 0.0%` cho package không test; chưa rõ **có ghi khối vào profile** không | Task 01 xác minh; ảnh hưởng `noTests` và `diff` |
| Nhiều module | `go test` chạy trong từng module (Makefile lặp) | Một bước mỗi module, ghép ở agent |
| TS | Mỗi gói một cấu hình vitest riêng (`frontend/config`, `desktop/config`, `agent/vitest.config.ts`) | Giai đoạn B theo gói, cần `@vitest/coverage-v8` cùng phiên bản `vitest 4.1.5` |

Correction relative to CR: CR-083 viết "21 mục `go.work`" — đúng (đã đếm). CR-083 2.5 cho `git diff <mergeBase>` bao gồm cả tệp untracked "chưa có trong diff": đúng, lấy bằng `ls-files --others --exclude-standard -z` (không `git status`).

## 5. Giải pháp

### 5.1 Cây file (mới)

```
quality-coverage-go-profile.ts / .test.ts         task 02  parse profile, ánh xạ module→đường dẫn, loại trừ
quality-diff-changed-lines.ts / .test.ts          task 03  dòng thêm (số + văn bản) so với merge-base; dùng chung 084, 091
quality-diff-coverage.ts / .test.ts               task 04  covered/uncovered/uncoveredRanges, totals
(quality-profile-catalog: profile coverage-go)    task 05
quality-coverage-collector.ts, quality-coverage-handler.ts / .test.ts   task 05-06  quality.coverage
quality-coverage-vitest-report.ts / .test.ts      task 07  (cổng duyệt O12)
__fixtures__/coverage-go/                         task 01
```

### 5.2 Profile `coverage-go` (task 05)

`{ kind:"coverage", cwd:"backend-go/<module>", perModule:true, scopeStrategy:"go-modules", argv:["{bin:go}","test","-covermode=set","-coverprofile={tmp:<module>.out}","./..."], timeoutMs: 900000, heavy:true, requires:[{bin:go}], env:{ set:{GOFLAGS:"-mod=readonly"}, allowExtra:[] } }`. Không `-coverpkg`, không `-tags`, không ghi vào worktree (`-coverprofile` trỏ `{tmp:…}`; vitest `--coverage.reportsDirectory={tmp:…}`).

### 5.3 Diff coverage (task 04)

Dòng đã thêm/sửa `L` của tệp `F`: các khối có `start ≤ L ≤ end`. Không khối → không thực thi được, **loại** khỏi cả tử và mẫu; có khối: **phủ** nếu mọi khối chứa `L` có `count>0`, **chưa phủ** nếu có ít nhất một khối `count==0` (K4, bảo thủ). `diffCoverage = covered/(covered+uncovered)`; mẫu số 0 → `null` + `reason:"no_executable_changed_lines"`. Tệp Go đổi nhưng **không có khối nào** trong profile (module/pkg không test, tệp sinh) → `excludedFiles:[{path,reason:"no_coverage_blocks"}]`, không im lặng.

## 6. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Agent chỉ `measured` | §5.6 |
| 2 | `-covermode=set`, không `-coverpkg`, không `integration` | CR K8, hợp đồng §5.6 |
| 3 | Không ghi vào worktree | `git status` không đổi (K6) |
| 4 | Một thành phần diff dùng chung | README quality-signals §3 (083/084/091) |
| 5 | Tệp không có khối → `excludedFiles`, không mẫu số | Tránh 100% giả |
| 6 | Mẫu số 0 → `null` | K3 |
| 7 | TS sau cổng duyệt | O12 |

## 7. Tiêu chí chấp nhận

- [x] `coverage-go` chỉ tồn tại dạng tên; `quality.run` với lệnh/args tự do bị từ chối.
- [x] Workspace Go mẫu 2 module (1 không test): `report` đủ `totals`, `files`, hàm; module không test nằm ở `diff.noTests`.
- [x] Import path → đường dẫn tương đối đúng; không đường dẫn tuyệt đối; khối ngoài module đếm `unmappedBlocks`; `*.pb.go`, `gen/`, `usecasetest/`, `testutil` ngoài mẫu số.
- [x] Diff: thêm hàm được test, thêm hàm không test, sửa chú thích → đúng; chỉ chú thích → `diffCoverage:null`; tệp untracked tính toàn bộ dòng thêm; tên Unicode không sai ánh xạ.
- [x] `scope=changed` chạy đúng module có tệp đổi, `diff.partial:true`; `git status` không đổi sau run.
- [x] Payload > 2 000 tệp/1 MiB cắt theo `pct` thấp, `truncated`, `totalCount` đúng.
- [x] Run chưa xong → `RUN_IN_PROGRESS`; không có bước coverage → `available:false`; run huỷ → `RUN_CANCELLED`; thiếu provider TS → `ENV_NOT_READY coverage_provider_missing`.

## 8. Kiểm thử

Test theo task. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-coverage-go-profile.test.ts src/relay/quality-diff-changed-lines.test.ts src/relay/quality-diff-coverage.test.ts src/relay/quality-coverage-handler.test.ts`; sau cùng `pnpm test`. Tích hợp thật (có Go): repo git tạm + module Go nhỏ; không chạy trong CI mặc định.

## 9. Rủi ro và chưa kiểm chứng

- **Chưa chạy** `go test -coverprofile` trên Orca: thời gian/RAM của 21 module, test cần DB/Docker (suy từ workflow, chưa chạy).
- Heuristic K4 làm số thấp hơn dịch vụ ngoài: ghi định nghĩa ở UI (backend).
- `-covermode=set` bỏ lỡ số lần chạy.
- Worktree bẩn: số đo gắn `dirty`; khoá `tree_hash` thuộc backend (Q5 CR).
- vitest v8 với alias/`server.deps.inline` có thể sai source map; tệp không test không vào báo cáo nếu thiếu `coverage.include`.
- Windows/macOS: ánh xạ đường dẫn `\`, không hỗ trợ MVP.

## 10. Câu hỏi mở

1. Duyệt `@vitest/coverage-v8` (O12) ở gói nào trước? Đề xuất `agent/`.
2. `go tool cover -func` có đáng chi phí? Mặc định: chỉ cho tệp đã đổi.
3. Danh sách loại trừ mặc định thống nhất với CR-036 bằng cách nào (chưa có hợp đồng)?
