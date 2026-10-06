# BE-CV-TASK-038-04: Quy tắc so sánh proto (`proto.*`) bám `buf breaking` loại `FILE`

**From Solution:** BE-CV-SOL-038-contract-diff
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/contractdiff/{contract_diff.go, compatibility.go, proto_diff_rules.go}` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-038-01, BE-CV-TASK-038-02
**Status:** [ ] TODO

---

## Context

Solution 2.D (proto). Hàm thuần `DiffProto(base, head ProtoSchema) []ContractChange` trên kiểu schema nhỏ (service/rpc/message/field/enum/oneof/reserved/package/go_package); adapter của 032 ánh xạ kiểu của nó sang kiểu này. `buf breaking FILE` còn quy tắc chưa liệt kê (rủi ro đã nêu); CI là nguồn chân lý.

## Việc cần làm

1. `contract_diff.go`, `compatibility.go`: kiểu `ContractChange`, `ConsumerRef`, `Summary`; `Compatibility` có thứ tự `breaking > risky > compatible > unknown` và hàm `Worst`; `Summary` đếm.
2. `proto_diff_rules.go`: bảng quy tắc table-driven (`ruleId`, điều kiện, mức): `proto.service-removed`, `rpc-removed`, `rpc-type-changed`, `rpc-streaming-changed`, `message-removed` (chỉ khi còn được tham chiếu ở base), `field-removed` (`risky` nếu số **và** tên có trong `reserved` của head), `field-number-changed`, `field-renamed`, `field-type-changed` (hoán đổi cùng wire chỉ `sint32`↔`sint32`… cùng cỡ; `string`↔`bytes` ghi `details.note`; `int32/uint32/int64/uint64/bool` **không** hoán đổi), `field-label-changed` (singular↔repeated, `optional` bật/tắt), `enum-value-removed`, `enum-value-number-changed`, `package-changed`, `go-package-changed`, `oneof-changed`; thêm: `rpc-added`, `message-added`, `field-added`, `enum-value-added` ⇒ `compatible`; `comment-or-option-only` ⇒ chỉ khi `detail=FULL`.
3. Mỗi thay đổi có `id` ổn định (`sha256(ruleId|name)[:12]`), `name` đầy đủ (`orca.infrafleet.v1.InfraFleetService.GetX`), `kind` (`proto-service|proto-rpc|proto-message|proto-field|proto-enum`), `change`, `files`, `details` (`before`, `after`, `fieldNumber`…).
4. `consumers`: để trống ở đây; use case điền từ `RpcEdge` (TASK-038-07).
5. Tệp `added`/`removed` nguyên tệp ⇒ sinh thay đổi `added` (compatible)/`removed` (breaking cho service/rpc/message) theo nội dung.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/contractdiff/ -run Proto`: mỗi `ruleId` một ca breaking/compatible từ fixture TASK-038-01; `reserved` hạ `risky`; chỉ đổi comment ⇒ rỗng (`SUMMARY`) và một mục (`FULL`); đổi tên trường vừa đổi số; thêm enum; `oneof`; hoán vị đầu vào ⇒ cùng đầu ra; không panic với schema rỗng.

## Tiêu chí hoàn thành

- [ ] Mọi dòng bảng quy tắc có test.
- [ ] `Summary` đếm đúng; `id` ổn định.

## Rủi ro và lưu ý

- Ghi chú trong comment đầu tệp: "bám `FILE` của `buf`; khác biệt với CI là lỗi của file này".
