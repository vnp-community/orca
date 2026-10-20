# Đặc tả Ontology số hoá tài liệu thiết kế, phiên bản 0.4.0

Trạng thái: bản đề xuất để duyệt. Đã chạy thử trên đồ thị 12.871 phần tử (gồm 6.341 khối nguồn) và 15.482 cạnh trích từ 13 TDD của PBOM. Ontology này không phụ thuộc PBOM; PBOM chỉ là dữ liệu thử.

## 1. Mục tiêu và phạm vi

Ontology phải số hoá được năm loại tri thức của một tài liệu thiết kế:

1. Mô hình dữ liệu: thực thể, thuộc tính, chỉ mục, nơi lưu, phân loại dữ liệu, thời hạn lưu.
2. Mối quan hệ: cấu trúc, truy vết, phụ thuộc, hành vi, vận hành.
3. Rule: ràng buộc, chính sách, bất biến, điều kiện trước và sau, bảng quyết định.
4. Hằng số và cấu hình: giá trị dùng chung, đơn vị, giới hạn, giá trị theo giai đoạn.
5. Logic nghiệp vụ: thuật toán, công thức, mô hình trạng thái, luồng, thao tác.

Và phải thể hiện toàn bộ phổ từ logic bắt buộc đến logic còn mở. Phần còn mở không được để lọt thành ô trống im lặng: mọi chỗ thiếu phải thuộc một trong các dạng ở mục 5, có chủ và có hạn khi cần.

Ngoài phạm vi của lõi v0.4: kiểu dữ liệu ngôn ngữ lập trình cụ thể, định dạng trình bày của tài liệu Word. Context triển khai phụ thuộc repo có thể được cung cấp bởi hồ sơ downstream và snapshot repo.

### 1.2 Yêu cầu bổ sung: đầu vào cho tech spec

**REQ-KG-TS-01:** KG TDD phải chứa đủ tri thức có cấu trúc, nguồn gốc và phiên bản cho từng capability/change để sinh tech spec có thể triển khai, gồm mục đích/phạm vi, baseline và requirement delta, kịch bản nghiệm thu, hợp đồng/hành vi/dữ liệu, quyết định thiết kế, tác động/phụ thuộc, task và kiểm chứng. Ví dụ đầu ra là OpenSpec change qua skill openspec-propose với proposal.md, design.md, tasks.md và spec deltas.

Hợp đồng, ma trận tri thức, delta ontology đề xuất và cổng TS-01..TS-10 nằm tại [TDD KG TechSpec Readiness v0.5](TDD_KG_TechSpec_Readiness_v0.5.md). Đây là yêu cầu bổ sung cho phiên bản kế tiếp, chưa được cài trong artefact v0.4. Tính hợp lệ về schema, maturity specified có Gap, dựng lại nguyên văn và cổng thay đổi không phát sinh lỗi mới đều chưa đủ để khẳng định implementation_ready. Readiness phải kiểm theo scope, profile và snapshot; draft còn thiếu thông tin phải ghi rõ blocker.

### 1.1 Mức đáp ứng ba yêu cầu của v0.4 (đã kiểm trên dữ liệu)

| Yêu cầu | Mức đáp ứng | Bằng chứng | Còn thiếu |
|---|---|---|---|
| 1. Số hoá và viết lại thành văn bản | Một phần | Dựng lại không mất mát đạt (11 trên 11 tệp trùng băm); dựng lại chỉ từ cấu trúc phủ 47,8% từ | Mã nguồn nhúng (7%), đoạn văn (37%), tiêu đề (2%) chưa số hoá; chưa có tầng sinh văn bản |
| 2. Thêm, sửa, xoá mà giữ toàn vẹn | Có cơ chế, bảo vệ chưa đủ | Động cơ nguyên khối, 15 trên 15 ca kiểm thử đạt; chặn xoá còn tham chiếu, ghi đè, trùng mã, cạnh sai miền, giá trị ngoài từ vựng | Cổng chỉ bảo vệ theo rule đã chạy được; 4.657 lượt đánh giá thiếu dữ liệu; nền có 6.422 vi phạm |
| 3. Đầu vào cho agent sinh TDD từ tài liệu thiết kế và yêu cầu thay đổi | Có hợp đồng, chưa đủ điều kiện dựng | Lớp ChangeRequest, ChangeSet; hồ sơ URD, SRS, SAD, TDD; phát hiện giá trị lệch, trùng danh tính, khuyết | Thượng nguồn chưa được số hoá nội dung; X-03, X-05 chưa chạy; chưa có bộ vàng |

## 2. Siêu mô hình

### 2.1 Phần tử, quan hệ, bằng chứng

- **Phần tử (Element)**: mọi thứ có mã trong kho. Có 88 lớp trong 13 gói, chung một lớp gốc. Thuộc tính của lớp gốc: id, name, owner_doc, origin, lifecycle, maturity, priority, scope_status, deferred_to, source_ref, confidence, extracted_by, review_status, gaps, ext.
- **Quan hệ**: 88 quan hệ có kiểu, mỗi quan hệ khai báo miền, miền giá trị, nghịch đảo, bội số và đặc tính (acyclic, symmetric). Kiểm tra miền và miền giá trị có xét lớp con.
- **Bằng chứng (source_ref)**: mỗi phần tử và mỗi cạnh trỏ về khối nguồn, vị trí và băm trích dẫn. Phần tử không có bằng chứng chỉ được phép ở mức `sketch`.
- **Mở rộng có kiểm soát (ext)**: thuộc tính ngoài lược đồ được giữ lại trong `ext`, không bị bỏ; định kỳ xét đưa vào lớp (mục 9).

### 2.2 Ba đối tượng giá trị dùng chung

**Quantity** (đại lượng có đơn vị), thay cho số trần và chuỗi:

| Trường | Ý nghĩa |
|---|---|
| value, min, max | Giá trị hoặc khoảng; ít nhất một trong ba, hoặc có slot_status khác `set` |
| unit | Mã trong sổ đơn vị (Phụ lục D); mọi đơn vị quy đổi được về đơn vị gốc của đại lượng |
| operator | `<`, `<=`, `=`, `>=`, `>`, `range` |
| percentile | p50, p90, p95, p99, p99.9 |
| basis | Cơ sở của giá trị (mục 5.1) |
| slot_status | Trạng thái ô khi chưa có giá trị (mục 5.1) |

**Gap** (khuyết có chủ đích hoặc phát hiện được), xem mục 5.2.

**SourceRef**: block, doc, quote_hash, quote, conf.

### 2.3 Mức chín muồi (maturity) của phần tử

| Mức | Nghĩa | Yêu cầu tối thiểu |
|---|---|---|
| sketch | Mới nhắc tới, chưa mô tả | id, name, nguồn |
| specified | Đã mô tả đủ để thiết kế | Mọi thuộc tính M có giá trị hoặc có Gap hợp lệ |
| verified | Đã có kiểm chứng | Có verified_by tới TestCase |
| baselined | Đã chốt đường cơ sở | Không còn Gap mở, OpenIssue liên quan đã đóng |

Quy ước ba mức bắt buộc của thuộc tính: **M** bắt buộc từ mức `specified`; **C** bắt buộc khi điều kiện ghi trong ghi chú đúng; **O** tuỳ chọn.

## 3. Mô hình dữ liệu

Các lớp chính: Entity, Attribute, Index, DataStore, Schema, View, Sequence, Enumeration, DataClass, RetentionPolicy, DbRole, Message.

| Cần thể hiện | Cách thể hiện |
|---|---|
| Thực thể và thuộc tính | Entity `has_attribute` Attribute; Attribute có datatype, nullable, default, unique, enumerates |
| Khoá ngoại | Attribute `references_entity` Entity |
| Chỉ mục | Index `indexes` Entity, thuộc tính unique, partial, columns |
| Nơi lưu | `stored_in` DataStore hoặc Schema; `retained_by` RetentionPolicy |
| Phân loại và bảo mật dữ liệu | `classified_as` DataClass; Credential cho bí mật |
| Miền giá trị rời rạc | Enumeration có danh sách giá trị; thuộc tính `enumerates` Enumeration |
| Thực thể ở hai tầng (khái niệm và vật lý) | `realizes` từ thực thể thiết kế đến thực thể thượng nguồn; `realized_in` Artefact |
| Đọc ghi | Thành phần, thao tác, tác vụ `reads` và `writes` Entity, DataStore, View |

Lưu ý: mọi khuyết ở đây (ví dụ chưa có kiểu dữ liệu) khai báo bằng Gap, không để trống.

## 4. Phân loại ràng buộc và rule

### 4.1 Modality: mức bắt buộc của một rule

| Modality | Nghĩa | Hệ quả khi vi phạm |
|---|---|---|
| must | Bắt buộc | Lỗi, chặn đường cơ sở |
| must_not | Cấm | Lỗi |
| should | Nên | Cảnh báo, cần lý do nếu bỏ |
| may | Được phép | Thông tin |

### 4.2 Các loại rule trong mô hình

| Loại | Lớp | Ví dụ |
|---|---|---|
| Yêu cầu | Requirement, QualityTarget | Chức năng, phi chức năng |
| Quy tắc nghiệp vụ | BusinessRule, DecisionRule | Ma trận giấy phép, hạn SLA |
| Ràng buộc kỹ thuật | Constraint | Giới hạn kích thước, thời gian |
| Chính sách | Policy, RoutePolicy, RateLimitProfile, RetentionPolicy | Giới hạn tốc độ, lưu giữ |
| Điều kiện | Condition với condition_kind: precondition, postcondition, guard, invariant | Điều kiện chuyển trạng thái |
| Biện pháp kiểm soát | Control | Xác thực, nhật ký kiểm toán |

### 4.3 Rule của chính ontology (kiểm tra chất lượng tài liệu)

Có 74 rule, chia 10 nhóm (cấu trúc S, từ vựng V, chín muồi M, khuyết G, truy vết T, logic L, hằng số K, thay đổi C, trình bày P, liên tài liệu X). Danh mục đầy đủ ở Phụ lục E và tệp `rules.yaml`. Mỗi rule là dữ liệu gồm: id, nhóm, tiêu đề, modality, severity, phạm vi, điều kiện áp dụng, khẳng định, có miễn trừ được không, tham số, hướng khắc phục.

**Kết quả ba trị**: pass, fail, waived, not_applicable, insufficient_data. `insufficient_data` khác `fail`: nó nói rằng dữ liệu hiện có chưa đủ để kết luận (ví dụ chưa có bảng quyết định có cấu trúc thì chưa kiểm tra được độ phủ). Không đổi thành pass.

**Miễn trừ (Waiver)**: một phần tử cho một rule, có người duyệt, lý do, ngày hết hạn. Hết hạn thì kết quả trở lại fail. Rule đánh dấu `waivable: false` không miễn trừ được.

## 5. Phổ logic: từ bắt buộc đến còn mở

Đây là phần cốt lõi của yêu cầu "để trống, thiếu, bổ sung sau". Mỗi ô thông tin (slot) của một phần tử nằm ở đúng một trạng thái.

### 5.1 Trạng thái ô (SlotStatus) và cơ sở giá trị (EvidenceBasis)

| SlotStatus | Nghĩa | Bắt buộc kèm |
|---|---|---|
| set | Đã có giá trị | basis |
| tbd | Cố ý chưa quyết, sẽ bổ sung | owner, due (rule G-02) |
| unknown | Chưa biết, chưa ai nhận | owner, due (rule G-02) |
| assumed | Đang dùng giả định | Assumption hoặc OpenIssue (G-03) |
| proposed | Đề xuất của tài liệu, chưa được duyệt | Decision hoặc OpenIssue (G-03) |
| to_measure | Giá trị chờ đo | Metric hoặc TestCase (G-06) |
| deferred | Dời sang mốc sau | deferred_to (G-05) |
| not_applicable | Không áp dụng | Lý do |

| EvidenceBasis | Nghĩa |
|---|---|
| specified | Nêu rõ trong nguồn thượng nguồn |
| derived | Suy ra có công thức từ giá trị khác |
| proposed | Đề xuất thiết kế |
| assumed | Giả định chưa kiểm chứng |
| to_measure | Mục tiêu thiết kế chờ đo |
| measured | Đã đo |
| unknown | Chưa rõ cơ sở |

### 5.2 Gap

Khuyết được ghi thành đối tượng, không để ô trống.

| Trường | Ý nghĩa |
|---|---|
| slot | Tên thuộc tính khuyết |
| status | tbd, unknown, assumed, proposed, to_measure, deferred, not_applicable |
| reason | Vì sao khuyết (tối thiểu 5 ký tự) |
| intent | `by_design` (khuyết có chủ đích) hoặc `detected` (công cụ hoặc người rà soát phát hiện) |
| owner, due | Bắt buộc khi status là tbd hoặc unknown |
| link | Tham chiếu tới OpenIssue, Assumption hoặc Decision |

Ví dụ (ngân sách độ trễ chưa có số, chờ POC):

```json
{"id": "Budget:ext_authz", "class": "Budget", "maturity": "specified",
 "props": {"scope": "ext_authz",
           "target": {"unit": "ms", "percentile": "p95", "basis": "to_measure", "slot_status": "to_measure"}},
 "gaps": [{"slot": "target", "status": "to_measure", "intent": "by_design",
           "reason": "Chờ số đo POC-01", "link": "OpenIssue:OI-12"}]}
```

### 5.3 OpenIssue

OpenIssue là phần còn mở có tên riêng. Trường: issue_kind (question, tbd_value, assumption_validation, decision_pending, deferred, deviation, risk_acceptance, schema_gap, upstream_change, external_dependency), question, owner, due, status (open, in_progress, resolved, wont_fix), proposed_default (giá trị tạm dùng trong lúc chờ), resolution (bắt buộc khi resolved).

Quan hệ: phần tử `blocked_by` OpenIssue; Decision hoặc Deviation `resolves` OpenIssue; Assumption `validated_by` TestCase, Milestone hoặc Metric.

### 5.4 Phổ đầy đủ

| Tầng | Dạng | Cách biểu diễn | Rule kiểm tra |
|---|---|---|---|
| 1 | Bắt buộc, đã đủ | Thuộc tính M có giá trị | M-01 |
| 2 | Bắt buộc có điều kiện | Thuộc tính C | M-02 |
| 3 | Tuỳ chọn | Thuộc tính O | không |
| 4 | Còn mở có chủ đích | Gap tbd + OpenIssue | G-02, G-04, G-07 |
| 5 | Giả định | Gap assumed + Assumption | G-03 |
| 6 | Đề xuất chưa duyệt | Gap proposed + Decision hoặc OpenIssue | G-03 |
| 7 | Chờ đo | Gap to_measure + Metric hoặc TestCase | G-06 |
| 8 | Dời giai đoạn | scope_status=deferred, deferred_to | G-05 |
| 9 | Không áp dụng | Gap not_applicable + lý do | G-01 |
| 10 | Chưa biết, chưa ai nhận | Gap unknown, phải được giao owner | G-02 |
| 11 | Khuyết do công cụ phát hiện | Gap intent=detected | G-09, M-04 |

Nguyên tắc: một phần tử không được lên `baselined` khi còn Gap mở (M-04). Khuyết `detected` phải được xử lý, chuyển thành `by_design` (có chủ, có hạn) hoặc được điền, trước khi chốt đường cơ sở.

## 6. Biểu diễn logic nghiệp vụ

| Loại logic | Lớp | Cách biểu diễn có cấu trúc | Chỗ còn mở thể hiện bằng |
|---|---|---|---|
| Quy tắc theo điều kiện | BusinessRule, DecisionRule | rule_form, severity, modality | Gap trên rule_form hoặc severity |
| Bảng quyết định | DecisionTable + DecisionRule | inputs là Enumeration, outputs, hit_policy (unique, first, any, collect); mỗi dòng có when, then | Dòng thiếu tổ hợp: L-01 báo thiếu; luật mặc định: L-03 |
| Thuật toán | Algorithm | inputs, outputs, deterministic, steps; Condition pre và post qua `has_condition` | Gap trên pre, post, complexity |
| Công thức | Formula | expression, variables (tên, đơn vị, miền), result_unit; Constant qua `uses_constant` | Gap trên variables |
| Mô hình trạng thái | StateMachine, State, Transition | initial_state, terminal_states; Transition có trigger, guard, action, from, to | Chuyển trạng thái thiếu guard hoặc action: Gap |
| Điều kiện | Condition | condition_kind (precondition, postcondition, guard, invariant), expression, violation_error | Gap trên expression |
| Luồng | Flow | các bước, thao tác, tác vụ liên quan | Gap |
| Thao tác ghi | Operation | idempotent, idempotency_key, concurrency_control | L-11 |
| Tác vụ nền | Job | trigger_kind, schedule, job_key, idempotent | M-01 |
| Giá trị phụ thuộc điều kiện | DecisionTable | ví dụ retry là `read` cho GET, `none` cho PATCH: mô hình thành bảng quyết định, không để chuỗi tự do | V-01 cho chuỗi tự do |

Mô hình trạng thái có bốn rule toàn vẹn: một trạng thái đầu (L-04), chuyển trạng thái có nguồn và đích cùng mô hình (L-05), mọi trạng thái đạt tới được (L-06), trạng thái cuối không có lối ra (L-07).

Bảng quyết định có hai rule độ phủ: đầy đủ (L-01) và loại trừ (L-02), cộng luật mặc định cho `first` (L-03). Hiện chưa chạy được vì dữ liệu chưa có DecisionTable (mục 8).

## 7. Hằng số, cấu hình, từ vựng, đơn vị

### 7.1 Constant và ConfigItem

| | Constant | ConfigItem |
|---|---|---|
| Bản chất | Giá trị thiết kế dùng chung, đúng một nơi định nghĩa | Khoá cấu hình khi chạy |
| Trường chính | key, constant_kind, value (Quantity), basis, mutable, by_phase | key, type, default, env_var, secret, unit, runtime_changeable |
| Quan hệ | Phần tử dùng `uses_constant` | ConfigItem `configures` thành phần |
| Rule | K-01, K-03, K-04 | K-01, K-02, K-05 |

`by_phase` ghi giá trị theo giai đoạn (ví dụ POC và sản xuất). Hằng số bí mật không có giá trị mặc định trong tài liệu (K-05, không miễn trừ).

### 7.2 Từ vựng có kiểm soát

49 từ vựng (Phụ lục C), mỗi từ vựng có danh sách giá trị chuẩn và bí danh. Bí danh gồm tiếng Việt và biến thể trong văn bản thật (ví dụ "Tích hợp" thành `integration`). Bí danh kết thúc bằng dấu `*` là khớp theo tiền tố. Giá trị ngoài từ vựng bị rule V-01 báo; muốn thêm giá trị phải qua quy trình ở mục 9.

### 7.3 Sổ đơn vị

17 đơn vị (Phụ lục D), mỗi đơn vị có đại lượng, hệ số quy về đơn vị gốc và bí danh. Hệ quả: so sánh min..max (K-03) và nhất quán đơn vị (V-05) làm được sau quy đổi. Đơn vị ghép như "yêu cầu/phút/người dùng" chưa có trong sổ; V-02 báo để bổ sung (mục 8).

## 8. Mô hình thay đổi và động cơ bảo toàn tính toàn vẹn

### 8.1 Lớp và vòng đời

| Lớp | Vai trò |
|---|---|
| ChangeRequest | Ý định thay đổi: nguồn (change_source), người đề nghị, lý do, tài liệu đích, trạng thái (submitted, analysed, approved, applied, rejected, withdrawn) |
| ChangeSet | Tập thao tác áp dụng nguyên khối; gắn một ChangeRequest; có base_version là bản kho mà nó dựa vào |
| ChangeOp | Một thao tác nguyên tử: op, target, path, before, after, cascade, seq |

Các loại thao tác (ChangeOpKind): add, modify, remove, rename, reclassify, supersede, merge, link, unlink. Động cơ hiện hỗ trợ add, modify, rename, reclassify, remove, link, unlink; supersede và merge mới được khai báo, chưa cài. Chính sách xử lý khi xoá (CascadePolicy): `restrict` (chặn nếu còn bị tham chiếu), `cascade` (xoá kèm phần tử con sở hữu), `detach` (cắt liên kết), `redirect` (chuyển tham chiếu sang phần tử khác). Mức tác động (ImpactKind): broken, stale, needs_review, check.

### 8.2 Thuật toán áp dụng bộ thay đổi

1. Kiểm tra yêu cầu đã duyệt, có người duyệt (C-06).
2. Kiểm tra `base_version` bằng số hiệu bản kho hiện hành; lệch nghĩa là có thay đổi xen vào, phải lập lại bộ thay đổi (C-04).
3. Áp từng thao tác theo `seq` trên một bản sao:
   - Đích phải tồn tại (C-02); thêm mới phải có mã chưa dùng và lớp hợp lệ (C-03).
   - Sửa và xoá phải mang `before` khớp trạng thái hiện hành, chống ghi đè thay đổi người khác (C-04).
   - Xoá phải có chính sách; `restrict` bị chặn nếu còn cạnh vào từ phần tử không bị xoá cùng bộ (C-01).
   - Phần tử `baselined` chỉ đổi khi thao tác hạ mức chín muồi hoặc có `approved_baseline_change` (C-08).
4. Tính tập tác động: từ mỗi phần tử bị đổi, đi qua các quan hệ ở bảng 8.4, theo hướng và độ sâu khai báo.
5. Đánh dấu `needs_review` cho phần tử bị tác động; phần tử `verified` hoặc `baselined` bị tác động hạ về `specified` (C-07, C-08).
6. Chạy lại toàn bộ rule trên bản sao và so với bản gốc. Có vi phạm mức `error` mới thì huỷ cả bộ (C-05). Vi phạm mức thấp hơn mới phát sinh được liệt kê thành khuyết cần xử lý, không chặn.
7. Áp dụng nguyên khối hoặc không áp dụng gì (C-09). Bản gốc không bị sửa trong lúc xét. Thành công thì tăng số hiệu bản kho và ghi nhật ký.

### 8.3 Ví dụ

Yêu cầu thay đổi CR-0007: nâng giới hạn kích thước SBOM lên gấp đôi.

```json
{"id": "CS-0007", "request": "CR-0007", "base_version": 0, "ops": [
  {"seq": 1, "op": "modify", "target": "Constant:limits.sbom_max_bytes", "path": "props.value",
    "before": {"value": 52428800, "unit": "B"}, "after": {"value": 104857600, "unit": "B"}}]}
```

Kết quả mong đợi: chấp nhận; liệt kê các phần tử dùng hằng số (`uses_constant`, mức `stale`) và phần tử nối ngân sách, ràng buộc; đánh dấu cần duyệt lại. Nếu bộ thay đổi sửa `max_body` mà quên cấu hình Gateway, khuyết hiện ở rule X-01 (cùng khoá khác giá trị giữa tài liệu).

### 8.4 Bảng lan truyền tác động (nằm trong `profiles.yaml`)

| Quan hệ | Hướng lan | Mức tác động | Độ sâu |
|---|---|---|---|
| satisfies | in | needs_review | 2 |
| derives_from | in | needs_review | 2 |
| constrained_by | in | needs_review | 1 |
| budgeted_by | in | needs_review | 1 |
| uses_constant | in | stale | 1 |
| uses_algorithm | in | needs_review | 1 |
| uses_formula | in | needs_review | 1 |
| decides_by | in | needs_review | 1 |
| has_rule | both | needs_review | 1 |
| verified_by | out | needs_review | 1 |
| implemented_by | out | needs_review | 1 |
| configures | both | check | 1 |
| has_attribute | both | needs_review | 1 |
| references_entity | in | needs_review | 1 |
| has_state | both | needs_review | 1 |
| in_machine | both | needs_review | 1 |
| transition_from | both | needs_review | 1 |
| transition_to | both | needs_review | 1 |
| enforces | in | check | 1 |
| enumerates | both | needs_review | 1 |
| part_of | both | check | 1 |
| provides | both | check | 1 |
| exposes | both | check | 1 |
| accepts | both | needs_review | 1 |
| returns | both | needs_review | 1 |
| may_fail_with | both | check | 1 |
| reads | both | check | 1 |
| writes | both | needs_review | 1 |
| depends_on | in | check | 2 |

### 8.5 Kiểm thử động cơ trên dữ liệu thật (đồ thị 12.871 phần tử)

| Ca | Mong đợi | Thực tế | Rule chặn | Kết quả |
|---|---|---|---|---|
| T01 xoá thực thể còn bị tham chiếu, restrict | rejected | rejected | C-01 | đạt |
| T02 xoá thực thể kèm chính sách cascade | accepted | accepted |  | đạt |
| T03 sửa hằng số đúng before | accepted | accepted |  | đạt |
| T04 sửa với before sai (ghi đè thay đổi xen vào) | rejected | rejected | C-04 | đạt |
| T05 thêm phần tử trùng mã | rejected | rejected | C-03 | đạt |
| T06 nối cạnh sai miền (Requirement has_state State) | rejected | rejected | C-05 | đạt |
| T07 sửa enum ngoài từ vựng | rejected | rejected | C-05 | đạt |
| T08 nguyên khối: thao tác 1 đúng, thao tác 2 lỗi | rejected | rejected | C-02 | đạt |
| T09 thêm Requirement chưa có phần tử thoả mãn | accepted | accepted |  | đạt |
| T10 yêu cầu chưa duyệt | rejected | rejected | C-06 | đạt |
| T11 base_version lỗi thời | rejected | rejected | C-04 | đạt |
| T12 xoá redirect sang phần tử khác cùng lớp | accepted | accepted |  | đạt |
| T13 sửa phần tử baselined không hạ maturity | rejected | rejected | C-08 | đạt |
| T14 phần tử bị tác động được đánh dấu cần duyệt lại (C-07) | all marked | 2/2 |  | đạt |
| T15 đồ thị gốc không bị sửa khi huỷ hoặc áp dụng | unchanged | unchanged |  | đạt |

Ý nghĩa: các lỗi thao tác điển hình (xoá còn tham chiếu, ghi đè, trùng mã, cạnh sai miền, giá trị ngoài từ vựng, bộ thay đổi dở dang, yêu cầu chưa duyệt, sửa phần tử đã chốt) đều bị chặn đúng rule; đồ thị gốc không bị sửa khi huỷ.

### 8.6 Giới hạn của cơ chế

- Tác động tính theo cấu trúc đồ thị, không hiểu ngữ nghĩa: biết phần tử nào liên quan, không biết đoạn văn nào cần viết lại.
- Cổng C-05 chỉ chặn được những gì rule hiện có phát hiện. Rule trả `insufficient_data` (bảng quyết định, trạng thái đầu, điều kiện trước sau) không bảo vệ gì. Nền đang có 6.422 vi phạm, nên cổng bảo đảm "không tệ đi", không bảo đảm "đúng".
- Đồng thời chỉ kiểm soát bằng `base_version`: hai bộ thay đổi song song cùng đường cơ sở thì bộ áp sau bị từ chối và phải lập lại; chưa có gộp tự động.
- Động cơ chạy trên tệp JSON; chưa phải kho giao dịch.

## 9. Số hoá văn bản và dựng lại

### 9.1 Ba tầng

| Tầng | Nội dung | Bảo đảm |
|---|---|---|
| L0, khối nguồn | `Block`: mọi dòng của tệp nguồn thuộc đúng một khối nguyên văn có thứ tự; nối các khối trả đúng tệp | Không mất mát (rule P-05, kiểm bằng băm; 11 trên 11 tệp trùng) |
| L1, cấu trúc | Phần tử, thuộc tính, cạnh, gắn với khối bằng `binds` | Truy vấn, kiểm tra được; độ phủ ở mục 9.3 |
| L2, sinh văn bản | Dựng lại câu chữ từ L1 | Chưa xây; hiện dựng lại từ L1 chỉ ra danh sách có cấu trúc |

Cách dựng lại thực dụng (hỗn hợp): với từng khối, nếu phần tử gắn vào đủ để tái tạo (độ phủ từ từ 0,9 trở lên) thì sinh từ cấu trúc; nếu không thì giữ nguyên văn khối. Tài liệu dựng ra luôn đầy đủ, và phần sinh từ cấu trúc tăng dần khi bước trích tốt lên.

### 9.2 Mẫu tài liệu

`DocumentTemplate` TPL-TDD gồm 9 `SectionSpec` rút ra từ cấu trúc quan sát được ở 13 TDD: .1 mục tiêu và truy vết, .2 thành phần, .3 dữ liệu, .4 API, .5 thiết kế động, .6 lỗi, bảo mật, hiệu năng, quan sát, .7 kiểm thử, .8 phân rã công việc, .9 vấn đề mở. Mục .1, .2, .7 có mặt ở cả 13 TDD nên đặt bắt buộc. Danh sách `holds` (lớp thuộc mục nào) chỉ gồm lớp có mục nhà rõ rệt; lớp xuyên suốt (Constraint, Control, Decision, Assumption, Risk, Dependency, Event, Job, ConfigItem, QualityTarget, Interface, Artefact, Term) không bị giới hạn. Mẫu được hiệu chỉnh theo chính 13 TDD đang có nên có nguy cơ khớp quá mức; cần duyệt lại khi có TDD thứ 14.

### 9.3 Số đo độ phủ dựng lại từ cấu trúc

| Loại khối | Số khối | Độ phủ từ | Khối đạt từ 0,9 | Khối có phần tử gắn |
|---|---|---|---|---|
| Dòng bảng | 3.757 | 66,4% | 43,0% | 77,7% |
| Mục danh sách | 758 | 43,7% | 15,4% | 69,0% |
| Đoạn văn | 920 | 37,3% | 7,2% | 60,2% |
| Mã nguồn | 206 | 7,4% | 4,9% | 5,8% |
| Tiêu đề | 376 | 1,7% | 1,6% | 2,1% |
| Toàn bộ | 6.017 | 47,8% | 30,1% | 66,7% |

Độ phủ từ là tỷ lệ từ phân biệt của khối gốc có trong bản dựng từ cấu trúc; đây là cận trên thô, không xét thứ tự và ngữ pháp, nên độ phủ thật thấp hơn hoặc bằng.

Kết luận: số hoá hiện chủ yếu phủ bảng; đoạn văn, mã nguồn (DDL, cấu hình, mã giao diện) và tiêu đề gần như chưa được số hoá. Dựng lại không mất mát đã có (L0); dựng lại chỉ từ cấu trúc chưa đạt. Hai hướng nâng độ phủ: (a) mô hình hoá mã nguồn nhúng (DDL thành Entity, Attribute, Index; cấu hình thành ConfigItem; chữ ký giao diện thành Operation, Message); (b) mô hình hoá tiêu đề và đoạn dẫn thành `Section` với khe văn xuôi rõ ràng, hoặc khai báo `prose_only`.

## 10. Hồ sơ liên tài liệu và hợp đồng đầu vào của agent

### 10.1 Hồ sơ theo loại tài liệu (`profiles.yaml`)

| Loại | Vai trò | Lớp sở hữu | Dạng mã | Thuộc tính bắt buộc tiêu biểu |
|---|---|---|---|---|
| URD | Yêu cầu người dùng | Requirement, Scenario, QualityTarget, BusinessRule, StateMachine, Term, Stakeholder | FR-[A-Z]{2,3}-\d{2}, NFR-[A-Z]{2}-\d{2}, UC-\d{2,3}, BRL-\d{2,3}, SM-\d{2} | Requirement: statement, priority, requirement_kind; QualityTarget: characteristic, target, basis |
| SRS | Đặc tả yêu cầu phần mềm | Operation, Entity, Scope, ErrorCode, BusinessRule, Requirement, Interface | API-[A-Z]-\d{2}, ENT-\d{2}, PBOM-[A-Z]{2,4}-\d{4} | Operation: method, path_or_name, idempotent; Entity: level |
| SAD | Kiến trúc phần mềm | Decision, Constraint, Component, Module, Layer, DeploymentUnit, Risk, Alternative | ADR-\d{2,3}, CT-\d{2}, SVC-\d{2}, M-\d{2} | Decision: status, rationale; Constraint: constraint_kind, statement |
| TDD | Thiết kế kỹ thuật chi tiết | Component, Interface, Entity, Attribute, Index, StateMachine, Transition, Algorithm, Control, Metric, TestCase, WorkItem, OpenIssue, ConfigItem, Constant | WI-(TDD)?\d{2}-\d{2,3}, TC-[A-Z0-9]+-\d{2,3}, OQ-\d{2}-\d{2}, OA-[A-Z0-9]+-\d{2} | WorkItem: estimate; TestCase: kind, level, expected |
| CR | Yêu cầu thay đổi | ChangeRequest, ChangeSet, ChangeOp | CR-\d{4}, CS-\d{4} | ChangeRequest: change_source, requester, rationale, status, target_docs |

Hồ sơ chỉ thắt chặt lõi (mục 15). Hồ sơ URD, SRS, SAD hiện ở mức khung: đã khai báo lớp sở hữu, dạng mã và thuộc tính bắt buộc tiêu biểu; chưa có mẫu mục như TPL-TDD và chưa số hoá nội dung.

### 10.2 Phát hiện xung đột và khuyết

| Loại | Rule | Trạng thái |
|---|---|---|
| Khuyết trong một tài liệu (thiếu thuộc tính, phần còn mở không chủ, truy vết hổng) | M-01, G-02..G-07, T-01..T-08 | Chạy được |
| Cùng khoá, khác giá trị giữa các tài liệu | X-01, X-02 | Chạy được, trên dữ liệu trước khi gộp |
| Trùng danh tính | X-04 | Chạy được (cần quy tắc gộp) |
| Rule xung đột nhau | L-14 | Chỉ kiểm khi cạnh `conflicts_with` đã khai báo; chưa tự phát hiện |
| Thượng nguồn đổi mà hạ nguồn chưa cập nhật | X-03 | Chưa chạy: thượng nguồn chưa có phiên bản và giá trị dạng cấu trúc |
| TDD khác thượng nguồn mà không có Deviation | X-05 | Chưa chạy, lý do như trên |
| Thay đổi mới gây vi phạm | C-05 | Chạy được |

### 10.3 Hợp đồng agent sinh hoặc sửa TDD

Đầu vào: lát cắt đồ thị thượng nguồn theo hồ sơ (URD, SRS, SAD), đồ thị TDD hiện có, các ChangeRequest đã duyệt. Đầu ra duy nhất được phép: một ChangeSet (JSON theo lược đồ `ChangeOp`), không ghi thẳng vào kho. Các cổng, theo thứ tự:

1. Lược đồ JSON của ChangeOp và của phần tử mới (cổng ghi).
2. Điều kiện C-02, C-03, C-04, C-06, C-08 của động cơ.
3. Hồi quy C-05: không có vi phạm `error` mới.
4. Báo cáo khuyết và xung đột: khuyết mới (G, M, T) và xung đột mới (X) liệt kê cho người duyệt; khuyết `detected` chưa xử lý thì không lên `baselined`.
5. Dựng lại văn bản, so khác biệt với bản trước (chỉ các khối bị tác động thay đổi).
6. Người duyệt bộ thay đổi; sau đó mới áp dụng.

### 10.4 Điều kiện cần trước khi dựng agent

- Số hoá URD, SRS, SAD bằng cùng quy trình và hồ sơ ở 10.1. Hiện các phần tử thượng nguồn trong đồ thị chỉ là mã tham chiếu (SRS: 284 thao tác, 46 thực thể; URD/SRS: 79 yêu cầu; SAD: 15 quyết định) và gần như không có thuộc tính; riêng điểm cuối API có đường dẫn và quyền từ bảng SRS. Thiếu bước này agent không đối chiếu được giá trị với thượng nguồn (X-03, X-05).
- Bộ vàng (golden set): tập cặp (CR, ChangeSet đúng) do người lập, để đo agent. Chưa có.
- Chạy lại trích theo lược đồ v0.4 để thuộc tính bắt buộc, Quantity, Gap được điền.

## 11. Kết quả chạy 74 rule trên đồ thị v0.4

| Rule | Đạt | Không đạt | Thiếu dữ liệu | Không áp dụng | Nội dung |
|---|---|---|---|---|---|
| S-01 | 12871 | 0 | 0 | 0 | Lớp phải thuộc lược đồ |
| S-02 | 15471 | 11 | 0 | 0 | Quan hệ đúng miền, miền giá trị, có xét lớp con |
| S-03 | 15482 | 0 | 0 | 0 | Quan hệ phải là tên chuẩn, không dùng tên đã loại |
| S-04 | 2 | 1 | 0 | 0 | Quan hệ đặc tính acyclic không có chu trình |
| S-05 | 1532 | 10 | 0 | 0 | Quan hệ n-1 có tối đa một đích mỗi nguồn |
| S-06 | 12868 | 0 | 3 | 0 | Phần tử có đúng một tài liệu sở hữu |
| S-07 | 12871 | 0 | 0 | 0 | Mã phần tử đúng khuôn của lớp và duy nhất toàn kho |
| S-08 | 15482 | 0 | 0 | 0 | Cạnh tham chiếu phần tử tồn tại |
| V-01 | 52739 | 13 | 0 | 0 | Giá trị enum thuộc từ vựng (sau chuẩn hoá bí danh) |
| V-02 | 159 | 5 | 0 | 0 | Đơn vị thuộc sổ đơn vị |
| V-03 | 787 | 0 | 0 | 0 | Quantity đúng cấu trúc: có value hoặc có slot_status |
| V-04 | 0 | 0 | 14 | 0 | Quantity số phải có đơn vị khi đại lượng có thứ nguyên |
| V-05 | 0 | 0 | 1 | 0 | Cùng một đại lượng không dùng hai đơn vị khác nhau trong cùng phạm vi |
| M-01 | 8181 | 2006 | 2099 | 585 | Thuộc tính bắt buộc (M) phải có từ mức specified |
| M-02 | 0 | 0 | 0 | 12871 | Thuộc tính có điều kiện (C) bắt buộc khi điều kiện đúng |
| M-03 | 0 | 0 | 0 | 12871 | Phần tử mức verified phải có ít nhất một verified_by trỏ đến TestCase |
| M-04 | 0 | 0 | 0 | 12871 | Phần tử mức baselined không còn Gap mở |
| M-05 | 0 | 0 | 1 | 0 | Phần tử phụ thuộc không chín hơn phần tử mà nó dựa vào |
| G-01 | 122 | 0 | 0 | 0 | Gap đúng cấu trúc: slot, status, lý do |
| G-02 | 0 | 52 | 0 | 0 | Gap tbd/unknown có chủ và hạn |
| G-03 | 12 | 51 | 0 | 0 | Giá trị assumed hoặc proposed phải liên kết Assumption hoặc OpenIssue |
| G-04 | 16 | 142 | 0 | 0 | OpenIssue có người xử lý, hạn, trạng thái |
| G-05 | 0 | 0 | 0 | 0 | Phần tử deferred có mốc dời đến |
| G-06 | 3 | 4 | 0 | 0 | Giá trị to_measure có kế hoạch đo (Metric hoặc TestCase) |
| G-07 | 0 | 0 | 158 | 0 | OpenIssue quá hạn |
| G-08 | 0 | 0 | 54 | 0 | Quyết định phải đóng OpenIssue mà nó nêu |
| G-09 | 0 | 0 | 1 | 0 | Khuyết khai báo có chủ đích không được trùng với thiếu sót phát hiện |
| T-01 | 72 | 7 | 0 | 0 | Requirement được ít nhất một phần tử thiết kế thoả mãn |
| T-02 | 0 | 0 | 79 | 0 | Requirement P0 có ít nhất một TestCase |
| T-03 | 1017 | 794 | 0 | 0 | Phần tử thiết kế truy được lên nguồn thượng nguồn |
| T-04 | 69 | 284 | 0 | 0 | WorkItem trỏ tới phần tử thiết kế mà nó hiện thực |
| T-05 | 289 | 129 | 0 | 0 | TestCase kiểm chứng ít nhất một phần tử |
| T-06 | 184 | 113 | 0 | 0 | ErrorCode được dùng ít nhất một nơi |
| T-07 | 49 | 32 | 0 | 0 | Entity được hiện thực bởi lược đồ lưu trữ |
| T-08 | 32 | 43 | 0 | 0 | Budget nối Slo hoặc QualityTarget |
| L-01 | 0 | 0 | 13 | 0 | Bảng quyết định đầy đủ: mọi tổ hợp đầu vào có luật phủ |
| L-02 | 0 | 0 | 13 | 0 | Bảng quyết định loại trừ: không hai luật cùng khớp (hit_policy=unique) |
| L-03 | 0 | 0 | 1 | 0 | Bảng quyết định có thứ tự (first) có luật mặc định cuối |
| L-04 | 0 | 0 | 41 | 0 | Mô hình trạng thái có đúng một trạng thái đầu |
| L-05 | 120 | 47 | 0 | 0 | Chuyển trạng thái có nguồn và đích thuộc cùng mô hình |
| L-06 | 0 | 0 | 41 | 0 | Mọi trạng thái đạt tới được từ trạng thái đầu |
| L-07 | 0 | 0 | 41 | 0 | Trạng thái cuối không có chuyển đi; trạng thái không cuối có lối ra |
| L-08 | 92 | 0 | 75 | 0 | Chuyển trạng thái có sự kiện kích hoạt, tuỳ chọn guard và action |
| L-09 | 0 | 0 | 24 | 0 | Thuật toán có điều kiện trước, điều kiện sau, đầu vào, đầu ra |
| L-10 | 0 | 0 | 0 | 1 | Công thức khai báo biến, đơn vị đầu ra và miền hợp lệ |
| L-11 | 0 | 0 | 438 | 0 | Thao tác ghi khai báo idempotency và điều khiển đồng thời |
| L-12 | 0 | 0 | 1 | 0 | Invariant của Entity tham chiếu được thuộc tính có thật |
| L-13 | 0 | 0 | 649 | 0 | Rule có modality và mức nghiêm trọng |
| L-14 | 0 | 1 | 0 | 0 | Rule xung đột phải có quyết định phân xử |
| K-01 | 303 | 0 | 0 | 0 | Tên hằng số duy nhất trong phạm vi |
| K-02 | 125 | 164 | 1 | 0 | Hằng số có kiểu, giá trị mặc định hoặc Gap, đơn vị khi có thứ nguyên |
| K-03 | 2 | 0 | 13 | 0 | Giá trị trong khoảng min..max đã khai báo |
| K-04 | 13 | 0 | 0 | 0 | Hằng số có người dùng (configures hoặc constrained_by) |
| K-05 | 3 | 0 | 0 | 0 | Bí mật không có giá trị mặc định trong tài liệu |
| K-06 | 0 | 0 | 1 | 0 | Cùng tham số khác giá trị giữa các tài liệu phải có Decision |
| C-01 | 0 | 0 | 0 | 1 | Xoá phần tử còn bị tham chiếu phải có chính sách xử lý |
| C-02 | 0 | 0 | 0 | 1 | Đích của thao tác phải tồn tại (trừ add) |
| C-03 | 0 | 0 | 0 | 1 | Phần tử thêm mới có mã chưa dùng, lớp hợp lệ, cạnh đúng miền và miền giá trị |
| C-04 | 0 | 0 | 0 | 1 | Thao tác sửa mang giá trị trước khớp trạng thái hiện hành (chống ghi đè) |
| C-05 | 0 | 0 | 0 | 1 | Áp dụng bộ thay đổi không tạo thêm vi phạm mức error |
| C-06 | 0 | 0 | 0 | 1 | Bộ thay đổi chỉ áp dụng khi yêu cầu đã duyệt |
| C-07 | 0 | 0 | 0 | 1 | Phần tử bị tác động phải được đánh dấu cần duyệt lại |
| C-08 | 0 | 0 | 0 | 1 | Phần tử baselined chỉ đổi qua bộ thay đổi đã duyệt, và hạ mức chín muồi |
| C-09 | 0 | 0 | 0 | 1 | Bộ thay đổi áp dụng nguyên khối: một thao tác lỗi thì không thao tác nào được áp dụng |
| P-01 | 13 | 0 | 0 | 0 | Tài liệu có mẫu và mọi mục bắt buộc của mẫu xuất hiện |
| P-02 | 2765 | 234 | 893 | 0 | Phần tử được định nghĩa trong mục thuộc lớp mục đó cho phép |
| P-03 | 3534 | 2105 | 0 | 702 | Khối nguồn hoặc gắn với phần tử, hoặc khai báo prose_only |
| P-04 | 12 | 121 | 0 | 0 | Dựng lại từ cấu trúc đạt ngưỡng độ phủ từng mục |
| P-05 | 11 | 0 | 0 | 0 | Dựng lại không mất mát: nối các khối theo seq trả đúng bản gốc |
| X-01 | 4 | 2 | 0 | 0 | Cùng khoá, khác giá trị giữa các tài liệu phải có Decision hoặc Deviation |
| X-02 | 0 | 1 | 0 | 0 | Cùng thuộc tính của một thực thể, khác kiểu hoặc khả rỗng giữa các tài liệu |
| X-03 | 0 | 0 | 1 | 0 | Phần tử suy ra từ thượng nguồn đã đổi phiên bản bị đánh dấu cũ |
| X-04 | 4576 | 50 | 0 | 0 | Phần tử trùng danh tính (cùng lớp, tên chuẩn hoá) phải gộp hoặc khai báo khác nhau |
| X-05 | 0 | 0 | 1 | 0 | TDD khác thượng nguồn phải có Deviation |

Chú thích: "Thiếu dữ liệu" là `insufficient_data`. Rule C chỉ được đánh giá khi có ChangeSet (mục 8.5), nên ở lần chạy tĩnh này ghi "không áp dụng".

## 12. Điều kết quả cho biết

1. **Phần cấu trúc sạch** như ở v0.3: còn 11 cạnh sai miền, 10 vi phạm n-1 (chủ yếu một thành phần dưới hai mã), 1 chu trình `depends_on` giữa M-03 và M-04.
2. **Phần còn mở chưa được quản lý** như ở v0.3: 142 vấn đề mở thiếu hạn, 52 khuyết chưa chủ, 51 giá trị giả định hoặc đề xuất chưa nối.
3. **Toàn vẹn liên tài liệu**: kiểm được 7 cặp giá trị cùng khoá giữa các TDD (trước khi gộp ít khoá trùng nhau), 3 cặp lệch. `limits.sbom_max_bytes` ghi 50 MiB ở TDD-01 và 50 MB ở TDD-04 trong khi giá trị 52.428.800 byte đúng bằng 50 MiB (lệch ký hiệu đơn vị, đã đối chiếu văn bản gốc); kiểu của hằng số này ghi `int` và `int64`; kiểu của `projects.gate_failure_mode` ghi `varchar(6)` ở TDD-04 và `text` ở TDD-07, nhưng văn bản gốc chỉ thấy `varchar(6)` nên nhiều khả năng là sai sót trích.
4. **Trùng danh tính**: 50 nhóm phần tử trùng tên trong cùng lớp (đã loại mã lỗi, ca kiểm thử, đầu việc, chuyển trạng thái vì tên trùng hợp lệ); cần quy tắc gộp.
5. **Độ phủ khối**: 2.105 khối nội dung chưa gắn phần tử (P-03); 121 trên 133 mục (tài liệu, mục) có độ phủ từ dưới 0,9 (P-04).
6. **Mẫu**: cả 13 TDD có đủ mục bắt buộc (P-01); 234 phần tử được định nghĩa ngoài mục nhà của lớp mình (P-02).

## 13. Việc cần làm tiếp theo

| # | Việc | Rule | Loại |
|---|---|---|---|
| 1 | Số hoá URD, SRS, SAD theo hồ sơ 10.1 (cùng quy trình, có đo chất lượng) | X-03, X-05, T-01 | Công cụ và dữ liệu |
| 2 | Lập bộ vàng (CR, ChangeSet đúng) từ 20 đến 30 thay đổi thật | | Người |
| 3 | Mô hình hoá mã nguồn nhúng (DDL, cấu hình, chữ ký) thành phần tử | P-03, P-04 | Công cụ |
| 4 | Thêm `Section` và khe văn xuôi; khai báo `prose_only` cho khối văn xuôi thuần | P-03 | Ontology và công cụ |
| 5 | Chạy lại trích theo lược đồ v0.4 | M-01 | Công cụ |
| 6 | Quy tắc gộp thực thể trùng (50 nhóm, các thành phần dưới hai mã) | X-04, S-05 | Công cụ |
| 7 | Lượt LLM nối đầu việc, ca kiểm thử với phần tử thiết kế | T-04, T-05 | Công cụ |
| 8 | Cấu trúc hoá bảng quyết định, trạng thái đầu và cuối, điều kiện trước và sau; cài rule L-01..L-07 | L-01..L-09 | TDD và công cụ |
| 9 | Giao chủ và hạn cho 142 vấn đề mở và 52 khuyết tbd; nối giả định, đề xuất | G-02..G-04 | Quản lý |
| 10 | Sửa lệch ký hiệu MB và MiB; thống nhất kiểu `int` và `int64` | X-01 | TDD |
| 11 | Chuyển động cơ thay đổi sang kho giao dịch khi cần nhiều người dùng đồng thời | C-04 | Công cụ |
| 12 | Cài X-03, X-05 khi thượng nguồn đã có phiên bản và giá trị cấu trúc | X-03, X-05 | Công cụ |
| 13 | Cài `supersede` và `merge` trong động cơ | C-03 | Công cụ |

## 14. Hạn chế đã biết

- Một lượt trích LLM; cạnh dưới 0,8 tin cậy chỉ đúng khoảng 38% (mẫu 360 bản ghi). Kết quả rule kế thừa độ sai này.
- Độ phủ dựng lại là cận trên theo từ, không phải so sánh câu chữ.
- Mẫu TDD và danh sách `holds` hiệu chỉnh theo chính dữ liệu thử.
- Bảng tác động và các ngưỡng là đề xuất, chưa được chuyên gia nghiệp vụ duyệt.
- Rule L-01..L-09, X-03, X-05 chưa chạy được; trong lúc đó cổng C-05 không bảo vệ các lỗi thuộc những rule này.
- Bộ kiểm thử động cơ gồm 15 ca do tác giả viết, chưa phải bộ vàng độc lập.
- Ánh xạ sang tiêu chuẩn ISO chưa được kiểm chứng.

## 15. Quản trị ontology

- **Đánh số phiên bản** MAJOR.MINOR.PATCH: MAJOR khi đổi nghĩa hoặc xoá lớp, quan hệ; MINOR khi thêm lớp, quan hệ, từ vựng, rule; PATCH khi sửa bí danh, ghi chú. Phiên bản hiện tại 0.4.0; trước 1.0 mọi MINOR được phép đổi.
- **Quan hệ đã loại** nằm trong `DEPRECATED` kèm cách chuyển (đảo chiều hoặc giữ nguyên); rule S-03 chặn dùng lại. Đã loại: `tests`, `applies_to`.
- **Thêm giá trị từ vựng**: đề xuất kèm ví dụ từ văn bản thật; được thêm khi có từ hai ví dụ độc lập hoặc một ví dụ có người duyệt.
- **Thuộc tính ngoài lược đồ (`ext`)**: thống kê hàng quý; khoá xuất hiện từ 3 phần tử trở lên được xét đưa vào lớp.
- **Hồ sơ theo lớp**: lõi (mọi tài liệu) → hồ sơ loại tài liệu (TDD, URD, SRS, SAD) → hồ sơ dự án. Hồ sơ chỉ được thắt chặt (thêm bắt buộc, thu hẹp từ vựng), không nới lỏng lõi.
- **Nguồn duy nhất**: `onto3.py` và `rules3.py`; mọi tệp khác sinh ra bằng `gen3.py`, không sửa tay.

## 16. Tệp đi kèm

| Tệp | Nội dung |
|---|---|
| `tdd-ontology-v0.4.ttl`, `tdd-shapes-v0.4.ttl` | OWL và SHACL |
| `schemas/*.schema.json` | 88 JSON Schema cho cổng ghi |
| `classes.yaml`, `relations.yaml`, `vocab.yaml`, `units.yaml`, `rules.yaml`, `profiles.yaml` | Mô hình dạng dữ liệu, gồm hồ sơ và bảng lan truyền tác động |
| `kg4.json` | Đồ thị v0.4 (phần tử, cạnh, khối nguồn) |
| `blocks4.jsonl`, `blocks4_checks.json` | Kho khối nguồn nguyên văn và băm đối chiếu |
| `validate4.json`, `findings_v4.csv`, `render_report.json`, `test_changeset_kg4.json` | Kết quả rule, độ phủ dựng lại, kiểm thử động cơ |
| `tools/`, `tests/` | onto3.py (nguồn duy nhất), rules3.py, gen3.py, migrate3.py, migrate4.py, blockstore.py, render.py, validate3.py, changeset.py, test_changeset.py |

---

# Phụ lục

## Phụ lục A. Lớp và thuộc tính

### A.1 Tài liệu và tri thức

**Document** (Tài liệu), lớp cha Element. Một tài liệu thuộc họ tài liệu của dự án.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| doc_type | enum:DocumentType | M |  |
| version | str | M |  |
| profile | str | M | Hồ sơ loại tài liệu |

**DocumentVersion** (Phiên bản tài liệu), lớp cha Element. Một phiên bản có băm nội dung.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| content_hash | str | M |  |
| released_at | date | O |  |

**Baseline** (Đường cơ sở), lớp cha Element. Bộ phiên bản được ghim cùng nhau.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| members | list:ref:DocumentVersion | M |  |
| signed_by | str | C |  |

**Section** (Mục), lớp cha Element. Mục của tài liệu.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| number | str | M |  |
| title | str | M |  |

**SourceBlock** (Khối nguồn), lớp cha Element. Khối văn bản nhỏ nhất có neo.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| kind | enum:BlockKind | M |  |
| line | int | M |  |
| text_hash | str | M |  |

**Term** (Thuật ngữ), lớp cha Element. Thuật ngữ miền hoặc kỹ thuật.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| definition | text | M |  |
| synonyms | list:str | O |  |

**Standard** (Chuẩn), lớp cha Element. Tiêu chuẩn, RFC, quy ước bên ngoài.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| identifier | str | M |  |
| edition | str | O |  |

### A.2 Yêu cầu thượng nguồn

**Stakeholder** (Bên liên quan), lớp cha Element. Nhóm hoặc vai trò có lợi ích trong hệ thống.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| concerns | list:str | O |  |

**Concern** (Mối quan tâm), lớp cha Element. Mối quan tâm của bên liên quan (ISO/IEC/IEEE 42010).

**Requirement** (Yêu cầu), lớp cha Element. Yêu cầu chức năng, phi chức năng, ràng buộc, giao diện hoặc dữ liệu.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| req_kind | enum:RequirementKind | M |  |
| statement | text | M |  |
| acceptance | text | C | Bắt buộc với yêu cầu ưu tiên P0 |

**BusinessRule** (Quy tắc nghiệp vụ), lớp cha Element. Quy tắc nghiệp vụ có cấu trúc điều kiện và kết quả (mục 7.1).

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| rule_form | json | C | when, then, otherwise, exceptions; bắt buộc khi quy tắc đã chín muồi |
| precedence | int | O | Thứ tự ưu tiên khi nhiều quy tắc cùng áp dụng |
| rule_kind | enum:BusinessRuleKind | O |  |
| severity | enum:RuleLevel | O | Mức tác động khi vi phạm |

**Scenario** (Kịch bản), lớp cha Element. Ca sử dụng hoặc kịch bản người dùng.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| actors | list:ref:Stakeholder | M |  |
| main_flow | ref:Flow | O |  |
| end_conditions | text | O |  |

**Milestone** (Mốc), lớp cha Element. Mốc giao hàng hoặc mục tiêu POC có tiêu chí đo.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| criteria | list:str | M |  |
| target_date | date | O |  |

**PriorityLevel** (Mức ưu tiên), lớp cha Element. Mức ưu tiên theo mục tiêu POC.

**QualityCharacteristic** (Đặc tính chất lượng), lớp cha Element. Đặc tính theo ISO/IEC 25010.

### A.3 Cấu trúc hệ thống

**System** (Hệ thống), lớp cha Element. Hệ thống đang thiết kế.

**Layer** (Tầng), lớp cha Element. Tầng kiến trúc.

**Module** (Mô-đun), lớp cha Element. Mô-đun theo SAD; đơn vị sở hữu dữ liệu và ranh giới.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| responsibility | text | M |  |
| runtime_unit | str | O |  |

**Component** (Thành phần), lớp cha Element. Thành phần cài đặt (dịch vụ, gói, bộ điều hợp, ứng dụng).

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| component_kind | enum:ComponentKind | M |  |
| responsibility | text | M |  |
| tech | str | C | Bắt buộc khi component_kind là service, ui_app, worker |
| package | str | O | Đường dẫn gói mã |
| stateless | bool | O |  |
| scaling_unit | str | O |  |
| runtime_unit | str | O |  |
| db_role | ref:DbRole | O |  |

**DeploymentUnit** (Đơn vị triển khai), lớp cha Element. Tệp nhị phân, ảnh, gói triển khai.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| artifact | str | O |  |
| scaling | str | O |  |

**Environment** (Môi trường), lớp cha Element. Môi trường chạy (dev, staging, prod).

**ExternalSystem** (Hệ thống ngoài), lớp cha Element. Hệ thống không do dự án sở hữu.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| contract | str | O |  |
| trust_level | str | O |  |

**Dependency** (Phụ thuộc ngoài), lớp cha Element. Thư viện hoặc dịch vụ bên thứ ba.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| license | str | M |  |
| version | str | C | Bắt buộc khi phụ thuộc là thư viện |
| approved | bool | O | Thuộc danh mục công nghệ được duyệt |

**Artefact** (Tệp liên kết), lớp cha Element. OpenAPI, DDL, YAML, tài liệu ngoài.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| path | str | M |  |
| format | str | O |  |
| artefact_kind | enum:ArtefactKind | O |  |

**Screen** (Màn hình), lớp cha Element. Màn hình hoặc luồng giao diện người dùng.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| route | str | O |  |
| roles | list:ref:Role | O |  |

### A.4 Giao diện và truy cập

**Interface** (Giao diện), lớp cha Element. Điểm giao tiếp giữa các phần.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| protocol | enum:Protocol | M |  |
| style | str | O |  |
| contract_ref | ref:Artefact | C | Bắt buộc khi protocol là http hoặc grpc |
| authn | str | O |  |
| versioning | str | O |  |
| timeout | qty | O |  |

**Operation** (Thao tác), lớp cha Element. Thao tác của giao diện, ví dụ điểm cuối API.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| method | str | C | Bắt buộc với giao diện http |
| path_or_name | str | M |  |
| idempotent | bool | M |  |
| idempotency_key | bool | C | Bắt buộc khi thao tác ghi và idempotent |
| concurrency_control | enum:ConcurrencyControl | C | Bắt buộc với thao tác sửa |
| authn | str | O |  |
| timeout | qty | O |  |
| roles | list:ref:Role | O |  |
| sla_ref | ref:Slo | O |  |

**Message** (Thông điệp), lớp cha Element. Cấu trúc dữ liệu vào, ra hoặc sự kiện.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| schema_ref | ref:Artefact | O |  |

**Event** (Sự kiện), lớp cha Element. Sự kiện miền hoặc tích hợp.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| topic | str | M |  |
| channel | enum:EventChannel | M |  |
| delivery | enum:DeliverySemantics | M |  |
| ordering_key | str | O |  |
| payload | ref:Message | O |  |

**ErrorCode** (Mã lỗi), lớp cha Element. Mã lỗi nghiệp vụ hoặc kỹ thuật.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| code | str | M |  |
| http_status | int | M |  |
| slug | str | O |  |
| retryable | bool | M |  |
| when | text | M | Điều kiện phát sinh |
| handling | text | O |  |

**Scope** (Quyền), lớp cha Element. Scope của token hoặc quyền thao tác.

**Role** (Vai trò), lớp cha Element. Vai trò người dùng hoặc máy.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| principal_type | enum:PrincipalType | O |  |

**Credential** (Thông tin xác thực), lớp cha Element. Token, cookie, khoá, bí mật và vòng đời của chúng.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| credential_kind | enum:CredentialKind | M |  |
| storage | str | M |  |
| lifetime | qty | C | Bắt buộc khi có hạn dùng |
| rotation | text | C |  |
| revocation | text | C |  |

**Policy** (Chính sách), lớp cha Element. Chính sách áp dụng cho thao tác, dữ liệu hoặc luồng.

**RoutePolicy** (Chính sách định tuyến), lớp cha Policy. Chính sách của một tuyến ở cổng vào.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| upstream | str | M |  |
| auth | str | M |  |
| principals | str | O |  |
| rate_limit | ref:RateLimitProfile | M |  |
| timeout | qty | M |  |
| retry | enum:RetryMode | M |  |
| max_body | qty | M |  |

**RateLimitProfile** (Hạn mức tần suất), lớp cha Policy. Hồ sơ giới hạn tần suất.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| limit | qty | M |  |
| key_by | str | M | ip, principal, project |

**RetentionPolicy** (Chính sách lưu giữ), lớp cha Policy. Thời hạn lưu và cách xoá.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| period | qty | M |  |
| action | enum:RetentionAction | M |  |
| legal_basis | str | O |  |

### A.5 Dữ liệu

**DataStore** (Kho dữ liệu), lớp cha Element. CSDL, hàng đợi, hệ lưu tệp.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| technology | str | M |  |
| owner_module | ref:Module | O |  |

**Schema** (Lược đồ CSDL), lớp cha Element. Không gian tên của bảng, sở hữu bởi một mô-đun.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| owner_role | ref:DbRole | O |  |

**Entity** (Thực thể), lớp cha Element. Thực thể nghiệp vụ hoặc bảng vật lý.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| level | enum:EntityLevel | M | logical hoặc physical |
| table_name | str | C | Bắt buộc khi level=physical |
| store | ref:DataStore | C |  |
| pk | str | C | Bắt buộc khi level=physical |
| soft_delete | bool | O |  |
| partition | str | O |  |
| data_class | enum:DataClassVocab | O |  |
| fillfactor | int | O |  |

**Attribute** (Cột hoặc thuộc tính), lớp cha Element. Thuộc tính dữ liệu hoặc cột.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| datatype | str | M |  |
| nullable | bool | M |  |
| default | str | O |  |
| enum_ref | ref:Enumeration | O |  |
| enum_values | list:str | O |  |
| unit | str | O |  |
| pii | bool | O |  |
| pk | bool | O |  |
| unique | bool | O |  |
| constraint | expr | O |  |

**Index** (Chỉ mục), lớp cha Element. Chỉ mục của bảng.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| columns | list:str | M |  |
| unique | bool | M |  |
| partial | expr | O |  |
| method | str | O |  |

**View** (Khung nhìn), lớp cha Element. View hoặc view vật hoá.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| definition | expr | O |  |
| materialized | bool | O |  |

**Sequence** (Dãy số), lớp cha Element. Sequence của CSDL.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| gap_allowed | bool | O |  |

**Trigger** (Trigger CSDL), lớp cha Element. Trigger thi hành quy tắc ở tầng dữ liệu.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| timing | str | M |  |
| enforces_text | text | O |  |

**DbRole** (Vai trò CSDL), lớp cha Element. Vai trò truy cập CSDL.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| privileges | text | O |  |

**Migration** (Di trú), lớp cha Element. Tệp di trú lược đồ.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| path | str | M |  |
| reversible | bool | O |  |

**DataClass** (Phân loại dữ liệu), lớp cha Element. Mức nhạy cảm của dữ liệu.

**Enumeration** (Tập giá trị), lớp cha Element. Tập giá trị liệt kê của miền (trạng thái, vai trò).

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| values | list:str | M |  |
| closed | bool | M | true nếu tập không được mở rộng nếu không đổi lược đồ |

### A.6 Hành vi và logic nghiệp vụ

**Flow** (Luồng), lớp cha Element. Chuỗi bước xử lý có thứ tự.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| trigger | str | O |  |
| steps | list:str | O |  |

**StateMachine** (Mô hình trạng thái), lớp cha Element. Mô hình trạng thái của một thực thể.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| subject_entity | ref:Entity | M |  |
| initial_state | ref:State | M |  |
| terminal_states | list:ref:State | C | Bắt buộc nếu có trạng thái cuối |

**State** (Trạng thái), lớp cha Element. Một trạng thái.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| final | bool | O |  |
| meaning | text | O |  |

**Transition** (Chuyển trạng thái), lớp cha Element. Chuyển giữa hai trạng thái.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| trigger | str | M |  |
| guard | expr | O |  |
| action | text | O |  |

**Job** (Tác vụ nền), lớp cha Element. Công việc chạy nền hoặc theo lịch.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| trigger_kind | enum:JobTrigger | M |  |
| schedule | str | C | Bắt buộc khi trigger_kind=schedule |
| queue | str | O |  |
| queue_priority | int | O |  |
| job_key | str | M | Khoá chống chạy trùng |
| idempotent | bool | M |  |

**Algorithm** (Thuật toán), lớp cha Element. Thuật toán có đầu vào, đầu ra, bất biến.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| inputs | list:str | M |  |
| outputs | list:str | M |  |
| complexity | str | O |  |
| deterministic | bool | M |  |
| steps | list:str | O |  |

**Formula** (Công thức), lớp cha Element. Biểu thức tính với biến có đơn vị.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| expression | expr | M |  |
| variables | json | M | tên, đơn vị, miền giá trị |
| result_unit | str | C |  |

**DecisionTable** (Bảng quyết định), lớp cha Element. Bảng điều kiện và kết quả (ma trận giấy phép, hạn SLA).

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| inputs | list:ref:Enumeration | M | Miền của từng đầu vào |
| outputs | list:str | M |  |
| hit_policy | enum:HitPolicy | M |  |

**DecisionRule** (Dòng quyết định), lớp cha Element. Một dòng của bảng quyết định hoặc một quy tắc có mã.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| when | json | M |  |
| then | json | M |  |
| level | enum:RuleLevel | O |  |
| grace | qty | O |  |

**Condition** (Điều kiện), lớp cha Element. Điều kiện tiên quyết, hậu điều kiện, bất biến hoặc điều kiện chuyển.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| condition_kind | enum:ConditionKind | M |  |
| expression | expr | M |  |
| violation_error | ref:ErrorCode | O |  |

### A.7 Chất lượng và quan sát

**QualityTarget** (Mục tiêu chất lượng), lớp cha Element. Mục tiêu đo được gắn đặc tính chất lượng.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| characteristic | ref:QualityCharacteristic | M |  |
| target | qty | M |  |
| basis | enum:EvidenceBasis | M |  |
| measurement_ref | ref:Element | C | Bắt buộc khi basis là to_measure hoặc assumed |

**Budget** (Ngân sách), lớp cha Element. Ngân sách tài nguyên hoặc thời gian của một phần.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| scope | str | M |  |
| target | qty | M |  |
| basis | enum:EvidenceBasis | M |  |
| parent_budget | ref:Budget | O |  |

**Metric** (Chỉ số), lớp cha Element. Chỉ số quan sát (Prometheus).

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| metric_type | enum:MetricType | M |  |
| unit | str | C | Bắt buộc khi có đơn vị |
| labels | list:str | M |  |

**AlertRule** (Quy tắc cảnh báo), lớp cha Element. Điều kiện phát cảnh báo trên chỉ số.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| condition | expr | M |  |
| severity | enum:Severity | M |  |
| action | text | M |  |
| for | qty | O |  |

**Slo** (Mục tiêu mức dịch vụ), lớp cha Element. SLO trên một chỉ số.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| objective | qty | M |  |
| window | qty | M |  |

**LogSpec** (Đặc tả nhật ký), lớp cha Element. Cấu trúc và mức nhật ký.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| fields | list:str | M |  |
| level | str | O |  |
| pii_policy | text | O |  |

**TraceSpec** (Đặc tả truy vết phân tán), lớp cha Element. Cách lan truyền và lấy mẫu vết.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| propagation | str | M |  |
| sampling | str | O |  |

### A.8 Bảo mật và rủi ro

**Threat** (Mối đe doạ), lớp cha Element. Mối đe doạ nhận diện được.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| category | str | M |  |
| asset | str | O |  |

**Control** (Biện pháp kiểm soát), lớp cha Element. Biện pháp giảm rủi ro hoặc đe doạ.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| control_kind | enum:ControlKind | M |  |
| standard_ref | ref:Standard | O |  |

**Risk** (Rủi ro), lớp cha Element. Rủi ro đã đánh giá.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| likelihood | enum:RiskLevel | M |  |
| impact | enum:RiskLevel | M |  |
| treatment | enum:RiskTreatment | M |  |
| owner | str | M |  |

### A.9 Quyết định, giả định và phần còn mở

**Decision** (Quyết định), lớp cha Element. Quyết định thiết kế.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| context | text | M |  |
| chosen | text | M |  |
| rationale | text | M |  |
| reversible | bool | M |  |
| decided_by | str | M |  |
| decided_on | date | C | Bắt buộc khi lifecycle=approved |

**Alternative** (Phương án), lớp cha Element. Phương án đã cân nhắc.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| pros | text | O |  |
| cons | text | O |  |

**Assumption** (Giả định), lớp cha Element. Điều được coi là đúng chưa kiểm chứng.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| statement | text | M |  |
| validation_plan | text | M | Cách và lúc kiểm chứng |
| validate_by | ref:Milestone | M |  |
| impact_if_false | text | M |  |
| owner | str | M |  |

**Constraint** (Ràng buộc), lớp cha Element. Ràng buộc kỹ thuật, nghiệp vụ, pháp lý hoặc hiệu năng.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| constraint_kind | enum:ConstraintKind | M |  |
| statement | text | M |  |
| limit | qty | O |  |
| enforced_by | ref:Element | O |  |

**OpenIssue** (Vấn đề mở), lớp cha Element. Điều chưa quyết, chưa biết hoặc chưa làm, có chủ và hạn.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| issue_kind | enum:OpenIssueKind | M |  |
| question | text | M |  |
| owner | str | M |  |
| due | date | M | Bắt buộc khi đạt đường cơ sở |
| status | enum:OpenIssueStatus | M |  |
| proposed_default | text | O | Giá trị hay giả định tạm dùng trong lúc chờ |
| resolution | text | C | Bắt buộc khi status=resolved |

**Deviation** (Lệch so với thượng nguồn), lớp cha Element. Chỗ TDD khác tài liệu thượng nguồn có chủ ý.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| upstream_ref | ref:Element | M |  |
| reason | text | M |  |
| approved_by | str | C |  |

**Waiver** (Miễn trừ), lớp cha Element. Miễn trừ có thời hạn cho một luật.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| rule_id | str | M |  |
| target | ref:Element | M |  |
| reason | text | M |  |
| approved_by | str | M |  |
| expires | date | M |  |

### A.10 Hằng số và cấu hình

**Constant** (Hằng số), lớp cha Element. Giá trị thiết kế dùng chung, có đúng một nơi định nghĩa.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| key | str | M | Ví dụ limits.sbom_max_bytes |
| constant_kind | enum:ConstantKind | M |  |
| value | qty | M |  |
| basis | enum:EvidenceBasis | M |  |
| mutable | bool | M |  |
| by_phase | json | O | Giá trị theo giai đoạn, ví dụ POC và sau POC |
| config_item | ref:ConfigItem | O |  |

**ConfigItem** (Cấu hình), lớp cha Element. Khoá cấu hình khi chạy.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| key | str | M |  |
| type | str | M |  |
| default | str | M |  |
| env_var | str | O |  |
| secret | bool | M |  |
| unit | str | O |  |
| runtime_changeable | bool | O |  |

### A.11 Triển khai và kiểm thử

**WorkItem** (Đầu việc), lớp cha Element. Đầu việc triển khai.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| estimate | str | M | S, M, L, XL |
| milestone | ref:Milestone | O |  |
| definition_of_done | text | C | Bắt buộc với P0 |

**Gate** (Cổng), lớp cha Element. Cổng chất lượng của mốc.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| criteria | list:str | M |  |

**TestCase** (Ca kiểm thử), lớp cha Element. Ca kiểm thử.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| level | enum:TestLevel | M |  |
| kind | enum:TestKind | M |  |
| expected | text | M |  |
| tool | str | O |  |
| automation | enum:Automation | O |  |
| preconditions | text | O |  |

### A.12 Thay đổi và tác động

**ChangeRequest** (Yêu cầu thay đổi), lớp cha Element. Đề nghị thay đổi một hay nhiều tài liệu, có lý do và nguồn.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| change_source | enum:ChangeSource | M |  |
| requester | str | M |  |
| rationale | text | M |  |
| status | enum:ChangeStatus | M |  |
| target_docs | list:ref:Document | M |  |
| decision_by | str | C | Bắt buộc khi status=approved |
| decided_at | date | C |  |

**ChangeSet** (Bộ thay đổi), lớp cha Element. Tập thao tác áp dụng nguyên khối (hoặc không áp dụng gì).

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| request | ref:ChangeRequest | M |  |
| base_version | str | M | Phiên bản kho mà bộ thay đổi dựa vào |
| result_version | str | C | Bắt buộc khi status=applied |
| status | enum:ChangeStatus | M |  |
| applied_at | date | C |  |

**ChangeOp** (Thao tác thay đổi), lớp cha Element. Một thao tác nguyên tử trong bộ thay đổi.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| op | enum:ChangeOpKind | M |  |
| target | str | M | Mã phần tử hoặc khoá cạnh |
| path | str | C | Tên thuộc tính khi modify |
| before | json | C | Giá trị trước, bắt buộc khi modify hoặc remove để phát hiện ghi đè |
| after | json | C | Giá trị sau, bắt buộc khi add hoặc modify |
| cascade | enum:CascadePolicy | C | Bắt buộc khi remove |
| seq | int | M | Thứ tự áp dụng |

### A.13 Trình bày và dựng lại

**DocumentTemplate** (Mẫu tài liệu), lớp cha Element. Cấu trúc mục và lớp phần tử cho một loại tài liệu.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| doc_type | enum:DocumentType | M |  |
| template_version | str | M |  |

**SectionSpec** (Đặc tả mục), lớp cha Element. Một mục trong mẫu: vị trí, lớp phần tử chứa, dạng trình bày.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| number | str | M |  |
| title | str | M |  |
| seq | int | M |  |
| holds | list:str | M | Các lớp phần tử được định nghĩa trong mục |
| render_mode | enum:RenderMode | M |  |
| required | bool | M |  |
| min_recall | num | O | Ngưỡng dựng lại từ cấu trúc, mặc định theo hồ sơ |

**Block** (Khối nguồn), lớp cha Element. Một khối văn bản có thứ tự của tài liệu (đoạn, dòng bảng, mục danh sách, mã), lưu nguyên văn để dựng lại không mất.

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| doc | ref:Document | M |  |
| section | str | M |  |
| seq | int | M |  |
| block_kind | str | M |  |
| text | text | M |  |
| text_hash | str | M |  |
| prose_only | bool | O | Khối chỉ là văn xuôi, không chứa phần tử; khai báo tường minh |

### A.0 Lớp gốc Element (kế thừa bởi mọi lớp)

| Thuộc tính | Kiểu | Mức | Ghi chú |
|---|---|---|---|
| id | str | M | Bất biến, duy nhất toàn kho |
| name | str | M | Tên ngắn dùng trong văn bản |
| summary | text | O | Một câu |
| owner_doc | ref:Document | M | Tài liệu sở hữu duy nhất |
| origin | enum:Origin | M | own, upstream, external |
| lifecycle | enum:LifecycleStatus | M |  |
| maturity | enum:Maturity | M | Mức chín muồi của chính phần tử |
| priority | enum:PriorityLevel | C | Bắt buộc với phần tử thiết kế và đầu việc, kế thừa từ kế hoạch |
| scope_status | enum:ScopeStatus | O | in_scope (mặc định), deferred, out_of_scope |
| deferred_to | ref:Milestone | C | Bắt buộc khi scope_status=deferred |
| source_ref | list:SourceRef | M | Khối nguồn, vị trí, quote_hash |
| version_introduced | str | M |  |
| version_removed | str | O |  |
| confidence | num | M | 0 đến 1; 1 với dữ liệu xác định hoặc đã duyệt |
| extracted_by | enum:ExtractionMethod | M |  |
| review_status | enum:ReviewStatus | M |  |
| reviewed_by | str | C | Bắt buộc khi review_status=approved |
| reviewed_at | date | C |  |
| gaps | list:Gap | O | Các khuyết có chủ đích hoặc đã phát hiện, xem mục 6 |
| tags | list:str | O |  |
| ext | json | O | Thuộc tính ngoài lược đồ, chờ đưa vào lớp |

## Phụ lục B. Quan hệ

| Quan hệ | Miền | Miền giá trị | Nghịch đảo | Bội số | Đặc tính | Ý nghĩa |
|---|---|---|---|---|---|---|
| part_of | Section, Component, Module, Attribute, Index, State, Transition, Entity, Enumeration, DecisionRule, Condition | Section, Document, Module, System, Entity, StateMachine, Component, DecisionTable, Operation, Layer, Algorithm | has_part | n-1 | acyclic | Thành phần thuộc cái chứa nó |
| describes | Section | mọi lớp | described_in | n-n |  | Mục mô tả phần tử |
| owns | Document | mọi lớp | owned_by | 1-n |  | Tài liệu sở hữu duy nhất một phần tử |
| depends_on | Module, Component, WorkItem, Document, Milestone, Job | Module, Component, WorkItem, Document, Dependency, ExternalSystem, Milestone, Job, Algorithm | required_by | n-n | acyclic_for:Module,WorkItem | Phụ thuộc |
| deployed_on | DeploymentUnit, Component, Module | Environment |  | n-n |  | Chạy trên môi trường |
| realized_in | Interface, Entity, Operation, Message | Artefact |  | n-n |  | Hiện thực bằng tệp liên kết (OpenAPI, DDL) |
| supersedes | mọi lớp | mọi lớp | superseded_by | 1-1 | acyclic | Phiên bản kế |
| conflicts_with | mọi lớp | mọi lớp |  | n-n | symmetric | Mâu thuẫn do công cụ hoặc người ghi |
| derives_from | mọi lớp | Requirement, Decision, QualityTarget, BusinessRule, Scenario, Constraint, Milestone, Standard | derived_into | n-n |  | Xuất phát từ |
| satisfies | Module, Component, Job, Algorithm, Formula, DecisionTable, Interface, Operation, Message, Event, Entity, DataStore, Flow, StateMachine, Transition, Control, Policy, Screen, Trigger, Decision, ConfigItem, WorkItem | Requirement, QualityTarget, BusinessRule, Scenario, Milestone, Constraint | satisfied_by | n-n |  | Đáp ứng yêu cầu |
| verified_by | mọi lớp | TestCase | verifies | n-n |  | Được kiểm bởi ca kiểm thử; thay cho quan hệ `tests` của v0.2 |
| implemented_by | Component, Operation, Entity, Module, Job, Control, StateMachine, Flow, Algorithm, Interface, Event, Policy | WorkItem | implements | n-n |  | Được cài đặt bởi đầu việc |
| scheduled_in | WorkItem | Milestone | schedules | n-1 |  |  |
| gated_by | Milestone, WorkItem | Gate |  | n-n |  |  |
| has_priority | mọi lớp | PriorityLevel |  | n-1 |  | Ưu tiên |
| deferred_to | mọi lớp | Milestone, PriorityLevel |  | n-1 |  | Hoãn tới mốc hoặc mức ưu tiên |
| cites | mọi lớp | Standard |  | n-n |  | Viện dẫn chuẩn |
| uses_term | mọi lớp | Term |  | n-n |  |  |
| provides | Component, Module | Interface, Operation | provided_by | n-n |  |  |
| consumes | Component, Module, Job, Operation | Interface, Event, Operation, ExternalSystem | consumed_by | n-n |  |  |
| exposes | Interface, Component | Operation | exposed_by | 1-n |  |  |
| accepts | Operation | Message |  | n-n |  |  |
| returns | Operation | Message |  | n-n |  |  |
| may_fail_with | Operation, Component, Job, Interface, RoutePolicy | ErrorCode |  | n-n |  | Có thể trả lỗi |
| raises | Component, Job, Transition, Algorithm | ErrorCode |  | n-n |  |  |
| requires_scope | Operation, RoutePolicy | Scope |  | n-n |  |  |
| requires_role | Operation, Screen, Transition | Role |  | n-n |  |  |
| publishes | Component, Module, Job, Operation | Event | published_by | n-n |  |  |
| subscribes | Component, Module, Job | Event | subscribed_by | n-n |  |  |
| emits | Transition, Operation, Job | Event |  | n-n |  |  |
| governed_by | Operation, Interface, Entity, Component, Job | Policy, RoutePolicy, RateLimitProfile, RetentionPolicy | governs | n-n |  | Chịu chính sách |
| routes_to | RoutePolicy | Operation, Interface, Component |  | n-n |  |  |
| uses_credential | Component, Operation, Job, Interface | Credential |  | n-n |  |  |
| has_attribute | Entity, Message, View | Attribute | attribute_of | 1-n |  |  |
| references_entity | Attribute, Entity | Entity |  | n-n |  | Khoá ngoại hoặc tham chiếu |
| realizes | Entity | Entity | realized_by | n-n |  | Bảng vật lý hiện thực thực thể nghiệp vụ |
| indexes | Index | Entity | indexed_by | n-1 |  |  |
| stored_in | Entity, Index, View, Sequence | DataStore, Schema |  | n-1 |  |  |
| reads | Component, Module, Operation, Job, Algorithm, Flow | Entity, DataStore, View |  | n-n |  | Đọc dữ liệu |
| writes | Component, Module, Operation, Job, Algorithm, Flow, Trigger, Migration | Entity, DataStore, Sequence |  | n-n |  | Ghi dữ liệu |
| retained_by | Entity, DataStore, Attribute | RetentionPolicy |  | n-1 |  |  |
| classified_as | Entity, Attribute, Message, Credential | DataClass, DataClassVocab |  | n-1 |  |  |
| enumerates | Attribute, DecisionTable, Message | Enumeration |  | n-1 |  |  |
| has_state | StateMachine | State |  | 1-n |  |  |
| in_machine | Transition | StateMachine |  | n-1 |  |  |
| transition_from | Transition | State |  | n-1 |  |  |
| transition_to | Transition | State |  | n-1 |  |  |
| triggered_by | Transition, Job, Flow | Event, Operation, Job |  | n-n |  |  |
| guarded_by | Transition, Operation | Condition |  | n-n |  |  |
| has_condition | mọi lớp | Condition |  | n-n |  | Điều kiện tiên quyết, hậu điều kiện hoặc bất biến |
| uses_formula | Algorithm, DecisionRule, Component, Job | Formula |  | n-n |  |  |
| uses_constant | mọi lớp | Constant | constant_of | n-n |  | Dùng hằng số thay vì ghi trị số |
| decides_by | BusinessRule, Algorithm, Operation, Job | DecisionTable |  | n-n |  |  |
| has_rule | DecisionTable | DecisionRule |  | 1-n |  |  |
| enforces | Component, Control, Trigger, Operation, Job, Index, Attribute, Entity | BusinessRule, Constraint, Condition | enforced_by | n-n |  | Thi hành quy tắc hoặc ràng buộc |
| configures | ConfigItem | Component, Module, Job, Operation, Policy | configured_by | n-n |  |  |
| measured_by | QualityTarget, Slo, Budget | Metric |  | n-n |  |  |
| observed_by | Operation, Component, Job, Interface, Entity, Control | Metric |  | n-n |  |  |
| alerts_on | AlertRule | Metric |  | n-1 |  |  |
| protects_slo | AlertRule | Slo, QualityTarget |  | n-n |  |  |
| targets | Slo | QualityTarget |  | n-1 |  |  |
| logs_with | Component, Module, Job | LogSpec |  | n-n |  |  |
| traces_with | Component, Module, Job | TraceSpec |  | n-n |  |  |
| threatens | Threat | mọi lớp |  | n-n |  |  |
| mitigates | Control, Decision, Component, Policy | Risk, Threat | mitigated_by | n-n |  |  |
| protects | Control | Entity, Credential, Interface, Operation, DataStore, Component, Module, Job, Flow, Dependency, Artefact |  | n-n |  | Biện pháp bảo vệ phần tử |
| decided_by | mọi lớp | Decision |  | n-n |  |  |
| chooses | Decision | Alternative |  | 1-n |  |  |
| rejects | Decision | Alternative |  | 1-n |  |  |
| assumes | mọi lớp | Assumption |  | n-n |  |  |
| validated_by | Assumption | TestCase, Milestone, Metric, WorkItem |  | n-n |  | Cách kiểm chứng giả định |
| constrained_by | mọi lớp | Constraint | constrains | n-n |  | Chịu ràng buộc |
| blocked_by | mọi lớp | OpenIssue | blocks | n-n |  | Bị chặn hoặc phụ thuộc vào vấn đề mở |
| resolves | Decision, Deviation, Assumption, TestCase, WorkItem | OpenIssue |  | n-n |  |  |
| deviates_from | Deviation | mọi lớp |  | n-n |  |  |
| waives | Waiver | mọi lớp |  | n-n |  | Miễn trừ cho phần tử |
| budgeted_by | mọi lớp | Budget | budget_of | n-n |  | Phần tử chịu ngân sách tài nguyên hoặc thời gian |
| uses_algorithm | Component, Module, Job, Operation, Control, Flow | Algorithm | algorithm_of | n-n |  | Dùng thuật toán |
| has_op | ChangeSet | ChangeOp | op_of | n-1 |  | Bộ thay đổi gồm các thao tác |
| realizes_request | ChangeSet | ChangeRequest | realized_by_set | n-1 |  | Bộ thay đổi hiện thực một yêu cầu |
| affects | ChangeRequest, ChangeSet | mọi lớp | affected_by | n-n |  | Phần tử bị thay đổi hoặc bị tác động |
| changes | ChangeOp | mọi lớp | changed_by | n-n |  | Phần tử là đích của thao tác |
| has_section_spec | DocumentTemplate | SectionSpec | section_spec_of | 1-n |  | Mẫu gồm các đặc tả mục |
| conforms_to | Document, DocumentVersion | DocumentTemplate | template_of | n-1 |  | Tài liệu theo mẫu |
| has_block | Section | Block | block_of | 1-n |  | Mục gồm các khối nguồn có thứ tự |
| binds | Block | mọi lớp | bound_by | n-n |  | Khối là nơi phần tử được định nghĩa hoặc nêu (căn cứ dựng lại) |
| spec_of | SectionSpec | Section |  | n-n |  | Đặc tả mục áp vào mục cụ thể |
| derived_from_version | mọi lớp | DocumentVersion |  | n-1 |  | Phần tử suy ra từ phiên bản thượng nguồn nào (phát hiện cũ) |

Quan hệ đã loại: `tests` → `verified_by` (flip); `applies_to` → `constrained_by` (flip); `configures_legacy` → `configures` (same)

## Phụ lục C. Từ vựng có kiểm soát

**LifecycleStatus**: Vòng đời; external = do tài liệu khác quản lý

Giá trị: `draft`, `proposed`, `approved`, `deprecated`, `removed`, `external`

**Origin**: Nguồn sở hữu của phần tử

Giá trị: `own`, `upstream`, `external`

**Maturity**: Mức chín muồi: phác, đã đặc tả đủ thuộc tính bắt buộc, đã kiểm chứng, đã vào đường cơ sở

Giá trị: `sketch`, `specified`, `verified`, `baselined`

**PriorityLevel**: P0 bắt buộc cho POC; P1 ngay sau POC; P2 giai đoạn sau

Giá trị: `P0`, `P1`, `P2`

**ScopeStatus**: 

Giá trị: `in_scope`, `deferred`, `out_of_scope`

**ExtractionMethod**: 

Giá trị: `rule`, `llm`, `human`

**ReviewStatus**: 

Giá trị: `auto`, `needs_review`, `approved`, `rejected`

**EvidenceBasis**: Căn cứ của một giá trị: nêu trong thượng nguồn; suy ra; đề xuất của TDD; giả định; chờ đo; đã đo; chưa rõ

Giá trị: `specified`, `derived`, `proposed`, `assumed`, `to_measure`, `measured`, `unknown`

Bí danh chuẩn hoá: "đã chỉ định" → `specified`; "srs" → `specified`; "thượng nguồn" → `specified`; "specified" → `specified`; "thiết kế" → `proposed`; "design target" → `proposed`; "target" → `proposed`; "đề xuất" → `proposed`; "proposed" → `proposed`; "mặc định đề xuất" → `proposed`; "mục tiêu thiết kế" → `proposed`; "giả định" → `assumed`; "giả định thiết kế" → `assumed`; "giả định khởi đầu" → `assumed`; "assumed" → `assumed`; "giả thiết" → `assumed`; "mục tiêu thiết kế, chờ poc đo" → `to_measure`; "chờ poc đo" → `to_measure`; "cần đo" → `to_measure`; "sẽ đo" → `to_measure`; "measured" → `measured`; "đã đo" → `measured`; "đo được" → `measured`

**SlotStatus**: Trạng thái của một ô thuộc tính (mục 6)

Giá trị: `set`, `tbd`, `unknown`, `not_applicable`, `deferred`, `assumed`, `proposed`, `to_measure`

**OpenIssueKind**: Loại phần còn mở; upstream_change = đề nghị sửa tài liệu thượng nguồn; external_dependency = chờ bên ngoài

Giá trị: `question`, `tbd_value`, `assumption_validation`, `decision_pending`, `deferred`, `deviation`, `risk_acceptance`, `schema_gap`, `upstream_change`, `external_dependency`

Bí danh chuẩn hoá: "câu hỏi" → `question`; "vấn đề mở" → `question`; "giả định" → `assumption_validation`; "lệch nguồn" → `deviation`; "chờ leg" → `decision_pending`; "chờ quyết định" → `decision_pending`; "đề nghị sửa*" → `upstream_change`; "đề xuất sửa*" → `upstream_change`; "phụ thuộc" → `external_dependency`; "chưa có giá trị" → `tbd_value`; "rủi ro" → `risk_acceptance`

**OpenIssueStatus**: 

Giá trị: `open`, `in_progress`, `resolved`, `wont_fix`

**Modality**: 

Giá trị: `must`, `must_not`, `should`, `may`

**Severity**: 

Giá trị: `error`, `warning`, `info`

**RuleOutcome**: 

Giá trị: `pass`, `fail`, `waived`, `not_applicable`, `insufficient_data`

**RuleLevel**: 

Giá trị: `pass`, `warn`, `fail`, `info`

**DocumentType**: 

Giá trị: `BRD`, `PRD`, `URD`, `SRS`, `SAD`, `TDD`, `Appendix`, `Contract`, `Policy`

**BlockKind**: 

Giá trị: `para`, `row`, `item`, `code`, `head`

**RequirementKind**: 

Giá trị: `functional`, `non_functional`, `constraint`, `interface`, `data`

**BusinessRuleKind**: 

Giá trị: `validation`, `derivation`, `authorization`, `lifecycle`, `calculation`, `policy`

**ComponentKind**: 

Giá trị: `service`, `library`, `package`, `adapter`, `worker`, `ui_app`, `gateway`, `cli`, `ui_component`, `other`

Bí danh chuẩn hoá: "dịch vụ" → `service`; "gói" → `package`; "goi" → `package`; "bộ điều hợp" → `adapter`

**ArtefactKind**: 

Giá trị: `openapi`, `ddl`, `yaml`, `external_doc`, `schema`, `script`, `other`

**Protocol**: Giao thức

Giá trị: `http`, `grpc`, `event`, `cli`, `file`, `in_process`, `db_view`

Bí danh chuẩn hoá: "http" → `http`; "https" → `http`; "rest*" → `http`; "openapi 3.1" → `http`; "grpc" → `grpc`; "go interface" → `in_process`; "typescript" → `in_process`; "go port" → `in_process`; "go" → `in_process`; "postgresql view" → `db_view`; "view" → `db_view`; "cli" → `cli`

**ConcurrencyControl**: 

Giá trị: `none`, `etag_if_match`, `row_version`, `optimistic_lock`, `pessimistic_lock`

**EventChannel**: 

Giá trị: `outbox`, `listen_notify`, `in_process`, `webhook`, `queue`, `public`

Bí danh chuẩn hoá: "outbox" → `outbox`; "listen" → `listen_notify`; "internal" → `in_process`; "public" → `public`

**DeliverySemantics**: 

Giá trị: `at_least_once`, `at_most_once`, `exactly_once`, `best_effort`

Bí danh chuẩn hoá: "ít nhất một lần" → `at_least_once`; "có thể mất" → `best_effort`

**PrincipalType**: 

Giá trị: `user`, `api_token`, `client`, `service`, `any`

**CredentialKind**: 

Giá trị: `api_token`, `jwt`, `session_cookie`, `refresh_token`, `client_secret`, `signing_key`, `encryption_key`, `password`, `cookie`

**RetryMode**: 

Giá trị: `none`, `connect_only`, `read`, `all`

**RetentionAction**: 

Giá trị: `delete`, `summarize`, `archive`, `anonymize`

**EntityLevel**: 

Giá trị: `logical`, `physical`

**DataClassVocab**: 

Giá trị: `public`, `internal`, `confidential`, `restricted`

**JobTrigger**: 

Giá trị: `schedule`, `event`, `manual`, `on_demand`

**HitPolicy**: 

Giá trị: `unique`, `first`, `priority`, `any`, `collect`

**ConditionKind**: 

Giá trị: `precondition`, `postcondition`, `guard`, `invariant`, `trigger`

**MetricType**: 

Giá trị: `counter`, `gauge`, `histogram`, `summary`

**ControlKind**: 

Giá trị: `preventive`, `detective`, `corrective`

**RiskLevel**: 

Giá trị: `low`, `medium`, `high`

Bí danh chuẩn hoá: "thấp" → `low`; "trung bình" → `medium`; "cao" → `high`

**RiskTreatment**: 

Giá trị: `mitigate`, `accept`, `transfer`, `avoid`

Bí danh chuẩn hoá: "chấp nhận" → `accept`; "giảm thiểu" → `mitigate`; "giảm" → `mitigate`; "chuyển giao" → `transfer`; "tránh" → `avoid`

**ConstraintKind**: 

Giá trị: `technical`, `business`, `legal`, `performance`, `data`, `security`, `operational`

**ConstantKind**: 

Giá trị: `limit`, `rate`, `timeout`, `retention`, `size`, `threshold`, `weight`, `default`

**TestLevel**: Mức kiểm thử

Giá trị: `unit`, `integration`, `contract`, `security`, `non_functional`, `e2e`, `poc_measurement`, `acceptance`

Bí danh chuẩn hoá: "đơn vị" → `unit`; "đơn vị (thuộc tính)" → `unit`; "unit" → `unit`; "tích hợp" → `integration`; "integration" → `integration`; "hợp đồng" → `contract`; "contract" → `contract`; "bảo mật" → `security`; "security" → `security`; "phi chức năng" → `non_functional`; "non-functional" → `non_functional`; "non_functional" → `non_functional`; "e2e" → `e2e`; "đầu cuối" → `e2e`; "phép đo poc-01" → `poc_measurement`; "phép đo poc" → `poc_measurement`; "chấp nhận" → `acceptance`

**TestKind**: 

Giá trị: `functional`, `performance`, `security`, `resilience`, `usability`, `compatibility`

**Automation**: 

Giá trị: `automated`, `semi`, `manual`

**ChangeSource**: Nguồn của yêu cầu thay đổi

Giá trị: `upstream_change`, `new_requirement`, `defect`, `review_comment`, `decision`, `clarification`, `refactor`

**ChangeStatus**: Vòng đời yêu cầu thay đổi và bộ thay đổi

Giá trị: `submitted`, `analysed`, `approved`, `applied`, `rejected`, `withdrawn`

**ChangeOpKind**: Loại thao tác thay đổi

Giá trị: `add`, `modify`, `remove`, `rename`, `reclassify`, `supersede`, `merge`, `link`, `unlink`

**CascadePolicy**: Cách xử lý phần tử còn bị tham chiếu khi xoá: chặn, xoá lan, cắt liên kết, chuyển tham chiếu

Giá trị: `restrict`, `cascade`, `detach`, `redirect`

**ImpactKind**: Mức tác động: broken = vi phạm ràng buộc; stale = giá trị suy ra đã cũ; needs_review = cần duyệt lại; check = nên kiểm tra

Giá trị: `broken`, `stale`, `needs_review`, `check`

**RenderMode**: Dạng trình bày của một mục

Giá trị: `table`, `list`, `prose`, `code`, `mixed`

## Phụ lục D. Sổ đơn vị

| Mã | Đại lượng | Hệ số về đơn vị gốc | Bí danh |
|---|---|---|---|
| s | time | 1.0 | s, giây, sec, second, seconds |
| ms | time | 0.001 | ms, mili giây, milisecond |
| min | time | 60.0 | min, phút, minute, minutes |
| h | time | 3600.0 | h, giờ, hour, hours, giờ việt nam |
| d | time | 86400.0 | d, ngày, day, days |
| mo | time | 2592000.0 | tháng, month, months |
| B | size | 1.0 | b, byte, bytes |
| KB | size | 1000.0 | kb |
| MB | size | 1000000.0 | mb |
| GB | size | 1000000000.0 | gb |
| KiB | size | 1024.0 | kib |
| MiB | size | 1048576.0 | mib |
| GiB | size | 1073741824.0 | gib |
| % | ratio | 0.01 | %, phần trăm, percent |
| count | count | 1.0 | lần, dòng, bản ghi, component, kết nối, connection, connections |
| per_min | rate | 0.016666666666666666 | lần/phút, dòng/phút, req/min, /phút, yêu cầu/phút, lần/phút/danh tính |
| per_s | rate | 1.0 | dòng/giây, req/s, lần/giây |

## Phụ lục E. Danh mục rule

| Mã | Nhóm | Tiêu đề | Modality | Mức | Áp dụng khi | Khẳng định | Miễn trừ |
|---|---|---|---|---|---|---|---|
| S-01 | structure | Lớp phải thuộc lược đồ | must | error | luôn | class(e) ∈ CLASSES | không |
| S-02 | structure | Quan hệ đúng miền, miền giá trị, có xét lớp con | must | error | luôn | class(src) ⊑ dom(rel) ∧ class(dst) ⊑ rng(rel) | không |
| S-03 | structure | Quan hệ phải là tên chuẩn, không dùng tên đã loại | must | error | luôn | rel ∉ DEPRECATED | không |
| S-04 | structure | Quan hệ đặc tính acyclic không có chu trình | must | error | luôn | không tồn tại chu trình | không |
| S-05 | structure | Quan hệ n-1 có tối đa một đích mỗi nguồn | must | error | luôn | |out(src,rel)| ≤ 1 | không |
| S-06 | structure | Phần tử có đúng một tài liệu sở hữu | must | error | maturity ≥ specified | |owner_doc| = 1 | không |
| S-07 | structure | Mã phần tử đúng khuôn của lớp và duy nhất toàn kho | must | error | luôn | id khớp pattern(class) ∧ duy nhất | không |
| S-08 | structure | Cạnh tham chiếu phần tử tồn tại | must | error | luôn | dst ∈ Elements (hoặc là tham chiếu ngoài khai báo) | không |
| V-01 | vocabulary | Giá trị enum thuộc từ vựng (sau chuẩn hoá bí danh) | must | error | luôn | norm(v) ∈ values(vocab) | không |
| V-02 | vocabulary | Đơn vị thuộc sổ đơn vị | must | error | có unit | unit ∈ UNITS | không |
| V-03 | vocabulary | Quantity đúng cấu trúc: có value hoặc có slot_status | must | error | luôn | value≠null ∨ slot_status ∈ {tbd,unknown,to_measure,deferred,proposed,assumed} | không |
| V-04 | vocabulary | Quantity số phải có đơn vị khi đại lượng có thứ nguyên | must | warn | value≠null | unit≠null ∨ kind ∈ {count,ratio,flag} | có |
| V-05 | vocabulary | Cùng một đại lượng không dùng hai đơn vị khác nhau trong cùng phạm vi | should | info | luôn | đơn vị nhất quán sau quy đổi | có |
| M-01 | maturity | Thuộc tính bắt buộc (M) phải có từ mức specified | must | error | maturity ≥ specified | ∀p∈M(class): p≠null ∨ p có Gap hợp lệ | có |
| M-02 | maturity | Thuộc tính có điều kiện (C) bắt buộc khi điều kiện đúng | must | error | điều kiện của thuộc tính | p≠null ∨ Gap hợp lệ | có |
| M-03 | maturity | Phần tử mức verified phải có ít nhất một verified_by trỏ đến TestCase | must | warn | maturity ≥ verified | ∃ verified_by | có |
| M-04 | maturity | Phần tử mức baselined không còn Gap mở | must | error | maturity = baselined | gaps(open) = ∅ | không |
| M-05 | maturity | Phần tử phụ thuộc không chín hơn phần tử mà nó dựa vào | should | warn | luôn | maturity(src) ≤ maturity(dst) khi dst là own | có |
| G-01 | gap | Gap đúng cấu trúc: slot, status, lý do | must | error | luôn | slot ∈ props(class) ∧ status ∈ SlotStatus ∧ reason≠null | không |
| G-02 | gap | Gap tbd/unknown có chủ và hạn | must | warn | status ∈ {tbd,unknown} | owner≠null ∧ due≠null | có |
| G-03 | gap | Giá trị assumed hoặc proposed phải liên kết Assumption hoặc OpenIssue | must | warn | basis ∈ {assumed,proposed} | ∃ assumes→Assumption ∨ ∃ blocked_by→OpenIssue ∨ ∃ decided_by→Decision | có |
| G-04 | gap | OpenIssue có người xử lý, hạn, trạng thái | must | warn | status ≠ closed | owner≠null ∧ due≠null ∧ status≠null | có |
| G-05 | gap | Phần tử deferred có mốc dời đến | must | error | scope_status = deferred | deferred_to≠null | có |
| G-06 | gap | Giá trị to_measure có kế hoạch đo (Metric hoặc TestCase) | must | warn | basis = to_measure | ∃ measured_by→Metric ∨ ∃ verified_by→TestCase | có |
| G-07 | gap | OpenIssue quá hạn | should | warn | due < hôm nay ∧ status≠closed | false | có |
| G-08 | gap | Quyết định phải đóng OpenIssue mà nó nêu | should | info | luôn | ∀ resolves→OpenIssue: status=resolved | có |
| G-09 | gap | Khuyết khai báo có chủ đích không được trùng với thiếu sót phát hiện | may | info | luôn | intent ∈ {by_design, detected}; detected phải xử lý trước baselined | có |
| T-01 | trace | Requirement được ít nhất một phần tử thiết kế thoả mãn | must | warn | scope_status=in_scope | ∃ satisfies | có |
| T-02 | trace | Requirement P0 có ít nhất một TestCase | must | warn | priority=P0 | ∃ verified_by→TestCase (trực tiếp hoặc qua thiết kế) | có |
| T-03 | trace | Phần tử thiết kế truy được lên nguồn thượng nguồn | must | warn | origin=own | ∃ derives_from ∨ satisfies ∨ realizes tới UPSTREAM_REQ | có |
| T-04 | trace | WorkItem trỏ tới phần tử thiết kế mà nó hiện thực | must | warn | luôn | ∃ implements | có |
| T-05 | trace | TestCase kiểm chứng ít nhất một phần tử | must | warn | luôn | ∃ verifies (out-edge của verified_by đảo) | có |
| T-06 | trace | ErrorCode được dùng ít nhất một nơi | should | info | luôn | ∃ in-edge may_fail_with ∨ raises | có |
| T-07 | trace | Entity được hiện thực bởi lược đồ lưu trữ | must | warn | origin=own | ∃ realized_in | có |
| T-08 | trace | Budget nối Slo hoặc QualityTarget | should | info | luôn | ∃ derives_from→QualityTarget ∨ ∃ in-edge targets/measured_by from Slo | có |
| L-01 | logic | Bảng quyết định đầy đủ: mọi tổ hợp đầu vào có luật phủ | must | error | hit_policy đã khai báo và đầu vào là rời rạc | phủ toàn bộ tích Descartes miền đầu vào, hoặc có luật mặc định | có |
| L-02 | logic | Bảng quyết định loại trừ: không hai luật cùng khớp (hit_policy=unique) | must | error | hit_policy=unique | mọi cặp luật có điều kiện rời nhau | có |
| L-03 | logic | Bảng quyết định có thứ tự (first) có luật mặc định cuối | must | warn | hit_policy=first | luật cuối có điều kiện hằng đúng | có |
| L-04 | logic | Mô hình trạng thái có đúng một trạng thái đầu | must | error | maturity ≥ specified | initial_state≠null ∧ initial_state ∈ has_state | có |
| L-05 | logic | Chuyển trạng thái có nguồn và đích thuộc cùng mô hình | must | error | luôn | from,to ∈ states(machine) | có |
| L-06 | logic | Mọi trạng thái đạt tới được từ trạng thái đầu | must | warn | luôn | reachable(initial) = states | có |
| L-07 | logic | Trạng thái cuối không có chuyển đi; trạng thái không cuối có lối ra | must | warn | luôn | terminal ⇒ out=∅; ¬terminal ⇒ out≠∅ | có |
| L-08 | logic | Chuyển trạng thái có sự kiện kích hoạt, tuỳ chọn guard và action | must | warn | maturity ≥ specified | trigger≠null ∨ ∃ triggered_by | có |
| L-09 | logic | Thuật toán có điều kiện trước, điều kiện sau, đầu vào, đầu ra | must | warn | maturity ≥ specified | inputs,outputs≠null ∧ ∃ has_condition kind=pre ∧ ∃ has_condition kind=post ∨ Gap | có |
| L-10 | logic | Công thức khai báo biến, đơn vị đầu ra và miền hợp lệ | must | warn | maturity ≥ specified | variables≠null ∧ result_unit≠null ∨ Gap | có |
| L-11 | logic | Thao tác ghi khai báo idempotency và điều khiển đồng thời | must | warn | method ∈ {POST,PUT,PATCH,DELETE} | idempotent≠null ∧ concurrency_control≠null ∨ Gap | có |
| L-12 | logic | Invariant của Entity tham chiếu được thuộc tính có thật | must | error | luôn | vars(expr) ⊆ attrs(entity) | có |
| L-13 | logic | Rule có modality và mức nghiêm trọng | must | warn | luôn | modality≠null ∧ severity≠null | có |
| L-14 | logic | Rule xung đột phải có quyết định phân xử | must | warn | luôn | ∃ decided_by→Decision | có |
| K-01 | constant | Tên hằng số duy nhất trong phạm vi | must | error | luôn | (scope,key) duy nhất | không |
| K-02 | constant | Hằng số có kiểu, giá trị mặc định hoặc Gap, đơn vị khi có thứ nguyên | must | warn | maturity ≥ specified | type≠null ∧ (default≠null ∨ Gap) | có |
| K-03 | constant | Giá trị trong khoảng min..max đã khai báo | must | error | có min hoặc max | min ≤ default ≤ max (sau quy đổi đơn vị) | có |
| K-04 | constant | Hằng số có người dùng (configures hoặc constrained_by) | should | info | luôn | ∃ in-edge | có |
| K-05 | constant | Bí mật không có giá trị mặc định trong tài liệu | must | error | secret=true | default=null | không |
| K-06 | constant | Cùng tham số khác giá trị giữa các tài liệu phải có Decision | must | warn | luôn | giá trị trùng key giữa TDD khớp nhau ∨ ∃ Decision | có |
| C-01 | change | Xoá phần tử còn bị tham chiếu phải có chính sách xử lý | must | error | luôn | cascade ∈ {cascade,detach,redirect} ∨ mọi cạnh vào đều bị xoá trong cùng ChangeSet | không |
| C-02 | change | Đích của thao tác phải tồn tại (trừ add) | must | error | op ≠ add | target ∈ Elements ∪ Edges | không |
| C-03 | change | Phần tử thêm mới có mã chưa dùng, lớp hợp lệ, cạnh đúng miền và miền giá trị | must | error | luôn | id ∉ Elements ∧ class ∈ CLASSES ∧ S-02 | không |
| C-04 | change | Thao tác sửa mang giá trị trước khớp trạng thái hiện hành (chống ghi đè) | must | error | luôn | before = giá trị hiện tại | không |
| C-05 | change | Áp dụng bộ thay đổi không tạo thêm vi phạm mức error | must | error | luôn | failures_error(after) ⊆ failures_error(before) | không |
| C-06 | change | Bộ thay đổi chỉ áp dụng khi yêu cầu đã duyệt | must | error | status = applied | request.status = approved ∧ decision_by≠null | không |
| C-07 | change | Phần tử bị tác động phải được đánh dấu cần duyệt lại | must | warn | status = applied | ∀ impacted(needs_review|stale|broken): review_status = needs_review | có |
| C-08 | change | Phần tử baselined chỉ đổi qua bộ thay đổi đã duyệt, và hạ mức chín muồi | must | error | target.maturity = baselined | maturity(after) < baselined cho đến khi duyệt lại | không |
| C-09 | change | Bộ thay đổi áp dụng nguyên khối: một thao tác lỗi thì không thao tác nào được áp dụng | must | error | luôn | atomic | không |
| P-01 | present | Tài liệu có mẫu và mọi mục bắt buộc của mẫu xuất hiện | must | warn | luôn | conforms_to ∧ ∀ SectionSpec.required ⇒ ∃ Section | có |
| P-02 | present | Phần tử được định nghĩa trong mục thuộc lớp mục đó cho phép | should | info | có SectionSpec | class ∈ SectionSpec.holds | có |
| P-03 | present | Khối nguồn hoặc gắn với phần tử, hoặc khai báo prose_only | must | warn | luôn | ∃ binds ∨ prose_only | có |
| P-04 | present | Dựng lại từ cấu trúc đạt ngưỡng độ phủ từng mục | should | warn | luôn | recall ≥ min_recall | có |
| P-05 | present | Dựng lại không mất mát: nối các khối theo seq trả đúng bản gốc | must | error | luôn | hash(render_lossless) = hash(source) | không |
| X-01 | xdoc | Cùng khoá, khác giá trị giữa các tài liệu phải có Decision hoặc Deviation | must | warn | luôn | giá trị đồng nhất ∨ ∃ decided_by | có |
| X-02 | xdoc | Cùng thuộc tính của một thực thể, khác kiểu hoặc khả rỗng giữa các tài liệu | must | warn | luôn | datatype, nullable đồng nhất | có |
| X-03 | xdoc | Phần tử suy ra từ thượng nguồn đã đổi phiên bản bị đánh dấu cũ | should | warn | derived_from_version≠null | version(upstream) = derived_from_version | có |
| X-04 | xdoc | Phần tử trùng danh tính (cùng lớp, tên chuẩn hoá) phải gộp hoặc khai báo khác nhau | must | warn | luôn | ∄ cặp trùng chưa gộp | có |
| X-05 | xdoc | TDD khác thượng nguồn phải có Deviation | must | warn | origin=own ∧ có thượng nguồn | giá trị khớp ∨ ∃ deviates_from | có |
