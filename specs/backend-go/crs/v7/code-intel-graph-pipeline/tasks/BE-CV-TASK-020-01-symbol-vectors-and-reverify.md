# BE-CV-TASK-020-01: Re-verify nền proto/service và khoá vector kiểm thử `SymbolRef` dùng chung với agent

**From Solution:** BE-CV-SOL-020-canonical-graph-model
**Priority:** P0
**Service:** `code-intel-service` (test data) · `proto`
**File:** `backend-go/services/code-intel-service/testdata/symbol-vectors/symbol-key-vectors.json` (mới), `.../testdata/symbol-vectors/README.md` (mới, chỉ mô tả schema vector)
**Depends on:** BE-CV-SOL-010 (module `code-intel-service` tồn tại; nếu chưa, task này chỉ chuẩn bị file JSON và ghi vào PR)
**Status:** [ ] TODO

---

## Context

Hợp đồng PQ-20 buộc agent và backend dùng **cùng** quy tắc khoá và **cùng** vector kiểm thử (CR-CV-070). Chưa có nơi chứa vector (SOL-020 mục 7, Q2). Task này (a) re-verify hiện trạng trước khi viết proto, (b) tạo vector từ các ví dụ **đã đọc** trong CR-020 mục 1, để task 04–06 viết test trước khi viết mã.

## Việc cần làm

1. Re-verify (chỉ đọc, ghi kết quả vào mô tả PR): `ls backend-go/proto/orca` (không được có `codeintel/` trừ khi SOL-010 đã tạo), `ls backend-go/services` (có/không `code-intel-service`), đọc `backend-go/proto/buf.yaml` (`lint STANDARD`, `breaking FILE`) và `buf.gen.yaml`; xác nhận `buf` và `protoc-gen-go`, `protoc-gen-go-grpc` có trong môi trường (`which buf protoc-gen-go protoc-gen-go-grpc`). Nếu SOL-010 đã chọn tên module khác, sửa đường dẫn trong các task 02–08.
2. Tạo `symbol-key-vectors.json`: mảng đối tượng `{name, tool, input, expectKey, expectStartLine, notes}`. Tối thiểu 14 ca, lấy từ CR-020 mục 1 và agent contract §2.6: (1) GitNexus `Method:backend-go/services/infra-fleet-service/internal/usecase/relay.go:Relay.Execute#2` → `method:backend-go/services/infra-fleet-service/internal/usecase/relay.go:Relay.Execute`; (2) CodeGraph `qualifiedName "Relay::Execute"` → cùng khoá; (3) `NewRelayByDevServer` GitNexus `startLine 33` → `34`, CodeGraph `34` → `34`; (4) hai `Method` cùng tệp/tên khác arity → `#<arity>`; (5) Property TS; (6) `Section` → `doc:<path>:L29:Features`; (7) Route GitNexus `Route:/voice-settings`; (8) Route CodeGraph `<file>::POST:/local` (không hợp nhất); (9) File `file:Casks/orca.rb:orca.rb`; (10) Folder `folder:tests:tests`; (11) field Kotlin `expo.modules.twowayaudio::AudioEngine::SAMPLE_RATE`; (12) cluster `cluster::comm_6117`; (13) flow `flow::proc_0_checkspanel`; (14) cặp khớp phụ `prefetchManagedWorktreeCreateBase` (GitNexus `…RuntimeWorktreeCreationCommands.prefetchManagedWorktreeCreateBase#1` dòng 1041; CodeGraph `…::prefetchManagedWorktreeCreateBase` dòng 1042) → một nút, hai id.
3. Thêm ca âm: kind lạ (`native_kind` không có trong bảng) → `VALUE`; đường dẫn `\\wsl$\Ubuntu\home\u\a.ts` → `CODEINTEL_PATH_NOT_ALLOWED`; `/home/u/repo/../x` → lỗi; Unicode NFD → NFC.
4. Ghi trong README của thư mục: file được **sao chép nguyên văn** sang `agent/` bởi `AG-CV-SOL-070` (hoặc ngược lại); thay đổi một bên phải đổi bên kia trong cùng đợt.

## Kiểm thử

- `jq empty backend-go/services/code-intel-service/testdata/symbol-vectors/symbol-key-vectors.json` (JSON hợp lệ).
- `jq 'length' …` ≥ 14 ca dương + ≥ 4 ca âm.
- Test dùng vector ở TASK-020-04..06 (chưa chạy ở task này).

## Tiêu chí hoàn thành

- [ ] Kết quả re-verify (4 lệnh ở việc 1) ghi trong PR.
- [ ] File vector có đủ ca ở việc 2–3, mỗi ca có `notes` nêu nguồn CR/mục.
- [ ] Không chứa đường dẫn tuyệt đối thật của người dùng hay secret.

## Rủi ro và lưu ý

- Các số dòng 33/34, 1041/1042 lấy từ CR (đo ngày 2026-10-05), **chưa kiểm lại**; nếu kiểm lại bằng công cụ thật thì ghi số đo vào `notes`.
- Chỉ một cặp khớp tay; CR-CV-070 sẽ mở rộng vector bằng fixture vàng.
