# BE-CV-TASK-038-02: Proto `codeintel_contract_diff.proto` (`ContractDiff`, `GetContractDiff`)

**From Solution:** BE-CV-SOL-038-contract-diff
**Priority:** P2
**Service:** `code-intel-service` · `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_contract_diff.proto` (mới); `backend-go/proto/orca/codeintel/v1/codeintel.proto` (sửa: một dòng `rpc`)
**Depends on:** BE-CV-TASK-037-02 (`Finding`), BE-CV-SOL-020 (`SourceRef`, `SymbolRef`, `ResultMeta`), BE-CV-TASK-036-02 (`IndexFreshness`)
**Status:** [ ] TODO

---

## Context

Hợp đồng §2.1 dòng 16; PQ-30. `ContractWarning` thuộc `codeintel_contract.proto` (032, nội bộ) và **không** dùng ở đây.

## Việc cần làm

1. Message theo solution 2.B; số field gán theo thứ tự khai báo ở CR-038 §2.1 và ghi vào comment đầu tệp (không đổi sau). `ContractChange.kind`, `change`, `compatibility` là `string` chữ thường (PQ-32); `details` là `map<string,string>`.
2. `GetContractDiffRequest{selector=1, base_ref=2, kinds=3 (repeated string, ≤ 32), detail=4 (enum UNSPECIFIED=0, SUMMARY, FULL), if_none_match=5}`; `GetContractDiffResponse{diff=1, meta=2, index_freshness=3}`; `sources` nằm ở `meta.sources`.
3. `rpc GetContractDiff` thêm vào `service CodeIntelService` (không RPC khác).
4. `ViewKind`: thêm `VIEW_KIND_CONTRACT_DIFF` nếu thiếu (qua `BE-CV-SOL-020`).
5. Kiểm quy tắc import (hợp đồng §2.1): `contract_diff` import `findings` và `common` một chiều.

## Kiểm thử

- `buf lint`; `buf breaking --against '.git#branch=main,subdir=backend-go/proto'`.
- Round-trip trong `internal/adapter/grpc/contract_diff_proto_test.go` (mới) với `ContractChange` đủ `consumers`, `evidence`, `details`.

## Tiêu chí hoàn thành

- [ ] `buf` xanh; mọi trường UI §4.5 có chỗ trong proto.
- [ ] Số field ghi trong comment; không RPC thiếu message.

## Rủi ro và lưu ý

- `kinds` ngoài tập `proto|ws-channel|route|migration` ⇒ `CODEINTEL_INVALID_PARAMS` (kiểm ở use case, không phải proto).
