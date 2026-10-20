# Đề xuất luồng agent sinh TDD từ tài liệu thượng nguồn (v0.1)

> Bản lịch sử. Luồng hiện hành được đề xuất tại [v0.2](TDD_Agent_Harness_Proposal_v0.2.md), thống nhất nguồn động và bổ sung readiness cho tech spec.

Phạm vi: agent nhận URD, SRS, SAD và tài liệu bổ sung, sinh bộ TDD. Thiết kế dựa trên chuỗi công cụ đã chạy thật ở bước số hoá (trích xác định, trích bằng mô hình có kiểm chứng, gộp, bộ luật, mẫu kiểm độc lập) và ontology khung v0.2. Mọi con số chi phí dưới đây suy ra từ các lượt trích đã chạy; **chi phí sinh TDD chưa đo**. Ánh xạ sang tiêu chuẩn (ISO/IEC/IEEE 1016, 42010, 29148) lấy từ hiểu biết chung, chưa đối chiếu lại văn bản tiêu chuẩn.

## 1. Nguyên tắc thiết kế

| # | Nguyên tắc | Lý do rút ra từ thực tế |
|---|---|---|
| 1 | **Sinh dữ liệu có cấu trúc trước, văn bản sau.** Agent tạo phần tử và quan hệ theo ontology, kiểm chứng xong mới kết xuất ra Word hoặc Markdown. | Tính nhất quán giữa 13 TDD khó đạt khi chỉ sinh văn xuôi; ở bộ TDD hiện có, các lỗi mã không thống nhất (5 kiểu mã ca kiểm thử, 3 kiểu mã vấn đề mở, WI-nn-nn so với WI-TDDnn-nn) đều là lỗi do sinh độc lập. |
| 2 | **Mọi phần tử sinh ra phải có gốc.** Mỗi phần tử có cạnh `derives_from` hoặc `satisfies` tới phần tử thượng nguồn, hoặc được ghi là giả định kèm vấn đề mở. | Ở TDD hiện có, 69/352 đầu việc nối được tới thiết kế và 44/118 yêu cầu nối được tới ca kiểm thử, vì truy vết không được ép lúc sinh. |
| 3 | **Hạt nhân dùng chung trước, TDD sau.** Sổ định danh, sơ đồ mã lỗi, hằng số, danh mục sự kiện, scope, tiền tố chỉ số được đóng băng trước khi sinh từng TDD. | Hằng số lệch (`evidence_max_bytes` 20 MB so với 25 MB), chỉ số định nghĩa ở hai TDD, M-03 và M-04 đọc lẫn nhau. |
| 4 | **Xác định trước, mô hình sau.** Bảng, mã, số liệu, hợp đồng máy đọc được lấy bằng mã chương trình; mô hình chỉ làm phần ngữ nghĩa. | Pilot regex đạt 42% khối có mã, độ chính xác định nghĩa 25/25 mẫu; phần mô hình đạt 80% đúng hoàn toàn trên mẫu kiểm. |
| 5 | **Mọi lần ghi đi qua cổng kiểm chứng.** Agent không ghi thẳng vào kho; công cụ ghi từ chối bản ghi sai lược đồ, sai miền/đích, thiếu nguồn. | Công cụ `extcheck` đã chặn mọi trích dẫn bịa (0 trích dẫn sai chữ lọt qua); lỗi còn lại chỉ là sai ngữ nghĩa (2%). |
| 6 | **Chạy lại và tiếp tục được.** Mỗi đơn vị việc có sổ trạng thái; hỏng giữa chừng thì làm tiếp đúng phần dở. | 12 agent song song chạm giới hạn phiên; hai agent phải tự phát hiện điểm tiếp tục sai vì tôi tính tiến độ bằng "khối lớn nhất đã xử lý". |

## 2. Đầu vào: hai chế độ, một kho thượng nguồn

```
Word (theo ISO)  ──► Bộ nhập (docx → khối có neo) ──► Trích xác định + trích bằng mô hình theo hồ sơ loại tài liệu ──┐
                                                                                                                  ├─► Kho tri thức thượng nguồn (UKG)
Tri thức gốc (KG, đã đúng ontology loại tài liệu) ─────────────────────────────────────────────────────────────────┘
Hợp đồng máy đọc được (OpenAPI, DDL, YAML cấu hình) ──► nạp trực tiếp, không qua mô hình ──────────────────────────► UKG
```

Cả hai chế độ ra cùng một giao diện: phần tử + quan hệ + nguồn (`tài liệu, phiên bản, vị trí, quote_hash`). Agent sinh TDD chỉ đọc UKG, không đọc Word. Khác biệt duy nhất: dữ liệu từ Word mang `extracted_by=llm` và độ tin cậy; dữ liệu từ KG gốc mang `extracted_by=human` hoặc `rule`.

### 2.1. Hồ sơ theo loại tài liệu (đề xuất)

Mỗi loại tài liệu là một hồ sơ (cùng khuôn `profile.yaml` đã có): mẫu mã, lớp sở hữu, quan hệ bắt buộc, mẫu mục. Tập lớp lấy từ ontology lõi.

| Loại | Lớp sở hữu | Quan hệ bắt buộc | Kiểm tra riêng (ví dụ) |
|---|---|---|---|
| BRD, PRD | `Stakeholder`, `Concern`, `Milestone` (phạm vi POC), `PriorityLevel`, `Constraint` | mục tiêu → ưu tiên → mốc | Mỗi mục tiêu POC có tiêu chí đo |
| URD | `Scenario` (UC), `BusinessRule` (BRL), `Requirement` (cấp người dùng), `Term` | UC → BRL, UC → yêu cầu | Mỗi UC có tác nhân và điều kiện kết thúc |
| SRS | `Requirement` (FR, NFR), `Operation` (API), `Entity` (ENT), `StateMachine` (SM), `Scope`, `ErrorCode`, `QualityTarget` | API → FR/UC/BRL; API → scope; ENT → trạng thái | Mỗi API có phương thức, đường dẫn, quyền; mỗi NFR có số đo |
| SAD | `Module` (M), `Component` (SVC), `Decision` (ADR), `Constraint` (CT), `Dependency`, `Layer`, `DeploymentUnit`, kịch bản chất lượng (QA) | M → ENT, M → API nhóm; ADR → phương án | Đồ thị phụ thuộc mô-đun phải không chu trình (luật R-47) |
| Phụ lục thuật toán | `Component` (loại thuật toán, ALG) | ALG → yêu cầu, ALG → đầu vào/ra | Mỗi ALG có đầu vào, đầu ra, độ phức tạp |
| Hợp đồng máy đọc được | `Artefact`, `Operation`, `Entity`, `Attribute` | `realized_in` | So khớp OpenAPI với SRS (R-23) |
| Danh mục công nghệ và chính sách | `Standard`, `Dependency`, `Control` | `cites`, `constrained_by` | Phụ thuộc ngoài phải nằm trong danh mục được duyệt |

Tài liệu đầu vào bổ sung nên có: danh mục công nghệ được duyệt (với dự án cần kiểm giấy phép và CVE thì đây là đầu vào bắt buộc), tiêu chuẩn mã hoá và quy ước đặt tên của tổ chức, mẫu TDD của tổ chức nếu có.

## 3. Luồng chạy (8 cổng)

```
G0 Nhập ─► G1 Cổng thượng nguồn ─► G2 Lập kế hoạch TDD ─► G3 Hạt nhân dùng chung ─► G4 Soạn từng TDD ─► G5 Kiểm liên TDD ─► G6 Kết xuất ─► G7 Duyệt ─► G8 Đường cơ sở
                     │ lỗi            │                                              ▲   │ sửa                                      │ kiểm vòng tròn
                     └─► trả về người sở hữu tài liệu                                  └───┴─ Vòng sửa (tối đa N lần)               └─► trích lại và so với ý định
```

| Cổng | Việc | Tác nhân | Điều kiện qua cổng |
|---|---|---|---|
| G0 Nhập | Chuyển Word thành khối có neo, trích xác định, trích bằng mô hình, kiểm chứng, gộp (đã có: `det.py`, `extcheck.py`, `merge.py`). Nạp OpenAPI, DDL | Bộ nhập, Bộ trích | 0 lỗi kiểm chứng; độ phủ khối ≥ ngưỡng; mẫu kiểm đạt |
| G1 Cổng thượng nguồn | Kiểm **chính tài liệu đầu vào** trước khi dùng: SRS khớp SAD (đường dẫn, scope, mô-đun), số yêu cầu mồ côi, chu trình mô-đun, mã trùng | Bộ kiểm (xác định) + Giám khảo (mô hình) | Lỗi mức `error` phải có quyết định của người (sửa, miễn trừ bằng vấn đề mở). Lý do: lỗi thượng nguồn lan xuống mọi TDD (ví dụ OQ-01-01: đường dẫn SAD khác SRS; chu trình M-03 và M-04 nằm sẵn trong SAD) |
| G2 Lập kế hoạch | Từ SAD chia mô-đun thành danh sách TDD, bản đồ sở hữu (phần tử → TDD chủ), ma trận phụ thuộc, ưu tiên theo mục tiêu POC, thứ tự soạn | Bộ lập kế hoạch | Mỗi phần tử thượng nguồn có đúng một TDD chủ (R-41); đồ thị phụ thuộc không chu trình; bao đóng ưu tiên (R-48) |
| G3 Hạt nhân dùng chung | Sinh và đóng băng: sổ định danh (quy ước mã cho WI, TC, OI, lỗi), sơ đồ mã lỗi theo mô-đun, bảng hằng số (`limits.*`), danh mục sự kiện, scope, quy tắc đặt tên chỉ số, thuật ngữ | Bộ hạt nhân + người duyệt | Người duyệt phê chuẩn; sau đó chỉ đọc |
| G4 Soạn TDD | Với từng TDD: lấy gói ngữ cảnh, sinh phần tử và quan hệ theo bản mẫu mục, qua cổng ghi; vòng sửa khi luật báo lỗi | Bộ soạn (mỗi TDD một tác tử, song song có giới hạn) | Luật cấp TDD đạt: R-10..R-17, R-30, R-50, R-60..R-63, độ phủ truy vết đạt ngưỡng |
| G5 Kiểm liên TDD | Luật cấp bộ: sở hữu duy nhất, hằng số, chu trình, ưu tiên, phần tử trùng, tham chiếu treo | Bộ kiểm | 0 `error`; `warning` có xử lý |
| G6 Kết xuất | Từ kho sinh Word (hoặc Markdown) theo bản mẫu; văn xuôi do mô hình viết **từ phần tử đã duyệt**, không thêm sự kiện mới. Kiểm vòng tròn: trích lại văn bản vừa sinh, so với kho; lệch là lỗi | Bộ kết xuất, Bộ trích | Độ lệch vòng tròn dưới ngưỡng |
| G7 Duyệt | Hàng đợi duyệt: cạnh `needs_review`, cảnh báo, miễn trừ; duyệt mẫu theo tỉ lệ cấu hình | Người | Chuyển trạng thái `approved` |
| G8 Đường cơ sở | Ghim phiên bản thượng nguồn, băm nội dung, công bố; khi thượng nguồn đổi chạy truy vấn tác động và chỉ soạn lại phần bị ảnh hưởng | Bộ quản lý thay đổi | Đường cơ sở được ký |

## 4. Vai trò agent

| Agent | Đầu vào | Đầu ra | Công cụ chính | Song song | Ghi chú |
|---|---|---|---|---|---|
| Bộ nhập, Bộ trích | Word, KG gốc, hợp đồng máy đọc được | UKG có nguồn | `docx_to_blocks`, `extract_det`, `extract_llm`, `check_ext` | theo tài liệu | Đã có bản chạy được |
| Bộ kiểm thượng nguồn | UKG | báo cáo G1 | `run_rules(scope=upstream)` | không | Xác định là chính |
| Bộ lập kế hoạch | UKG (SAD, SRS) | kế hoạch TDD, bản đồ sở hữu, thứ tự | `kg_query`, `alloc_owner`, `check_graph` | không | Người duyệt kế hoạch |
| Bộ hạt nhân | kế hoạch, hồ sơ dự án | hạt nhân đóng băng | `registry_*`, `propose_constant` | không | Người duyệt |
| Bộ soạn (n bản) | gói ngữ cảnh, bản mẫu mục, hạt nhân | phần tử TDD qua cổng ghi | `get_context_pack`, `propose_element`, `propose_relation`, `run_rules(scope=tdd)` | có giới hạn (mục 6) | Mỗi bản chỉ ghi vào TDD của mình |
| Giám khảo | mẫu phần tử + khối nguồn | điểm đúng, một phần, sai | chỉ đọc | có | Độc lập với bộ soạn, không thấy lý do của bộ soạn |
| Bộ sửa | danh sách phát hiện | bản ghi sửa | `propose_*` | theo TDD | Tối đa N vòng, hết vòng thì chuyển người |
| Bộ kết xuất | phần tử đã duyệt, bản mẫu | Word hoặc Markdown | `render_docx`, `render_md` | theo TDD | Không được thêm dữ kiện mới |
| Bộ quản lý thay đổi | thượng nguồn đổi | tập TDD và phần tử bị ảnh hưởng | `diff_upstream`, `impact(element)` | không | Dựa `quote_hash` và đồ thị |

## 5. Mặt công cụ (MCP) cho agent

Agent chỉ thấy các hàm sau, không có quyền ghi tệp tuỳ ý vào kho:

| Hàm | Tác dụng | Ghi chú |
|---|---|---|
| `get_context_pack(task_id)` | Trả gói ngữ cảnh đã dựng cho đơn vị việc | Mục 6 |
| `kg_query(sql hoặc sparql)` | Truy vấn chỉ đọc, có giới hạn số dòng | |
| `search_upstream(text)` | Tìm toàn văn trong khối thượng nguồn | Cho trường hợp đồ thị thiếu |
| `alloc_id(class, scope)` | Cấp mã mới từ sổ định danh | Agent không tự đặt mã |
| `propose_element(...)`, `propose_relation(...)` | Ghi có kiểm chứng: lớp, miền/đích, nguồn bắt buộc, mã tồn tại | Từ chối kèm lý do máy đọc được |
| `run_rules(scope)` | Chạy bộ luật, trả phát hiện | |
| `waive(finding, open_issue)` | Miễn trừ có lý do, cần người duyệt | |
| `render(tdd_id, format)` | Kết xuất | Chỉ bộ kết xuất dùng |

Kinh nghiệm: công cụ kiểm chứng trả lỗi có số dòng và lý do ngắn thì agent tự sửa được trong 1 đến 2 vòng (các lượt trích vừa qua đa số đạt 0 lỗi sau vòng sửa đầu).

## 6. Gói ngữ cảnh và ngân sách

Không đưa nguyên tài liệu vào ngữ cảnh. Mỗi đơn vị việc (một mục của một TDD) nhận một gói dựng bằng duyệt đồ thị:

```yaml
task: TDD-05/8.3          # TDD, mục
blueprint: sections/interfaces.yaml
owned_elements: [...]     # phần tử mà kế hoạch giao cho TDD này (mã, tên, thuộc tính)
upstream_slice:           # phần tử thượng nguồn trong 2 bước quan hệ từ owned_elements
  requirements: [...]
  operations: [...]       # kèm phương thức, đường dẫn, scope từ SRS
  decisions: [...]        # ADR, ràng buộc
  quotes: [...]           # trích dẫn nguyên văn các khối thượng nguồn liên quan, có mã khối
kernel:                   # đọc từ hạt nhân đóng băng
  id_scheme, error_scheme, constants, events, metric_prefix
already_written: [...]    # phần tử đã sinh trong TDD này (tránh trùng, dùng lại key)
rules_in_force: [R-10, R-30, ...]
budget: {max_context_tokens: ..., max_output_tokens: ...}
```

Ngân sách và vận hành (rút từ các lượt chạy):
- Giới hạn song song cố định thấp (đề xuất 4 đến 6) kèm chờ lùi dần khi gặp giới hạn tốc độ; lần chạy 12 agent cùng lúc đã chạm giới hạn phiên và mất một phần việc.
- Sổ tiến độ ở mức đơn vị việc (`task_id`, trạng thái, số bản ghi, thời điểm), không suy tiến độ từ vị trí lớn nhất đã xử lý.
- Mỗi agent ghi tệp theo đợt nhỏ; công cụ nối và kiểm tra là bước riêng.

## 7. Bản mẫu mục TDD (blueprint)

Bản mẫu là dữ liệu, không phải câu lệnh rải rác. Ví dụ rút gọn:

```yaml
- id: x.3 Giao diện
  generates: [Interface, Operation, Message, ErrorCode(use)]
  from: [Operation(SRS), Module(SAD)]
  requires_relations: [satisfies, requires_scope, may_fail_with, implemented_by]
  rules: [R-23, R-30, R-55, R-61]
  render: table(Mã, Phương thức và đường dẫn, Ưu tiên, Quyền, Ghi chú thiết kế)
- id: x.4 Mô hình dữ liệu
  generates: [Entity, Attribute, Index, Migration]
  from: [Entity(SRS), Module(SAD)]
  requires_relations: [realizes, has_attribute, references_entity]
  rules: [R-30, R-63]
- id: x.7 Kiểm thử
  generates: [TestCase]
  requires_relations: [tests, verified_by]   # ép: mỗi yêu cầu P0 có ít nhất một ca
  rules: [R-62, R-70]
```

Bộ TDD hiện có dùng cấu trúc x.1 đến x.9; có thể đối chiếu với các quan điểm thiết kế của ISO/IEC/IEEE 1016 (ngữ cảnh, thành phần, phụ thuộc, thông tin, giao diện, cấu trúc, tương tác, trạng thái, thuật toán, tài nguyên) để thêm mục thiếu. Việc đối chiếu cần xác minh với văn bản tiêu chuẩn.

## 8. Cổng chất lượng và ngưỡng

| Cổng | Chỉ số | Ngưỡng đề xuất | Căn cứ |
|---|---|---|---|
| Ghi bản ghi | 0 lỗi lược đồ, miền/đích, quote | bắt buộc | `extcheck` |
| Trích từ Word (G0) | độ chính xác mẫu kiểm | ≥ 90% đúng hoặc đúng một phần ở phần tự chấp nhận | đo được: 99% ở phần tự chấp nhận (92% đúng hoàn toàn) |
| Tự chấp nhận | cạnh conf ≥ 0,9; nút ≥ 0,7 | như mô hình hiện tại | hiệu chỉnh trên 360 mẫu |
| Truy vết | yêu cầu P0 có thiết kế đáp ứng; có ca kiểm thử; đầu việc có gốc | 100% (P0), ≥ 90% (còn lại) | thực tế bộ TDD hiện có: 94%, 37%, 20% |
| Nhất quán | luật liên TDD | 0 `error` | R-10, R-41, R-43, R-47, R-48 |
| Kết xuất | độ lệch vòng tròn | đề xuất ≤ 5% phần tử lệch (cần hiệu chỉnh) | chưa đo |
| Duyệt mẫu | tỉ lệ duyệt mẫu | 10% phần tự chấp nhận | đề xuất, chưa hiệu chỉnh |

## 9. Đánh giá agent bằng TDD hiện có làm đối chứng

Ta đã có bộ TDD v1.0 và đồ thị của nó (6.528 phần tử, 10.672 cạnh). Dùng làm tập đối chứng vàng:

1. Nạp URD, SRS, SAD vào UKG; **không** cho agent thấy TDD hiện có.
2. Cho agent sinh bộ TDD.
3. Đo: độ phủ phần tử (phần tử của TDD hiện có mà bản sinh có tương đương), độ phủ quan hệ, số vi phạm luật trên mỗi 1.000 phần tử, độ phủ truy vết, độ ổn định giữa hai lần chạy.
4. Người chấm mẫu độc lập (như đã làm) cho phần chênh: phần bản sinh có mà bản gốc không có là bổ sung hợp lệ hay bịa.

Hạn chế: bản gốc cũng do mô hình sinh và còn lỗi (xem phát hiện), nên đây là đối chứng tương đối, không phải chân lý tuyệt đối.

## 10. Bài học từ các lượt chạy đã làm, và yêu cầu rút ra cho harness

| Quan sát | Yêu cầu |
|---|---|
| Dải mã ("API-K-32..34", "API-T-15..T-21", "FR-INV-01 đến FR-INV-08") làm công cụ báo sai thiếu phủ | Bộ nhập phải bung dải mã thành cạnh có kiểu `covers` |
| Dòng đầu việc và ca kiểm thử bị bỏ khỏi bước mô hình nên không nối về thành phần | Bộ soạn phải sinh liên kết `implemented_by`, `tests` ngay lúc sinh, không vá sau |
| Các agent báo lược đồ thiếu: lớp cho cảnh báo vận hành, log, trace; `Index` thuộc bảng; `Metric` nối `Job` hay `Component`; `Constraint` không là miền của `verified_by`; thuật toán ALG phải dùng lớp `Element` | Lên ontology v0.3 trước khi sinh |
| 64 nút `Element`, 48 nhóm nút trùng tên | Thêm bước phân giải thực thể (so khớp tên chuẩn hoá, gợi ý gộp, người duyệt) |
| Mã cục bộ trùng giữa các TDD (TH-, RP-, SK-...) | Sổ định danh cấp mã, không để agent tự đặt |
| Ưu tiên của thành phần do mô hình trích được suy từ TDD chứa nó | Ưu tiên là thuộc tính bắt buộc, lấy từ kế hoạch G2 và từ mục tiêu POC, không suy |
| Mẫu kiểm: lỗi chủ yếu là "một phần" (đầu mút không nêu trong khối, quan hệ quá mạnh) | Giám khảo chấm theo hai mức, sửa quan hệ yếu thay vì xoá |
| Trích dẫn nguyên văn được kiểm bằng chương trình chặn hoàn toàn trích dẫn bịa | Giữ nguyên bắt buộc `quote` cho dữ liệu từ Word; với dữ liệu sinh, bắt buộc cạnh tới gốc thượng nguồn thay cho `quote` |

## 11. Ước lượng chi phí và lộ trình

Số đo có sẵn (cộng từ báo cáo của từng agent): mỗi lượt agent trích tốn khoảng 100.000 đến 335.000 token; trích 13 TDD (một lượt chạy riêng TDD-11 và một lượt hoàn thành) cộng ba lượt bổ sung khối sót tốn khoảng 2,8 triệu token; bốn agent chấm mẫu độc lập tốn thêm khoảng 0,5 triệu. Chưa tính phần đã tiêu hao ở lượt chạy đầu bị ngắt do giới hạn phiên. Sinh TDD sẽ khác: ngữ cảnh đọc ít hơn nhờ gói ngữ cảnh, nhưng đầu ra nhiều hơn và có vòng sửa. **Chưa đo.** Đề xuất lấy số đo từ giai đoạn M1 rồi nhân.

| Giai đoạn | Nội dung | Kết quả kiểm được |
|---|---|---|
| M0 | Ontology v0.3, hồ sơ URD, SRS, SAD, bộ nhập cho Word, bung dải mã, phân giải thực thể | UKG của bộ tài liệu hiện có đạt ngưỡng G0; mọi luật G1 chạy được |
| M1 | G2, G3 và G4 cho **một** TDD (ví dụ TDD-09, vì có bản gốc để đối chiếu), có đo token | Số đo mục 9 cho một TDD; ước lượng chi phí thật |
| M2 | Toàn bộ 13 TDD, G5, giới hạn song song và sổ tiến độ | 0 `error` liên TDD; so với tập đối chứng |
| M3 | Kết xuất Word, kiểm vòng tròn | Bản Word đúng bản mẫu; độ lệch vòng tròn đo được |
| M4 | Quản lý thay đổi: đổi thượng nguồn, soạn lại phần bị ảnh hưởng | Truy vấn tác động khớp với soạn lại thật |

## 12. Rủi ro

| Rủi ro | Giảm thiểu |
|---|---|
| Mô hình bịa phần tử không có gốc | Cổng ghi từ chối phần tử không có cạnh tới gốc hoặc giả định; giám khảo độc lập chấm mẫu |
| Văn xuôi kết xuất lệch dữ liệu | Kiểm vòng tròn; bộ kết xuất không có quyền thêm dữ kiện |
| Lỗi thượng nguồn lan xuống mọi TDD | Cổng G1 chặn; quyết định của người ghi bằng vấn đề mở |
| Chi phí và giới hạn tốc độ | Giới hạn song song, sổ tiến độ, ngân sách token theo đơn vị việc, chỉ soạn lại phần bị ảnh hưởng |
| Ontology không đủ cho nội dung thực | Lược đồ có phiên bản; agent báo chỗ thiếu thành vấn đề mở thay vì ép vào lớp sai |
| Phụ thuộc một nhà cung cấp mô hình | Giữ lớp mô hình sau một giao diện; lưu câu lệnh, phiên bản, thời điểm cho mỗi bản ghi (PROV-O) |

## 13. Quyết định cần anh chốt

1. **Kho tri thức:** giữ SQLite cho giai đoạn M0 đến M2 (đủ cho quy mô hiện tại, khoảng 6.500 phần tử mỗi bộ TDD), hay đi thẳng PostgreSQL hoặc cơ sở dữ liệu đồ thị. Đề xuất: SQLite đến hết M2, đổi khi nhiều người cùng duyệt.
2. **Mức tự động của G3 và G7:** hạt nhân dùng chung và duyệt cuối do người phê chuẩn từng mục, hay chấp nhận tự động theo ngưỡng khi mẫu kiểm đạt. Đề xuất: G3 luôn qua người, G7 duyệt mẫu.
3. **Nguồn đầu vào mặc định:** Word hay KG gốc. Đề xuất hỗ trợ cả hai, nhưng chuẩn hoá KG gốc làm đích dài hạn (tác giả viết SRS, SAD trực tiếp theo ontology, Word là bản kết xuất).
4. **Phạm vi tài liệu đầu vào ngoài URD, SRS, SAD:** có đưa danh mục công nghệ được duyệt, tiêu chuẩn mã hoá và mẫu TDD của tổ chức vào bắt buộc không. Đề xuất: có, vì chúng quyết định chất lượng mục "thành phần và phụ thuộc ngoài".
