# Bộ đặc tả ontology liên domain cho toàn vòng đời dự án

Phiên bản thiết kế: **0.2-draft — 2026-10-08**. Đây là hợp đồng thiết kế đề xuất, chưa phải runtime/schema triển khai và chưa tự thay thế normative specs đã duyệt trong portfolio. Các từ MUST/SHALL mô tả yêu cầu đối với implementation tương lai. Bản này chi tiết hóa [thiết kế domain đã thống nhất](../Software_Delivery_Domain_Ontologies_Design.md), giữ [quy trình phát triển đã chốt](../TDD_Document_Workflow_Consolidated_Proposal.md).

Mục tiêu: số hóa tài liệu thành knowledge có nguồn; kiểm tra toàn vẹn trong từng domain và nhất quán xuyên domain; quản lý trạng thái ban đầu, thay đổi và phát sinh; truy được từ nhu cầu đến thiết kế, thực thi và nghiệm thu theo đúng phiên bản. CR, SRS, SAD, TDD, CS có ontology riêng. Foundation, mapping, workflow và verification hỗ trợ liên thông, không biến chúng thành các lớp con của TDD.

## Danh mục đặc tả

| Tài liệu | Nội dung |
|---|---|
| [00 — Foundation và logic](00-foundation-and-logic.md) | Identity, revision, assertion, scope, thời gian, datatype, biểu thức, authority, evidence |
| [01 — CR](01-cr-ontology.md) | Nhu cầu, mục tiêu, phạm vi, business case, criteria, dependency, authorization và acceptance |
| [02 — SRS](02-srs-ontology.md) | Requirement, business rule, use case, data/interface, NFR, constraint và delta |
| [03 — SAD](03-sad-ontology.md) | Drivers, views, elements, decisions, topology, quality strategy và conformance |
| [04 — TDD](04-tdd-ontology.md) | Behavior, API/event, data, transaction, security, resilience, vận hành và nghĩa vụ thiết kế |
| [05 — Change Spec và verification](05-change-spec-and-verification.md) | Change items, targets, tasks, tests/executions, acceptance và adapter |
| [06 — Mapping và tính nhất quán](06-cross-domain-mappings.md) | Mapping signatures, composition, semantic obligations, conflict và impact closure |
| [07 — Số hóa và workflow](07-digitization-and-lifecycle.md) | Ingestion, reconciliation, bootstrap, change, emergency, baseline, publish và invalidation |
| [08 — Validation và nghiệm thu](08-validation-and-acceptance.md) | Rule engine contract, gates, coverage, counterexamples, acceptance suite, migration |
| [09 — Ví dụ contract và truy vấn](09-contract-examples-and-implementation-boundaries.md) | Payload minh họa, meta-schema, logic nghiệp vụ, findings và competency queries |
| [Rule catalog](rule-catalog.json) | Index máy đọc được của các rule đã định danh trong tài liệu; không phải executable validators |

## Cách đọc và hiện thực hóa

Đọc 00 → domain cần quan tâm → 06 → 07 → 08. Data dictionary quy định fields/cardinality; các bảng rule bổ sung invariants có ID. Mỗi implementation phải hiện thực cả hai, không chỉ danh sách rule có ID. Các field bắt buộc có thể thiếu trong extraction candidate nhưng MUST sinh missing obligation; không đủ điều kiện baseline/ready cho đến gate thích hợp.

Ontology quản lý **nghĩa phát biểu** độc lập với vị trí tài liệu. Section SRS có thể chứa thiết kế được yêu cầu bắt buộc; phân loại theo authority và semantics, không theo tên file. Một SAD view và một TDD module dùng chung tên không trở thành cùng entity. Quan hệ `refines`, `allocatedTo`, `realizes` giải thích liên hệ.

Không có lời hứa tự động chứng minh mọi câu văn nghiệp vụ. Phần formalizable được kiểm bằng constraints/logic/model checks trong supported fragment; phần còn lại cần semantic review có evidence. Missing/unknown/unsupported không biến thành pass. Kết luận chỉ có giá trị trong baseline, applicability scope, rule version và evidence đã pin.

## Các quyết định thiết kế quan trọng

- Phân biệt raw artifact, semantic assertion, accepted revision, mapping và execution evidence.
- Identity ổn định; revision bất biến; lịch sử và phủ định/removal không bị xóa.
- Scope là dữ liệu có cấu trúc: project/product/release/environment/variant/time/actor/workload.
- Lưu nguồn mâu thuẫn song song; không dùng majority vote, confidence hoặc timestamp để tự quyết định chân lý.
- Readiness theo từng gate; accepted requirement không đồng nghĩa implemented, test passed không đồng nghĩa BA accepted.
- Change-spec generation chỉ sau human impact approval và actual TDD realization đủ; nhãn draft không bypass gate này.
- SAD readonly trong luồng product change; khởi tạo hoặc đổi SAD dùng architecture workstream có thẩm quyền riêng.
- OpenSpec là adapter. Canonical semantics và rule catalog không phụ thuộc OpenSpec folder/status.

## Quy mô và kiểm tra bản thiết kế

Bộ đặc tả có **145 rule định danh**, **20 loại mapping**, **50 scenario nghiệm thu dự kiến** và **15 competency queries**, ngoài các field/cardinality constraints trong data dictionaries. [Kết quả kiểm tra artifact](design-artifact-checks.json) chỉ xác nhận cấu trúc/liên kết/catalog và JSON minh họa; không phải kết quả chạy validators hay nghiệm thu nghiệp vụ.

## Ranh giới bàn giao hiện tại

Bộ tài liệu gồm model/logic contracts, rule index và scenario nghiệm thu thiết kế. Chưa tạo OWL/SHACL/JSON Schema thực thi, chưa migrate dữ liệu, chưa đổi runtime v0.6 và chưa khẳng định kiểm chứng đầy đủ các tài liệu PBOM. Việc đó cần implementation theo contract và benchmark/human acceptance độc lập.
