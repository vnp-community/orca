# Tài liệu thiết kế kỹ thuật (TDD) — TDD-PBOM

> Tài liệu dành cho tech leader, sinh tự động từ dữ liệu đã trích từ các tài liệu nguồn (URD, SRS, SAD) bằng luật, không dùng mô hình ngôn ngữ để viết nội dung. Không có nội dung nào được bổ sung ngoài dữ liệu đã trích: phần nào chưa có thì ghi **THIẾU** kèm lý do.

*Phiên bản 1 · sinh từ bản dữ liệu `snap-000001`*

## Mục lục

- Giới thiệu tài liệu
  - 0.1 Thông tin tài liệu
  - 0.2 Tình trạng dữ liệu theo phần
  - 0.3 Cách đọc
- [1. Phạm vi và nguồn](#ch-1)
  - 1.1 Mục tiêu
  - 1.2 Yêu cầu chức năng
  - 1.3 Phi mục tiêu
  - 1.4 Ràng buộc
  - 1.5 Bối cảnh hệ thống
- [2. Phân rã hệ thống](#ch-2)
  - 2.1 Container và thành phần chạy
  - 2.2 Module của dịch vụ lõi
  - 2.3 Phụ thuộc giữa các thành phần
  - 2.4 Phân bổ triển khai
- [3. Hành vi](#ch-3)
  - 3.1 Vòng đời trạng thái
  - 3.2 Quy tắc nghiệp vụ
  - 3.3 Luồng xử lý, thuật toán, bảng quyết định
- [4. Hợp đồng giao diện](#ch-4)
  - 4.1 Giao diện đã có mô tả
  - 4.2 Giao diện được nhắc tới nhưng chưa có hợp đồng
- [5. Dữ liệu](#ch-5)
  - 5.1 Thực thể
  - 5.2 Thuộc tính của từng thực thể
  - 5.3 Toàn vẹn dữ liệu
  - 5.4 Thông tin dữ liệu còn thiếu
- [6. Giao dịch và khả năng chịu lỗi](#ch-6)
  - 6.1 Mã lỗi
  - 6.2 Chính sách chịu lỗi nêu trong nguồn
- [7. Di trú dữ liệu](#ch-7)
  - 7.1 Di trú nêu trong nguồn
  - 7.2 Thông tin di trú còn thiếu
- [8. Chất lượng và vận hành](#ch-8)
  - 8.1 Chỉ tiêu và ràng buộc chất lượng, vận hành
  - 8.2 Thông tin còn thiếu
- [9. Bảo mật](#ch-9)
  - 9.1 Yêu cầu và ràng buộc bảo mật đã có
  - 9.2 Thông tin còn thiếu
- [10. Quyết định, giả định và thông tin còn thiếu](#ch-10)
  - 10.1 Quyết định kiến trúc (ADR)
  - 10.2 Quyết định nêu trong văn xuôi
  - 10.3 Giả định
  - 10.4 Thông tin còn thiếu trong dữ liệu đã trích
  - 10.5 Mâu thuẫn giữa các nguồn
  - 10.6 Rủi ro
  - 10.7 Tổng hợp mục THIẾU của toàn tài liệu
- [11. Kiểm chứng và truy vết](#ch-11)
  - 11.1 Tiêu chí nghiệm thu gắn yêu cầu
  - 11.2 Test case, kịch bản và nghĩa vụ thiết kế
  - 11.3 Yêu cầu chức năng → phần tử thiết kế
  - 11.4 Yêu cầu phi chức năng → phần tử thiết kế
  - 11.5 Nhóm yêu cầu → thành phần kiến trúc (bảng truy vết của SAD)
  - 11.6 Độ phủ truy vết
- [A. Phụ lục](#ch-A)
  - A.1 Bảng tra mã
  - A.2 Danh sách đầy đủ thông tin còn thiếu
  - A.3 Phần tử chưa xếp vào phần nào

## Giới thiệu tài liệu

### 0.1 Thông tin tài liệu

| Mục | Giá trị |
|---|---|
| Mã tài liệu | TDD-PBOM |
| Phiên bản | 1 |
| Chủ sở hữu, người duyệt, ngày duyệt | **THIẾU** — dữ liệu đã trích không lưu thông tin này |
| Tài liệu nguồn | SAD v1.0; SRS v1.0; URD v1.0 |
| Trạng thái | chưa sẵn sàng (blocked): còn thông tin bắt buộc bị thiếu hoặc chưa rõ |
| Mức chín | 1014 phần tử bản phác / chờ rà soát — chưa được tech lead rà soát |
| Quy mô dữ liệu thiết kế | 1014 phần tử, 1557 quan hệ, 553 chỗ còn thiếu, 0 mâu thuẫn |
| Bản dữ liệu đã dùng để sinh tài liệu | snap-000001 · a5cb4cdc9ccf955ea0577c595ae85b7c28755ee22c19caa0ddcd802013ea9e61 |

### 0.2 Tình trạng dữ liệu theo phần

| Phần | Số mục dữ liệu | Mục THIẾU | Tình trạng |
|---|---|---|---|
| [1. Phạm vi và nguồn](#ch-1) | 100 | 3 | một phần (còn mục THIẾU) |
| [2. Phân rã hệ thống](#ch-2) | 18 | 2 | một phần (còn mục THIẾU) |
| [3. Hành vi](#ch-3) | 111 | 1 | một phần (còn mục THIẾU) |
| [4. Hợp đồng giao diện](#ch-4) | 2 | 1 | một phần (còn mục THIẾU) |
| [5. Dữ liệu](#ch-5) | 68 | 2 | một phần (còn mục THIẾU) |
| [6. Giao dịch và khả năng chịu lỗi](#ch-6) | 38 | 1 | một phần (còn mục THIẾU) |
| [7. Di trú dữ liệu](#ch-7) | 2 | 1 | một phần (còn mục THIẾU) |
| [8. Chất lượng và vận hành](#ch-8) | 38 | 4 | một phần (còn mục THIẾU) |
| [9. Bảo mật](#ch-9) | 15 | 3 | một phần (còn mục THIẾU) |
| [10. Quyết định, giả định và thông tin còn thiếu](#ch-10) | 22 | 2 | một phần (còn mục THIẾU) |
| [11. Kiểm chứng và truy vết](#ch-11) | 18 | 1 | một phần (còn mục THIẾU) |

### 0.3 Cách đọc

- Tài liệu dành cho tech leader, sinh tự động từ dữ liệu đã trích từ các tài liệu nguồn (URD, SRS, SAD) bằng luật, không dùng mô hình ngôn ngữ để viết nội dung. Không có nội dung nào được bổ sung ngoài dữ liệu đã trích: phần nào chưa có thì ghi **THIẾU** kèm lý do.
- Cột **Nguồn** ghi tài liệu và mục của tài liệu đó (ví dụ `SRS §5.1`); nội dung văn xuôi ghi số dòng. Số liệu do luật tính ra ghi 'tổng hợp' hoặc không có cột nguồn.
- Mã như FR-…, BRL-…, ENT-…, NFR-…, ADR-… lấy từ tài liệu nguồn; phụ lục A.1 tra mã. Mã nội bộ của hệ thống sinh dữ liệu không xuất hiện trong tài liệu.
- **THIẾU** = chưa có trong dữ liệu đã trích; không được điền bằng suy đoán. Tổng hợp ở mục 10.7.

<a id="ch-1"></a>
## 1. Phạm vi và nguồn

### 1.1 Mục tiêu

> **[THIẾU] Mục tiêu của hệ thống.** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §1.5 “Stakeholder và mối quan tâm”; SAD §2 “Động lực kiến trúc”; SAD §2.2 “Giả định về tổ chức và hạ tầng”; SAD §2.4 “Động lực được ưu tiên khi xung đột”; SAD §3 “Bối cảnh và phạm vi hệ thống”; SAD §3.1 “Actor”; SAD §3.2 “Hệ thống ngoài”; SAD §3.3 “Sơ đồ bối cảnh”

### 1.2 Yêu cầu chức năng

#### 1.2.1 Quản trị và nền tảng (FR-ADM) (4)

| Mã | Yêu cầu | Nguồn |
|---|---|---|
| <a id="FR-ADM-01"></a>FR-ADM-01 | Hệ thống phải xác thực người dùng bằng SSO OIDC và ánh xạ nhóm IdP sang vai trò hệ thống theo `Policy(type=idp_mapping)`; phiên tối đa 8 giờ, idle 30 phút | SRS §5.1, URD §7.1 |
| <a id="FR-ADM-02"></a>FR-ADM-02 | Hệ thống phải phân quyền theo vai trò hệ thống (SEC, ARB, LEG, PROC, ADM) và vai trò dự án (Viewer, Contributor, Owner); quyền là hợp các vai trò, từ chối theo mặc định, thực thi ở tầng API và truy vấn | SRS §5.1, URD §7.1 |
| <a id="FR-ADM-03"></a>FR-ADM-03 | Hệ thống phải cấp, hiển thị một lần, hết hạn và thu hồi API token theo dự án với scope tối thiểu, chỉ lưu hash; cấp OAuth client cho hệ thống ngoài | SRS §5.1, URD §7.1 |
| <a id="FR-ADM-04"></a>FR-ADM-04 | Hệ thống phải cho SEC cấu hình SLA, gate, EOL, limits và LEG cấu hình ma trận license, asset_rule, retention thành Policy có phiên bản, mô phỏng trước khi kích hoạt, hiệu lực không cần deploy | SRS §5.1, URD §7.1 |

#### 1.2.2 Kiểm kê tech stack (FR-INV) (8)

| Mã | Yêu cầu | Nguồn |
|---|---|---|
| <a id="FR-INV-01"></a>FR-INV-01 | Hệ thống phải tạo, sửa, lưu trữ Project, Environment, Service; gán owner (TL) và SLA profile cho dự án; không xoá vật lý | SRS §5.2, URD §7.2 |
| <a id="FR-INV-02"></a>FR-INV-02 | Hệ thống phải có `POST /api/v1/sbom` nhận CycloneDX ≥ 1.5 hoặc SPDX ≥ 2.3 (JSON) kèm project, service, environment, commit, build_id và trả `snapshot_id`, `evaluation_id` | SRS §5.2, URD §7.2 |
| <a id="FR-INV-03"></a>FR-INV-03 | Hệ thống phải có CLI `pbom` (`scan`, `submit`, `gate` của URD; thêm `credits`, `baseline`, `login`, `whoami` ở Phụ lục B mục 5) sinh SBOM từ manifest, lock file, image và trả exit code theo kết quả | SRS §5.2, URD §7.2 |
| <a id="FR-INV-04"></a>FR-INV-04 | Hệ thống phải cho khai báo tay Usage hạ tầng, nhắc xác nhận lại mỗi 30 ngày, quá hạn chuyển Stale; Usage Stale vẫn được đánh giá rủi ro | SRS §5.2, URD §7.2 |
| <a id="FR-INV-05"></a>FR-INV-05 | Hệ thống phải lưu cây phụ thuộc của snapshot và hiển thị đường phụ thuộc từ trực tiếp tới gián tiếp | SRS §5.2, URD §7.2 |
| <a id="FR-INV-06"></a>FR-INV-06 | Hệ thống phải cho khai báo ngữ cảnh sử dụng (`modified_source`, `distributed_to_third_party`, `network_exposed`, `features_used`) ở cấp Usage hoặc Service và kế thừa cho snapshot sau | SRS §5.2, URD §7.2 |
| <a id="FR-INV-07"></a>FR-INV-07 | Hệ thống nên so sánh hai snapshot: thêm, bớt, đổi phiên bản, đổi license | SRS §5.2, URD §7.2 |
| <a id="FR-INV-08"></a>FR-INV-08 | Hệ thống nên nhập hàng loạt Usage từ Excel hoặc CSV và báo lỗi theo dòng | SRS §5.2, URD §7.2 |

#### 1.2.3 Cơ sở tri thức (FR-KB) (10)

| Mã | Yêu cầu | Nguồn |
|---|---|---|
| <a id="FR-KB-01"></a>FR-KB-01 | Hệ thống phải đồng bộ tăng dần OSV, GitHub Advisory, Go vulndb mỗi 1 giờ qua Connector | SRS §5.3, URD §7.3 |
| <a id="FR-KB-02"></a>FR-KB-02 | Hệ thống phải đồng bộ NVD (CVE và CPE) mỗi 2 giờ, CISA KEV mỗi 6 giờ, EPSS hằng ngày | SRS §5.3, URD §7.3 |
| <a id="FR-KB-03"></a>FR-KB-03 | Hệ thống phải đồng bộ endoflife.date hằng ngày cho slug đã ánh xạ (`eol_slug`) | SRS §5.3, URD §7.3 |
| <a id="FR-KB-04"></a>FR-KB-04 | Hệ thống phải thu thập metadata repo (release gần nhất, archived, license, số release 12 tháng) hằng tuần | SRS §5.3, URD §7.3 |
| <a id="FR-KB-05"></a>FR-KB-05 | Hệ thống phải đăng ký nguồn advisory vendor (RSS, Atom, URL) theo Component; mục mới tạo advisory chưa phân loại cho SEC | SRS §5.3, URD §7.3 |
| <a id="FR-KB-06"></a>FR-KB-06 | Hệ thống phải hợp nhất alias CVE, GHSA, GO-ID, OSV thành một Vulnerability | SRS §5.3, URD §7.3 |
| <a id="FR-KB-07"></a>FR-KB-07 | Hệ thống phải lưu lịch sử đồng bộ và cảnh báo SEC khi một nguồn lỗi quá 24 giờ | SRS §5.3, URD §7.3 |
| <a id="FR-KB-08"></a>FR-KB-08 | Hệ thống phải cho ADM đăng ký, bật/tắt, gỡ phiên bản Connector; chỉ bật sau khi qua kiểm thử conformance (NFR-MA-05) | SRS §2.1 |
| <a id="FR-KB-09"></a>FR-KB-09 | Hệ thống phải cho ADM/SEC tạo DataSource từ Connector: endpoint, tham chiếu bí mật, lịch, priority; thử kết nối trước khi bật; hiển thị health và freshness | SRS §2.1 |
| <a id="FR-KB-10"></a>FR-KB-10 | Hệ thống phải cho SEC chạy đồng bộ thủ công, nhập file thủ công (CSV/JSON/XLSX) cho một DataSource, xem reject report và chạy lại sau khi sửa ánh xạ | SRS §2.1 |

#### 1.2.4 Lỗ hổng bảo mật (FR-VUL) (8)

| Mã | Yêu cầu | Nguồn |
|---|---|---|
| <a id="FR-VUL-01"></a>FR-VUL-01 | Hệ thống phải tự đối chiếu (matching) để tạo hoặc cập nhật Finding khi có Vulnerability mới hoặc Usage mới | SRS §5.4, URD §7.4 |
| <a id="FR-VUL-02"></a>FR-VUL-02 | Hệ thống phải tính Priority từ CVSS, EPSS, KEV, độ quan trọng môi trường, network_exposed, trực tiếp hoặc gián tiếp và hiển thị cả severity gốc | SRS §5.4, URD §7.4 |
| <a id="FR-VUL-03"></a>FR-VUL-03 | Hệ thống phải tính `sla_due_at` theo SLA profile (Critical 48 giờ, High 7 ngày, Medium 30 ngày); bộ đếm chạy từ lúc tạo Finding | SRS §5.4, URD §7.4 |
| <a id="FR-VUL-04"></a>FR-VUL-04 | Hệ thống phải có workflow triage Open, Triaging, Affected hoặc Fixed hoặc Not affected kèm justification VEX chuẩn | SRS §5.4, URD §7.4 |
| <a id="FR-VUL-05"></a>FR-VUL-05 | Hệ thống phải ghi nhớ quyết định Not affected theo (vulnerability, component, ngữ cảnh) và tự mở lại khi ngữ cảnh đổi | SRS §5.4, URD §7.4 |
| <a id="FR-VUL-06"></a>FR-VUL-06 | Hệ thống nên hiển thị fix recommendation: phiên bản vá gần nhất, loại nâng cấp (patch, minor, major), cảnh báo breaking change | SRS §5.4, URD §7.4 |
| <a id="FR-VUL-07"></a>FR-VUL-07 | Hệ thống phải tự đóng Finding (Fixed) khi snapshot mới không còn phiên bản bị ảnh hưởng; lưu `fixed_in_version` và thời gian vá thực tế | SRS §5.4, URD §7.4 |
| <a id="FR-VUL-08"></a>FR-VUL-08 | Hệ thống nên xuất VEX (CycloneDX VEX, OpenVEX) theo dự án | SRS §5.4, URD §7.4 |

#### 1.2.5 Vòng đời và bảo trì (FR-LCM) (6)

| Mã | Yêu cầu | Nguồn |
|---|---|---|
| <a id="FR-LCM-01"></a>FR-LCM-01 | Hệ thống phải hiển thị release line, ngày EOL, EOS và số ngày còn lại cho mỗi Usage | SRS §5.5, URD §7.5 |
| <a id="FR-LCM-02"></a>FR-LCM-02 | Hệ thống phải cảnh báo EOL tại T-180, T-90, T-30, T-0 và hằng tuần sau EOL (cấu hình được) | SRS §5.5, URD §7.5 |
| <a id="FR-LCM-03"></a>FR-LCM-03 | Hệ thống phải cho khai báo EOL tay trong Catalog khi endoflife.date không có | SRS §5.5, URD §7.5 |
| <a id="FR-LCM-04"></a>FR-LCM-04 | Hệ thống nên tính health score (0 đến 100) và cảnh báo khi không có release quá 12 tháng hoặc repo archived | SRS §5.5, URD §7.5 |
| <a id="FR-LCM-05"></a>FR-LCM-05 | Hệ thống nên có version lag rule cấu hình theo component; vi phạm cảnh báo Medium | SRS §5.5, URD §7.5 |
| <a id="FR-LCM-06"></a>FR-LCM-06 | Hệ thống nên đánh dấu thành phần hoặc phiên bản deprecated kèm thay thế đề xuất | SRS §5.5, URD §7.5 |

#### 1.2.6 License phần mềm (FR-LIC) (5)

| Mã | Yêu cầu | Nguồn |
|---|---|---|
| <a id="FR-LIC-01"></a>FR-LIC-01 | Hệ thống phải chuẩn hoá license về SPDX ID; biểu thức OR chọn phương án ít hạn chế nhất; không xác định thì NOASSERTION và nhóm Unknown | SRS §5.6, URD §7.6 |
| <a id="FR-LIC-02"></a>FR-LIC-02 | Hệ thống phải đánh giá theo ma trận (nhóm license, mô hình phân phối, có sửa mã), cấu hình được | SRS §5.6, URD §7.6 |
| <a id="FR-LIC-03"></a>FR-LIC-03 | Hệ thống phải phát hiện license drift giữa các phiên bản, cảnh báo SEC, ARB và mọi dự án đang dùng, đánh dấu phiên bản ranh giới | SRS §5.6, URD §7.6 |
| <a id="FR-LIC-04"></a>FR-LIC-04 | Hệ thống nên đính kèm văn bản điều khoản theo phiên bản (BSL Additional Use Grant, Change Date) và nhắc rà soát | SRS §5.6, URD §7.6 |
| <a id="FR-LIC-05"></a>FR-LIC-05 | Hệ thống nên xuất báo cáo license và NOTICE cho bản phân phối | SRS §5.6, URD §7.6 |

#### 1.2.7 Tech Catalog (FR-CAT) (8)

| Mã | Yêu cầu | Nguồn |
|---|---|---|
| <a id="FR-CAT-01"></a>FR-CAT-01 | Hệ thống phải quản lý CatalogEntry theo layer (ngôn ngữ, framework, CSDL, cache, message bus, gateway, secret, giám sát, base image…) | SRS §5.7, URD §7.7 |
| <a id="FR-CAT-02"></a>FR-CAT-02 | Hệ thống phải có ring Adopt, Trial, Hold, Banned; xem dạng Radar và bảng | SRS §5.7, URD §7.7 |
| <a id="FR-CAT-03"></a>FR-CAT-03 | Hệ thống phải tự hiển thị license, CVE mở theo line, EOL từng line, health score, số dự án dùng; trường biên tập: rationale, alternatives, phiên bản tối thiểu | SRS §5.7, URD §7.7 |
| <a id="FR-CAT-04"></a>FR-CAT-04 | Hệ thống nên gợi ý cập nhật phiên bản tối thiểu khi có bản vá CVE từ High trở lên | SRS §5.7, URD §7.7 |
| <a id="FR-CAT-05"></a>FR-CAT-05 | Hệ thống phải đổi ring và phiên bản tối thiểu qua Change Request do ARB duyệt và lưu audit | SRS §5.7, URD §7.7 |
| <a id="FR-CAT-06"></a>FR-CAT-06 | Hệ thống nên có `next_review_at` (mặc định 90 ngày) và nhắc khi quá hạn | SRS §5.7, URD §7.7 |
| <a id="FR-CAT-07"></a>FR-CAT-07 | Hệ thống nên xuất version-pin baseline (JSON, YAML) theo layer cho CI | SRS §5.7, URD §7.7 |
| <a id="FR-CAT-08"></a>FR-CAT-08 | Hệ thống nên lưu Comparison sheet gắn layer để tái sử dụng | SRS §5.7, URD §7.7 |

#### 1.2.8 Lựa chọn tech stack (FR-SEL) (6)

| Mã | Yêu cầu | Nguồn |
|---|---|---|
| <a id="FR-SEL-01"></a>FR-SEL-01 | Hệ thống phải có wizard tạo đề xuất stack cho dự án mới theo mô hình phân phối và ràng buộc khách hàng | SRS §5.8, URD §7.8 |
| <a id="FR-SEL-02"></a>FR-SEL-02 | Hệ thống nên so sánh 2 đến 4 ứng viên theo tiêu chí chuẩn | SRS §5.8, URD §7.8 |
| <a id="FR-SEL-03"></a>FR-SEL-03 | Hệ thống nên tự đánh dấu ứng viên vi phạm ràng buộc dự án | SRS §5.8, URD §7.8 |
| <a id="FR-SEL-04"></a>FR-SEL-04 | Hệ thống nên xuất Tech Stack Review Report (DOCX, PDF, Markdown) có dấu "valid until" | SRS §5.8, URD §7.8 |
| <a id="FR-SEL-05"></a>FR-SEL-05 | Hệ thống phải chuyển công nghệ ngoài Catalog thành Change Request; production chỉ dùng sau khi ARB duyệt hoặc có Decision ngoại lệ | SRS §5.8, URD §7.8 |
| <a id="FR-SEL-06"></a>FR-SEL-06 | Hệ thống nên chốt đề xuất thành Usage "planned" để giám sát từ trước khi có code | SRS §5.8, URD §7.8 |

#### 1.2.9 Quản trị, thông báo và báo cáo (FR-GOV) (7)

| Mã | Yêu cầu | Nguồn |
|---|---|---|
| <a id="FR-GOV-01"></a>FR-GOV-01 | Hệ thống phải có Decision registry (ADR, Risk acceptance, License exception, Catalog change) với trường bắt buộc và `expires_at` ≤ 12 tháng | SRS §5.9, URD §7.9 |
| <a id="FR-GOV-02"></a>FR-GOV-02 | Hệ thống phải trả đối tượng về trạng thái vi phạm khi Decision hết hạn hoặc thu hồi; nhắc trước 14 ngày | SRS §5.9, URD §7.9 |
| <a id="FR-GOV-03"></a>FR-GOV-03 | Hệ thống phải có audit log chỉ ghi thêm cho mọi thay đổi trạng thái (ai, khi nào, trước, sau) | SRS §5.9, URD §7.9 |
| <a id="FR-GOV-04"></a>FR-GOV-04 | Hệ thống phải thông báo qua email và webhook (Teams, Slack, Telegram); routing theo owner và severity; digest; chống trùng | SRS §5.9, URD §7.9 |
| <a id="FR-GOV-05"></a>FR-GOV-05 | Hệ thống phải nhắc tại 75% SLA và chuyển cấp xử lý tới PM và SEC khi vi phạm SLA | SRS §5.9, URD §7.9 |
| <a id="FR-GOV-06"></a>FR-GOV-06 | Hệ thống phải tự tạo báo cáo hằng tháng (PDF, XLSX) theo dự án và toàn công ty, gửi email | SRS §5.9, URD §7.9 |
| <a id="FR-GOV-07"></a>FR-GOV-07 | Hệ thống nên xuất gói tuân thủ khách hàng: SBOM, VEX, license, tóm tắt CVE và SLA | SRS §5.9, URD §7.9 |

#### 1.2.10 Policy gate CI (FR-CI) (5)

| Mã | Yêu cầu | Nguồn |
|---|---|---|
| <a id="FR-CI-01"></a>FR-CI-01 | Hệ thống phải có `POST /api/v1/evaluate` (hoặc kèm submit) trả pass, warn hoặc fail và danh sách vi phạm có link | SRS §5.10, URD §7.10 |
| <a id="FR-CI-02"></a>FR-CI-02 | Hệ thống phải áp rule fail và warn mặc định theo BRL-40, BRL-41 | SRS §5.10, URD §7.10 |
| <a id="FR-CI-03"></a>FR-CI-03 | Hệ thống nên cấu hình rule theo dự án; ghi đè tạm thời phải có Decision | SRS §5.10, URD §7.10 |
| <a id="FR-CI-04"></a>FR-CI-04 | Hệ thống phải có template pipeline GitLab CI, GitHub Actions, Jenkins | SRS §5.10, URD §7.10 |
| <a id="FR-CI-05"></a>FR-CI-05 | Hệ thống nên import kết quả Trivy, Grype, OSV-Scanner, npm audit như nguồn Finding bổ sung | SRS §5.10, URD §7.10 |

#### 1.2.11 Giao diện (FR-UI) (10)

| Mã | Yêu cầu | Nguồn |
|---|---|---|
| <a id="FR-UI-01"></a>FR-UI-01 | Hệ thống phải có màn hình Company overview cho SEC, ARB: Finding mở, % SLA, top 10 rủi ro, EOL 90 ngày, license vi phạm, dự án chưa nộp SBOM quá 14 ngày | SRS §5.11, URD §7.11 |
| <a id="FR-UI-02"></a>FR-UI-02 | Hệ thống phải có Project detail với tab Stack, Findings, Lifecycle, License, Decisions, Snapshots | SRS §5.11, URD §7.11 |
| <a id="FR-UI-03"></a>FR-UI-03 | Hệ thống phải có Component detail: line, CVE theo phiên bản, EOL, license history, dự án đang dùng, Catalog entry | SRS §5.11, URD §7.11 |
| <a id="FR-UI-04"></a>FR-UI-04 | Hệ thống phải có My work: Finding được giao, hạn SLA, hành động đề xuất | SRS §5.11, URD §7.11 |
| <a id="FR-UI-05"></a>FR-UI-05 | Hệ thống nên có Tech Radar: radar và bảng Catalog | SRS §5.11, URD §7.11 |
| <a id="FR-UI-06"></a>FR-UI-06 | Hệ thống phải có blast radius search "ai đang dùng X phiên bản Y" trả ≤ 3 giây | SRS §5.11, URD §7.11 |
| <a id="FR-UI-07"></a>FR-UI-07 | Hệ thống phải có tab Assets của dự án: danh sách theo loại, nguồn, license, entitlement, finding; lọc "chưa có entitlement", "sắp hết hạn" | SRS §5.11, URD §7.12 |
| <a id="FR-UI-08"></a>FR-UI-08 | Hệ thống phải có màn hình Entitlements cho PROC, SEC: kho license, hạn dùng, dự án dùng, chứng từ | SRS §5.11, URD §7.12 |
| <a id="FR-UI-09"></a>FR-UI-09 | Hệ thống phải có Asset detail: metadata đã đọc, đường dẫn file, dự án dùng, lịch sử khai báo, ảnh preview | SRS §5.11, URD §7.12 |
| <a id="FR-UI-10"></a>FR-UI-10 | Hệ thống nên có Asset Catalog cho DES: tìm và lọc "hỗ trợ tiếng Việt", "dùng được cho app thu phí" | SRS §5.11, URD §7.12 |

#### 1.2.12 Tài sản phi mã nguồn (FR-AST) (19)

| Mã | Yêu cầu | Nguồn |
|---|---|---|
| <a id="FR-AST-01"></a>FR-AST-01 | Hệ thống phải quét font, ảnh, SVG, Lottie, audio, video trong repo và thư mục build; bỏ qua theo `.pbomignore`; xuất khối `assets` trong SBOM hoặc `pbom-assets.json` | SRS §5.12, URD §7.12 |
| <a id="FR-AST-02"></a>FR-AST-02 | Hệ thống phải phát hiện khai báo font gián tiếp (CSS, link Google hay Adobe Fonts, package, pubspec, manifest mobile) | SRS §5.12, URD §7.12 |
| <a id="FR-AST-03"></a>FR-AST-03 | Hệ thống phải đọc metadata font (bảng `name`, `OS/2.achVendID`, `fsType`) | SRS §5.12, URD §7.12 |
| <a id="FR-AST-04"></a>FR-AST-04 | Hệ thống phải nhận diện license font từ nameID 13 và 14; không khớp thì Commercial hoặc Unknown | SRS §5.12, URD §7.12 |
| <a id="FR-AST-05"></a>FR-AST-05 | Hệ thống phải đọc metadata ảnh XMP, IPTC, EXIF và `<metadata>` của SVG | SRS §5.12, URD §7.12 |
| <a id="FR-AST-06"></a>FR-AST-06 | Hệ thống nên nhận diện nguồn stock, marketplace theo tên file và metadata; chặn ảnh preview có watermark | SRS §5.12, URD §7.12 |
| <a id="FR-AST-07"></a>FR-AST-07 | Hệ thống phải cho khai báo tay tài sản không quét được; nhắc xác nhận lại mỗi 90 ngày | SRS §5.12, URD §7.12 |
| <a id="FR-AST-08"></a>FR-AST-08 | Hệ thống nên khử trùng lặp theo SHA-256, gộp biến thể font; pHash chỉ gợi ý | SRS §5.12, URD §7.12 |
| <a id="FR-AST-09"></a>FR-AST-09 | Hệ thống có thể tích hợp Figma REST API để đọc `fontFamily` và thư viện ngoài | SRS §5.12, URD §7.12 |
| <a id="FR-AST-10"></a>FR-AST-10 | Hệ thống phải có thư viện AssetLicenseTerm chuẩn hoá do LEG duyệt; cho thêm điều khoản riêng | SRS §5.12, URD §7.12 |
| <a id="FR-AST-11"></a>FR-AST-11 | Hệ thống phải có rule engine sinh AssetFinding khi nộp scan, khi Entitlement đổi và theo job hằng ngày | SRS §5.12, URD §7.12 |
| <a id="FR-AST-12"></a>FR-AST-12 | Hệ thống phải có kho Entitlement (CRUD, chứng từ, liên kết tài sản, phạm vi, dự án) | SRS §5.12, URD §7.12 |
| <a id="FR-AST-13"></a>FR-AST-13 | Hệ thống phải cảnh báo Entitlement thuê bao tại T-60, T-30, T-7, T-0 | SRS §5.12, URD §7.12 |
| <a id="FR-AST-14"></a>FR-AST-14 | Hệ thống nên đếm và so sánh giới hạn định lượng; vượt 80% cảnh báo, vượt 100% `limit_exceeded` | SRS §5.12, URD §7.12 |
| <a id="FR-AST-15"></a>FR-AST-15 | Hệ thống nên sinh Credits, Attribution (MD, HTML, JSON) và kiểm tra bản phát hành chứa bản mới nhất | SRS §5.12, URD §7.12 |
| <a id="FR-AST-16"></a>FR-AST-16 | Hệ thống nên có Asset Catalog với ring Approved, Conditional, Banned và `next_review_at` | SRS §5.12, URD §7.12 |
| <a id="FR-AST-17"></a>FR-AST-17 | Hệ thống nên sinh handover checklist và xuất PDF, XLSX phụ lục biên bản | SRS §5.12, URD §7.12 |
| <a id="FR-AST-18"></a>FR-AST-18 | Hệ thống phải đưa tài sản vào policy gate chung, tách nhóm "Components" và "Assets" | SRS §5.12, URD §7.12 |
| <a id="FR-AST-19"></a>FR-AST-19 | Hệ thống phải triage AssetFinding: liên kết Entitlement, thay hoặc xoá, false positive, Risk accepted qua Decision | SRS §5.12, URD §7.12 |

#### 1.2.13 Yêu cầu nêu trong văn xuôi (không nằm trong bảng yêu cầu) (1)

| Yêu cầu | Nguồn |
|---|---|
| <a id="FACT-e514b95cdd"></a>Điều kiện xong của use case: Mỗi use case chỉ được coi là xong khi đạt đủ DoD-G bên dưới cộng DoD riêng ở mục UC | URD §11.2 |

### 1.3 Phi mục tiêu

> **[THIẾU] Phi mục tiêu (những gì hệ thống cố ý không làm).** Chưa có phần tử phi mục tiêu trong dữ liệu đã trích, nên tài liệu không tự suy diễn. Cần BA / chủ sản phẩm xác nhận; tài liệu nguồn có đoạn 'ngoài phạm vi' (trích nguyên văn bên dưới, chưa được đưa vào dữ liệu thiết kế). *Cần xác nhận bởi: BA / chủ sản phẩm.*

Trích nguyên văn từ SAD §1.2, dòng 19: Ngoài phạm vi (thuộc TDD): OpenAPI đầy đủ từng endpoint, DDL và chỉ mục vật lý, cấu hình pipeline CI/CD cụ thể, kịch bản kiểm thử chi tiết.

Trích nguyên văn từ SRS §1.2, dòng 25: Ngoài phạm vi v1.0:

- Mô hình đa khách hàng (multi-tenant) — OI-04.
- Nguồn dữ liệu trả phí (Snyk DB, Sonatype) — OI-06; thêm sau bằng connector.
- Tự động sửa mã hoặc mở pull request nâng cấp phiên bản.
- Quét mã nguồn tĩnh (SAST) và quét bí mật; PBOM chỉ nhập kết quả từ công cụ khác.
- Tư vấn pháp lý: dữ liệu điều khoản chỉ là dữ liệu khởi tạo để Legal rà soát (OI-05, OI-08).

### 1.4 Ràng buộc

| Ràng buộc | Nguyên văn nguồn | Nguồn |
|---|---|---|
| <a id="FACT-0be763b5f1"></a>Đề xuất chọn gói ở Draft: Đưa ra ít nhất một đề xuất chọn gói kèm căn cứ (rủi ro, license, EOL); Decision chỉ ở trạng thái Draft | Đưa ra ít nhất một đề xuất chọn gói kèm căn cứ (rủi ro, license, EOL); Decision chỉ ở trạng thái Draft. | SAD §11.3 |
| <a id="FACT-7d24179a65"></a>Truy vết lựa chọn công nghệ: mỗi lựa chọn công nghệ phải truy được về một thuộc tính chất lượng (Mục 2.3) và một quyết định kiến trúc (Mục 4) | mỗi lựa chọn công nghệ phải truy được về một thuộc tính chất lượng (Mục 2.3) và một quyết định kiến trúc (Mục 4) | SAD §1.1 |
| <a id="FACT-b5172a1065"></a>Phiên bản tech stack phải chọn và ghim: Phiên bản không ghi ở đây: phải chọn và ghim sau khi đưa từng ứng viên qua chính PBOM (CVE, license, EOL) | Phiên bản không ghi ở đây: phải chọn và ghim sau khi đưa từng ứng viên qua chính PBOM (CVE, license, EOL). | SAD §11 |

### 1.5 Bối cảnh hệ thống

> **[THIẾU] Actor, hệ thống ngoài, phụ thuộc bên ngoài.** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §1.5 “Stakeholder và mối quan tâm”; SAD §2 “Động lực kiến trúc”; SAD §2.2 “Giả định về tổ chức và hạ tầng”; SAD §2.4 “Động lực được ưu tiên khi xung đột”; SAD §3 “Bối cảnh và phạm vi hệ thống”; SAD §3.1 “Actor”; SAD §3.2 “Hệ thống ngoài”; SAD §3.3 “Sơ đồ bối cảnh”

<a id="ch-2"></a>
## 2. Phân rã hệ thống

### 2.1 Container và thành phần chạy

| Mã | Thành phần | Trách nhiệm, giao tiếp, kho dữ liệu | Nguồn |
|---|---|---|---|
| <a id="CT-01"></a>CT-01 | Web SPA | Giao tiếp: HTTPS tới Gateway<br>Kho dữ liệu: Không<br>Trách nhiệm: Giao diện song ngữ Việt/Anh, WCAG 2.1 AA; không giữ bí mật | SAD §5.1 |
| <a id="CT-02"></a>CT-02 | CLI `pbom` | Giao tiếp: HTTPS tới Gateway với API token<br>Kho dữ liệu: Không<br>Trách nhiệm: Nộp SBOM, gọi gate, quét tài sản, xuất baseline, Credits; một binary Go | SAD §5.1 |
| <a id="CT-03"></a>CT-03 | API Gateway | Giao tiếp: Route tới SVC-01, SVC-02, SVC-03<br>Kho dữ liệu: Không<br>Trách nhiệm: TLS, định tuyến, rate limit, xác minh JWT và API token, ràng buộc thô theo scope và vai trò, CORS, header bảo mật | SAD §5.1 |
| <a id="SVC-01"></a>SVC-01 | Authentication Service | Giao tiếp: HTTPS; gọi IdP; đọc khóa từ kho bí mật<br>Kho dữ liệu: `authn` (ENT-01, ENT-06, session)<br>Trách nhiệm: OIDC Relying Party, cấp và làm mới JWT, quản lý phiên, API token, OAuth2 client credentials, JWKS | SAD §5.1 |
| <a id="SVC-02"></a>SVC-02 | Authorization Service | Giao tiếp: HTTPS; phát sự kiện qua outbox<br>Kho dữ liệu: `authz` (ENT-02)<br>Trách nhiệm: Vai trò, gán vai trò theo dự án và hệ thống, ánh xạ nhóm IdP, phát snapshot và sự kiện đổi quyền, API kiểm tra quyền | SAD §5.1 |
| <a id="SVC-03"></a>SVC-03 | Core API | Giao tiếp: HTTPS tới Gateway; sự kiện qua outbox<br>Kho dữ liệu: `inventory`, `kb`, `risk`, `gate`, `catalog`, `asset`, `gov`<br>Trách nhiệm: 7 module nghiệp vụ (Mục 5.2), thực thi ràng buộc tinh, phục vụ REST `/api/v1` | SAD §5.1 |
| <a id="WRK-01"></a>WRK-01 | Ingest worker | Giao tiếp: Nhận job từ hàng đợi<br>Kho dữ liệu: PostgreSQL, object store<br>Trách nhiệm: Phân tích SBOM, chuẩn hoá purl, cập nhật Usage, tóm tắt snapshot cũ | SAD §5.1 |
| <a id="WRK-02"></a>WRK-02 | Matcher | Giao tiếp: Nhận job; đọc KB<br>Kho dữ liệu: PostgreSQL<br>Trách nhiệm: Matching Usage với Vulnerability (ALG-01), tạo Finding, tính Priority và SLA (ALG-02, ALG-03), re-matching theo lô | SAD §5.1 |
| <a id="WRK-03"></a>WRK-03 | Scheduler | Giao tiếp: Tạo job<br>Kho dữ liệu: PostgreSQL (`job_locks`)<br>Trách nhiệm: Lập lịch đồng bộ connector, job 15 phút (SLA), job hằng ngày (EOL, Decision, Entitlement) trước 06:00 giờ Việt Nam | SAD §5.1 |
| <a id="WRK-04"></a>WRK-04 | Notifier | Giao tiếp: Egress tới kênh ngoài<br>Kho dữ liệu: PostgreSQL<br>Trách nhiệm: Relay outbox, tạo Alert, digest 08:30, gửi email, Teams, Slack, Telegram và webhook, thuật toán thử lại | SAD §5.1 |
| <a id="WRK-05"></a>WRK-05 | Report generator | Giao tiếp: Nhận job<br>Kho dữ liệu: PostgreSQL, object store<br>Trách nhiệm: Sinh PDF, XLSX, CSV, gói tuân thủ, NOTICE, xuất VEX và audit | SAD §5.1 |

### 2.2 Module của dịch vụ lõi

| Mã | Module | Chi tiết (gồm quy tắc phụ thuộc giữa module) | Nguồn |
|---|---|---|---|
| <a id="M-01"></a>M-01 | Inventory (`inventory`) | Nhóm API: API-P, API-I<br>Thực thể sở hữu: ENT-03, ENT-04, ENT-05, ENT-17, ENT-18, ENT-19<br>Trách nhiệm: Dự án, môi trường, service; nhận SBOM; Usage và ngữ cảnh; xác nhận định kỳ<br>(Quy tắc phụ thuộc giữa module)<br>Lý do: Là nền dữ liệu, không phụ thuộc nghiệp vụ<br>Qua sự kiện: Phát sự kiện<br>Từ: M-01, M-02<br>Được gọi đồng bộ: Không gọi module khác | SAD §5.2 |
| <a id="M-02"></a>M-02 | Knowledge Base (`kb`) | Nhóm API: API-K<br>Thực thể sở hữu: ENT-07..16, ENT-20<br>Trách nhiệm: Component, version, release line, Vulnerability, License; connector, data source, import; pipeline ingest, validate, normalise, publish<br>(Ánh xạ thực thể vào schema)<br>Ghi chú: `raw_records` là staging; có hạn dọn<br>Thực thể (SRS Phụ lục A): ENT-07..16, ENT-20 | SAD §5.2 |
| <a id="M-03"></a>M-03 | Risk (`risk`) | Nhóm API: API-V<br>Thực thể sở hữu: ENT-21, ENT-22<br>Trách nhiệm: Finding, VEX, vòng đời và EOL, license drift, điểm sức khoẻ component<br>(Quy tắc phụ thuộc giữa module)<br>Lý do: Tính Priority cần Usage, ngữ cảnh, nguồn KEV, EPSS<br>Qua sự kiện: Nhận Usage mới, KB cập nhật<br>Từ: M-03 Risk<br>Được gọi đồng bộ: M-01, M-02, M-04 (interface đọc)<br>(Nợ kỹ thuật chủ động chấp nhận và điều kiện tách dịch vụ)<br>Hành động: Tách `risk` và matcher thành service riêng; ranh giới đã sẵn<br>Điều kiện: M-03 hoặc WRK-02 cần scale khác hẳn phần còn lại | SAD §5.2 |
| <a id="M-04"></a>M-04 | Gate và Policy (`gate`) | Nhóm API: API-C, API-P (policy)<br>Thực thể sở hữu: ENT-24, ENT-25<br>Trách nhiệm: Policy có phiên bản, đánh giá gate pass, warn, fail; ma trận license<br>(Quy tắc phụ thuộc giữa module)<br>Lý do: Gate cần đọc Usage, KB, Finding, license với độ trễ thấp<br>Qua sự kiện: Nhận cập nhật Policy<br>Từ: M-04 Gate<br>Được gọi đồng bộ: M-01, M-02, M-03 (interface đọc) | SAD §5.2 |
| <a id="M-05"></a>M-05 | Catalog và Selection (`catalog`) | Nhóm API: API-T<br>Thực thể sở hữu: ENT-28..31<br>Trách nhiệm: Radar, Change Request, Baseline, Stack Proposal, tra cứu và so sánh<br>(Quy tắc phụ thuộc giữa module)<br>Lý do: Usage planned khi finalize Stack Proposal<br>Qua sự kiện: Phát sự kiện đổi Catalog<br>Từ: M-05 Catalog<br>Được gọi đồng bộ: M-01, M-02, M-03 (interface đọc) | SAD §5.2 |
| <a id="M-06"></a>M-06 | Asset Compliance (`asset`) | Nhóm API: API-S<br>Thực thể sở hữu: ENT-33..42<br>Trách nhiệm: Tài sản phi mã nguồn, điều khoản, Entitlement, rule, Credits, bàn giao, Figma<br>(Quy tắc phụ thuộc giữa module)<br>Lý do: Ràng buộc dự án, service<br>Qua sự kiện: Phát Finding tài sản<br>Từ: M-06 Asset<br>Được gọi đồng bộ: M-01 (interface đọc) | SAD §5.2 |
| <a id="M-07"></a>M-07 | Governance (`gov`) | Nhóm API: API-G, API-P (hệ thống)<br>Thực thể sở hữu: ENT-23, ENT-26, ENT-27, ENT-32, ENT-43..46<br>Trách nhiệm: Decision và phê duyệt, Alert, thông báo, Report, Evidence, Audit, webhook, event<br>(Quy tắc phụ thuộc giữa module)<br>Lý do: Áp kết quả Decision lên đối tượng mà không phụ thuộc ngược<br>Qua sự kiện: Nhận sự kiện audit, alert<br>Từ: M-07 Governance<br>Được gọi đồng bộ: Mọi module (interface một chiều, kiểu `DecisionApplier`) | SAD §5.2 |

### 2.3 Phụ thuộc giữa các thành phần

> **[THIẾU] Sơ đồ và quan hệ phụ thuộc giữa các thành phần.** Chưa có quan hệ phụ thuộc dạng có cấu trúc; quy tắc phụ thuộc chỉ có dạng chữ trong bảng module (nếu nguồn có).
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §3.3 “Sơ đồ bối cảnh”; SAD §5 “Góc nhìn logic: building block”; SAD §5.1 “Container”; SAD §5.3 “Quy tắc phụ thuộc giữa module”; SAD §5.5 “Sơ đồ container”; SAD §6.7 “Sơ đồ thành phần xác thực và phân quyền”; SAD §9.6 “Sơ đồ triển khai”; SRS §3.2 “Thành phần và phân bổ yêu cầu”

### 2.4 Phân bổ triển khai

> **[THIẾU] Thành phần nào chạy ở đâu (môi trường, mạng, mức triển khai).** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §2.2 “Giả định về tổ chức và hạ tầng”; SAD §9 “Góc nhìn triển khai và hạ tầng”; SAD §9.1 “Hai mức triển khai”; SAD §9.2 “Phân vùng mạng”; SAD §9.3 “Chiến lược scale”; SAD §9.4 “Môi trường và phân phối”; SAD §9.5 “Triển khai không gián đoạn”; SAD §9.6 “Sơ đồ triển khai”

<a id="ch-3"></a>
## 3. Hành vi

### 3.1 Vòng đời trạng thái

41 thực thể có vòng đời trạng thái trong tài liệu nguồn.

<a id="SM-ENT-01"></a>
<a id="SM-ENT-01:Active"></a>
<a id="SM-ENT-01:Disabled"></a>
<a id="SM-ENT-01:Provisioned"></a>
#### 3.1.1 Vòng đời của User (ENT-01)

Trạng thái: Active, Disabled, Provisioned.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-01:_m_i_-_Provisioned:4636e9"></a>(mới) | Provisioned | ADM mời hoặc JIT từ IdP | Email hợp lệ, `idp_subject` duy nhất | Gửi email mời | SRS §8.2 |
| <a id="TR-SM-ENT-01:Active-_Disabled:fbda19"></a>Active | Disabled | ADM vô hiệu hoá hoặc IdP báo Disabled | Hiệu lực ≤ 15 phút (NFR-SE-01) | Thu hồi phiên; RoleAssignment chuyển Revoked | SRS §8.2 |
| <a id="TR-SM-ENT-01:Disabled-_Active:3401ec"></a>Disabled | Active | ADM bật lại | IdP không còn Disabled | Không tự khôi phục vai trò đã Revoked | SRS §8.2 |
| <a id="TR-SM-ENT-01:Provisioned-_Active:3488c9"></a>Provisioned | Active | Đăng nhập OIDC lần đầu | Token IdP hợp lệ | Ghi `last_login_at`; áp ánh xạ nhóm IdP thành RoleAssignment | SRS §8.2 |
| <a id="TR-SM-ENT-01:Provisioned-_Disabled:b5d7a6"></a>Provisioned | Disabled | ADM vô hiệu hoá | - | Như trên | SRS §8.2 |

<a id="SM-ENT-02"></a>
<a id="SM-ENT-02:Active"></a>
<a id="SM-ENT-02:Expired"></a>
<a id="SM-ENT-02:Revoked"></a>
#### 3.1.2 Vòng đời của RoleAssignment (ENT-02)

Trạng thái: Active, Expired, Revoked.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-02:_m_i_-_Active:ae6e98"></a>(mới) | Active | ADM cấp hoặc đồng bộ nhóm IdP | Vai trò khớp `scope` (CHECK); UNIQUE một phần khi Active | Ghi audit | SRS §8.2 |
| <a id="TR-SM-ENT-02:Active-_Expired:6f0074"></a>Active | Expired | Job hằng ngày khi `expires_at` qua | Có `expires_at` | Quyền mất hiệu lực ngay | SRS §8.2 |
| <a id="TR-SM-ENT-02:Active-_Revoked:ef5924"></a>Active | Revoked | ADM thu hồi, hoặc `source=idp_group` mà user rời nhóm | - | Ghi audit; huỷ cache quyền | SRS §8.2 |

<a id="SM-ENT-03"></a>
<a id="SM-ENT-03:Active"></a>
<a id="SM-ENT-03:Archived"></a>
<a id="SM-ENT-03:Handover"></a>
#### 3.1.3 Vòng đời của Project (ENT-03)

Trạng thái: Active, Archived, Handover.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-03:_m_i_-_Active:2013a3"></a>(mới) | Active | Owner hoặc ADM tạo dự án | `code` duy nhất | - | SRS §8.2 |
| <a id="TR-SM-ENT-03:Active-_Handover:36f8c9"></a>Active | Handover | Mở HandoverChecklist (ENT-41 ở Draft) | Quyền Owner hoặc PROC | Khoá gán entitlement mới ngoài checklist | SRS §8.2 |
| <a id="TR-SM-ENT-03:Handover-_Active:44cea7"></a>Handover | Active | ADM huỷ bàn giao | Checklist chưa Closed | Mở lại thao tác ghi | SRS §8.2 |
| <a id="TR-SM-ENT-03:Handover-_Archived:09bb10"></a>Handover | Archived | HandoverChecklist chuyển Closed | BRL-50: mọi mục bắt buộc Done hoặc Waived | Dự án chỉ đọc; ApiToken Active chuyển Revoked; giữ bằng chứng theo Mục 7.6 | SRS §8.2 |

<a id="SM-ENT-04"></a>
<a id="SM-ENT-04:Active"></a>
<a id="SM-ENT-04:Retired"></a>
#### 3.1.4 Vòng đời của Environment (ENT-04)

Trạng thái: Active, Retired.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-04:_m_i_-_Active:b7aef8"></a>(mới) | Active | Owner tạo | Tối đa một `prod` Active mỗi dự án | - | SRS §8.2 |
| <a id="TR-SM-ENT-04:Active-_Retired:f64067"></a>Active | Retired | Owner ngừng dùng | Đóng cùng lúc các Usage còn Active | Usage chuyển Closed; SbomSnapshot Current giữ nguyên để tra cứu | SRS §8.2 |

<a id="SM-ENT-05"></a>
<a id="SM-ENT-05:Active"></a>
<a id="SM-ENT-05:Retired"></a>
#### 3.1.5 Vòng đời của Service (ENT-05)

Trạng thái: Active, Retired.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-05:_m_i_-_Active:25a2f5"></a>(mới) | Active | Owner tạo hoặc tự đăng ký từ SBOM đầu tiên | Project Active | - | SRS §8.2 |
| <a id="TR-SM-ENT-05:Active-_Retired:58f043"></a>Active | Retired | Owner ngừng dịch vụ | - | Usage của service chuyển Closed; Finding liên quan đóng `usage_closed` | SRS §8.2 |

<a id="SM-ENT-06"></a>
<a id="SM-ENT-06:Active"></a>
<a id="SM-ENT-06:Expired"></a>
<a id="SM-ENT-06:Revoked"></a>
#### 3.1.6 Vòng đời của ApiToken (ENT-06)

Trạng thái: Active, Expired, Revoked.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-06:_m_i_-_Active:9566cf"></a>(mới) | Active | Owner hoặc ADM tạo (`api_token` gắn một dự án, BRL-53; `oauth_client` do ADM cấp) | Có `expires_at` | Giá trị token hiện một lần; lưu `token_hash` | SRS §8.2 |
| <a id="TR-SM-ENT-06:Active-_Expired:b25a31"></a>Active | Expired | Job khi `expires_at` qua | - | Từ chối request; Alert nhắc hết hạn gửi trước đó | SRS §8.2 |
| <a id="TR-SM-ENT-06:Active-_Revoked:6ba439"></a>Active | Revoked | Người cấp hoặc ADM thu hồi | Hiệu lực ≤ 1 phút | Ghi audit; xoá cache xác thực | SRS §8.2 |

<a id="SM-ENT-07"></a>
<a id="SM-ENT-07:Disabled"></a>
<a id="SM-ENT-07:Enabled"></a>
<a id="SM-ENT-07:Registered"></a>
<a id="SM-ENT-07:Retired"></a>
#### 3.1.7 Vòng đời của Connector (ENT-07)

Trạng thái: Disabled, Enabled, Registered, Retired.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-07:_m_i_-_Registered:23172f"></a>(mới) | Registered | Đăng ký connector và phiên bản | `(name, version)` duy nhất | `conformance_status=not_run` | SRS §8.3 |
| <a id="TR-SM-ENT-07:Disabled-_Enabled:219dba"></a>Disabled | Enabled | ADM bật lại | Conformance còn `passed` | - | SRS §8.3 |
| <a id="TR-SM-ENT-07:Enabled-_Disabled:f0a4a6"></a>Enabled | Disabled | ADM tắt | - | DataSource dùng connector này chuyển Disabled | SRS §8.3 |
| <a id="TR-SM-ENT-07:Registered_ho_c_Disabled-_Retired:debb02"></a>Registered hoặc Disabled | Retired | ADM ngừng | Không DataSource Enabled nào dùng | Chỉ còn để tra cứu | SRS §8.3 |
| <a id="TR-SM-ENT-07:Registered-_Enabled:6c7b87"></a>Registered | Enabled | ADM bật | `conformance_status=passed` | Cho phép DataSource dùng | SRS §8.3 |

<a id="SM-ENT-08"></a>
<a id="SM-ENT-08:Disabled"></a>
<a id="SM-ENT-08:Draft"></a>
<a id="SM-ENT-08:Enabled"></a>
#### 3.1.8 Vòng đời của DataSource (ENT-08)

Trạng thái: Disabled, Draft, Enabled.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-08:_m_i_-_Draft:57569a"></a>(mới) | Draft | ADM tạo | `kind` thuộc `supported_kinds` của connector | - | SRS §8.3 |
| <a id="TR-SM-ENT-08:Disabled-_Enabled:1900d7"></a>Disabled | Enabled | ADM bật lại | Thử kết nối đạt | Tính lại `freshness` | SRS §8.3 |
| <a id="TR-SM-ENT-08:Draft-_Enabled:d57480"></a>Draft | Enabled | ADM bật sau khi thử kết nối đạt | Kết nối thử thành công; Connector Enabled | Lập lịch ImportJob | SRS §8.3 |
| <a id="TR-SM-ENT-08:Enabled-_Disabled:f0a4a6"></a>Enabled | Disabled | ADM tắt | - | Huỷ job Queued; job Running chạy hết | SRS §8.3 |

<a id="SM-ENT-09"></a>
<a id="SM-ENT-09:Cancelled"></a>
<a id="SM-ENT-09:Failed"></a>
<a id="SM-ENT-09:Partial"></a>
<a id="SM-ENT-09:Queued"></a>
<a id="SM-ENT-09:Running"></a>
<a id="SM-ENT-09:Succeeded"></a>
#### 3.1.9 Vòng đời của ImportJob (ENT-09)

Trạng thái: Cancelled, Failed, Partial, Queued, Running, Succeeded.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-09:_m_i_-_Queued:ab033f"></a>(mới) | Queued | Lịch, chạy tay, upload file hoặc API (API-K-17, API-K-20, API-K-21) | DataSource Enabled | - | SRS §8.3 |
| <a id="TR-SM-ENT-09:Queued-_Cancelled:d98d93"></a>Queued | Cancelled | ADM huỷ | - | - | SRS §8.3 |
| <a id="TR-SM-ENT-09:Queued-_Running:4adf0a"></a>Queued | Running | Worker nhận job | Khoá tư vấn theo `data_source_id` (NFR-RE-02) | Ghi `started_at` | SRS §8.3 |
| <a id="TR-SM-ENT-09:Running-_Cancelled:ef66d1"></a>Running | Cancelled | ADM huỷ | Dừng ở ranh giới lô | Rollback lô đang xử lý | SRS §8.3 |
| <a id="TR-SM-ENT-09:Running-_Failed:7b0df0"></a>Running | Failed | Lỗi kết nối hoặc tỉ lệ reject vượt ngưỡng | Tỉ lệ > ngưỡng | Không công bố, giữ last-known-good; Alert `stale_source` khi quá hạn | SRS §8.3 |
| <a id="TR-SM-ENT-09:Running-_Partial:33fd31"></a>Running | Partial | Hết dữ liệu | 0 < tỉ lệ reject ≤ `limits.import_error_threshold` (1%) | Công bố phần hợp lệ; ghi lỗi từng bản ghi | SRS §8.3 |
| <a id="TR-SM-ENT-09:Running-_Succeeded:5ba75c"></a>Running | Succeeded | Hết dữ liệu, không bản ghi bị reject | Tỉ lệ reject = 0 | Công bố dữ liệu; cập nhật `last_success_at`, `freshness=Fresh` | SRS §8.3 |

<a id="SM-ENT-10"></a>
<a id="SM-ENT-10:Ingested"></a>
<a id="SM-ENT-10:Normalized"></a>
<a id="SM-ENT-10:Rejected"></a>
<a id="SM-ENT-10:Superseded"></a>
<a id="SM-ENT-10:Validated"></a>
#### 3.1.10 Vòng đời của RawRecord (ENT-10)

Trạng thái: Ingested, Normalized, Rejected, Superseded, Validated.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-10:_m_i_-_Ingested:252b2e"></a>(mới) | Ingested | Worker ghi bản ghi thô | UNIQUE `(data_source_id, source_key, checksum)`: trùng thì bỏ qua | - | SRS §8.3 |
| <a id="TR-SM-ENT-10:Ingested-_Rejected:02fed2"></a>Ingested | Rejected | Không qua schema | - | Tăng bộ đếm reject của job | SRS §8.3 |
| <a id="TR-SM-ENT-10:Ingested-_Validated:685101"></a>Ingested | Validated | Qua JSON Schema | Hợp lệ | - | SRS §8.3 |
| <a id="TR-SM-ENT-10:Normalized-_Superseded:cd2356"></a>Normalized | Superseded | Có bản ghi mới cùng `source_key` khác checksum | - | Bản mới đi lại từ Ingested | SRS §8.3 |
| <a id="TR-SM-ENT-10:Validated-_Normalized:e4ded6"></a>Validated | Normalized | Chuẩn hoá và hợp nhất (ALG-12 khi xung đột nguồn) | Ánh xạ được về thực thể chuẩn | Ghi hoặc cập nhật Vulnerability, Component, ReleaseLine | SRS §8.3 |
| <a id="TR-SM-ENT-10:Validated-_Rejected:fecef0"></a>Validated | Rejected | Chuẩn hoá thất bại | - | Như trên | SRS §8.3 |

<a id="SM-ENT-11"></a>
<a id="SM-ENT-11:Active"></a>
<a id="SM-ENT-11:Archived"></a>
<a id="SM-ENT-11:Merged"></a>
#### 3.1.11 Vòng đời của Component (ENT-11)

Trạng thái: Active, Archived, Merged.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-11:_m_i_-_Active:9318be"></a>(mới) | Active | Chuẩn hoá từ nguồn hoặc SEC tạo tay | PURL duy nhất | - | SRS §8.3 |
| <a id="TR-SM-ENT-11:Active-_Archived:4cf69e"></a>Active | Archived | SEC lưu trữ | Không Usage Active | Ẩn khỏi tìm kiếm mặc định | SRS §8.3 |
| <a id="TR-SM-ENT-11:Active-_Merged:499e3d"></a>Active | Merged | SEC gộp trùng vào Component khác | Có bản đích Active | Chuyển ComponentVersion, Usage, alias sang bản đích | SRS §8.3 |
| <a id="TR-SM-ENT-11:Archived-_Active:3d0002"></a>Archived | Active | SEC khôi phục | - | - | SRS §8.3 |

<a id="SM-ENT-12"></a>
<a id="SM-ENT-12:Active"></a>
<a id="SM-ENT-12:Yanked"></a>
#### 3.1.12 Vòng đời của ComponentVersion (ENT-12)

Trạng thái: Active, Yanked.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-12:_m_i_-_Active:4c4cfb"></a>(mới) | Active | Chuẩn hoá hoặc gặp trong SBOM | Component tồn tại | Tính lại ReleaseLine | SRS §8.3 |
| <a id="TR-SM-ENT-12:Active-_Yanked:c94954"></a>Active | Yanked | Registry báo bị gỡ | Nguồn registry | Alert cho Usage đang dùng; Finding không tự sinh | SRS §8.3 |
| <a id="TR-SM-ENT-12:Yanked-_Active:a85eea"></a>Yanked | Active | Registry bỏ cờ | - | - | SRS §8.3 |

<a id="SM-ENT-13"></a>
<a id="SM-ENT-13:EOL"></a>
<a id="SM-ENT-13:Supported"></a>
<a id="SM-ENT-13:Unknown"></a>
#### 3.1.13 Vòng đời của ReleaseLine (ENT-13)

Trạng thái: EOL, Supported, Unknown.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-13:_m_i___Unknown-_EOL:e8e2de"></a>(mới), Unknown | EOL | Có dữ liệu nguồn | Ngày EOL đã qua | Tạo Finding `type=eol` cho Usage liên quan; Alert `eol` | SRS §8.3 |
| <a id="TR-SM-ENT-13:_m_i___Unknown-_Security-only:fed720"></a>(mới), Unknown | Security-only | Có dữ liệu nguồn | Đã qua mốc active support, chưa tới EOL | - | SRS §8.3 |
| <a id="TR-SM-ENT-13:_m_i___Unknown-_Supported:121d17"></a>(mới), Unknown | Supported | Có dữ liệu nguồn (BRL-17) | Ngày kết thúc hỗ trợ ở tương lai | - | SRS §8.3 |
| <a id="TR-SM-ENT-13:_m_i_-_Unknown:d97ce9"></a>(mới) | Unknown | Tạo dòng khi chưa có ngày hỗ trợ | - | - | SRS §8.3 |
| <a id="TR-SM-ENT-13:Security-only__EOL-_Supported:6b10a8"></a>Security-only, EOL | Supported | Nguồn gia hạn hỗ trợ | Ngày mới ở tương lai | Đóng Finding `eol` liên quan | SRS §8.3 |
| <a id="TR-SM-ENT-13:Security-only-_EOL:5b0fca"></a>Security-only | EOL | Job hằng ngày khi `eol_date` qua | - | Như trên | SRS §8.3 |
| <a id="TR-SM-ENT-13:Supported__Security-only__EOL-_Unknown:fb2d71"></a>Supported, Security-only, EOL | Unknown | Nguồn mất dữ liệu | Không còn nguồn hợp lệ | Giữ giá trị cũ trong lịch sử | SRS §8.3 |
| <a id="TR-SM-ENT-13:Supported-_Security-only:d6084b"></a>Supported | Security-only | Job hằng ngày khi qua mốc active support | Ngày theo nguồn ưu tiên | Alert `eol` theo mốc trong Policy `eol` | SRS §8.3 |

<a id="SM-ENT-14"></a>
<a id="SM-ENT-14:Published"></a>
<a id="SM-ENT-14:Unclassified"></a>
<a id="SM-ENT-14:Withdrawn"></a>
#### 3.1.14 Vòng đời của Vulnerability (ENT-14)

Trạng thái: Published, Unclassified, Withdrawn.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-14:_m_i_-_Published:f64c3c"></a>(mới) | Published | Chuẩn hoá advisory đủ dữ liệu | Đủ severity và AffectedRange (BRL-09) | Job khớp Usage, sinh Finding | SRS §8.3 |
| <a id="TR-SM-ENT-14:_m_i_-_Unclassified:24a148"></a>(mới) | Unclassified | Chuẩn hoá advisory | Chưa có severity hoặc phạm vi ảnh hưởng | Alert `unclassified_advisory` | SRS §8.3 |
| <a id="TR-SM-ENT-14:Published-_Withdrawn:3cc76d"></a>Published | Withdrawn | Nguồn rút advisory | Mọi nguồn đồng thuận rút | Finding mở đóng `withdrawn` | SRS §8.3 |
| <a id="TR-SM-ENT-14:Unclassified-_Published:60162d"></a>Unclassified | Published | Nguồn bổ sung hoặc SEC phân loại | Đủ severity và AffectedRange | Như trên | SRS §8.3 |
| <a id="TR-SM-ENT-14:Withdrawn-_Published:62605c"></a>Withdrawn | Published | Nguồn phục hồi advisory | - | Finding tạo lại theo dedupe_key | SRS §8.3 |

<a id="SM-ENT-16"></a>
<a id="SM-ENT-16:Active"></a>
<a id="SM-ENT-16:Deprecated"></a>
#### 3.1.15 Vòng đời của License (ENT-16)

Trạng thái: Active, Deprecated.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-16:_m_i_-_Active:1738ae"></a>(mới) | Active | Nạp từ danh sách SPDX (EIF-10) hoặc LEG tạo | `spdx_id` duy nhất | - | SRS §8.3 |
| <a id="TR-SM-ENT-16:Active-_Deprecated:a18dde"></a>Active | Deprecated | Nguồn SPDX đánh dấu deprecated hoặc LEG đổi | - | Vẫn tra ma trận; gợi ý mã thay thế | SRS §8.3 |
| <a id="TR-SM-ENT-16:Deprecated-_Active:6c729d"></a>Deprecated | Active | LEG khôi phục | - | - | SRS §8.3 |

<a id="SM-ENT-17"></a>
<a id="SM-ENT-17:Current"></a>
<a id="SM-ENT-17:Received"></a>
<a id="SM-ENT-17:Rejected"></a>
<a id="SM-ENT-17:Summarized"></a>
<a id="SM-ENT-17:Superseded"></a>
#### 3.1.16 Vòng đời của SbomSnapshot (ENT-17)

Trạng thái: Current, Received, Rejected, Summarized, Superseded.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-17:_m_i_-_Received:01aedf"></a>(mới) | Received | CI hoặc người dùng nộp SBOM (API nhóm K) | `content_hash` chưa có cho cùng service và environment; token đúng dự án | Lưu file vào object storage | SRS §8.4 |
| <a id="TR-SM-ENT-17:Current-_Superseded:a0265f"></a>Current | Superseded | Có snapshot Current mới cùng `(service_id, environment_id)` | - | Giữ 24 tháng chi tiết | SRS §8.4 |
| <a id="TR-SM-ENT-17:Received-_Current:8d4fad"></a>Received | Current | Phân tích thành công | Hợp lệ | Snapshot Current cũ chuyển Superseded; đối soát Usage (ALG-01); kích hoạt Evaluation | SRS §8.4 |
| <a id="TR-SM-ENT-17:Received-_Rejected:a4a14b"></a>Received | Rejected | Kiểm tra định dạng CycloneDX hoặc SPDX thất bại | Sai schema hoặc vượt giới hạn kích thước | Trả lỗi chi tiết cho CI; không đổi Usage | SRS §8.4 |
| <a id="TR-SM-ENT-17:Superseded-_Summarized:42f3c6"></a>Superseded | Summarized | Job hằng ngày khi quá `limits.snapshot_detail_months` (24) | - | Xoá chi tiết thành phần, giữ bản tóm tắt | SRS §8.4 |

<a id="SM-ENT-18"></a>
<a id="SM-ENT-18:Active"></a>
<a id="SM-ENT-18:Closed"></a>
<a id="SM-ENT-18:Stale"></a>
#### 3.1.17 Vòng đời của Usage (ENT-18)

Trạng thái: Active, Closed, Stale.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-18:_m_i_-_Active:9f39e6"></a>(mới) | Active | SBOM Current chứa component, hoặc nhập tay, import, kế hoạch (`origin`) | UNIQUE `(service_id, environment_id, component_version_id)` | Sinh Finding qua kết hợp với AffectedRange | SRS §8.4 |
| <a id="TR-SM-ENT-18:Active__Stale-_Closed:071884"></a>Active, Stale | Closed | SBOM Current mới không còn component (ALG-01), hoặc Service, Environment Retired | Với `origin=sbom`: không cần xác nhận | Finding mở đóng `usage_closed` | SRS §8.4 |
| <a id="TR-SM-ENT-18:Active-_Stale:6a4fdd"></a>Active | Stale | Job khi `origin=manual` quá `confirm_due_at` mà chưa xác nhận | - | Alert nhắc xác nhận | SRS §8.4 |
| <a id="TR-SM-ENT-18:Closed-_Active:6a6a1f"></a>Closed | Active | Component xuất hiện lại trong SBOM | - | Mở lại cùng bản ghi; Finding đóng do `usage_closed` được xét lại | SRS §8.4 |
| <a id="TR-SM-ENT-18:Stale-_Active:874e8a"></a>Stale | Active | Chủ dịch vụ xác nhận, hoặc SBOM mới chứa lại | - | Đặt lại `confirm_due_at` | SRS §8.4 |

<a id="SM-ENT-19"></a>
<a id="SM-ENT-19:Declared"></a>
<a id="SM-ENT-19:Superseded"></a>
#### 3.1.18 Vòng đời của UsageContext (ENT-19)

Trạng thái: Declared, Superseded.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-19:_m_i_-_Declared:84028d"></a>(mới) | Declared | Owner khai báo ngữ cảnh sử dụng (đích `service` hoặc `usage`) | Đích thuộc dự án của người khai báo | Tính lại Priority của Finding liên quan (ALG-02) | SRS §8.4 |
| <a id="TR-SM-ENT-19:Declared-_Superseded:b3f807"></a>Declared | Superseded | Khai báo bản mới cho cùng đích | - | Giữ bản cũ để tra lịch sử | SRS §8.4 |

<a id="SM-ENT-20"></a>
<a id="SM-ENT-20:Confirmed"></a>
<a id="SM-ENT-20:Detected"></a>
<a id="SM-ENT-20:Overridden"></a>
#### 3.1.19 Vòng đời của LicenseObservation (ENT-20)

Trạng thái: Confirmed, Detected, Overridden.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-20:_m_i_-_Detected:a45845"></a>(mới) | Detected | SBOM, registry, package_metadata hoặc scan phát hiện | UNIQUE `(component_version_id, source)` | Chọn license hiệu lực (`effective`) theo độ tin cậy | SRS §8.4 |
| <a id="TR-SM-ENT-20:Detected__Confirmed-_Overridden:3f489f"></a>Detected, Confirmed | Overridden | LEG ghi đè bằng giá trị thủ công | Có lý do; hiệu lực cần Decision `license_exception` nếu thuộc tính bị chặn (BRL-24) | Phát hiện license drift (ALG-06) nếu khác phiên bản liền kề | SRS §8.4 |
| <a id="TR-SM-ENT-20:Detected-_Confirmed:3c8dba"></a>Detected | Confirmed | LEG xác nhận | Vai trò LEG | Đặt `effective`; ghi audit | SRS §8.4 |
| <a id="TR-SM-ENT-20:Overridden-_Confirmed:90c517"></a>Overridden | Confirmed | LEG hoàn tác ghi đè | - | Khôi phục giá trị đã xác nhận | SRS §8.4 |

<a id="SM-ENT-21"></a>
<a id="SM-ENT-21:Affected"></a>
<a id="SM-ENT-21:Fixed"></a>
<a id="SM-ENT-21:Not_affected"></a>
<a id="SM-ENT-21:Open"></a>
<a id="SM-ENT-21:Risk_accepted"></a>
<a id="SM-ENT-21:Triaging"></a>
#### 3.1.20 Vòng đời của Finding (ENT-21)

Trạng thái: Affected, Fixed, Not_affected, Open, Risk_accepted, Triaging.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-21:_m_i_-_Open:39aefe"></a>(mới) | Open | Job khớp Usage với Vulnerability hoặc ReleaseLine EOL, hoặc AssetFinding | Khử trùng theo `dedupe_key`; Priority theo ALG-02 | Tính `sla_due_at`; Alert `finding_new`; webhook | SRS §8.5 |
| <a id="TR-SM-ENT-21:Fixed__Not__affected__Risk__accepted-_Open:2b81d3"></a>Fixed, Not_affected, Risk_accepted | Open | API-V-08 mở lại; hoặc Decision hết hạn, bị thu hồi; hoặc SBOM mới lại chứa phiên bản dính lỗi | Với Risk_accepted: Decision Expired hoặc Revoked | SLA chạy lại; Alert `finding_new` hoặc `decision_expired` | SRS §8.5 |
| <a id="TR-SM-ENT-21:Open__Triaging__Affected-_Fixed:017f71"></a>Open, Triaging, Affected | Fixed | Nguồn rút advisory, hoặc Usage Closed | Vulnerability Withdrawn hoặc Usage Closed | SLA dừng; `closed_reason=withdrawn` hoặc `usage_closed` (từ điển chưa nêu trạng thái đích của hai lý do này; SRS chọn Fixed, xem R-12 ở Mục 10.6) | SRS §8.5 |
| <a id="TR-SM-ENT-21:Open__Triaging__Affected-_Fixed:67c6ec"></a>Open, Triaging, Affected | Fixed | API-V-07 hoặc SBOM mới không còn phiên bản dính lỗi | Với tự động: phiên bản mới ngoài AffectedRange; với tay: có bằng chứng | SLA dừng; `closed_reason=fixed_auto` hoặc `fixed_manual` | SRS §8.5 |
| <a id="TR-SM-ENT-21:Open__Triaging__Affected-_Risk__accepted:8e96dd"></a>Open, Triaging, Affected | Risk_accepted | API-V-06 chấp nhận rủi ro | Decision `risk_acceptance` ở trạng thái Approved, còn hạn (BRL-14) | SLA dừng; `closed_reason=risk_accepted`; Finding gắn `decision_id` | SRS §8.5 |
| <a id="TR-SM-ENT-21:Open-_Triaging:607ce2"></a>Open | Triaging | API-V-03 nhận xử lý, hoặc API-V-09 gán người xử lý | Người gọi có vai trò SEC hoặc Contributor của dự án | Ghi `assignee_id` | SRS §8.5 |
| <a id="TR-SM-ENT-21:Triaging-_Affected:bc8e1e"></a>Triaging | Affected | API-V-05 xác nhận bị ảnh hưởng | Có ghi chú | Tạo VexStatement `affected` nếu bật; SLA tiếp tục | SRS §8.5 |
| <a id="TR-SM-ENT-21:Triaging-_Not__affected:4094c9"></a>Triaging | Not_affected | API-V-04 | Bắt buộc `justification` (BRL-12); VexStatement do SEC duyệt qua API-V-13 khi chính sách yêu cầu | SLA dừng; `closed_reason=not_affected` | SRS §8.5 |

<a id="SM-ENT-22"></a>
<a id="SM-ENT-22:Issued"></a>
<a id="SM-ENT-22:Superseded"></a>
#### 3.1.21 Vòng đời của VexStatement (ENT-22)

Trạng thái: Issued, Superseded.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-22:_m_i_-_Issued:dcc784"></a>(mới) | Issued | Finding đổi sang Not_affected, Affected hoặc Fixed; hoặc duyệt qua API-V-13 | `justification` bắt buộc khi `vex_status=not_affected` (BRL-12) | Đưa vào VEX export (OpenVEX) | SRS §8.5 |
| <a id="TR-SM-ENT-22:Issued-_Superseded:614c2d"></a>Issued | Superseded | Có VexStatement mới cho cùng cặp Finding và Vulnerability | - | Bản cũ giữ trong lịch sử | SRS §8.5 |

<a id="SM-ENT-23"></a>
<a id="SM-ENT-23:Approved"></a>
<a id="SM-ENT-23:Draft"></a>
<a id="SM-ENT-23:Expired"></a>
<a id="SM-ENT-23:Pending"></a>
<a id="SM-ENT-23:Rejected"></a>
<a id="SM-ENT-23:Revoked"></a>
<a id="SM-ENT-23:Superseded"></a>
#### 3.1.22 Vòng đời của Decision (ENT-23)

Trạng thái: Approved, Draft, Expired, Pending, Rejected, Revoked, Superseded.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-23:_m_i_-_Draft:42cfc2"></a>(mới) | Draft | API-G-01 tạo | Đủ `type`, `scope_type`, `subject_type` | - | SRS §8.5 |
| <a id="TR-SM-ENT-23:Approved-_Expired:514cdd"></a>Approved | Expired | Job hằng ngày khi `expires_at` qua | - | Alert `decision_expired`; Finding gắn Decision quay về Open; nhắc trước hạn bằng `decision_expiring` | SRS §8.5 |
| <a id="TR-SM-ENT-23:Approved-_Revoked:df865d"></a>Approved | Revoked | ADM, ARB hoặc SEC thu hồi (API nhóm G) | Có lý do | Finding gắn Decision quay về Open; hạ tác dụng ngoại lệ | SRS §8.5 |
| <a id="TR-SM-ENT-23:Approved-_Superseded:759456"></a>Approved | Superseded | Decision mới thay thế được duyệt | Cùng phạm vi và chủ đề | Bản mới có hiệu lực; bản cũ giữ tra cứu | SRS §8.5 |
| <a id="TR-SM-ENT-23:Draft-_Pending:e3b9ab"></a>Draft | Pending | API-G-05 gửi duyệt | Đủ trường bắt buộc theo `type`; `expires_at` với rủi ro và ngoại lệ (BRL-13) | Tạo yêu cầu cho người duyệt theo ma trận vai trò; Alert cho ARB, SEC, LEG, PROC | SRS §8.5 |
| <a id="TR-SM-ENT-23:Pending-_Approved:be9207"></a>Pending | Approved | API-G-06 duyệt | Đủ số người duyệt đúng vai trò; người duyệt khác người tạo; mỗi người duyệt một lần (UNIQUE) | Khoá nội dung; áp tác dụng theo `type` (đóng Finding, hạ gate, bật ngoại lệ); ghi `decision_approvals` | SRS §8.5 |
| <a id="TR-SM-ENT-23:Pending-_Draft:d258e2"></a>Pending | Draft | Người tạo rút lại (API nhóm G) | Chưa có quyết định cuối | Xoá yêu cầu duyệt | SRS §8.5 |
| <a id="TR-SM-ENT-23:Pending-_Rejected:b0de94"></a>Pending | Rejected | Một người duyệt chọn reject (API nhóm G) | Có nhận xét | Thông báo người tạo | SRS §8.5 |

<a id="SM-ENT-24"></a>
<a id="SM-ENT-24:Active"></a>
<a id="SM-ENT-24:Draft"></a>
<a id="SM-ENT-24:Superseded"></a>
#### 3.1.23 Vòng đời của Policy (ENT-24)

Trạng thái: Active, Draft, Superseded.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-24:_m_i_-_Draft:5df3b6"></a>(mới) | Draft | Người có quyền tạo (SEC, LEG, ADM tuỳ `type`) | Nội dung đúng JSON Schema theo `type` | - | SRS §8.5 |
| <a id="TR-SM-ENT-24:Active-_Superseded:8a5d11"></a>Active | Superseded | Có bản Active mới cùng `(type, name, scope_type, scope_id)` | - | Đánh giá mới dùng bản mới; Evaluation cũ vẫn trỏ bản cũ | SRS §8.5 |
| <a id="TR-SM-ENT-24:Draft-_Active:43c708"></a>Draft | Active | Kích hoạt | Hợp lệ; với `retention`: `audit_min_days` ≥ 1095, `evidence_years_after_asset_end` ≥ 5, `purge_requires_legal_confirmation=true` | Bản Active cũ cùng khoá chuyển Superseded; ghi `policy.activated` | SRS §8.5 |

<a id="SM-ENT-25"></a>
<a id="SM-ENT-25:Completed"></a>
<a id="SM-ENT-25:Error"></a>
<a id="SM-ENT-25:Pending"></a>
#### 3.1.24 Vòng đời của Evaluation (ENT-25)

Trạng thái: Completed, Error, Pending.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-25:_m_i_-_Pending:73faf9"></a>(mới) | Pending | Trigger `submit`, `evaluate_api`, `rescan` hoặc `job` | Có snapshot hoặc đầu vào hợp lệ | - | SRS §8.5 |
| <a id="TR-SM-ENT-25:Pending-_Completed:eff545"></a>Pending | Completed | Chạy xong rule | Không lỗi | Ghi `result` (pass, warn, fail); trả kết quả cho CI | SRS §8.5 |
| <a id="TR-SM-ENT-25:Pending-_Error:a8899b"></a>Pending | Error | Lỗi nội bộ hoặc quá thời gian | - | Áp `gate_failure_mode` (open hoặc closed) của dự án; ghi `failure_mode_applied`; Alert cho SEC | SRS §8.5 |

<a id="SM-ENT-26"></a>
<a id="SM-ENT-26:Acknowledged"></a>
<a id="SM-ENT-26:Created"></a>
<a id="SM-ENT-26:Escalated"></a>
<a id="SM-ENT-26:Resolved"></a>
<a id="SM-ENT-26:Sent"></a>
<a id="SM-ENT-26:Suppressed"></a>
#### 3.1.25 Vòng đời của Alert (ENT-26)

Trạng thái: Acknowledged, Created, Escalated, Resolved, Sent, Suppressed.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-26:_m_i_-_Created:690bf4"></a>(mới) | Created | Sự kiện nghiệp vụ theo `type` | Khử trùng theo `dedupe_key` | - | SRS §8.5 |
| <a id="TR-SM-ENT-26:Created-_Sent:fcf2bb"></a>Created | Sent | Worker gửi qua `channel` | Kênh sẵn sàng | Ghi `sent_at`; thử lại theo NFR-RE | SRS §8.5 |
| <a id="TR-SM-ENT-26:Created-_Suppressed:34a667"></a>Created | Suppressed | Gộp digest, snooze, trùng | `suppressed_reason` bắt buộc; không dùng snooze cho Critical, KEV, SLA breached (BRL-39) | - | SRS §8.5 |
| <a id="TR-SM-ENT-26:Sent__Acknowledged__Escalated-_Resolved:a8372c"></a>Sent, Acknowledged, Escalated | Resolved | Nguyên nhân hết (Finding đóng, Decision gia hạn, nguồn phục hồi) | - | Ghi `resolved_at` | SRS §8.5 |
| <a id="TR-SM-ENT-26:Sent__Acknowledged-_Escalated:79ca75"></a>Sent, Acknowledged | Escalated | Job khi quá hạn phản hồi theo Policy | Chưa Resolved | Gửi lên cấp trên theo `escalation` | SRS §8.5 |
| <a id="TR-SM-ENT-26:Sent-_Acknowledged:96a2c2"></a>Sent | Acknowledged | Người nhận xác nhận | - | Dừng nhắc lại | SRS §8.5 |

<a id="SM-ENT-29"></a>
<a id="SM-ENT-29:Applied"></a>
<a id="SM-ENT-29:Approved"></a>
<a id="SM-ENT-29:Draft"></a>
<a id="SM-ENT-29:Pending_ARB"></a>
<a id="SM-ENT-29:Rejected"></a>
#### 3.1.26 Vòng đời của CatalogChangeRequest (ENT-29)

Trạng thái: Applied, Approved, Draft, Pending_ARB, Rejected.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-29:_m_i_-_Draft:0292cc"></a>(mới) | Draft | Người có quyền đề xuất (`change` là add, ring, version hoặc retire) | Component tồn tại | - | SRS §8.6 |
| <a id="TR-SM-ENT-29:Approved-_Applied:53dd62"></a>Approved | Applied | Hệ thống ghi vào Catalog | Giao dịch ghi CatalogEntry thành công | Đổi ring; tăng `catalog_revision`; sinh Baseline | SRS §8.6 |
| <a id="TR-SM-ENT-29:Draft-_Pending__ARB:8d483f"></a>Draft | Pending_ARB | Người đề xuất gửi | Đủ lý do và phân tích tác động | Alert cho ARB; tạo Decision `catalog_change` (trạng thái Pending) | SRS §8.6 |
| <a id="TR-SM-ENT-29:Pending__ARB-_Approved:ad9cf4"></a>Pending_ARB | Approved | ARB duyệt (Decision `catalog_change` Approved) | Đủ người duyệt | - | SRS §8.6 |
| <a id="TR-SM-ENT-29:Pending__ARB-_Rejected:354e07"></a>Pending_ARB | Rejected | ARB từ chối | Có nhận xét | Thông báo người đề xuất | SRS §8.6 |

<a id="SM-ENT-30"></a>
<a id="SM-ENT-30:Archived"></a>
<a id="SM-ENT-30:Draft"></a>
<a id="SM-ENT-30:Finalized"></a>
#### 3.1.27 Vòng đời của StackProposal (ENT-30)

Trạng thái: Archived, Draft, Finalized.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-30:_m_i_-_Draft:cc4792"></a>(mới) | Draft | Owner hoặc Contributor tạo đề xuất stack | Project Active | - | SRS §8.6 |
| <a id="TR-SM-ENT-30:Draft__Finalized-_Archived:3ee99e"></a>Draft, Finalized | Archived | Chủ đề xuất lưu trữ | - | Usage `planned` chưa thành thực tế bị đóng | SRS §8.6 |
| <a id="TR-SM-ENT-30:Draft-_Finalized:8261ad"></a>Draft | Finalized | Chốt đề xuất (FR-SEL-06) | Không lựa chọn nào thuộc ring Banned | Sinh Usage `origin=planned`; có thể sinh Report `tech_stack_review` | SRS §8.6 |

<a id="SM-ENT-31"></a>
<a id="SM-ENT-31:Generated"></a>
<a id="SM-ENT-31:Superseded"></a>
#### 3.1.28 Vòng đời của Baseline (ENT-31)

Trạng thái: Generated, Superseded.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-31:_m_i_-_Generated:8bb09b"></a>(mới) | Generated | Sau mỗi CR Applied (theo `format` json hoặc yaml, từng `layer_key`) | UNIQUE `(catalog_revision, format, layer_key)` | Phát hành cho dự án | SRS §8.6 |
| <a id="TR-SM-ENT-31:Generated-_Superseded:7c2c33"></a>Generated | Superseded | Baseline cùng `layer_key` và `format` ở `catalog_revision` mới hơn | - | Giữ để tra cứu | SRS §8.6 |

<a id="SM-ENT-32"></a>
<a id="SM-ENT-32:Expired"></a>
<a id="SM-ENT-32:Failed"></a>
<a id="SM-ENT-32:Generating"></a>
<a id="SM-ENT-32:Ready"></a>
<a id="SM-ENT-32:Requested"></a>
#### 3.1.29 Vòng đời của Report (ENT-32)

Trạng thái: Expired, Failed, Generating, Ready, Requested.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-32:_m_i_-_Requested:8452b3"></a>(mới) | Requested | Người dùng yêu cầu (`manual`) hoặc lịch (`schedule`) | Quyền theo `type` và `scope_type` | Xếp hàng job | SRS §8.6 |
| <a id="TR-SM-ENT-32:Failed-_Requested:40296c"></a>Failed | Requested | Người yêu cầu thử lại | - | Xếp hàng lại | SRS §8.6 |
| <a id="TR-SM-ENT-32:Generating-_Failed:b5d264"></a>Generating | Failed | Lỗi sinh hoặc quá thời gian | - | Ghi lỗi; Alert cho người yêu cầu | SRS §8.6 |
| <a id="TR-SM-ENT-32:Generating-_Ready:692faf"></a>Generating | Ready | Sinh xong tệp | Tệp ghi vào object storage thành công | Đặt `expires_at` theo `limits.report_days` (90); Alert khi đủ | SRS §8.6 |
| <a id="TR-SM-ENT-32:Ready-_Expired:b25a31"></a>Ready | Expired | Job khi `expires_at` qua | - | Xoá tệp; giữ bản ghi siêu dữ liệu | SRS §8.6 |
| <a id="TR-SM-ENT-32:Requested-_Generating:95ef10"></a>Requested | Generating | Worker nhận | - | - | SRS §8.6 |

<a id="SM-ENT-33"></a>
<a id="SM-ENT-33:Discovered"></a>
<a id="SM-ENT-33:Identified"></a>
<a id="SM-ENT-33:Retired"></a>
#### 3.1.30 Vòng đời của Asset (ENT-33)

Trạng thái: Discovered, Identified, Retired.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-33:_m_i_-_Discovered:99601d"></a>(mới) | Discovered | Scan (AssetScanRun), Figma hoặc nhập tay | UNIQUE `sha256`; `provenance` mặc định `third_party` (BRL-51) | Sinh AssetUsage; chạy kiểm tra AssetFinding | SRS §8.7 |
| <a id="TR-SM-ENT-33:Discovered__Identified-_Retired:49611f"></a>Discovered, Identified | Retired | Người có quyền lưu trữ | Không AssetUsage Active | Ẩn khỏi tìm kiếm mặc định; giữ chứng từ theo Mục 7.6 | SRS §8.7 |
| <a id="TR-SM-ENT-33:Discovered-_Identified:274bda"></a>Discovered | Identified | Xác định nguồn và license | Có AssetLicenseTerm và nguồn | Đóng AssetFinding liên quan nếu đủ điều kiện | SRS §8.7 |

<a id="SM-ENT-34"></a>
<a id="SM-ENT-34:Active"></a>
<a id="SM-ENT-34:Deprecated"></a>
<a id="SM-ENT-34:Draft"></a>
#### 3.1.31 Vòng đời của AssetLicenseTerm (ENT-34)

Trạng thái: Active, Deprecated, Draft.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-34:_m_i_-_Draft:eb3a1c"></a>(mới) | Draft | Seed hoặc LEG tạo | Đủ các cột điều khoản bắt buộc | - | SRS §8.7 |
| <a id="TR-SM-ENT-34:Active-_Deprecated:553f07"></a>Active | Deprecated | LEG thay bằng điều khoản mới | - | Tái đánh giá Asset liên quan | SRS §8.7 |
| <a id="TR-SM-ENT-34:Draft-_Active:89eac1"></a>Draft | Active | LEG xác nhận | Vai trò LEG; có chứng từ nguồn | Dùng cho đánh giá AssetFinding | SRS §8.7 |

<a id="SM-ENT-35"></a>
<a id="SM-ENT-35:Active"></a>
<a id="SM-ENT-35:Expired"></a>
<a id="SM-ENT-35:Expiring"></a>
<a id="SM-ENT-35:Revoked"></a>
#### 3.1.32 Vòng đời của Entitlement (ENT-35)

Trạng thái: Active, Expired, Expiring, Revoked.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-35:_m_i_-_Active:98d22c"></a>(mới) | Active | PROC hoặc LEG ghi nhận quyền sử dụng | Ít nhất một Evidence (BRL-47) | - | SRS §8.7 |
| <a id="TR-SM-ENT-35:Active__Expiring-_Expired:e8b8a1"></a>Active, Expiring | Expired | Job khi `valid_until` qua | - | AssetUsage ứng với entitlement mở lại AssetFinding; theo `continue_after_expiry` của term | SRS §8.7 |
| <a id="TR-SM-ENT-35:Active__Expiring-_Revoked:0be895"></a>Active, Expiring | Revoked | PROC hoặc LEG thu hồi | Có lý do | Như trên | SRS §8.7 |
| <a id="TR-SM-ENT-35:Active-_Expiring:404f13"></a>Active | Expiring | Job hằng ngày | Còn ≤ 60 ngày đến `valid_until` | Alert `entitlement_expiring` | SRS §8.7 |
| <a id="TR-SM-ENT-35:Expiring-_Active:d68c2b"></a>Expiring | Active | Gia hạn (`valid_until` mới) | Còn > 60 ngày; có Evidence gia hạn | Đóng Alert | SRS §8.7 |

<a id="SM-ENT-36"></a>
<a id="SM-ENT-36:Active"></a>
<a id="SM-ENT-36:Closed"></a>
#### 3.1.33 Vòng đời của AssetUsage (ENT-36)

Trạng thái: Active, Closed.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-36:_m_i_-_Active:020dcb"></a>(mới) | Active | Phát hiện bởi scanner, manual hoặc figma (`detected_by`) | UNIQUE `(asset_id, service_id)` | Kiểm tra entitlement; sinh AssetFinding nếu thiếu | SRS §8.7 |
| <a id="TR-SM-ENT-36:Active-_Closed:e5cbb5"></a>Active | Closed | Scan sau không thấy, hoặc người dùng đóng | - | AssetFinding mở chuyển theo kết quả đóng | SRS §8.7 |
| <a id="TR-SM-ENT-36:Closed-_Active:494426"></a>Closed | Active | Phát hiện lại | - | Mở lại cùng bản ghi | SRS §8.7 |

<a id="SM-ENT-37"></a>
<a id="SM-ENT-37:False_positive"></a>
<a id="SM-ENT-37:Open"></a>
<a id="SM-ENT-37:Resolved"></a>
<a id="SM-ENT-37:Risk_accepted"></a>
<a id="SM-ENT-37:Triaging"></a>
#### 3.1.34 Vòng đời của AssetFinding (ENT-37)

Trạng thái: False_positive, Open, Resolved, Risk_accepted, Triaging.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-37:_m_i_-_Approved__Conditional:65a664"></a>(mới) | Approved, Conditional | PROC hoặc LEG thêm tài sản, nguồn được duyệt sẵn (UC-38) | Conditional cần `vietnamese_support` đã kiểm tra nếu là font | Đặt `next_review_at` | SRS §8.7 |
| <a id="TR-SM-ENT-37:_m_i_-_Open:bf73cf"></a>(mới) | Open | Rule phát hiện vi phạm trên AssetUsage (`level` fail hoặc warn) | UNIQUE `(asset_usage_id, rule)`; `grandfather_until` hạ fail xuống warn (OI-09) | Alert `asset_finding` | SRS §8.7 |
| <a id="TR-SM-ENT-37:Approved__Conditional__Banned-_Retired:4d9945"></a>Approved, Conditional, Banned | Retired | Người có quyền lưu trữ | - | Ẩn khỏi lựa chọn | SRS §8.7 |
| <a id="TR-SM-ENT-37:Approved__Conditional-_Banned:5639b9"></a>Approved, Conditional | Banned | LEG cấm | Có lý do | AssetFinding cho AssetUsage hiện có | SRS §8.7 |
| <a id="TR-SM-ENT-37:Approved-_Conditional:dc1fdb"></a>Approved | Conditional | Đổi điều kiện | Có lý do | Gate cảnh báo | SRS §8.7 |
| <a id="TR-SM-ENT-37:Banned-_Conditional:3f90fc"></a>Banned | Conditional | LEG nới | Có lý do | - | SRS §8.7 |
| <a id="TR-SM-ENT-37:Conditional-_Approved:27093c"></a>Conditional | Approved | Hết điều kiện | LEG xác nhận | - | SRS §8.7 |
| <a id="TR-SM-ENT-37:Open__Triaging-_False__positive:b1ad83"></a>Open, Triaging | False_positive | Người xử lý đánh dấu | `resolution=false_positive`; có lý do | Ghi nhớ để không tái tạo với cùng `sha256` | SRS §8.7 |
| <a id="TR-SM-ENT-37:Open__Triaging-_Resolved:bc797f"></a>Open, Triaging | Resolved | Gắn entitlement, thay thế hoặc gỡ bỏ | `resolution` bắt buộc: entitlement_linked, replaced hoặc removed | Đóng Alert | SRS §8.7 |
| <a id="TR-SM-ENT-37:Open__Triaging-_Risk__accepted:15a6b2"></a>Open, Triaging | Risk_accepted | Chấp nhận rủi ro | Decision `asset_exception` Approved (BRL-42); `resolution=risk_accepted` | Gắn `decision_id` | SRS §8.7 |
| <a id="TR-SM-ENT-37:Open-_Triaging:e4385d"></a>Open | Triaging | Người xử lý nhận | Vai trò PROC hoặc LEG | Ghi `assignee_id` | SRS §8.7 |
| <a id="TR-SM-ENT-37:Resolved__False__positive__Risk__accepted-_Open:5fbc78"></a>Resolved, False_positive, Risk_accepted | Open | Scan lại thấy vi phạm tái phát, hoặc Decision hết hạn hay bị thu hồi | - | Xoá `resolution` | SRS §8.7 |

<a id="SM-ENT-39"></a>
<a id="SM-ENT-39:Completed"></a>
<a id="SM-ENT-39:Failed"></a>
<a id="SM-ENT-39:Running"></a>
#### 3.1.35 Vòng đời của AssetScanRun (ENT-39)

Trạng thái: Completed, Failed, Running.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-39:_m_i_-_Running:3fd899"></a>(mới) | Running | CI, lịch hoặc người dùng bắt đầu scan | Token đúng dự án | Ghi `started_at` | SRS §8.7 |
| <a id="TR-SM-ENT-39:Running-_Completed:6778d4"></a>Running | Completed | Quét xong | - | Đối soát AssetUsage (mở, đóng); sinh AssetFinding; trả kết quả gate | SRS §8.7 |
| <a id="TR-SM-ENT-39:Running-_Failed:b557c4"></a>Running | Failed | Lỗi hoặc quá thời gian | - | Áp `gate_failure_mode` của dự án; Alert cho PROC | SRS §8.7 |

<a id="SM-ENT-40"></a>
<a id="SM-ENT-40:Generated"></a>
<a id="SM-ENT-40:Superseded"></a>
#### 3.1.36 Vòng đời của CreditsDocument (ENT-40)

Trạng thái: Generated, Superseded.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-40:_m_i_-_Generated:886cf8"></a>(mới) | Generated | Sinh credits cho dự án (`format` md, html, json; `language` vi, en, bilingual) | UNIQUE `(project_id, generated_at)` | - | SRS §8.7 |
| <a id="TR-SM-ENT-40:Generated-_Superseded:51aa02"></a>Generated | Superseded | Sinh bản mới cho cùng dự án | - | Giữ bản cũ | SRS §8.7 |

<a id="SM-ENT-41"></a>
<a id="SM-ENT-41:Closed"></a>
<a id="SM-ENT-41:Draft"></a>
<a id="SM-ENT-41:In_progress"></a>
#### 3.1.37 Vòng đời của HandoverChecklist (ENT-41)

Trạng thái: Closed, Draft, In_progress.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-41:_m_i_-_Draft:d1af75"></a>(mới) | Draft | Hệ thống sinh từ Asset và Entitlement của dự án khi mở bàn giao | Project Active | Project chuyển Handover | SRS §8.7 |
| <a id="TR-SM-ENT-41:_m_i_-_Open:1ebd12"></a>(mới) | Open | Sinh cùng checklist | UNIQUE `(checklist_id, asset_id, entitlement_id)` | - | SRS §8.7 |
| <a id="TR-SM-ENT-41:Draft-_In__progress:274a12"></a>Draft | In_progress | Người phụ trách bắt đầu xử lý | Có `owner_id` cho mỗi mục | - | SRS §8.7 |
| <a id="TR-SM-ENT-41:In__progress-_Closed:24545f"></a>In_progress | Closed | Đóng checklist | BRL-50: mục có `licensee` là công ty và `transferable=false` đã được xử lý | Project chuyển Archived | SRS §8.7 |
| <a id="TR-SM-ENT-41:Open-_Done:18da5f"></a>Open | Done | Hoàn thành `proposed_action` (transfer, rebuy_by_client, replace, remove, accept_risk) | Có Evidence với transfer; có Decision với accept_risk | Ghi `decision_id` nếu có | SRS §8.7 |
| <a id="TR-SM-ENT-41:Open-_Waived:165956"></a>Open | Waived | Miễn có lý do | Decision Approved nếu mục bắt buộc (BRL-50) | Ghi audit | SRS §8.7 |

<a id="SM-ENT-42"></a>
<a id="SM-ENT-42:Linked"></a>
<a id="SM-ENT-42:Sync_error"></a>
<a id="SM-ENT-42:Unlinked"></a>
#### 3.1.38 Vòng đời của FigmaConnection (ENT-42)

Trạng thái: Linked, Sync_error, Unlinked.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-42:_m_i_-_Linked:995b96"></a>(mới) | Linked | Owner liên kết file Figma (EIF-16) | Token hợp lệ trong OpenBao | Chạy đồng bộ đầu tiên | SRS §8.7 |
| <a id="TR-SM-ENT-42:Linked__Sync__error-_Unlinked:8a6bd3"></a>Linked, Sync_error | Unlinked | Owner gỡ liên kết | - | Thu hồi token; giữ AssetUsage đã phát hiện | SRS §8.7 |
| <a id="TR-SM-ENT-42:Linked-_Sync__error:321872"></a>Linked | Sync_error | Đồng bộ lỗi liên tiếp hoặc token hết hạn | Vượt số lần thử lại | Alert cho Owner | SRS §8.7 |
| <a id="TR-SM-ENT-42:Sync__error-_Linked:846463"></a>Sync_error | Linked | Đồng bộ lại thành công | - | Xóa cờ lỗi | SRS §8.7 |

<a id="SM-ENT-43"></a>
<a id="SM-ENT-43:Available"></a>
<a id="SM-ENT-43:Purged"></a>
<a id="SM-ENT-43:Uploaded"></a>
#### 3.1.39 Vòng đời của Evidence (ENT-43)

Trạng thái: Available, Purged, Uploaded.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-43:_m_i_-_Uploaded:0ce028"></a>(mới) | Uploaded | Người dùng tải lên (`owner_type`, `kind`) | Quét mã độc và giới hạn kích thước đạt | Lưu object storage | SRS §8.8 |
| <a id="TR-SM-ENT-43:Available-_Purged:cf9fe2"></a>Available | Purged | Xoá sau khi quá hạn giữ | Quá `evidence_years_after_asset_end` (≥ 5) và có xác nhận pháp lý (`purge_requires_legal_confirmation`) | Xoá tệp, giữ bản ghi và hash; ghi audit | SRS §8.8 |
| <a id="TR-SM-ENT-43:Uploaded-_Available:cee125"></a>Uploaded | Available | Ghi và mã hoá thành công | Checksum khớp | Cho phép gắn vào Entitlement, Decision | SRS §8.8 |

<a id="SM-ENT-45"></a>
<a id="SM-ENT-45:Active"></a>
<a id="SM-ENT-45:Disabled"></a>
<a id="SM-ENT-45:Failing"></a>
#### 3.1.40 Vòng đời của WebhookSubscription (ENT-45)

Trạng thái: Active, Disabled, Failing.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-45:_m_i_-_Active:d08ad6"></a>(mới) | Active | Tạo subscription (`format` generic_json, teams, slack, telegram) | `scope_type=company` chỉ SEC, ADM | - | SRS §8.8 |
| <a id="TR-SM-ENT-45:Active__Failing-_Disabled:778a92"></a>Active, Failing | Disabled | Chủ sub hoặc ADM tắt | - | Ngừng tạo EventDelivery mới | SRS §8.8 |
| <a id="TR-SM-ENT-45:Active-_Failing:3b8de9"></a>Active | Failing | EventDelivery liên tiếp Dead_lettered | Vượt ngưỡng lỗi liên tiếp | Alert cho chủ subscription | SRS §8.8 |
| <a id="TR-SM-ENT-45:Disabled-_Active:ad8f8f"></a>Disabled | Active | Chủ sub hoặc ADM bật lại | URL hợp lệ | - | SRS §8.8 |
| <a id="TR-SM-ENT-45:Failing-_Active:9b4cf4"></a>Failing | Active | Giao thành công hoặc chủ sub sửa URL và thử lại đạt | - | - | SRS §8.8 |

<a id="SM-ENT-46"></a>
<a id="SM-ENT-46:Dead_lettered"></a>
<a id="SM-ENT-46:Delivered"></a>
<a id="SM-ENT-46:Pending"></a>
<a id="SM-ENT-46:Retrying"></a>
#### 3.1.41 Vòng đời của EventDelivery (ENT-46)

Trạng thái: Dead_lettered, Delivered, Pending, Retrying.

| Từ | Đến | Sự kiện | Điều kiện | Tác động kèm theo | Nguồn |
|---|---|---|---|---|---|
| <a id="TR-SM-ENT-46:_m_i_-_Pending:441f01"></a>(mới) | Pending | Sự kiện khớp `event_types` của subscription | UNIQUE `(subscription_id, event_id)` | Xuất outbox | SRS §8.8 |
| <a id="TR-SM-ENT-46:Dead__lettered-_Pending:eaa13c"></a>Dead_lettered | Pending | Gửi lại thủ công | Người có quyền quản subscription | Đặt lại đếm lần thử | SRS §8.8 |
| <a id="TR-SM-ENT-46:Pending-_Delivered:e3ac30"></a>Pending | Delivered | Phản hồi 2xx | Chữ ký HMAC hợp lệ | Ghi `delivered_at` | SRS §8.8 |
| <a id="TR-SM-ENT-46:Pending-_Retrying:7dbcf7"></a>Pending | Retrying | Phản hồi khác 2xx hoặc hết thời gian | Còn lượt thử | Đặt `next_retry_at` (backoff) | SRS §8.8 |
| <a id="TR-SM-ENT-46:Retrying-_Dead__lettered:78c6a1"></a>Retrying | Dead_lettered | Hết lượt thử | - | Ghi lỗi cuối; tính vào ngưỡng Failing | SRS §8.8 |
| <a id="TR-SM-ENT-46:Retrying-_Delivered:6dd70e"></a>Retrying | Delivered | Lần thử lại thành công | - | Như trên | SRS §8.8 |

### 3.2 Quy tắc nghiệp vụ

| Mã | Quy tắc | Liên quan (yêu cầu, thuật toán, cơ chế thực thi) | Nguồn |
|---|---|---|---|
| <a id="BRL-01"></a>BRL-01 | Component định danh bằng purl chuẩn hoá | ALG: ALG-01<br>FR: FR-INV-02, FR-INV-04, FR-INV-08<br>Thực thi: INT-01; API-I-01; 400 `invalid_purl` | SRS §10.4, URD §4.2 |
| <a id="BRL-02"></a>BRL-02 | Snapshot xác định bởi (service, môi trường, hash SBOM); nộp trùng idempotent | ALG: ALG-01<br>FR: FR-INV-02<br>Thực thi: INT-02; API-I-01 (`meta.deduplicated`) | SRS §10.4, URD §4.2 |
| <a id="BRL-03"></a>BRL-03 | Snapshot mới nhất là nguồn sự thật cho Usage SBOM | ALG: ALG-01<br>FR: FR-INV-02, FR-CI-05<br>Thực thi: INT-03; Usage đóng khi vắng | SRS §10.4, URD §4.2 |
| <a id="BRL-04"></a>BRL-04 | Usage khai báo tay xác nhận lại 30 ngày, quá hạn Stale | ALG: —<br>FR: FR-INV-04, FR-INV-08<br>Thực thi: API-I-11; job hằng ngày | SRS §10.4, URD §4.2 |
| <a id="BRL-05"></a>BRL-05 | Ngữ cảnh Service kế thừa cho Usage | ALG: ALG-02<br>FR: FR-INV-06, FR-VUL-01<br>Thực thi: API-I-13..15 | SRS §10.4, URD §4.2 |
| <a id="BRL-06"></a>BRL-06 | Phụ thuộc gián tiếp kế thừa môi trường và ngữ cảnh | ALG: ALG-01, ALG-02<br>FR: FR-INV-05, FR-INV-06, FR-VUL-01<br>Thực thi: API-I-18 | SRS §10.4, URD §4.2 |
| <a id="BRL-07"></a>BRL-07 | Vulnerability hợp nhất alias | ALG: ALG-04, ALG-12<br>FR: FR-KB-05, FR-KB-06, FR-VUL-01<br>Thực thi: INT-04; API-K-36..38 | SRS §10.4, URD §4.2 |
| <a id="BRL-08"></a>BRL-08 | Finding khi phiên bản trong affected range; duy nhất | ALG: ALG-04<br>FR: FR-VUL-01, FR-CI-05<br>Thực thi: INT-04 (`dedupe_key`) | SRS §10.4, URD §4.2 |
| <a id="BRL-09"></a>BRL-09 | Severity theo thứ tự CVSS v4.0, v3.1, nguồn | ALG: ALG-02, ALG-12<br>FR: FR-VUL-02<br>Thực thi: API-V-01 (`priority_inputs`) | SRS §10.4, URD §4.2 |
| <a id="BRL-10"></a>BRL-10 | Priority P1 đến P4 | ALG: ALG-02<br>FR: FR-VUL-02, FR-UI-01<br>Thực thi: API-V-01, API-V-17 | SRS §10.4, URD §4.2 |
| <a id="BRL-11"></a>BRL-11 | SLA theo severity; KEV luôn SLA Critical | ALG: ALG-03<br>FR: FR-ADM-04, FR-VUL-03, FR-UI-04<br>Thực thi: Policy `sla`; job SLA 15 phút | SRS §10.4, URD §4.2 |
| <a id="BRL-12"></a>BRL-12 | Not affected cần 1 trong 5 lý do VEX, mô tả ≥ 20 ký tự | ALG: ALG-04<br>FR: FR-VUL-04, FR-VUL-05, FR-VUL-08<br>Thực thi: API-V-04; 422 `validation_failed` | SRS §10.4, URD §4.2 |
| <a id="BRL-13"></a>BRL-13 | Not affected của Critical hoặc KEV cần SEC duyệt | ALG: —<br>FR: FR-VUL-04<br>Thực thi: API-V-13; 403 `separation_of_duties` | SRS §10.4, URD §4.2 |
| <a id="BRL-14"></a>BRL-14 | Risk accepted phải gắn Decision Approved còn hạn | ALG: —<br>FR: FR-VUL-04, FR-GOV-01 (cột BRL của FR chưa ghi, Mục 10.6)<br>Thực thi: API-V-06; 409 `decision_not_approved` | SRS §10.4, URD §4.2 |
| <a id="BRL-15"></a>BRL-15 | Tự Fixed khi snapshot mới hết; tự mở lại khi tái xuất hiện | ALG: ALG-04<br>FR: FR-VUL-01, FR-VUL-07<br>Thực thi: API-V-07, API-V-08; sự kiện từ API-I-01 | SRS §10.4, URD §4.2 |
| <a id="BRL-16"></a>BRL-16 | 75% SLA nhắc, 100% breached và chuyển cấp | ALG: ALG-03<br>FR: FR-VUL-03, FR-GOV-05<br>Thực thi: Job SLA; Alert `sla_reminder`, `sla_breached` | SRS §10.4, URD §4.2 |
| <a id="BRL-17"></a>BRL-17 | Thứ tự nguồn ngày EOL | ALG: ALG-09, ALG-12<br>FR: FR-KB-03, FR-LCM-01, FR-LCM-03, FR-CAT-03, FR-UI-03<br>Thực thi: API-K-34, API-K-35 | SRS §10.4, URD §4.2 |
| <a id="BRL-18"></a>BRL-18 | Cảnh báo EOL T-180, T-90, T-30, T-0, rồi hằng tuần | ALG: ALG-09<br>FR: FR-LCM-02<br>Thực thi: Job hằng ngày; Alert `eol` | SRS §10.4, URD §4.2 |
| <a id="BRL-19"></a>BRL-19 | EOL ở prod là vi phạm gate trừ khi có Decision | ALG: ALG-08, ALG-09<br>FR: FR-LCM-02, FR-CI-02<br>Thực thi: API-C-01 | SRS §10.4, URD §4.2 |
| <a id="BRL-20"></a>BRL-20 | Health dưới 40 hoặc ngừng bảo trì cảnh báo Medium | ALG: ALG-10<br>FR: FR-LCM-04, FR-CAT-03<br>Thực thi: API-V-24 | SRS §10.4, URD §4.2 |
| <a id="BRL-21"></a>BRL-21 | Version lag theo component cảnh báo Medium | ALG: ALG-09<br>FR: FR-LCM-05<br>Thực thi: API-V-23 | SRS §10.4, URD §4.2 |
| <a id="BRL-22"></a>BRL-22 | License chuẩn hoá SPDX; OR chọn ít hạn chế, AND chọn hạn chế nhất | ALG: ALG-05<br>FR: FR-ADM-04, FR-LIC-01<br>Thực thi: API-K-40, API-K-41 | SRS §10.4, URD §4.2 |
| <a id="BRL-23"></a>BRL-23 | Không xác định license: NOASSERTION, nhóm Unknown, block | ALG: ALG-05<br>FR: FR-ADM-04, FR-LIC-01<br>Thực thi: Policy `license` | SRS §10.4, URD §4.2 |
| <a id="BRL-24"></a>BRL-24 | Action license tra theo ma trận | ALG: ALG-05<br>FR: FR-ADM-04, FR-LIC-02, FR-LIC-05, FR-SEL-03, FR-GOV-07<br>Thực thi: API-V-25, API-P-30..37 | SRS §10.4, URD §4.2 |
| <a id="BRL-25"></a>BRL-25 | Phiên bản đổi nhóm license là ranh giới; gate chặn | ALG: ALG-05, ALG-06<br>FR: FR-LIC-03, FR-CI-02<br>Thực thi: API-V-26..28, API-C-01 | SRS §10.4, URD §4.2 |
| <a id="BRL-26"></a>BRL-26 | License source-available lưu văn bản điều khoản | ALG: ALG-06<br>FR: FR-LIC-03, FR-LIC-04<br>Thực thi: API-K-43..45 | SRS §10.4, URD §4.2 |
| <a id="BRL-27"></a>BRL-27 | Ring Adopt, Trial, Hold, Banned | ALG: ALG-08<br>FR: FR-CAT-02, FR-SEL-01, FR-SEL-06, FR-CI-02, FR-UI-05<br>Thực thi: API-T-01, API-T-05 | SRS §10.4, URD §4.2 |
| <a id="BRL-28"></a>BRL-28 | Đổi ring qua Change Request do ARB duyệt | ALG: —<br>FR: FR-CAT-05<br>Thực thi: API-T-13; 403 `separation_of_duties` | SRS §10.4, URD §4.2 |
| <a id="BRL-29"></a>BRL-29 | `next_review_at` ≤ 90 ngày, quá hạn Cần rà soát | ALG: —<br>FR: FR-CAT-01, FR-CAT-06<br>Thực thi: Job hằng ngày; Alert | SRS §10.4, URD §4.2 |
| <a id="BRL-30"></a>BRL-30 | Công nghệ ngoài Catalog chỉ ở môi trường không phải prod | ALG: ALG-08<br>FR: FR-CAT-05, FR-SEL-01, FR-SEL-05<br>Thực thi: API-T-10, API-C-01 | SRS §10.4, URD §4.2 |
| <a id="BRL-31"></a>BRL-31 | Review Report hiệu lực 30 ngày | ALG: —<br>FR: FR-SEL-04<br>Thực thi: API-T-41 (`valid_until`) | SRS §10.4, URD §4.2 |
| <a id="BRL-32"></a>BRL-32 | Ứng viên vi phạm ràng buộc bị đánh dấu đỏ | ALG: —<br>FR: FR-CAT-08, FR-SEL-01..03<br>Thực thi: API-T-25 (`flags[].level`), API-T-28 | SRS §10.4, URD §4.2 |
| <a id="BRL-33"></a>BRL-33 | Decision bắt buộc trường, hạn ≤ 12 tháng | ALG: ALG-11<br>FR: FR-GOV-01<br>Thực thi: INT-05; API-G-01; 422 `expiry_too_long` | SRS §10.4, URD §4.2 |
| <a id="BRL-34"></a>BRL-34 | Thẩm quyền duyệt theo loại Decision | ALG: ALG-11<br>FR: FR-GOV-01, FR-AST-10<br>Thực thi: API-G-06 (`required_roles`) | SRS §10.4, URD §4.2 |
| <a id="BRL-35"></a>BRL-35 | Người tạo không được duyệt | ALG: ALG-11<br>FR: FR-GOV-01<br>Thực thi: 403 `separation_of_duties` | SRS §10.4, URD §4.2 |
| <a id="BRL-36"></a>BRL-36 | Hết hạn hoặc thu hồi quay về vi phạm, nhắc trước 14 ngày | ALG: ALG-03, ALG-11<br>FR: FR-GOV-02<br>Thực thi: API-G-08, API-G-09; job hằng ngày | SRS §10.4, URD §4.2 |
| <a id="BRL-37"></a>BRL-37 | Audit chỉ thêm, không sửa | ALG: —<br>FR: FR-GOV-03<br>Thực thi: INT-08; trigger CSDL; API-G-40..44 | SRS §10.4, URD §4.2 |
| <a id="BRL-38"></a>BRL-38 | Routing thông báo, không gửi trùng | ALG: —<br>FR: FR-GOV-04<br>Thực thi: `alerts.dedupe_key`; API-G-15..23 | SRS §10.4, URD §4.2 |
| <a id="BRL-39"></a>BRL-39 | Không tắt Critical, KEV, SLA breached; email luôn gửi | ALG: —<br>FR: FR-GOV-04<br>Thực thi: API-G-21 (`channels` luôn có email) | SRS §10.4, URD §4.2 |
| <a id="BRL-40"></a>BRL-40 | Gate fail | ALG: ALG-08<br>FR: FR-ADM-04, FR-CI-01, FR-CI-02, FR-AST-18<br>Thực thi: API-C-01, API-C-07 | SRS §10.4, URD §4.2 |
| <a id="BRL-41"></a>BRL-41 | Gate warn | ALG: ALG-08<br>FR: FR-ADM-04, FR-CI-01, FR-CI-02, FR-AST-18<br>Thực thi: API-C-01, API-C-07 | SRS §10.4, URD §4.2 |
| <a id="BRL-42"></a>BRL-42 | Ghi đè gate chỉ qua Decision còn hạn | ALG: ALG-08, ALG-11<br>FR: FR-CI-01, FR-CI-03, FR-AST-19<br>Thực thi: API-C-07, API-G-06 | SRS §10.4, URD §4.2 |
| <a id="BRL-43"></a>BRL-43 | Không phản hồi 30 giây: fail-open hoặc fail-closed | ALG: ALG-08<br>FR: FR-CI-01, FR-CI-03<br>Thực thi: API-C-04; CLI exit code 30 | SRS §10.4, URD §4.2 |
| <a id="BRL-44"></a>BRL-44 | Asset định danh theo SHA-256 | ALG: ALG-07<br>FR: FR-UI-09, FR-AST-01, FR-AST-08<br>Thực thi: API-S-01, API-S-14 | SRS §10.4, URD §4.2 |
| <a id="BRL-45"></a>BRL-45 | Không rõ license và ngoài Asset Catalog: `unknown_source` | ALG: ALG-07<br>FR: FR-UI-10, FR-AST-04, FR-AST-16<br>Thực thi: API-S-37 | SRS §10.4, URD §4.2 |
| <a id="BRL-46"></a>BRL-46 | Tài sản thương mại hợp lệ khi có Entitlement | ALG: ALG-07<br>FR: FR-UI-08, FR-AST-12, FR-AST-14<br>Thực thi: API-S-25..33 | SRS §10.4, URD §4.2 |
| <a id="BRL-47"></a>BRL-47 | Entitlement bắt buộc licensee, điều khoản, chứng từ, hạn | ALG: ALG-07<br>FR: FR-UI-08, FR-AST-12<br>Thực thi: INT-07; API-S-26, API-S-29 | SRS §10.4, URD §4.2 |
| <a id="BRL-48"></a>BRL-48 | Cảnh báo Entitlement T-60, T-30, T-7, T-0 | ALG: ALG-07<br>FR: FR-AST-13<br>Thực thi: Job hằng ngày; Alert `entitlement_expiring` | SRS §10.4, URD §4.2 |
| <a id="BRL-49"></a>BRL-49 | Nghĩa vụ ghi công đạt khi có trong Credits | ALG: ALG-07<br>FR: FR-AST-15<br>Thực thi: API-S-38..42 | SRS §10.4, URD §4.2 |
| <a id="BRL-50"></a>BRL-50 | Bàn giao: Entitlement TECHWISE không chuyển nhượng xử lý trước | ALG: ALG-07<br>FR: FR-AST-17<br>Thực thi: API-S-43..47; 409 `handover_incomplete` | SRS §10.4, URD §4.2 |
| <a id="BRL-51"></a>BRL-51 | Tài sản tự tạo khai người tạo; của khách cần văn bản xác nhận | ALG: ALG-07<br>FR: FR-AST-07<br>Thực thi: API-S-10, API-S-12 | SRS §10.4, URD §4.2 |
| <a id="BRL-52"></a>BRL-52 | Dữ liệu dự án chỉ hiển thị cho thành viên và vai trò hệ thống | ALG: —<br>FR: FR-ADM-01, FR-ADM-02, FR-INV-01, FR-GOV-06, FR-UI-01..03, FR-UI-06..08<br>Thực thi: Cột Quyền của mọi endpoint; 404 khi không được thấy; lọc `project_id` | SRS §10.4, URD §4.2 |
| <a id="BRL-53"></a>BRL-53 | API token gắn một dự án, scope tối thiểu, hạn ≤ 365 ngày | ALG: —<br>FR: FR-ADM-03, FR-INV-02, FR-CI-04<br>Thực thi: API-P-22..29; CHECK `expires_at`; 422 `expiry_too_long` | SRS §10.4, URD §4.2 |
| <a id="FACT-0fbdd99a38"></a>— | Điều kiện chạy job purge: Job dọn (purge) chỉ chạy khi Policy retention ở trạng thái Active và có xác nhận của LEG | predicate: Job dọn (purge) chỉ chạy khi Policy retention ở trạng thái Active và có xác nhận của LEG | SRS §7.6 |
| <a id="FACT-5a112b3d4b"></a>— | Hiệu lực ma trận license: ma trận license chưa có hiệu lực chính thức trước khi LEG xác nhận (C-06) | predicate: ma trận license chưa có hiệu lực chính thức trước khi LEG xác nhận (C-06) | SAD §7.6 |
| <a id="FACT-5c0aff7fa0"></a>— | Trạng thái Finding: Finding có 3 trạng thái mở (bộ đếm thời gian SLA chạy) và 3 trạng thái đóng; mọi trạng thái đóng đều có đường quay lại Open khi điều kiện đóng không còn đúng | predicate: Finding có 3 trạng thái mở (bộ đếm thời gian SLA chạy) và 3 trạng thái đóng; mọi trạng thái đóng đều có đường quay lại Open khi điều kiện đóng không còn đúng | URD §4.4 |
| <a id="FACT-8b186bdc41"></a>— | Fail-closed khi SVC-02 không phản hồi: khi SVC-02 không phản hồi dùng snapshot cuối cùng tối đa 15 phút; quá hạn thì từ chối thao tác ghi, vẫn cho đọc theo snapshot cũ | predicate: khi SVC-02 không phản hồi dùng snapshot cuối cùng tối đa 15 phút; quá hạn thì từ chối thao tác ghi, vẫn cho đọc theo snapshot cũ | SAD §6.4 |
| <a id="FACT-39f805f126"></a>— | Audit chuyển trạng thái: Mọi chuyển trạng thái ghi audit log (ENT-44) kèm trước/sau và người hoặc tiến trình thực hiện | predicate: Mọi chuyển trạng thái ghi audit log (ENT-44) kèm trước/sau và người hoặc tiến trình thực hiện | URD §4.7 |
| <a id="FACT-95f031570e"></a>— | Xử lý lỗi nguồn ngoài: Nguồn lỗi thì giữ last-known-good, không xoá dữ liệu đã công bố; lỗi quá 24 giờ thì hiển thị nhãn dữ liệu cũ, phát Alert stale_source, dùng mirror OSV hoặc NVD nếu đã bật | predicate: Nguồn lỗi thì giữ last-known-good, không xoá dữ liệu đã công bố; lỗi quá 24 giờ thì hiển thị nhãn dữ liệu cũ, phát Alert stale_source, dùng mirror OSV hoặc NVD nếu đã bật | SRS §4.3 |
| <a id="FACT-524c83c013"></a>— | Thứ tự đổi chế độ token: (a) Gateway và Core cùng chấp nhận cả hai dạng; (b) đổi token.format; (c) chờ hết hạn token cũ (tối đa 10 phút với JWT, 8 giờ với phiên refresh); (d) gỡ dạng cũ | predicate: (a) Gateway và Core cùng chấp nhận cả hai dạng; (b) đổi token.format; (c) chờ hết hạn token cũ (tối đa 10 phút với JWT, 8 giờ với phiên refresh); (d) gỡ dạng cũ | SAD §6.8 |
| <a id="FACT-929a6e9c8f"></a>— | Ngưỡng lỗi lô nhập: Lô có tỉ lệ bản ghi bị từ chối vượt limits.import_error_threshold (mặc định 1%) thì Failed và không công bố | predicate: Lô có tỉ lệ bản ghi bị từ chối vượt limits.import_error_threshold (mặc định 1%) thì Failed và không công bố | SRS §4.3 |
| <a id="FACT-337170f462"></a>— | Điều kiện thêm BFF: có client thứ hai (mobile, agent) cần tổng hợp dữ liệu khác với SPA, hoặc SPA phải gọi quá nhiều endpoint cho một màn hình | predicate: có client thứ hai (mobile, agent) cần tổng hợp dữ liệu khác với SPA, hoặc SPA phải gọi quá nhiều endpoint cho một màn hình | SAD §5.6 |
| <a id="FACT-707503a85a"></a>— | Ghi audit trong CRUD: Mọi UC có thao tác ghi đều ghi thêm ENT-44 (AuditLog); D = xoá hoặc đóng, không xoá vật lý | predicate: Mọi UC có thao tác ghi đều ghi thêm ENT-44 (AuditLog); D = xoá hoặc đóng, không xoá vật lý | URD §4.8 |
| <a id="FACT-122935621c"></a>— | Vòng đời Decision: Decision tạo ở Draft, gửi duyệt sang Pending, duyệt xong sang Approved hoặc Rejected; khi Approved, DecisionApplier của module đích áp hiệu lực trong cùng giao dịch với đổi trạng thái Decision, lỗi áp thì cả hai quay lui; nhắc trước 14 ngày khi sắp hết hạn; hết hạn chuyển Expired; thay thế bằng Decision mới đặt bản cũ Superseded | predicate: Decision tạo ở Draft, gửi duyệt sang Pending, duyệt xong sang Approved hoặc Rejected; khi Approved, DecisionApplier của module đích áp hiệu lực trong cùng giao dịch với đổi trạng thái Decision, lỗi áp thì cả hai quay lui; nhắc trước 14 ngày khi sắp hết hạn; hết hạn chuyển Expired; thay thế bằng Decision mới đặt bản cũ Superseded | SAD §8.5 |
| <a id="FACT-5734404486"></a>— | Chống gửi trùng thông báo: không gửi trùng trong 24 giờ; digest hằng ngày lúc 08:30 | predicate: không gửi trùng trong 24 giờ; digest hằng ngày lúc 08:30 | SAD §8.4 |
| <a id="FACT-8389739387"></a>— | Thử lại webhook: thử lại tối đa 8 lần trong 24 giờ theo bậc 1 phút, 5 phút, 30 phút, 2 giờ, 6 giờ, 12 giờ, 24 giờ; từ 5 lần lỗi liên tiếp đầu mối đặt Failing; hết lượt thì Dead_lettered | predicate: thử lại tối đa 8 lần trong 24 giờ theo bậc 1 phút, 5 phút, 30 phút, 2 giờ, 6 giờ, 12 giờ, 24 giờ; từ 5 lần lỗi liên tiếp đầu mối đặt Failing; hết lượt thì Dead_lettered | SAD §8.4 |
| <a id="FACT-b56aa2bf7c"></a>— | Lọc dự án ở tầng repository: mọi truy vấn danh sách thêm điều kiện `project_id IN (dự án chủ thể được xem)` ở tầng repository, không lọc sau khi đã lấy dữ liệu | predicate: mọi truy vấn danh sách thêm điều kiện `project_id IN (dự án chủ thể được xem)` ở tầng repository, không lọc sau khi đã lấy dữ liệu | SAD §6.3 |
| <a id="FACT-b61e4285fa"></a>— | Ưu tiên MoSCoW: M = bắt buộc cho MVP (Phase 1); S = nên có (Phase 2); C = có thể có (Phase 3). Động từ "phải" = bắt buộc; "nên" = khuyến nghị. | predicate: M = bắt buộc cho MVP (Phase 1); S = nên có (Phase 2); C = có thể có (Phase 3). Động từ "phải" = bắt buộc; "nên" = khuyến nghị. | URD §1.5 |
| <a id="FACT-e573d8cece"></a>— | Email dự phòng và cảnh báo tồn đọng: Email là kênh dự phòng khi kênh khác lỗi; outbox và hàng đợi tồn đọng quá ngưỡng sinh cảnh báo vận hành | predicate: Email là kênh dự phòng khi kênh khác lỗi; outbox và hàng đợi tồn đọng quá ngưỡng sinh cảnh báo vận hành | SAD §8.4 |
| <a id="FACT-fb7cbd6c76"></a>— | Ưu tiên URD khi mâu thuẫn: khi có mâu thuẫn, URD này được ưu tiên cho phần yêu cầu người dùng và quy tắc nghiệp vụ | predicate: khi có mâu thuẫn, URD này được ưu tiên cho phần yêu cầu người dùng và quy tắc nghiệp vụ | URD §2 |

### 3.3 Luồng xử lý, thuật toán, bảng quyết định

> **[THIẾU] Luồng chính/phụ/lỗi, thuật toán, bảng quyết định, bất biến.** chưa có trong dữ liệu đã trích từ tài liệu nguồn; 13 thuật toán (mã ALG-…) được nhắc tới nhưng chưa có mô tả (xem phụ lục A.2).
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §6.1 “Luồng đăng nhập của người dùng (OIDC Authorization Code + PKCE)”; SAD §8 “Góc nhìn tiến trình và tích hợp”; SAD §8.2 “Pipeline dữ liệu ngoài”; SAD §8.3 “Matching và re-matching”; SAD §8.5 “Quy trình quyết định (Decision)”; SAD §8.6 “Job định kỳ”; SAD §11.4 “Quy trình đưa ứng viên qua PBOM”; URD §4.1 “Quy trình nghiệp vụ”

<a id="ch-4"></a>
## 4. Hợp đồng giao diện

### 4.1 Giao diện đã có mô tả

| Giao diện | Chi tiết | Nguồn |
|---|---|---|
| <a id="FACT-7b519fa0cd"></a>GET /events long-poll | cursor: cursor<br>method: GET<br>path: /events<br>timeout: 25 giây | SAD §8.4 |
| <a id="FACT-a5372a1327"></a>Metric bắt buộc | statement: Metric bắt buộc: độ trễ và lỗi theo route (p50, p95, p99); độ trễ gate; độ sâu hàng đợi job; độ trễ outbox; tuổi dữ liệu theo nguồn; tỷ lệ Rejected của pipeline; số Finding mở theo Priority; webhook Failing | SAD §10.2 |

### 4.2 Giao diện được nhắc tới nhưng chưa có hợp đồng

| Nhóm mã | Số mã được nhắc | Ví dụ (mã ← nhắc ở) |
|---|---|---|
| API | 8 | API-C ← M-04, API-G ← M-07, API-I ← M-01 |
| API-C | 9 | API-C-01 ← FR-INV-03, API-C-02 ← FR-INV-03, API-C-03 ← FR-CI-03 |
| API-G | 37 | API-G-01 ← FR-GOV-01, API-G-02 ← FR-UI-02, API-G-03 ← ENT-23 |
| API-I | 17 | API-I-01 ← FR-INV-02, API-I-02 ← ENT-17, API-I-03 ← FR-INV-07 |
| API-K | 41 | API-K-01 ← ENT-07, API-K-02 ← ENT-07, API-K-03 ← FR-KB-01 |
| API-P | 33 | API-P-01 ← FR-ADM-01, API-P-02 ← FR-ADM-03, API-P-03 ← FR-ADM-02 |
| API-S | 54 | API-S-01 ← FR-INV-03, API-S-02 ← ENT-39, API-S-03 ← ENT-39 |
| API-T | 37 | API-T-01 ← FR-CAT-01, API-T-02 ← FR-LCM-01, API-T-03 ← FR-CAT-03 |
| API-V | 26 | API-V-01 ← FR-VUL-01, API-V-02 ← FR-VUL-01, API-V-03 ← FR-VUL-04 |

> **[THIẾU] Hợp đồng của từng endpoint (đường dẫn, phương thức, đầu vào/ra, mã lỗi, xác thực, phiên bản, tương thích ngược, bên tiêu thụ).** Nguồn mới nêu mã giao diện, chưa có hợp đồng chi tiết trong dữ liệu đã trích.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §5.2 “Module của Core API”; SAD §5.4 “Định tuyến Gateway”; SAD §6 “Góc nhìn API Gateway, xác thực và phân quyền”; SAD §8.4 “Thông báo, webhook và sự kiện”; SRS §4 “Giao diện bên ngoài”; SRS §4.1 “Giao diện người dùng”; SRS §4.2 “Giao diện phần cứng”; SRS §4.3 “Giao diện phần mềm”

<a id="ch-5"></a>
## 5. Dữ liệu

### 5.1 Thực thể

| Mã | Thực thể | Mô tả | Định danh | API đọc | API ghi | Ghi chú | Nguồn |
|---|---|---|---|---|---|---|---|
| <a id="ENT-01"></a>ENT-01 | User | Người dùng đồng bộ từ IdP | idp_subject | API-P-01, API-P-03, API-P-04 | API-P-05 | Tạo ở lần đăng nhập SSO đầu (UC-01), không có POST /users | SRS §10.3, URD §4.5 |
| <a id="ENT-02"></a>ENT-02 | RoleAssignment | Vai trò của user theo phạm vi hệ thống hoặc dự án | (user, scope, role) | API-P-06 | API-P-07, API-P-08 | Còn sinh tự động từ idp_mapping khi đăng nhập | SRS §10.3, URD §4.5 |
| <a id="ENT-03"></a>ENT-03 | Project | Dự án | code | API-P-09, API-P-11, API-C-03, API-C-05, API-S-07 | API-P-10, API-P-12..14, API-C-04, API-S-08 | Xoá vật lý bị cấm (INT-09), dùng chuyển Archived | SRS §10.3, URD §4.5 |
| <a id="ENT-04"></a>ENT-04 | Environment | Môi trường của dự án | (project, name) | API-P-15 | API-P-16, API-P-17 | Retired thay cho xoá | SRS §10.3, URD §4.5 |
| <a id="ENT-05"></a>ENT-05 | Service | Service, module hoặc ứng dụng trong dự án | (project, name) | API-P-18, API-P-20 | API-P-19, API-P-21 | Retired thay cho xoá | SRS §10.3, URD §4.5 |
| <a id="ENT-06"></a>ENT-06 | ApiToken | Token tích hợp CI hoặc hệ thống ngoài | id (chỉ lưu hash) | API-P-22, API-P-26 | API-P-23..25, API-P-27..29 | Gồm oauth_client; secret hiển thị một lần | SRS §10.3, URD §4.5 |
| <a id="ENT-07"></a>ENT-07 | Connector | Module cắm thêm kết nối một loại nguồn/hệ thống ngoài (NFR-MA-05) | (name, version) | API-K-01, API-K-02 | API-K-07, API-K-24 | Đăng ký phiên bản mới bằng triển khai gói connector, không có POST /connectors (Mục 10.6) | SRS §10.3, URD §4.5 |
| <a id="ENT-08"></a>ENT-08 | DataSource | Thể hiện cấu hình của một Connector | id | API-K-03, API-K-05, API-K-13 | API-K-04, API-K-06, API-K-08..11 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-09"></a>ENT-09 | ImportJob | Một lần đồng bộ hoặc import thủ công (NFR-CO-06) | id | API-K-14..16 | API-K-11, API-K-12, API-K-17..19, API-K-20, API-K-21 | Tạo bởi lịch, bởi API-K-11 chạy tay hoặc bởi ingest | SRS §10.3, URD §4.5 |
| <a id="ENT-10"></a>ENT-10 | RawRecord | Bản ghi nguyên bản từ nguồn kèm provenance | (source, source_key, checksum) | API-K-16, API-K-23 | API-K-19 | Bản ghi thô do pipeline ghi (ALG-13); chỉ có thao tác reprocess | SRS §10.3, URD §4.5 |
| <a id="ENT-11"></a>ENT-11 | Component | Thành phần phần mềm định danh duy nhất | purl_base | API-K-25, API-K-27, API-K-30 | API-K-26, API-K-28, API-K-29, API-K-31 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-12"></a>ENT-12 | ComponentVersion | Một phiên bản cụ thể | purl đầy đủ | API-K-32, API-K-33 | API-K-21 | Sinh từ ALG-01 hoặc pipeline metadata; không có ghi tay (Mục 10.6) | SRS §10.3, URD §4.5 |
| <a id="ENT-13"></a>ENT-13 | ReleaseLine | Dòng phiên bản và vòng đời hỗ trợ | (component, line) | API-K-34, API-V-21, API-V-22, API-T-02 | API-K-35 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-14"></a>ENT-14 | Vulnerability | CVE, GHSA, GO-ID hoặc advisory vendor đã chuẩn hoá | id chính + aliases | API-K-36..38 | API-K-39, API-K-20 | Nạp từ pipeline (API-K-20 với ingest:advisory) | SRS §10.3, URD §4.5 |
| <a id="ENT-15"></a>ENT-15 | AffectedRange | Phạm vi phiên bản bị ảnh hưởng của một Vulnerability | (vuln, component, range) | API-K-37 | API-K-20, API-K-39 | Đọc kèm Vulnerability; ghi qua pipeline và gán advisory | SRS §10.3, URD §4.5 |
| <a id="ENT-16"></a>ENT-16 | License | Định danh license theo SPDX | spdx_id | API-K-40, API-K-41 | API-K-42 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-17"></a>ENT-17 | SbomSnapshot | Một lần nộp SBOM | (service, env, hash) | API-I-02..06 | API-I-01 | Bất biến sau khi tạo; chỉ nộp và đọc | SRS §10.3, URD §4.5 |
| <a id="ENT-18"></a>ENT-18 | Usage | Thành phần được dùng ở đâu | (service, env, component_version) | API-I-08, API-I-09, API-I-18 | API-I-07, API-I-10..12, API-I-16, API-I-01, API-T-32 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-19"></a>ENT-19 | UsageContext | Ngữ cảnh sử dụng ảnh hưởng đánh giá rủi ro | (target, version) | API-I-15, API-I-09 | API-I-13, API-I-14 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-20"></a>ENT-20 | LicenseObservation | License ghi nhận ở một phiên bản | (version, source) | API-K-43, API-V-25 | API-K-44, API-K-45 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-21"></a>ENT-21 | Finding | Vulnerability × Usage; có loại license khi vi phạm policy | (type, vuln, usage) | API-V-01, API-V-02, API-V-11, API-V-17, API-V-18 | API-V-03..10 | Tạo bởi ALG-04, không có POST /findings | SRS §10.3, URD §4.5 |
| <a id="ENT-22"></a>ENT-22 | VexStatement | Phán đoán VEX của Finding | id | API-V-12, API-V-14..16 | API-V-04, API-V-05, API-V-13 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-23"></a>ENT-23 | Decision | ADR hoặc Risk acceptance có hạn | id | API-G-02, API-G-03, API-G-10, API-G-11 | API-G-01, API-G-04..09 | Không sửa nội dung sau Approved (NFR-SE-05) | SRS §10.3, URD §4.5 |
| <a id="ENT-24"></a>ENT-24 | Policy | SLA profile, license policy, gate rule, asset rule (có phiên bản) | (type, version) | API-P-30, API-P-31, API-P-33, API-P-36, API-P-38 | API-P-32, API-P-34, API-P-35, API-P-37 | Phiên bản bất biến; kích hoạt sau simulate | SRS §10.3, URD §4.5 |
| <a id="ENT-25"></a>ENT-25 | Evaluation | Kết quả policy gate | id | API-C-02, API-C-06, API-C-07 | API-C-01, API-I-01 | Bất biến sau khi tạo | SRS §10.3, URD §4.5 |
| <a id="ENT-26"></a>ENT-26 | Alert | Cảnh báo đã phát | id | API-G-15, API-G-16, API-G-22, API-G-23 | API-G-17..19 | Sinh bởi hệ thống; người dùng chỉ đổi trạng thái | SRS §10.3, URD §4.5 |
| <a id="ENT-27"></a>ENT-27 | NotificationPreference | Cấu hình kênh và loại sự kiện user muốn nhận | (user, event_type) | API-G-20 | API-G-21 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-28"></a>ENT-28 | CatalogEntry | Mục Tech Radar | component_id | API-T-01..03, API-T-05, API-T-06, API-T-09, API-T-24, API-T-26 | API-T-04, API-T-08 | Ring đổi qua Change Request (API-T-13) | SRS §10.3, URD §4.5 |
| <a id="ENT-29"></a>ENT-29 | CatalogChangeRequest | Đề xuất thêm hoặc đổi ring/phiên bản Catalog | id | API-T-11, API-T-12 | API-T-10, API-T-13, API-T-14, API-T-33 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-30"></a>ENT-30 | StackProposal | Đề xuất tech stack cho dự án mới | id | API-T-29, API-T-30, API-T-41 | API-T-28, API-T-31..34 | Chưa lưu kết quả so sánh (Mục 10.6) | SRS §10.3, URD §4.5 |
| <a id="ENT-31"></a>ENT-31 | Baseline | Version-pin baseline xuất từ Catalog | (catalog_revision, format) | API-T-15..17, API-T-19..21 | API-T-18 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-32"></a>ENT-32 | Report | Báo cáo hoặc gói tuân thủ xuất ra | id | API-T-35, API-T-37..39 | API-T-36, API-T-40, API-G-42 | Tệp xuất giữ 7 ngày | SRS §10.3, URD §4.5 |
| <a id="ENT-33"></a>ENT-33 | Asset | Tài sản duy nhất theo nội dung (font, icon, ảnh, UI kit…) | sha256 | API-S-09, API-S-11, API-S-14 | API-S-01, API-S-10, API-S-12 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-34"></a>ENT-34 | AssetLicenseTerm | Điều khoản chuẩn hoá của một loại license tài sản | (code, version) | API-S-20, API-S-22 | API-S-21, API-S-23, API-S-24 | Duyệt theo BRL-34 | SRS §10.3, URD §4.5 |
| <a id="ENT-35"></a>ENT-35 | Entitlement | Quyền sử dụng đã mua hoặc được cấp | id | API-S-15, API-S-25, API-S-27, API-S-30, API-S-32, API-S-33 | API-S-26, API-S-28, API-S-29, API-S-31 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-36"></a>ENT-36 | AssetUsage | Tài sản được dùng ở đâu | (asset, service) | API-S-16, API-S-17 | API-S-18, API-S-19, API-S-01, API-S-10 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-37"></a>ENT-37 | AssetFinding | Vi phạm hoặc việc cần xử lý về tài sản | (usage, rule) | API-S-13, API-S-34, API-S-35 | API-S-36 | Sinh bởi ALG-07, không có POST | SRS §10.3, URD §4.5 |
| <a id="ENT-38"></a>ENT-38 | AssetCatalogEntry | Tài sản hoặc nguồn được duyệt sẵn | id | API-S-37 | API-S-52..54 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-39"></a>ENT-39 | AssetScanRun | Một lần quét tài sản | id | API-S-02, API-S-03, API-S-05, API-S-06 | API-S-01, API-S-04 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-40"></a>ENT-40 | CreditsDocument | Tài liệu credits/attribution sinh tự động | (project, generated_at) | API-S-39, API-S-40, API-S-42 | API-S-38 | API-S-41 chỉ kiểm tra ghi công | SRS §10.3, URD §4.5 |
| <a id="ENT-41"></a>ENT-41 | HandoverChecklist | Checklist bàn giao tài sản và entitlement | project_id | API-S-44, API-S-47 | API-S-43, API-S-45, API-S-46 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-42"></a>ENT-42 | FigmaConnection | Liên kết team/file Figma với dự án | (project, team) | API-S-49 | API-S-48, API-S-50, API-S-55, API-S-56 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-43"></a>ENT-43 | Evidence | Tệp chứng từ (hoá đơn, certificate, văn bản Additional Use Grant) | sha256 + owner | API-G-13, API-G-14, API-S-30 | API-G-12, API-S-29 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-44"></a>ENT-44 | AuditLog | Nhật ký bất biến mọi thay đổi | id (append-only) | API-G-40, API-G-41, API-G-43, API-G-44, API-P-42 | API-G-42 | Chỉ ghi thêm bởi hệ thống (BRL-37); API chỉ xuất và kiểm tra toàn vẹn | SRS §10.3, URD §4.5 |
| <a id="ENT-45"></a>ENT-45 | WebhookSubscription | Đăng ký nhận sự kiện cho hệ thống ngoài (NFR-CO-07) | id | API-G-29, API-G-30, API-G-35, API-G-36 | API-G-28, API-G-31..34 | — | SRS §10.3, URD §4.5 |
| <a id="ENT-46"></a>ENT-46 | EventDelivery | Một lần gửi sự kiện tới webhook | (subscription, event) | API-G-35, API-G-37, API-G-38 | API-G-39 | Sinh bởi hệ thống khi có sự kiện | SRS §10.3, URD §4.5 |

### 5.2 Thuộc tính của từng thực thể

Mức mô tả: logic. Kiểu vật lý chưa có trong dữ liệu đã trích từ tài liệu nguồn ở 261/261 thuộc tính (đánh dấu **THIẾU**). Cột 'Khoá' đánh dấu thuộc tính trùng tên với phần 'định danh' của thực thể trong nguồn.

#### 5.2.1 User (ENT-01)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-01:attribute:display_name"></a>display_name | — | **THIẾU** | URD §4.5 |
| <a id="ENT-01:attribute:email"></a>email | — | **THIẾU** | URD §4.5 |
| <a id="ENT-01:attribute:groups"></a>groups | — | **THIẾU** | URD §4.5 |
| <a id="ENT-01:attribute:idp_subject"></a>idp_subject | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-01:attribute:last_login"></a>last_login | — | **THIẾU** | URD §4.5 |

#### 5.2.2 RoleAssignment (ENT-02)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-02:attribute:expires_at"></a>expires_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-02:attribute:granted_by"></a>granted_by | — | **THIẾU** | URD §4.5 |
| <a id="ENT-02:attribute:project_id"></a>project_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-02:attribute:role"></a>role | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-02:attribute:user_id"></a>user_id | — | **THIẾU** | URD §4.5 |

#### 5.2.3 Project (ENT-03)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-03:attribute:code"></a>code | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-03:attribute:customer"></a>customer | — | **THIẾU** | URD §4.5 |
| <a id="ENT-03:attribute:monetized"></a>monetized | — | **THIẾU** | URD §4.5 |
| <a id="ENT-03:attribute:name"></a>name | — | **THIẾU** | URD §4.5 |
| <a id="ENT-03:attribute:owner_ids"></a>owner_ids | — | **THIẾU** | URD §4.5 |
| <a id="ENT-03:attribute:sla_profile_id"></a>sla_profile_id | — | **THIẾU** | URD §4.5 |

#### 5.2.4 Environment (ENT-04)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-04:attribute:criticality"></a>criticality | — | **THIẾU** | URD §4.5 |
| <a id="ENT-04:attribute:project_id"></a>project_id | — | **THIẾU** | URD §4.5 |

#### 5.2.5 Service (ENT-05)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-05:attribute:name"></a>name | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-05:attribute:project_id"></a>project_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-05:attribute:repo_url"></a>repo_url | — | **THIẾU** | URD §4.5 |

#### 5.2.6 ApiToken (ENT-06)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-06:attribute:expires_at"></a>expires_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-06:attribute:last_used_at"></a>last_used_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-06:attribute:project_id"></a>project_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-06:attribute:scopes"></a>scopes | — | **THIẾU** | URD §4.5 |
| <a id="ENT-06:attribute:token_hash"></a>token_hash | — | **THIẾU** | URD §4.5 |

#### 5.2.7 Connector (ENT-07)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-07:attribute:health"></a>health | — | **THIẾU** | URD §4.5 |
| <a id="ENT-07:attribute:name"></a>name | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-07:attribute:schema"></a>schema | — | **THIẾU** | URD §4.5 |
| <a id="ENT-07:attribute:version"></a>version | định danh theo nguồn | **THIẾU** | URD §4.5 |

#### 5.2.8 DataSource (ENT-08)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-08:attribute:connector_id"></a>connector_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-08:attribute:credential_ref"></a>credential_ref | — | **THIẾU** | URD §4.5 |
| <a id="ENT-08:attribute:endpoint"></a>endpoint | — | **THIẾU** | URD §4.5 |
| <a id="ENT-08:attribute:freshness"></a>freshness | — | **THIẾU** | URD §4.5 |
| <a id="ENT-08:attribute:priority"></a>priority | — | **THIẾU** | URD §4.5 |
| <a id="ENT-08:attribute:schedule"></a>schedule | — | **THIẾU** | URD §4.5 |

#### 5.2.9 ImportJob (ENT-09)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-09:attribute:cursor"></a>cursor | — | **THIẾU** | URD §4.5 |
| <a id="ENT-09:attribute:finished_at"></a>finished_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-09:attribute:source_id"></a>source_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-09:attribute:started_at"></a>started_at | — | **THIẾU** | URD §4.5 |

#### 5.2.10 RawRecord (ENT-10)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-10:attribute:checksum"></a>checksum | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-10:attribute:fetched_at"></a>fetched_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-10:attribute:job_id"></a>job_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-10:attribute:payload_ref"></a>payload_ref | — | **THIẾU** | URD §4.5 |
| <a id="ENT-10:attribute:reject_reason"></a>reject_reason | — | **THIẾU** | URD §4.5 |
| <a id="ENT-10:attribute:schema_version"></a>schema_version | — | **THIẾU** | URD §4.5 |
| <a id="ENT-10:attribute:source_id"></a>source_id | — | **THIẾU** | URD §4.5 |

#### 5.2.11 Component (ENT-11)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-11:attribute:aliases"></a>aliases | — | **THIẾU** | URD §4.5 |
| <a id="ENT-11:attribute:cpe"></a>cpe | — | **THIẾU** | URD §4.5 |
| <a id="ENT-11:attribute:ecosystem"></a>ecosystem | — | **THIẾU** | URD §4.5 |
| <a id="ENT-11:attribute:eol_slug"></a>eol_slug | — | **THIẾU** | URD §4.5 |
| <a id="ENT-11:attribute:name"></a>name | — | **THIẾU** | URD §4.5 |
| <a id="ENT-11:attribute:purl_base"></a>purl_base | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-11:attribute:steward"></a>steward | — | **THIẾU** | URD §4.5 |

#### 5.2.12 ComponentVersion (ENT-12)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-12:attribute:component_id"></a>component_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-12:attribute:is_latest_in_line"></a>is_latest_in_line | — | **THIẾU** | URD §4.5 |
| <a id="ENT-12:attribute:license_spdx"></a>license_spdx | — | **THIẾU** | URD §4.5 |
| <a id="ENT-12:attribute:release_date"></a>release_date | — | **THIẾU** | URD §4.5 |
| <a id="ENT-12:attribute:release_line"></a>release_line | — | **THIẾU** | URD §4.5 |
| <a id="ENT-12:attribute:version"></a>version | — | **THIẾU** | URD §4.5 |

#### 5.2.13 ReleaseLine (ENT-13)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-13:attribute:component_id"></a>component_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-13:attribute:eol_date"></a>eol_date | — | **THIẾU** | URD §4.5 |
| <a id="ENT-13:attribute:eos_date"></a>eos_date | — | **THIẾU** | URD §4.5 |
| <a id="ENT-13:attribute:line"></a>line | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-13:attribute:lts"></a>lts | — | **THIẾU** | URD §4.5 |
| <a id="ENT-13:attribute:source"></a>source | — | **THIẾU** | URD §4.5 |

#### 5.2.14 Vulnerability (ENT-14)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-14:attribute:aliases"></a>aliases | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-14:attribute:cvss"></a>cvss | — | **THIẾU** | URD §4.5 |
| <a id="ENT-14:attribute:epss"></a>epss | — | **THIẾU** | URD §4.5 |
| <a id="ENT-14:attribute:kev"></a>kev | — | **THIẾU** | URD §4.5 |
| <a id="ENT-14:attribute:published_at"></a>published_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-14:attribute:references"></a>references | — | **THIẾU** | URD §4.5 |
| <a id="ENT-14:attribute:severity"></a>severity | — | **THIẾU** | URD §4.5 |
| <a id="ENT-14:attribute:source_ids"></a>source_ids | — | **THIẾU** | URD §4.5 |
| <a id="ENT-14:attribute:summary"></a>summary | — | **THIẾU** | URD §4.5 |

#### 5.2.15 AffectedRange (ENT-15)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-15:attribute:component_id"></a>component_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-15:attribute:fixed_versions"></a>fixed_versions | — | **THIẾU** | URD §4.5 |
| <a id="ENT-15:attribute:range"></a>range | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-15:attribute:source"></a>source | — | **THIẾU** | URD §4.5 |
| <a id="ENT-15:attribute:vulnerability_id"></a>vulnerability_id | — | **THIẾU** | URD §4.5 |

#### 5.2.16 License (ENT-16)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-16:attribute:category"></a>category | — | **THIẾU** | URD §4.5 |
| <a id="ENT-16:attribute:deprecated_by"></a>deprecated_by | — | **THIẾU** | URD §4.5 |
| <a id="ENT-16:attribute:name"></a>name | — | **THIẾU** | URD §4.5 |
| <a id="ENT-16:attribute:osi_approved"></a>osi_approved | — | **THIẾU** | URD §4.5 |
| <a id="ENT-16:attribute:spdx_id"></a>spdx_id | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-16:attribute:text_ref"></a>text_ref | — | **THIẾU** | URD §4.5 |

#### 5.2.17 SbomSnapshot (ENT-17)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-17:attribute:build_id"></a>build_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-17:attribute:commit_sha"></a>commit_sha | — | **THIẾU** | URD §4.5 |
| <a id="ENT-17:attribute:environment_id"></a>environment_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-17:attribute:hash"></a>hash | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-17:attribute:service_id"></a>service_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-17:attribute:submitted_at"></a>submitted_at | — | **THIẾU** | URD §4.5 |

#### 5.2.18 Usage (ENT-18)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-18:attribute:component_version_id"></a>component_version_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-18:attribute:dependency_path"></a>dependency_path | — | **THIẾU** | URD §4.5 |
| <a id="ENT-18:attribute:environment_id"></a>environment_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-18:attribute:first_seen"></a>first_seen | — | **THIẾU** | URD §4.5 |
| <a id="ENT-18:attribute:last_seen"></a>last_seen | — | **THIẾU** | URD §4.5 |
| <a id="ENT-18:attribute:service_id"></a>service_id | — | **THIẾU** | URD §4.5 |

#### 5.2.19 UsageContext (ENT-19)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-19:attribute:distributed"></a>distributed | — | **THIẾU** | URD §4.5 |
| <a id="ENT-19:attribute:exposed_network"></a>exposed_network | — | **THIẾU** | URD §4.5 |
| <a id="ENT-19:attribute:features_used"></a>features_used | — | **THIẾU** | URD §4.5 |
| <a id="ENT-19:attribute:modified_source"></a>modified_source | — | **THIẾU** | URD §4.5 |

#### 5.2.20 LicenseObservation (ENT-20)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-20:attribute:component_version_id"></a>component_version_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-20:attribute:confidence"></a>confidence | — | **THIẾU** | URD §4.5 |
| <a id="ENT-20:attribute:detected_at"></a>detected_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-20:attribute:expression"></a>expression | — | **THIẾU** | URD §4.5 |
| <a id="ENT-20:attribute:override_reason"></a>override_reason | — | **THIẾU** | URD §4.5 |
| <a id="ENT-20:attribute:source"></a>source | định danh theo nguồn | **THIẾU** | URD §4.5 |

#### 5.2.21 Finding (ENT-21)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-21:attribute:assignee"></a>assignee | — | **THIẾU** | URD §4.5 |
| <a id="ENT-21:attribute:fixed_in_version"></a>fixed_in_version | — | **THIẾU** | URD §4.5 |
| <a id="ENT-21:attribute:sla_due_at"></a>sla_due_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-21:attribute:usage_id"></a>usage_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-21:attribute:vulnerability_id"></a>vulnerability_id | — | **THIẾU** | URD §4.5 |

#### 5.2.22 VexStatement (ENT-22)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-22:attribute:author"></a>author | — | **THIẾU** | URD §4.5 |
| <a id="ENT-22:attribute:detail"></a>detail | — | **THIẾU** | URD §4.5 |
| <a id="ENT-22:attribute:finding_id"></a>finding_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-22:attribute:status"></a>status | — | **THIẾU** | URD §4.5 |

#### 5.2.23 Decision (ENT-23)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-23:attribute:approved_at"></a>approved_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-23:attribute:approver"></a>approver | — | **THIẾU** | URD §4.5 |
| <a id="ENT-23:attribute:evidence_ids"></a>evidence_ids | — | **THIẾU** | URD §4.5 |
| <a id="ENT-23:attribute:expires_at"></a>expires_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-23:attribute:rationale"></a>rationale | — | **THIẾU** | URD §4.5 |
| <a id="ENT-23:attribute:scope"></a>scope | — | **THIẾU** | URD §4.5 |
| <a id="ENT-23:attribute:statement"></a>statement | — | **THIẾU** | URD §4.5 |
| <a id="ENT-23:attribute:subject"></a>subject | — | **THIẾU** | URD §4.5 |
| <a id="ENT-23:attribute:type"></a>type | — | **THIẾU** | URD §4.5 |

#### 5.2.24 Policy (ENT-24)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-24:attribute:approved_by"></a>approved_by | — | **THIẾU** | URD §4.5 |
| <a id="ENT-24:attribute:effective_from"></a>effective_from | — | **THIẾU** | URD §4.5 |
| <a id="ENT-24:attribute:rules"></a>rules | — | **THIẾU** | URD §4.5 |
| <a id="ENT-24:attribute:type"></a>type | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-24:attribute:version"></a>version | định danh theo nguồn | **THIẾU** | URD §4.5 |

#### 5.2.25 Evaluation (ENT-25)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-25:attribute:policy_versions"></a>policy_versions | — | **THIẾU** | URD §4.5 |
| <a id="ENT-25:attribute:snapshot_id"></a>snapshot_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-25:attribute:violations"></a>violations | — | **THIẾU** | URD §4.5 |

#### 5.2.26 Alert (ENT-26)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-26:attribute:ack_at"></a>ack_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-26:attribute:digest_group"></a>digest_group | — | **THIẾU** | URD §4.5 |
| <a id="ENT-26:attribute:related_ids"></a>related_ids | — | **THIẾU** | URD §4.5 |
| <a id="ENT-26:attribute:sent_at"></a>sent_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-26:attribute:severity"></a>severity | — | **THIẾU** | URD §4.5 |
| <a id="ENT-26:attribute:target"></a>target | — | **THIẾU** | URD §4.5 |
| <a id="ENT-26:attribute:type"></a>type | — | **THIẾU** | URD §4.5 |

#### 5.2.27 NotificationPreference (ENT-27)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-27:attribute:channels"></a>channels | — | **THIẾU** | URD §4.5 |
| <a id="ENT-27:attribute:digest_mode"></a>digest_mode | — | **THIẾU** | URD §4.5 |
| <a id="ENT-27:attribute:event_types"></a>event_types | — | **THIẾU** | URD §4.5 |
| <a id="ENT-27:attribute:snooze"></a>snooze | — | **THIẾU** | URD §4.5 |
| <a id="ENT-27:attribute:user_id"></a>user_id | — | **THIẾU** | URD §4.5 |

#### 5.2.28 CatalogEntry (ENT-28)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-28:attribute:alternatives"></a>alternatives | — | **THIẾU** | URD §4.5 |
| <a id="ENT-28:attribute:component_id"></a>component_id | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-28:attribute:layer"></a>layer | — | **THIẾU** | URD §4.5 |
| <a id="ENT-28:attribute:min_recommended_versions"></a>min_recommended_versions | — | **THIẾU** | URD §4.5 |
| <a id="ENT-28:attribute:next_review_at"></a>next_review_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-28:attribute:rationale"></a>rationale | — | **THIẾU** | URD §4.5 |
| <a id="ENT-28:attribute:reviewer"></a>reviewer | — | **THIẾU** | URD §4.5 |
| <a id="ENT-28:attribute:ring"></a>ring | — | **THIẾU** | URD §4.5 |

#### 5.2.29 CatalogChangeRequest (ENT-29)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-29:attribute:arb_decision"></a>arb_decision | — | **THIẾU** | URD §4.5 |
| <a id="ENT-29:attribute:entry_id"></a>entry_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-29:attribute:rationale"></a>rationale | — | **THIẾU** | URD §4.5 |
| <a id="ENT-29:attribute:requester"></a>requester | — | **THIẾU** | URD §4.5 |

#### 5.2.30 StackProposal (ENT-30)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-30:attribute:candidates"></a>candidates | — | **THIẾU** | URD §4.5 |
| <a id="ENT-30:attribute:layers"></a>layers | — | **THIẾU** | URD §4.5 |
| <a id="ENT-30:attribute:project_id"></a>project_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-30:attribute:rationale"></a>rationale | — | **THIẾU** | URD §4.5 |
| <a id="ENT-30:attribute:review_notes"></a>review_notes | — | **THIẾU** | URD §4.5 |

#### 5.2.31 Baseline (ENT-31)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-31:attribute:catalog_revision"></a>catalog_revision | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-31:attribute:format"></a>format | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-31:attribute:generated_at"></a>generated_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-31:attribute:items"></a>items | — | **THIẾU** | URD §4.5 |

#### 5.2.32 Report (ENT-32)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-32:attribute:file_ref"></a>file_ref | — | **THIẾU** | URD §4.5 |
| <a id="ENT-32:attribute:format"></a>format | — | **THIẾU** | URD §4.5 |
| <a id="ENT-32:attribute:period"></a>period | — | **THIẾU** | URD §4.5 |
| <a id="ENT-32:attribute:requested_by"></a>requested_by | — | **THIẾU** | URD §4.5 |
| <a id="ENT-32:attribute:scope"></a>scope | — | **THIẾU** | URD §4.5 |
| <a id="ENT-32:attribute:type"></a>type | — | **THIẾU** | URD §4.5 |

#### 5.2.33 Asset (ENT-33)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-33:attribute:asset_type"></a>asset_type | — | **THIẾU** | URD §4.5 |
| <a id="ENT-33:attribute:detected_license"></a>detected_license | — | **THIẾU** | URD §4.5 |
| <a id="ENT-33:attribute:name"></a>name | — | **THIẾU** | URD §4.5 |
| <a id="ENT-33:attribute:perceptual_hash"></a>perceptual_hash | — | **THIẾU** | URD §4.5 |
| <a id="ENT-33:attribute:provider_asset_id"></a>provider_asset_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-33:attribute:publisher"></a>publisher | — | **THIẾU** | URD §4.5 |
| <a id="ENT-33:attribute:sha256"></a>sha256 | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-33:attribute:source_url"></a>source_url | — | **THIẾU** | URD §4.5 |
| <a id="ENT-33:attribute:variant"></a>variant | — | **THIẾU** | URD §4.5 |

#### 5.2.34 AssetLicenseTerm (ENT-34)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-34:attribute:allowed_usage_types"></a>allowed_usage_types | — | **THIẾU** | URD §4.5 |
| <a id="ENT-34:attribute:attribution_required"></a>attribution_required | — | **THIẾU** | URD §4.5 |
| <a id="ENT-34:attribute:category"></a>category | — | **THIẾU** | URD §4.5 |
| <a id="ENT-34:attribute:code"></a>code | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-34:attribute:commercial_use"></a>commercial_use | — | **THIẾU** | URD §4.5 |
| <a id="ENT-34:attribute:limits"></a>limits | — | **THIẾU** | URD §4.5 |
| <a id="ENT-34:attribute:term_type"></a>term_type | — | **THIẾU** | URD §4.5 |
| <a id="ENT-34:attribute:terms_snapshot"></a>terms_snapshot | — | **THIẾU** | URD §4.5 |
| <a id="ENT-34:attribute:transferable"></a>transferable | — | **THIẾU** | URD §4.5 |

#### 5.2.35 Entitlement (ENT-35)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-35:attribute:asset_id_asset_scope"></a>asset_id/asset_scope | — | **THIẾU** | URD §4.5 |
| <a id="ENT-35:attribute:bound_projects"></a>bound_projects | — | **THIẾU** | URD §4.5 |
| <a id="ENT-35:attribute:license_term_code"></a>license_term_code | — | **THIẾU** | URD §4.5 |
| <a id="ENT-35:attribute:licensee"></a>licensee | — | **THIẾU** | URD §4.5 |
| <a id="ENT-35:attribute:order_id"></a>order_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-35:attribute:quantity"></a>quantity | — | **THIẾU** | URD §4.5 |
| <a id="ENT-35:attribute:transferable_to_client"></a>transferable_to_client | — | **THIẾU** | URD §4.5 |
| <a id="ENT-35:attribute:valid_from"></a>valid_from | — | **THIẾU** | URD §4.5 |
| <a id="ENT-35:attribute:valid_until"></a>valid_until | — | **THIẾU** | URD §4.5 |

#### 5.2.36 AssetUsage (ENT-36)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-36:attribute:asset_id"></a>asset_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-36:attribute:file_paths"></a>file_paths | — | **THIẾU** | URD §4.5 |
| <a id="ENT-36:attribute:first_seen"></a>first_seen | — | **THIẾU** | URD §4.5 |
| <a id="ENT-36:attribute:last_seen"></a>last_seen | — | **THIẾU** | URD §4.5 |
| <a id="ENT-36:attribute:service_id"></a>service_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-36:attribute:usage_types"></a>usage_types | — | **THIẾU** | URD §4.5 |

#### 5.2.37 AssetFinding (ENT-37)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-37:attribute:asset_usage_id"></a>asset_usage_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-37:attribute:due_at"></a>due_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-37:attribute:resolution"></a>resolution | — | **THIẾU** | URD §4.5 |
| <a id="ENT-37:attribute:rule"></a>rule | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-37:attribute:severity"></a>severity | — | **THIẾU** | URD §4.5 |

#### 5.2.38 AssetCatalogEntry (ENT-38)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-38:attribute:asset_or_source"></a>asset_or_source | — | **THIẾU** | URD §4.5 |
| <a id="ENT-38:attribute:conditions"></a>conditions | — | **THIẾU** | URD §4.5 |
| <a id="ENT-38:attribute:next_review_at"></a>next_review_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-38:attribute:reviewer"></a>reviewer | — | **THIẾU** | URD §4.5 |
| <a id="ENT-38:attribute:ring"></a>ring | — | **THIẾU** | URD §4.5 |
| <a id="ENT-38:attribute:vietnamese_support"></a>vietnamese_support | — | **THIẾU** | URD §4.5 |

#### 5.2.39 AssetScanRun (ENT-39)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-39:attribute:commit_sha"></a>commit_sha | — | **THIẾU** | URD §4.5 |
| <a id="ENT-39:attribute:files_scanned"></a>files_scanned | — | **THIẾU** | URD §4.5 |
| <a id="ENT-39:attribute:finished_at"></a>finished_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-39:attribute:service_id"></a>service_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-39:attribute:started_at"></a>started_at | — | **THIẾU** | URD §4.5 |

#### 5.2.40 CreditsDocument (ENT-40)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-40:attribute:entries"></a>entries | — | **THIẾU** | URD §4.5 |
| <a id="ENT-40:attribute:format"></a>format | — | **THIẾU** | URD §4.5 |
| <a id="ENT-40:attribute:generated_at"></a>generated_at | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-40:attribute:project_id"></a>project_id | — | **THIẾU** | URD §4.5 |

#### 5.2.41 HandoverChecklist (ENT-41)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-41:attribute:entitlement"></a>entitlement | — | **THIẾU** | URD §4.5 |
| <a id="ENT-41:attribute:export_ref"></a>export_ref | — | **THIẾU** | URD §4.5 |
| <a id="ENT-41:attribute:licensee"></a>licensee | — | **THIẾU** | URD §4.5 |
| <a id="ENT-41:attribute:project_id"></a>project_id | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-41:attribute:transferable"></a>transferable | — | **THIẾU** | URD §4.5 |

#### 5.2.42 FigmaConnection (ENT-42)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-42:attribute:file_keys"></a>file_keys | — | **THIẾU** | URD §4.5 |
| <a id="ENT-42:attribute:last_sync_at"></a>last_sync_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-42:attribute:project_id"></a>project_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-42:attribute:team_id"></a>team_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-42:attribute:token_ref"></a>token_ref | — | **THIẾU** | URD §4.5 |

#### 5.2.43 Evidence (ENT-43)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-43:attribute:filename"></a>filename | — | **THIẾU** | URD §4.5 |
| <a id="ENT-43:attribute:kind"></a>kind | — | **THIẾU** | URD §4.5 |
| <a id="ENT-43:attribute:owner_ref"></a>owner_ref | — | **THIẾU** | URD §4.5 |
| <a id="ENT-43:attribute:retention_until"></a>retention_until | — | **THIẾU** | URD §4.5 |
| <a id="ENT-43:attribute:sha256"></a>sha256 | định danh theo nguồn | **THIẾU** | URD §4.5 |
| <a id="ENT-43:attribute:size"></a>size | — | **THIẾU** | URD §4.5 |
| <a id="ENT-43:attribute:storage_ref"></a>storage_ref | — | **THIẾU** | URD §4.5 |

#### 5.2.44 AuditLog (ENT-44)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-44:attribute:action"></a>action | — | **THIẾU** | URD §4.5 |
| <a id="ENT-44:attribute:actor"></a>actor | — | **THIẾU** | URD §4.5 |
| <a id="ENT-44:attribute:after"></a>after | — | **THIẾU** | URD §4.5 |
| <a id="ENT-44:attribute:at"></a>at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-44:attribute:before"></a>before | — | **THIẾU** | URD §4.5 |
| <a id="ENT-44:attribute:entity_ref"></a>entity_ref | — | **THIẾU** | URD §4.5 |
| <a id="ENT-44:attribute:source_ip"></a>source_ip | — | **THIẾU** | URD §4.5 |

#### 5.2.45 WebhookSubscription (ENT-45)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-45:attribute:event_types"></a>event_types | — | **THIẾU** | URD §4.5 |
| <a id="ENT-45:attribute:scope"></a>scope | — | **THIẾU** | URD §4.5 |
| <a id="ENT-45:attribute:secret_ref"></a>secret_ref | — | **THIẾU** | URD §4.5 |
| <a id="ENT-45:attribute:status"></a>status | — | **THIẾU** | URD §4.5 |
| <a id="ENT-45:attribute:url"></a>url | — | **THIẾU** | URD §4.5 |

#### 5.2.46 EventDelivery (ENT-46)

| Thuộc tính | Khoá | Kiểu vật lý | Nguồn |
|---|---|---|---|
| <a id="ENT-46:attribute:attempt"></a>attempt | — | **THIẾU** | URD §4.5 |
| <a id="ENT-46:attribute:event_id"></a>event_id | — | **THIẾU** | URD §4.5 |
| <a id="ENT-46:attribute:last_error"></a>last_error | — | **THIẾU** | URD §4.5 |
| <a id="ENT-46:attribute:next_retry_at"></a>next_retry_at | — | **THIẾU** | URD §4.5 |
| <a id="ENT-46:attribute:status"></a>status | — | **THIẾU** | URD §4.5 |
| <a id="ENT-46:attribute:subscription_id"></a>subscription_id | — | **THIẾU** | URD §4.5 |

### 5.3 Toàn vẹn dữ liệu

| Mã | Ràng buộc | Cơ chế thực thi và nguồn gốc | Nguồn |
|---|---|---|---|
| <a id="FACT-0cdb51f711"></a>— | Ghim phụ thuộc | statement: Phụ thuộc ghim phiên bản; bản baseline phiên bản xuất từ Catalog dùng làm khóa build | SAD §10.7 |
| <a id="FACT-5165c69a79"></a>— | Quy ước khóa chính và thời gian | statement: Khóa chính uuid v7; cột row_version cho If-Match; created_at, updated_at giờ UTC; hiển thị giờ Việt Nam ở tầng giao diện | SAD §7.2 |
| <a id="FACT-91507b2cbc"></a>— | Trường mở rộng jsonb | statement: Trường mở rộng extension và labels (jsonb) trên resource lõi, có chỉ mục GIN khi cần truy vấn | SAD §7.2 |
| <a id="FACT-b0eb123e83"></a>— | UNIQUE phải chứa cột phân vùng | statement: Ở bảng phân vùng theo thời gian, khoá UNIQUE phải chứa cột phân vùng (PostgreSQL) | SRS §7.5 |
| <a id="FACT-d2e10eb3e2"></a>— | Kiểm chứng độ trễ bằng đo thực tế | statement: TDD phải kiểm chứng bằng đo thực tế trên bộ dữ liệu quy mô thiết kế trước khi chốt chỉ mục và ngưỡng phân vùng | SAD §7.3 |
| <a id="FACT-ffb15cec70"></a>— | Khoá ngoại ON DELETE RESTRICT | statement: Mọi quan hệ đều ON DELETE RESTRICT, trừ khi ghi khác | SRS §7.3 |
| <a id="INT-01"></a>INT-01 | Component duy nhất theo purl chuẩn hoá (bỏ version và qualifier); thành phần không có purl dùng `pkg:generic` kèm CPE và slug endoflife.date | Cơ chế thực thi: DB: UNIQUE `purl_base`. App: chuẩn hoá purl (ALG-01) trước khi ghi; trùng danh tính thì `Merged`, alias vào `component_aliases` (PK `alias`)<br>Nguồn: BRL-01<br>Thực thể: ENT-11 | SRS §7.4, URD §4.6 |
| <a id="INT-02"></a>INT-02 | SbomSnapshot duy nhất theo (service, môi trường, hash nội dung); nộp trùng hash là idempotent | Cơ chế thực thi: DB: UNIQUE `(service_id, environment_id, content_hash)`. App: gặp trùng thì trả Evaluation cũ với `meta.deduplicated=true`, không tạo bản ghi. Trigger: `environment_id` cùng dự án với `service_id`<br>Nguồn: BRL-02<br>Thực thể: ENT-17 | SRS §7.4, URD §4.6 |
| <a id="INT-03"></a>INT-03 | Snapshot mới nhất của (service, môi trường) là nguồn sự thật cho Usage nguồn SBOM; Usage không còn thì chuyển Closed, không xoá vật lý | Cơ chế thực thi: DB: UNIQUE một phần `(service_id, environment_id)` khi `status='Current'`. App: trong một giao dịch, snapshot Current cũ sang Superseded và Usage `origin=sbom` vắng mặt sang Closed (giữ `last_seen`)<br>Nguồn: BRL-03<br>Thực thể: ENT-17, ENT-18 | SRS §7.4, URD §4.6 |
| <a id="INT-04"></a>INT-04 | Finding duy nhất theo (Vulnerability, Usage); đổi trạng thái không tạo Finding mới | Cơ chế thực thi: DB: UNIQUE `dedupe_key`, với `type=vulnerability` khóa là (vulnerability, usage), mở rộng cho `license` và `eol` bằng mã đối tượng tương ứng. App: matching upsert theo khoá; xuất hiện lại thì mở lại bản ghi cũ<br>Nguồn: BRL-08, URD 4.4<br>Thực thể: ENT-21 | SRS §7.4, URD §4.6 |
| <a id="INT-05"></a>INT-05 | Mọi Decision có `expires_at` không quá 12 tháng kể từ ngày duyệt; gia hạn tạo Decision mới | Cơ chế thực thi: DB: CHECK `expires_at` khác rỗng và ≤ `approved_at` + 12 tháng khi `Approved`; `supersedes_id` cho gia hạn. Trigger: chặn UPDATE cột nội dung sau `Approved` (NFR-SE-05). App: gia hạn là tạo Decision mới<br>Nguồn: BRL-33, BRL-36<br>Thực thể: ENT-23 | SRS §7.4, URD §4.6 |
| <a id="INT-06"></a>INT-06 | Resource sinh từ dữ liệu ngoài (Vulnerability, ComponentVersion, ReleaseLine, License) có lineage về RawRecord và ImportJob | Cơ chế thực thi: App: bước công bố (Phụ lục C.13) ghi `raw_records.normalized_to` = `{entity, id}` cùng giao dịch với upsert; `affected_ranges.raw_record_id` trỏ ngược. DB: chỉ mục biểu thức trên `normalized_to` để tra ngược. Job đối soát hằng ngày báo resource không có RawRecord; mục tiêu 100%<br>Nguồn: NFR-CO-06<br>Thực thể: ENT-10, ENT-11 đến ENT-16 | SRS §7.4, URD §4.6 |
| <a id="INT-07"></a>INT-07 | Entitlement bắt buộc licensee, license_term_code, proof (Evidence) và valid_until hoặc vĩnh viễn | Cơ chế thực thi: DB: NOT NULL `licensee`, `license_term_code`, `license_term_version`; CHECK `perpetual` và `valid_until` rỗng, hoặc không `perpetual` và `valid_until` có giá trị; CHECK có `asset_id` hoặc `asset_scope`. Trigger (deferred): sang `Active` cần ≥ 1 Evidence `Available` của Entitlement. App: không xóa Evidence khi còn AssetUsage Active liên kết<br>Nguồn: URD 4.2.G, BRL-47<br>Thực thể: ENT-35, ENT-43 | SRS §7.4, URD §4.6 |
| <a id="INT-08"></a>INT-08 | AuditLog chỉ thêm, không sửa, không xoá; lưu ≥ 3 năm | Cơ chế thực thi: DB: role ứng dụng chỉ có INSERT và SELECT; trigger chặn UPDATE, DELETE; bỏ phân vùng chỉ qua migration khi đủ 3 năm và có xác nhận LEG (OI-16); chuỗi băm `prev_hash`, `hash` (mức S)<br>Nguồn: BRL-37<br>Thực thể: ENT-44 | SRS §7.4, URD §4.6 |
| <a id="INT-09"></a>INT-09 | Không xoá vật lý thực thể nghiệp vụ; đóng bằng trạng thái (Closed, Retired, Revoked, Archived) để giữ lịch sử | Cơ chế thực thi: DB: role ứng dụng không có DELETE trên bảng nghiệp vụ; FK RESTRICT. App: API không có DELETE, chỉ có lệnh chuyển trạng thái; ngoại lệ vật lý nêu ở Mục 7.1 và 7.6<br>Nguồn: URD 4.7<br>Thực thể: 46 thực thể | SRS §7.4, URD §4.6 |

Quy ước dữ liệu nêu trong văn xuôi:

| Quy ước | Nguồn |
|---|---|
| <a id="FACT-873d2e6032"></a>Cập nhật connector khi đổi schema: khi đổi schema, connector tương ứng cập nhật trong 5 ngày công | SRS §3.5 |
| <a id="FACT-e04513f453"></a>Độ tươi dữ liệu ngoài | SAD §8.2 |
| <a id="NFR-IC-01"></a>Định danh người dùng | SRS §6.3, URD §8.3 |
| <a id="NFR-IC-02"></a>Duy nhất và idempotent | SRS §6.3, URD §8.3 |
| <a id="NFR-IC-03"></a>Truy vết nguồn dữ liệu | SRS §6.3, URD §8.3 |
| <a id="NFR-IC-04"></a>Toàn vẹn tham chiếu | SRS §6.3, URD §8.3 |
| <a id="NFR-IC-05"></a>Chất lượng dữ liệu khai báo tay | SRS §6.3, URD §8.3 |

### 5.4 Thông tin dữ liệu còn thiếu

> **[THIẾU] Kiểu dữ liệu vật lý, độ dài, nullability, giá trị mặc định.** 261/261 thuộc tính chưa có kiểu vật lý.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §6.4 “Đồng bộ dữ liệu phân quyền”; SAD §7 “Góc nhìn dữ liệu”; SAD §7.1 “Ánh xạ thực thể vào schema”; SAD §7.3 “Chỉ mục, phân vùng và truy vấn nặng”; SAD §7.4 “Phân loại và bảo vệ dữ liệu”; SAD §7.5 “Sao lưu và khôi phục”; SAD §7.6 “Di trú”; SAD §8.2 “Pipeline dữ liệu ngoài”

> **[THIẾU] Khoá ngoại, chỉ mục, view, phân vùng, phân loại dữ liệu, retention, ownership.** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §6.4 “Đồng bộ dữ liệu phân quyền”; SAD §7 “Góc nhìn dữ liệu”; SAD §7.1 “Ánh xạ thực thể vào schema”; SAD §7.3 “Chỉ mục, phân vùng và truy vấn nặng”; SAD §7.4 “Phân loại và bảo vệ dữ liệu”; SAD §7.5 “Sao lưu và khôi phục”; SAD §7.6 “Di trú”; SAD §8.2 “Pipeline dữ liệu ngoài”

<a id="ch-6"></a>
## 6. Giao dịch và khả năng chịu lỗi

### 6.1 Mã lỗi

| Mã lỗi | HTTP | Ý nghĩa | Nguồn |
|---|---|---|---|
| <a id="ERR-conflict"></a>conflict | 409 | Xung đột dữ liệu chung | SRS §9.6 |
| <a id="ERR-decision_not_approved"></a>decision_not_approved | 409 | Dùng Decision chưa Approved | SRS §9.6 |
| <a id="ERR-duplicate"></a>duplicate | 409 | Trùng khóa duy nhất (ví dụ `dedupe_key`, `purl_base`) | SRS §9.6 |
| <a id="ERR-expiry_too_long"></a>expiry_too_long | 422 | Hạn quá trần cho phép (token > 365 ngày, Decision quá hạn tối đa) | SRS §9.6 |
| <a id="ERR-forbidden"></a>forbidden | 403 | Không đủ vai trò (đối tượng đã biết tồn tại) | SRS §9.6 |
| <a id="ERR-gone"></a>gone | 410 | Tài nguyên đã hết hạn (ví dụ tệp export quá 7 ngày) | SRS §9.6 |
| <a id="ERR-handover_incomplete"></a>handover_incomplete | 409 | Đóng bàn giao khi còn mục chưa xong | SRS §9.6 |
| <a id="ERR-idempotency_key_reuse"></a>idempotency_key_reuse | 422 | Cùng `Idempotency-Key`, khác nội dung | SRS §9.6 |
| <a id="ERR-insufficient_scope"></a>insufficient_scope | 403 | Token thiếu scope | SRS §9.6 |
| <a id="ERR-internal_error"></a>internal_error | 500 | Lỗi nội bộ; chỉ trả `request_id`, không lộ chi tiết | SRS §9.6 |
| <a id="ERR-invalid_client"></a>invalid_client | 401 | OAuth client sai hoặc bị khóa | SRS §9.6 |
| <a id="ERR-invalid_purl"></a>invalid_purl | 400 | purl sai cú pháp | SRS §9.6 |
| <a id="ERR-invalid_request"></a>invalid_request | 400 | Sai cú pháp, thiếu tham số, kiểu sai | SRS §9.6 |
| <a id="ERR-invalid_sbom"></a>invalid_sbom | 400 | SBOM không hợp lệ theo schema CycloneDX hoặc SPDX | SRS §9.6 |
| <a id="ERR-invalid_scope"></a>invalid_scope | 400 | Scope không có trong danh sách | SRS §9.6 |
| <a id="ERR-invalid_state_transition"></a>invalid_state_transition | 409 | Chuyển trạng thái không nằm trong bảng Mục 8 | SRS §9.6 |
| <a id="ERR-not_found"></a>not_found | 404 | Không tồn tại hoặc không được phép thấy | SRS §9.6 |
| <a id="ERR-payload_too_large"></a>payload_too_large | 413 | Body vượt giới hạn chung | SRS §9.6 |
| <a id="ERR-precondition_failed"></a>precondition_failed | 412 | `If-Match` lệch `row_version` | SRS §9.6 |
| <a id="ERR-precondition_required"></a>precondition_required | 428 | Thiếu `If-Match` | SRS §9.6 |
| <a id="ERR-project_archived"></a>project_archived | 409 | Ghi vào dự án `Archived` | SRS §9.6 |
| <a id="ERR-rate_limited"></a>rate_limited | 429 | Vượt giới hạn tốc độ; có `Retry-After` | SRS §9.6 |
| <a id="ERR-rules_schema_invalid"></a>rules_schema_invalid | 422 | `Policy.rules` sai schema C.14 | SRS §9.6 |
| <a id="ERR-sbom_too_large"></a>sbom_too_large | 413 | SBOM > 50 MB (`limits.sbom_max_bytes`) | SRS §9.6 |
| <a id="ERR-sbom_too_many_components"></a>sbom_too_many_components | 413 | SBOM > 20.000 component (`limits.sbom_max_components`) | SRS §9.6 |
| <a id="ERR-separation_of_duties"></a>separation_of_duties | 403 | Người đề nghị tự duyệt (NFR-SE-03) | SRS §9.6 |
| <a id="ERR-service_unavailable"></a>service_unavailable | 503 | Bảo trì hoặc quá tải; có `Retry-After` | SRS §9.6 |
| <a id="ERR-simulation_required"></a>simulation_required | 409 | Kích hoạt Policy khi chưa mô phỏng (FR-ADM-04) | SRS §9.6 |
| <a id="ERR-sla_profile_looser"></a>sla_profile_looser | 422 | SLA profile lỏng hơn mức nền của Policy | SRS §9.6 |
| <a id="ERR-timeout"></a>timeout | 504 | Quá 30 giây | SRS §9.6 |
| <a id="ERR-token_expired"></a>token_expired | 401 | Token hết hạn | SRS §9.6 |
| <a id="ERR-token_revoked"></a>token_revoked | 401 | Token bị thu hồi | SRS §9.6 |
| <a id="ERR-unauthenticated"></a>unauthenticated | 401 | Thiếu hoặc sai thông tin xác thực | SRS §9.6 |
| <a id="ERR-unsupported_media_type"></a>unsupported_media_type | 415 | `Content-Type` không hỗ trợ | SRS §9.6 |
| <a id="ERR-unsupported_spec_version"></a>unsupported_spec_version | 400 | CycloneDX < 1.5 hoặc SPDX < 2.3 (NFR-CO-01) | SRS §9.6 |
| <a id="ERR-validation_failed"></a>validation_failed | 422 | Sai ràng buộc trường; chi tiết ở `errors[]` | SRS §9.6 |

### 6.2 Chính sách chịu lỗi nêu trong nguồn

| Nội dung | Chi tiết | Nguồn |
|---|---|---|
| <a id="FACT-9eee0053b7"></a>Suy giảm có kiểm soát: Nếu vượt quy mô, hệ thống phải suy giảm có kiểm soát (phân trang, chạy nền) thay vì lỗi | Nếu vượt quy mô, hệ thống phải suy giảm có kiểm soát (phân trang, chạy nền) thay vì lỗi. | URD §8.9 |
| <a id="FACT-b9f0e4612f"></a>Suy giảm có kiểm soát khi vượt quy mô: Vượt quy mô thì hệ thống suy giảm có kiểm soát (phân trang, chạy nền trả 202) thay vì lỗi | Vượt quy mô thì hệ thống suy giảm có kiểm soát (phân trang, chạy nền trả 202) thay vì lỗi. | SRS §6.9 |

> **[THIẾU] Ranh giới transaction, isolation, nhất quán, saga/bù trừ, retry/backoff, timeout, idempotency, fallback, DLQ.** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §7.5 “Sao lưu và khôi phục”; SAD §8.7 “Xử lý sự cố”; SAD §10.3 “Độ tin cậy”; SRS §6.4 “Độ tin cậy (RE)”; URD §8.4 “Độ tin cậy (RE)”; URD §9.4 “Hỗ trợ và xử lý sự cố”

<a id="ch-7"></a>
## 7. Di trú dữ liệu

### 7.1 Di trú nêu trong nguồn

| Nội dung | Chi tiết | Nguồn |
|---|---|---|
| <a id="FACT-97ac47bdf8"></a>Quay lui và migration: Quay lui bằng ảnh cũ; migration đã chạy phải tương thích ngược một phiên bản | statement: Quay lui bằng ảnh cũ; migration đã chạy phải tương thích ngược một phiên bản | SAD §9.5 |
| <a id="FACT-fe99df37b1"></a>Migration tự động khi triển khai: Migration chạy tự động khi triển khai, trước khi instance mới nhận yêu cầu; một instance chạy tại một thời điểm nhờ job_locks | statement: Migration chạy tự động khi triển khai, trước khi instance mới nhận yêu cầu; một instance chạy tại một thời điểm nhờ job_locks | SRS §7.7 |

### 7.2 Thông tin di trú còn thiếu

> **[THIẾU] Schema trước/sau, các bước expand → migrate → contract, backfill, khoá bảng, downtime, tương thích, rollback.** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §7.5 “Sao lưu và khôi phục”; SAD §7.6 “Di trú”; SRS §7.7 “Di trú và seed dữ liệu”

<a id="ch-8"></a>
## 8. Chất lượng và vận hành

### 8.1 Chỉ tiêu và ràng buộc chất lượng, vận hành

#### 8.1.1 Hiệu năng (PE) (8)

| Mã | Yêu cầu | Chỉ tiêu và chi tiết | Nguồn |
|---|---|---|---|
| <a id="NFR-PE-01"></a>NFR-PE-01 | Nộp SBOM và đánh giá gate | Chỉ tiêu đo được: Nộp SBOM 2.000 component cộng evaluate: p95 ≤ 30 giây; evaluate trên snapshot 1.000 component: p95 ≤ 5 giây; chịu 60 lần nộp/phút; SBOM tối đa 50 MB và 20.000 component (`limits.sbom_max_bytes`, `limits.sbom_max_components`), vượt trả 413<br>Phương pháp: A<br>Truy vết: UC-05, UC-31, UC-37; OI-12; API-I-01, API-C-01<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-I-01, API-C-01; ALG-01, ALG-08<br>FR: FR-INV-02, FR-CI-01, FR-CI-05 | SRS §6.1, URD §8.1 |
| <a id="NFR-PE-02"></a>NFR-PE-02 | Matching và tính lại theo lô | Chỉ tiêu đo được: Re-matching toàn bộ sau đồng bộ ≤ 15 phút với 500 dự án, 1 triệu Usage; chạy nền, API đọc vẫn đạt NFR-PE-03 trong lúc chạy<br>Phương pháp: A<br>Truy vết: UC-11, UC-12; ALG-04<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: ALG-04 (chạy nền)<br>FR: FR-VUL-01 | SRS §6.1, URD §8.1 |
| <a id="NFR-PE-03"></a>NFR-PE-03 | Thao tác tương tác | Chỉ tiêu đo được: Trang, dashboard p95 ≤ 2 giây (40 dự án, 10.000 Finding); tìm component ≤ 1 giây; diff 2 snapshot 2.000 component ≤ 2 giây; import 5.000 dòng ≤ 60 giây; đổi ngữ cảnh hoặc chính sách áp lại ≤ 5 phút<br>Phương pháp: A<br>Truy vết: UC-06 đến UC-09, UC-18, UC-29<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-I-05, API-I-16, API-V-01, API-G-27; ALG-04 (áp lại)<br>FR: FR-INV-06..08, FR-ADM-04, FR-LIC-02, FR-UI-01..04 | SRS §6.1, URD §8.1 |
| <a id="NFR-PE-04"></a>NFR-PE-04 | Truy vấn nặng và xuất báo cáo | Chỉ tiêu đo được: Blast radius p95 ≤ 3 giây (500 dự án, 1 triệu Usage); Review Report PDF ≤ 2 phút; gói tuân thủ ≤ 5 phút (1.000 component)<br>Phương pháp: A<br>Truy vết: UC-08, UC-15, UC-20, UC-22 đến UC-25, UC-30; FR-UI-06<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-V-19, API-V-20, API-T-35, API-T-36, API-T-38<br>FR: FR-UI-06, FR-SEL-04, FR-GOV-07 | SRS §6.1, URD §8.1 |
| <a id="NFR-PE-05"></a>NFR-PE-05 | Độ tươi dữ liệu và thời gian cảnh báo | Chỉ tiêu đo được: OSV, GHSA, Go vulndb ≤ 1 giờ; NVD ≤ 2 giờ; KEV ≤ 6 giờ; EOL ≤ 24 giờ; CVE công bố đến owner nhận cảnh báo ≤ 24 giờ, độ trễ vào KB p95 ≤ 2 giờ; Finding Critical thông báo ≤ 5 phút. Đo từ `last_success_at`; `freshness=Stale` khi vượt `freshness_sla_minutes` (60 cho OSV)<br>Phương pháp: A<br>Truy vết: UC-10 đến UC-12, UC-28; ENT-08; EIF-01 đến EIF-07<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-K-13..16; ALG-13; `last_success_at`<br>FR: FR-KB-01..04, FR-KB-07 | SRS §6.1, URD §8.1 |
| <a id="NFR-PE-06"></a>NFR-PE-06 | Job định kỳ | Chỉ tiêu đo được: Job SLA mỗi 15 phút; job hằng ngày (EOL, Decision, Entitlement) chạy bù khi trễ, không chồng lấp (`job_locks`), hoàn tất trước 06:00 giờ Việt Nam<br>Phương pháp: T, A<br>Truy vết: UC-14, UC-16, UC-17, UC-19, UC-27, UC-36; ALG-03<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: Lập lịch; ALG-03; `job_locks`<br>FR: FR-VUL-03, FR-LCM-02, FR-GOV-02, FR-GOV-05, FR-AST-13 | SRS §6.1, URD §8.1 |
| <a id="NFR-PE-07"></a>NFR-PE-07 | Quét tài sản | Chỉ tiêu đo được: `pbom scan` repo 5.000 file tài sản (≤ 2 GB) ≤ 3 phút trên runner CI tiêu chuẩn 4 vCPU, 8 GB RAM, SSD; chỉnh được bằng `limits.asset_scan_max_files` (OI-14, chờ đo pilot)<br>Phương pháp: A<br>Truy vết: UC-05, UC-32; FR-AST-01<br>Ưu tiên: Should<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: CLI `pbom scan`<br>FR: FR-AST-01 | SRS §6.1, URD §8.1 |
| <a id="NFR-PE-08"></a>NFR-PE-08 | Đánh giá lại rule tài sản | Chỉ tiêu đo được: Rule tài sản đánh giá lại ≤ 5 phút sau khi Entitlement hoặc AssetUsage đổi<br>Phương pháp: T<br>Truy vết: UC-35; ALG-07<br>Ưu tiên: Should<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: ALG-07; API-S-34..36<br>FR: FR-AST-11 | SRS §6.1, URD §8.1 |

#### 8.1.2 Tương thích (CO) (7)

| Mã | Yêu cầu | Chỉ tiêu và chi tiết | Nguồn |
|---|---|---|---|
| <a id="NFR-CO-01"></a>NFR-CO-01 | Định dạng SBOM và định danh | Chỉ tiêu đo được: Nhận và xuất CycloneDX ≥ 1.5 và SPDX ≥ 2.3 (JSON); định danh bằng purl; 100% tệp mẫu hợp lệ qua công cụ xác thực chính thức; phiên bản thấp hơn trả 400 `unsupported_spec_version`<br>Phương pháp: T<br>Truy vết: UC-05; ENT-17<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-I-01, API-I-06; ALG-01<br>FR: FR-INV-02, FR-VUL-08 | SRS §6.2, URD §8.2 |
| <a id="NFR-CO-02"></a>NFR-CO-02 | CLI và pipeline | Chỉ tiêu đo được: CLI chạy trên Linux, macOS, Windows; template GitLab CI, GitHub Actions, Jenkins; đọc TTF, OTF, WOFF, WOFF2, PNG, JPG, SVG, WebP, AVIF, Lottie; Credits xuất MD, HTML, JSON<br>Phương pháp: T, D<br>Truy vết: UC-05, UC-32, UC-37, UC-39; FR-CI-04; EIF-11<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: CLI; API-C-08, API-C-09, API-S-38..42, API-S-43..47<br>FR: FR-INV-03, FR-CI-04, FR-AST-01, FR-AST-15, FR-AST-17 | SRS §6.2, URD §8.2 |
| <a id="NFR-CO-03"></a>NFR-CO-03 | Trao đổi dữ liệu và tích hợp | Chỉ tiêu đo được: Import CSV, XLSX theo mẫu; Figma REST API; webhook Teams, Slack, Telegram; API REST có phiên bản `/api/v1`<br>Phương pháp: T<br>Truy vết: UC-09, UC-40; EIF-14 đến EIF-16<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-I-16, API-K-11, API-S-48..50, API-G-28..39<br>FR: FR-INV-08, FR-KB-10, FR-AST-09, FR-GOV-04 | SRS §6.2, URD §8.2 |
| <a id="NFR-CO-04"></a>NFR-CO-04 | Nguồn dữ liệu ngoài | Chỉ tiêu đo được: Hỗ trợ OSV, GHSA, Go vulndb, NVD, CISA KEV, EPSS, endoflife.date; mirror cục bộ OSV và NVD; 0 tham số truy vấn chứa tên dự án hay khách hàng (kiểm tra bằng rà nhật ký yêu cầu ra)<br>Phương pháp: T, I<br>Truy vết: UC-10, UC-11, UC-16, UC-17, UC-19; EIF-01 đến EIF-07<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: ALG-12, ALG-13; EIF-01..07<br>FR: FR-KB-01..05 | SRS §6.2, URD §8.2 |
| <a id="NFR-CO-05"></a>NFR-CO-05 | Xuất baseline | Chỉ tiêu đo được: Version-pin baseline xuất JSON hoặc YAML và ánh xạ sang `go.mod`, `package.json`, `pom`, `Dockerfile`; 100% mục có `purl_base` và `min_version`<br>Phương pháp: T<br>Truy vết: UC-22; ENT-31<br>Ưu tiên: Should<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-T-15..21<br>FR: FR-CAT-07 | SRS §6.2, URD §8.2 |
| <a id="NFR-CO-06"></a>NFR-CO-06 | Import và đồng bộ dữ liệu ngoài qua connector, pipeline 4 bước: thu nhận, kiểm tra, chuẩn hoá, công bố | Chỉ tiêu đo được: Idempotent (chạy lại cùng dữ liệu không tạo trùng); đồng bộ tăng dần theo cursor, etag hoặc modified-since; lô có ≤ 1% bản ghi lỗi (`limits.import_error_threshold` = 0,01) vẫn công bố phần hợp lệ (Partial), mọi bản ghi lỗi có `reject_code` và `reject_reason`; 100% resource truy vết được về bản gốc, connector, thời điểm; sửa mapping bằng cấu hình không cần deploy; import 100.000 bản ghi ≤ 30 phút; nguồn lỗi giữ last-known-good; hai nguồn lệch ghi conflict, áp priority theo cấu hình<br>Phương pháp: T<br>Truy vết: UC-05, UC-09, UC-10, UC-11, UC-18; ENT-07 đến ENT-10; Phụ lục C.13; ALG-12<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-K-01..45; ALG-13<br>FR: FR-KB-01..10 | SRS §6.2, URD §8.2 |
| <a id="NFR-CO-07"></a>NFR-CO-07 | Hệ thống mở, sẵn sàng tích hợp: REST versioned có OpenAPI, webhook ký số có thử lại và dead-letter, inbound API, contract dữ liệu chuẩn, scoped token và OAuth2 client credentials | Chỉ tiêu đo được: 100% chức năng UI có API tương đương; tương thích ngược trong cùng major, deprecation ≥ 6 tháng; có contract test cho mỗi tích hợp; webhook giao ≥ 99,9% trong 5 phút (thử lại tối đa 24 giờ); rate limit cấu hình theo token; mọi lệnh gọi ghi audit<br>Phương pháp: T<br>Truy vết: UC-03, UC-05, UC-10, UC-31; FR-INV-02, FR-CI-05; EIF-11, EIF-14, EIF-19<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: Toàn bộ 284 endpoint (Mục 9)<br>FR: FR-INV-02, FR-CI-05, FR-ADM-03, FR-GOV-04, FR-UI-01..10 | SRS §6.2, URD §8.2 |

#### 8.1.3 Độ tin cậy (RE) (4)

| Mã | Yêu cầu | Chỉ tiêu và chi tiết | Nguồn |
|---|---|---|---|
| <a id="NFR-RE-01"></a>NFR-RE-01 | Chịu lỗi khi gọi dịch vụ | Chỉ tiêu đo được: Thử lại 3 lần theo backoff khi 429, 5xx, lỗi mạng; CLI không làm hỏng pipeline (exit code theo Phụ lục B mục 5); PBOM không phản hồi trong 30 giây thì gate theo fail-open (mặc định, trả warn) hoặc fail-closed cấu hình theo dự án<br>Phương pháp: T<br>Truy vết: UC-05, UC-10, UC-31; BRL-43, OI-02; ENT-03<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: CLI; API-C-01<br>FR: FR-INV-03, FR-CI-01, FR-KB-01 | SRS §6.4, URD §8.4 |
| <a id="NFR-RE-02"></a>NFR-RE-02 | Job an toàn | Chỉ tiêu đo được: Job idempotent; không gửi trùng thông báo trong 24 giờ (`alerts.dedupe_key`); chạy bù khi trễ; hai instance không xử lý trùng (`job_locks`, khóa tư vấn theo `data_source_id`); 0 Alert trùng trong kiểm thử hai instance<br>Phương pháp: T<br>Truy vết: UC-12, UC-14, UC-27, UC-36; BRL-38<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: `job_locks`; ALG-03<br>FR: FR-VUL-03, FR-GOV-02, FR-GOV-05, FR-AST-13, FR-LCM-02 | SRS §6.4, URD §8.4 |
| <a id="NFR-RE-03"></a>NFR-RE-03 | Kênh và hệ thống ngoài lỗi | Chỉ tiêu đo được: Webhook, Figma lỗi thử 3 lần rồi hoãn hoặc đổi kênh; email luôn là kênh dự phòng; EventDelivery Dead_lettered có thể gửi lại thủ công<br>Phương pháp: T<br>Truy vết: UC-28, UC-40; EIF-13 đến EIF-16<br>Ưu tiên: Should<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-G-28..39, API-S-48..50<br>FR: FR-GOV-04, FR-AST-09 | SRS §6.4, URD §8.4 |
| <a id="NFR-RE-04"></a>NFR-RE-04 | Khả dụng và suy giảm có kiểm soát | Chỉ tiêu đo được: Khả dụng ≥ 99,5% trong khung 07:00 đến 19:00 GMT+7, thứ Hai đến thứ Sáu, trừ ngày nghỉ lễ theo lịch công ty (OI-13, chờ), tính theo tháng; bảo trì theo lịch ngoài khung, báo trước ≥ 24 giờ; nguồn ngoài gián đoạn thì dùng mirror, hiển thị nhãn dữ liệu cũ khi lỗi quá 24 giờ, không dừng hệ thống<br>Phương pháp: A<br>Truy vết: UC-10; OI-13; `limits.notification.business_window`<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-P-39, API-P-40, API-K-13..16<br>FR: FR-KB-01, FR-KB-02, FR-KB-07 | SRS §6.4, URD §8.4 |

#### 8.1.4 Khả năng bảo trì (MA) (6)

| Mã | Yêu cầu | Chỉ tiêu và chi tiết | Nguồn |
|---|---|---|---|
| <a id="NFR-MA-01"></a>NFR-MA-01 | Mở rộng hệ sinh thái | Chỉ tiêu đo được: Thêm hệ sinh thái (Maven, PyPI, NuGet) qua plugin parser: 0 dòng mã lõi đổi (kiểm bằng diff), có test cho parser và ALG-04<br>Phương pháp: D, I<br>Truy vết: Hệ thống; ALG-01, ALG-04<br>Ưu tiên: Should<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: Plugin parser; ALG-01, ALG-04<br>FR: Hệ thống | SRS §6.6, URD §8.6 |
| <a id="NFR-MA-02"></a>NFR-MA-02 | Kiểm thử và tự tuân thủ | Chỉ tiêu đo được: Mã lõi (matching, license, gate, rule tài sản) có kiểm thử tự động, phủ dòng ≥ 80% (đề xuất); PBOM tự quét chính nó mỗi build, 0 component ngoài nhóm Permissive hoặc Weak-copyleft<br>Phương pháp: T, I<br>Truy vết: Hệ thống; DC-09, C-05<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: Kiểm thử tự động; PBOM tự quét chính nó (DC-09)<br>FR: Hệ thống | SRS §6.6, URD §8.6 |
| <a id="NFR-MA-03"></a>NFR-MA-03 | Cấu hình không cần deploy | Chỉ tiêu đo được: Thay đổi SLA, ma trận license, rule gate, mốc cảnh báo có hiệu lực ≤ 5 phút, không phát hành lại; có mô phỏng trước khi kích hoạt<br>Phương pháp: T<br>Truy vết: UC-04, UC-18; FR-ADM-04; ENT-24<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-P-30..37, API-C-04<br>FR: FR-ADM-04, FR-LIC-02, FR-CI-03 | SRS §6.6, URD §8.6 |
| <a id="NFR-MA-04"></a>NFR-MA-04 | Giải thích được | Chỉ tiêu đo được: 100% Finding hiển thị `priority_inputs`; mọi license action hiển thị `rule_ref` (`policy_version`, `matrix_cell`); mọi kết quả gate hiển thị rule áp dụng trong `violations`<br>Phương pháp: D<br>Truy vết: UC-07, UC-13; ENT-21, ENT-25<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-V-01, API-C-07; ALG-02, ALG-05, ALG-08<br>FR: FR-VUL-02, FR-LIC-02, FR-CI-01, FR-CI-02, FR-CAT-03 | SRS §6.6, URD §8.6 |
| <a id="NFR-MA-05"></a>NFR-MA-05 | Kiến trúc plugin, connector (discover, fetch, map, health) | Chỉ tiêu đo được: Thêm connector mới ≤ 5 ngày công (dev đã quen SDK), không đổi mã lõi; có SDK, connector mẫu, tài liệu; connector lỗi không làm tăng lỗi lõi (bulkhead, circuit breaker); health check và metric mỗi connector; test conformance tự động trước khi bật (`conformance_status=passed`); khi nguồn đổi schema, connector cập nhật ≤ 5 ngày công<br>Phương pháp: T, D<br>Truy vết: UC-05, UC-09, UC-10, UC-31, UC-41; ENT-07<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-K-01..24; ALG-13<br>FR: FR-KB-08..10 | SRS §6.6, URD §8.6 |
| <a id="NFR-MA-06"></a>NFR-MA-06 | Mô hình dữ liệu trung lập, mở rộng được | Chỉ tiêu đo được: Resource lõi có định danh ổn định (purl, URN), schema có phiên bản, `extension` và `labels` ≤ 16 KB, index GIN, lọc qua API; migration tự động, tương thích ngược trong cùng major; từ điển dữ liệu khớp schema, kiểm tự động trong CI, 0 lệch<br>Phương pháp: T<br>Truy vết: UC-10, UC-31; Mục 7.1, 7.7<br>Ưu tiên: Should<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: `extension`, `labels`; `filter[extension.x-...]`<br>FR: Hệ thống, FR-KB-01..10 | SRS §6.6, URD §8.6 |

#### 8.1.5 Khả năng tương tác và trải nghiệm (UX) (3)

| Mã | Yêu cầu | Chỉ tiêu và chi tiết | Nguồn |
|---|---|---|---|
| <a id="NFR-UX-01"></a>NFR-UX-01 | Giao diện dễ dùng | Chỉ tiêu đo được: Song ngữ Việt và Anh; WCAG 2.1 AA; hành động chính từ dashboard ≤ 3 bước; tooltip công thức cho 100% chỉ số<br>Phương pháp: D, I<br>Truy vết: UC-13, UC-20, UC-21, UC-23, UC-24, UC-29, UC-38; Mục 4.1<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-G-24..27<br>FR: FR-UI-01..10 | SRS §6.7, URD §8.7 |
| <a id="NFR-UX-02"></a>NFR-UX-02 | Biểu mẫu và báo cáo | Chỉ tiêu đo được: Báo lỗi theo từng trường; báo cáo xuất PDF, XLSX, CSV đồng nhất, 100% báo cáo ghi `data_as_of`<br>Phương pháp: D<br>Truy vết: UC-25, UC-33; ENT-32<br>Ưu tiên: Should<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-T-35..39, API-I-16<br>FR: FR-SEL-04, FR-GOV-06, FR-INV-08, FR-AST-07 | SRS §6.7, URD §8.7 |
| <a id="NFR-UX-03"></a>NFR-UX-03 | Thông báo và Credits | Chỉ tiêu đo được: Mẫu thông báo và Credits song ngữ, hiển thị đúng ở 360 px (không cuộn ngang)<br>Phương pháp: D<br>Truy vết: UC-28, UC-37; ENT-26, ENT-40<br>Ưu tiên: Should<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-G-15..23, API-S-38..42<br>FR: FR-GOV-04, FR-AST-15 | SRS §6.7, URD §8.7 |

#### 8.1.6 Kiểm toán và truy vết (AU) (1)

| Mã | Yêu cầu | Chỉ tiêu và chi tiết | Nguồn |
|---|---|---|---|
| <a id="NFR-AU-01"></a>NFR-AU-01 | Audit log | Chỉ tiêu đo được: Chỉ ghi thêm (BRL-37), lưu ≥ 3 năm (`audit_min_days` ≥ 1095); ghi ai, khi nào, đối tượng, trước và sau, lý do (bắt buộc với ghi đè, thu hồi, mở lại); 100% lệnh ghi và chuyển trạng thái có bản ghi; UPDATE, DELETE trên `audit_logs` bị chặn; xuất CSV cho kiểm toán<br>Phương pháp: T, I<br>Truy vết: UC-26, UC-30, UC-33 đến UC-35, UC-38, UC-39; INT-08; ENT-44<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-G-40..44, API-P-42; `audit_logs`<br>FR: FR-GOV-03 và mọi FR có lệnh ghi | SRS §6.8, URD §8.8 |

#### 8.1.7 Nêu trong văn xuôi của tài liệu nguồn (9)

| Yêu cầu | Chỉ tiêu và chi tiết | Nguồn |
|---|---|---|
| <a id="FACT-79dc5aea7a"></a>Quy mô thiết kế | 500 dự án, 1 triệu Usage, 40 dự án và 10.000 Finding trong phạm vi một người dùng, SBOM tối đa 20.000 component, repo tối đa 5.000 file tài sản | URD §8.9 |
| <a id="FACT-97b8eadccc"></a>PBOM tổng quan năng lực: PBOM phải cho TECHWISE biết mọi dự án đang dùng package, framework, middleware và tài sản thiết kế nào, cảnh báo CVE, EOL và vi phạm bản quyền theo SLA, và chặn lựa chọn không an toàn ngay trong CI | PBOM phải cho TECHWISE biết mọi dự án đang dùng package, framework, middleware và tài sản thiết kế nào, cảnh báo CVE, EOL và vi phạm bản quyền theo SLA, và chặn lựa chọn không an toàn ngay trong CI | URD |
| <a id="FACT-7320d119a1"></a>Ngân sách POST /sbom | Ngân sách đề xuất cho `POST /sbom` với 2.000 component (tổng p95 ≤ 30 giây): tiếp nhận và kiểm ≤ 3 giây; chuẩn hoá và upsert Usage ≤ 12 giây; matching nhanh ≤ 8 giây; đánh giá gate ≤ 5 giây (khớp NFR-PE-01); dự phòng 2 giây. | SAD §10.4 |
| <a id="FACT-17230afe4d"></a>Quy mô thiết kế: 500 dự án, 1 triệu Usage, SBOM tối đa 20.000 component, repo tối đa 5.000 file tài sản | Quy mô thiết kế: 500 dự án, 1 triệu Usage, SBOM tối đa 20.000 component, repo tối đa 5.000 file tài sản | SRS §3.5 |
| <a id="FACT-57542dc1c3"></a>CLI là binary Go không cần root: CLI là một binary Go, chạy không cần quyền root trên Linux, macOS, Windows | CLI là một binary Go, chạy không cần quyền root trên Linux, macOS, Windows | SRS §4.2 |
| <a id="FACT-b0bf4a0273"></a>CI bắt buộc | CI bắt buộc: biên dịch, lint, kiểm ranh giới module (Mục 5.3), `govulncheck` hoặc tương đương, sinh và kiểm mã từ OpenAPI, kiểm license theo allowlist, quét bí mật, ký ảnh. | SAD §10.7 |
| <a id="FACT-b5f5a1fced"></a>Thời gian gate | Thời gian gate p95 không quá 30 giây | SAD §11.3 |
| <a id="FACT-c3f5858496"></a>Quy mô thiết kế: 500 dự án, 1 triệu Usage, SBOM tối đa 20.000 component, repo tối đa 5.000 file tài sản; không phục vụ nhiều tổ chức (single tenant) | 500 dự án, 1 triệu Usage, SBOM tối đa 20.000 component, repo tối đa 5.000 file tài sản; không phục vụ nhiều tổ chức (single tenant) | SAD §2.2 |
| <a id="FACT-fb5613eda0"></a>Khả dụng mức B | khả dụng ≥ 99,5% trong giờ làm việc | SAD §9 |

### 8.2 Thông tin còn thiếu

> **[THIẾU] Mục tiêu khôi phục (RTO/RPO).** 4 yêu cầu nhắc đến khôi phục nhưng chưa nêu RTO/RPO: Khả dụng mức B, NFR-CO-07, NFR-RE, NFR-RE-04.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §7.5 “Sao lưu và khôi phục”; SAD §8.7 “Xử lý sự cố”; SAD §10.3 “Độ tin cậy”; SRS §6.4 “Độ tin cậy (RE)”; URD §8.4 “Độ tin cậy (RE)”; URD §9.4 “Hỗ trợ và xử lý sự cố”

> **[THIẾU] Mô hình tải (trung bình, đỉnh, payload, tăng trưởng), capacity, SLO, concurrency, cấu hình sẵn sàng cao.** chưa có trong dữ liệu đã trích từ tài liệu nguồn; các con số quy mô chỉ xuất hiện trong chỉ tiêu ở trên.

> **[THIẾU] Kiến trúc triển khai (môi trường, mạng, mức triển khai) và kế hoạch phát hành.** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §2.2 “Giả định về tổ chức và hạ tầng”; SAD §9 “Góc nhìn triển khai và hạ tầng”; SAD §9.1 “Hai mức triển khai”; SAD §9.2 “Phân vùng mạng”; SAD §9.3 “Chiến lược scale”; SAD §9.4 “Môi trường và phân phối”; SAD §9.5 “Triển khai không gián đoạn”; SAD §9.6 “Sơ đồ triển khai”

> **[THIẾU] Quan sát: log, metric, trace, cảnh báo.** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §10.2 “Quan sát”

<a id="ch-9"></a>
## 9. Bảo mật

### 9.1 Yêu cầu và ràng buộc bảo mật đã có

#### 9.1.1 Bảo mật (SE) (6)

| Mã | Yêu cầu | Chỉ tiêu và chi tiết | Nguồn |
|---|---|---|---|
| <a id="NFR-SE-01"></a>NFR-SE-01 | Xác thực | Chỉ tiêu đo được: SSO OIDC, 0 mật khẩu lưu trong PBOM; phiên tối đa 8 giờ, idle 30 phút; tài khoản vô hiệu ở IdP bị từ chối ≤ 15 phút<br>Phương pháp: T<br>Truy vết: UC-01; DC-01, FR-ADM-01; EIF-12<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-P-01<br>FR: FR-ADM-01 | SRS §6.5, URD §8.5 |
| <a id="NFR-SE-02"></a>NFR-SE-02 | Phân quyền | Chỉ tiêu đo được: RBAC theo dự án (Viewer, Contributor, Owner) và vai trò hệ thống; mặc định từ chối; thực thi ở API và truy vấn (lọc `project_id`); bộ kiểm thử xuyên dự án: 0 truy cập trái phép; đối tượng không được phép thấy trả 404, không lộ tồn tại<br>Phương pháp: T<br>Truy vết: UC-01 đến UC-03, UC-29, UC-30, UC-34; BRL-52<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: Cột Quyền của mọi endpoint (Mục 9.2); BRL-52<br>FR: FR-ADM-02 | SRS §6.5, URD §8.5 |
| <a id="NFR-SE-03"></a>NFR-SE-03 | Tách biệt nhiệm vụ | Chỉ tiêu đo được: Người đề nghị không được là người duyệt, kiểm ở API (403 `separation_of_duties`) cho Decision, CatalogChangeRequest, VEX Critical hoặc KEV, AssetLicenseTerm; 100% luồng duyệt có ca kiểm thử; nguyên tắc 4 mắt cho quyết định Critical hoặc KEV<br>Phương pháp: T<br>Truy vết: UC-13, UC-15, UC-26, UC-38; BRL-13, BRL-28, BRL-35<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: 403 `separation_of_duties`; API-V-03..05, API-G-01..14, API-T-10..14, API-S-20..24<br>FR: FR-VUL-04, FR-GOV-01, FR-CAT-05, FR-AST-10 | SRS §6.5, URD §8.5 |
| <a id="NFR-SE-04"></a>NFR-SE-04 | API token | Chỉ tiêu đo được: Chỉ lưu SHA-256 (`token_hash`), hiển thị một lần; scope tối thiểu `sbom:write`, `evaluate`; hạn ≤ 365 ngày (CHECK); thu hồi có hiệu lực ≤ 1 phút<br>Phương pháp: T<br>Truy vết: UC-03, UC-31; BRL-53; ENT-06<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-P-22..29<br>FR: FR-ADM-03 | SRS §6.5, URD §8.5 |
| <a id="NFR-SE-05"></a>NFR-SE-05 | Bảo vệ dữ liệu | Chỉ tiêu đo được: TLS ≥ 1.2 cho mọi kết nối; chứng từ mã hoá at-rest, lưu ≥ hạn dùng tài sản cộng 5 năm; chỉ gửi metadata và hash tài sản, không gửi nội dung file; Decision không sửa được sau Approved (trigger chặn UPDATE nội dung, ca kiểm thử phải bị từ chối)<br>Phương pháp: T, I<br>Truy vết: UC-04, UC-21, UC-26, UC-32, UC-34; OI-16; ENT-23, ENT-43<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-G-01..14, API-S-25..33, API-S-01<br>FR: FR-GOV-01, FR-AST-12, FR-AST-01 | SRS §6.5, URD §8.5 |
| <a id="NFR-SE-06"></a>NFR-SE-06 | Bí mật tích hợp | Chỉ tiêu đo được: Token Figma, khoá HMAC webhook, SMTP, secret OAuth client và token CLI lưu mã hoá ở OpenBao hoặc Vault; DB chỉ giữ `*_ref`; không hiển thị lại; quét log ứng dụng và `audit_logs` với bí mật thử: 0 lần xuất hiện<br>Phương pháp: T, I<br>Truy vết: UC-05, UC-40; DC-02; EIF-17<br>Ưu tiên: Must<br>(FR → NFR)<br>API, ALG hoặc thành phần kiểm: API-K-04..08, API-P-27, API-S-48..50, API-G-34<br>FR: FR-KB-09, FR-ADM-03, FR-AST-09, FR-GOV-04 | SRS §6.5, URD §8.5 |

#### 9.1.2 Nêu trong văn xuôi của tài liệu nguồn (9)

| Yêu cầu | Chỉ tiêu và chi tiết | Nguồn |
|---|---|---|
| <a id="FACT-1ba945afbc"></a>Thu hồi API token | Gateway gọi `introspect` của SVC-01 và giữ kết quả tối đa 60 giây | SAD §6.2 |
| <a id="FACT-03cc746b18"></a>Xuất dữ liệu khi ngừng sử dụng: Khi ngừng sử dụng hệ thống phải xuất SBOM, Decision, Entitlement và audit ra định dạng mở trước khi tắt, không xoá chứng từ trước thời hạn lưu | Khi ngừng sử dụng hệ thống phải xuất SBOM, Decision, Entitlement và audit ra định dạng mở trước khi tắt, không xoá chứng từ trước thời hạn lưu | SRS §7.6 |
| <a id="FACT-8e15874d4a"></a>Snapshot phân quyền: core và Gateway nạp snapshot khi khởi động, giữ trong bộ nhớ, làm mới khi nhận sự kiện và luôn hết hạn sau 60 giây nếu mất sự kiện | `core` và Gateway nạp snapshot khi khởi động, giữ trong bộ nhớ, làm mới khi nhận sự kiện (PostgreSQL LISTEN/NOTIFY hoặc hàng đợi) và luôn hết hạn sau 60 giây nếu mất sự kiện | SAD §6.4 |
| <a id="FACT-31ae305d68"></a>Sao lưu và khôi phục: PostgreSQL có sao lưu liên tục (WAL archiving, point-in-time recovery) và bản sao lưu đầy đủ hằng ngày; kho object bật phiên bản và sao chép; thử khôi phục ít nhất mỗi quý; sau khôi phục phải kiểm chuỗi hash của audit_logs | PostgreSQL có sao lưu liên tục (WAL archiving, point-in-time recovery) và bản sao lưu đầy đủ hằng ngày; kho object bật phiên bản và sao chép; thử khôi phục ít nhất mỗi quý. Sau khi khôi phục phải kiểm chuỗi hash của `audit_logs`. | SAD §7.5 |
| <a id="FACT-a7b8fd8252"></a>Nội dung bản ghi kiểm toán: Mỗi bản ghi có ai, khi nào, đối tượng, trước và sau, lý do, request_id; ghi qua outbox trong cùng giao dịch | Mỗi bản ghi có ai, khi nào, đối tượng, trước và sau, lý do, `request_id`; ghi qua outbox trong cùng giao dịch. | SAD §10.5 |
| <a id="FACT-c7aaff24b6"></a>Cảnh báo vận hành: Cảnh báo vận hành: nguồn quá hạn độ tươi, tỷ lệ lỗi pipeline > 1%, outbox tồn đọng, chuỗi hash audit gãy, lỗi 401 hoặc 403 tăng đột biến, chứng chỉ sắp hết hạn | Cảnh báo vận hành: nguồn quá hạn độ tươi, tỷ lệ lỗi pipeline > 1%, outbox tồn đọng, chuỗi hash audit gãy, lỗi 401 hoặc 403 tăng đột biến, chứng chỉ sắp hết hạn. | SAD §10.2 |
| <a id="FACT-d9f7c04ed9"></a>Skill danh tính máy, không duyệt: Skill dùng danh tính máy (API token hoặc OAuth2 client credentials) với scope tối thiểu; không có quyền duyệt; Decision do skill tạo không tự chuyển qua Pending | Skill dùng danh tính máy (API token hoặc OAuth2 client credentials) với scope tối thiểu theo SRS Phụ lục B; không có quyền duyệt; Decision do skill tạo không tự chuyển qua `Pending` | SAD §8.8 |
| <a id="FACT-e9a3db0731"></a>Thời hạn lưu trữ dữ liệu: Snapshot quá 24 tháng chỉ giữ bản tóm tắt; audit log giữ ≥ 3 năm; chứng từ Entitlement giữ ≥ thời hạn dùng tài sản + 5 năm | Snapshot quá 24 tháng chỉ giữ bản tóm tắt (UC-08); audit log giữ ≥ 3 năm (BRL-37); chứng từ Entitlement giữ ≥ thời hạn dùng tài sản + 5 năm (NFR-SE-05) | URD §9.5 |
| <a id="FACT-ef19e782e2"></a>Xuất dữ liệu khi ngừng sử dụng: Ngừng sử dụng hệ thống: xuất SBOM, Decision, Entitlement và audit ra định dạng mở trước khi tắt; không xoá chứng từ trước thời hạn lưu | xuất SBOM, Decision, Entitlement và audit ra định dạng mở trước khi tắt; không xoá chứng từ trước thời hạn lưu | URD §9.5 |

### 9.2 Thông tin còn thiếu

> **[THIẾU] Ranh giới tin cậy và ma trận phân quyền (vai trò × thao tác).** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §6 “Góc nhìn API Gateway, xác thực và phân quyền”; SAD §6.2 “Loại token”; SAD §6.3 “Mô hình phân quyền hai lớp”; SAD §6.4 “Đồng bộ dữ liệu phân quyền”; SAD §6.5 “Giao tiếp nội bộ (Zero-Trust)”; SAD §6.7 “Sơ đồ thành phần xác thực và phân quyền”; SAD §6.8 “Chế độ xác minh token có thể đổi bằng cấu hình (JWKS hoặc introspect)”; SAD §10.1 “Bảo mật: mối đe dọa và biện pháp”

> **[THIẾU] Mô hình mối đe doạ và biện pháp kiểm soát.** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §6 “Góc nhìn API Gateway, xác thực và phân quyền”; SAD §6.2 “Loại token”; SAD §6.3 “Mô hình phân quyền hai lớp”; SAD §6.4 “Đồng bộ dữ liệu phân quyền”; SAD §6.5 “Giao tiếp nội bộ (Zero-Trust)”; SAD §6.7 “Sơ đồ thành phần xác thực và phân quyền”; SAD §6.8 “Chế độ xác minh token có thể đổi bằng cấu hình (JWKS hoặc introspect)”; SAD §10.1 “Bảo mật: mối đe dọa và biện pháp”

> **[THIẾU] Vòng đời credential, bảo vệ dữ liệu, kiểm toán.** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §6 “Góc nhìn API Gateway, xác thực và phân quyền”; SAD §6.2 “Loại token”; SAD §6.3 “Mô hình phân quyền hai lớp”; SAD §6.4 “Đồng bộ dữ liệu phân quyền”; SAD §6.5 “Giao tiếp nội bộ (Zero-Trust)”; SAD §6.7 “Sơ đồ thành phần xác thực và phân quyền”; SAD §6.8 “Chế độ xác minh token có thể đổi bằng cấu hình (JWKS hoặc introspect)”; SAD §10.1 “Bảo mật: mối đe dọa và biện pháp”

<a id="ch-10"></a>
## 10. Quyết định, giả định và thông tin còn thiếu

### 10.1 Quyết định kiến trúc (ADR)

| Mã | Quyết định | Lý do, phương án đã loại, hệ quả | Nguồn |
|---|---|---|---|
| <a id="ADR-01"></a>ADR-01 | Golang cho toàn bộ backend, worker và CLI; monorepo, một toolchain. | Hệ quả: Cần quy ước dự án Go (layout, lint, error handling) trong TDD.<br>Lý do: CLI một binary tĩnh, không cần runtime; concurrency phù hợp ingest và matching; đội dùng Go làm ngôn ngữ chính.<br>Phương án đã loại: Java, Node.js (CLI phải kèm runtime) | SAD §4.2 |
| <a id="ADR-02"></a>ADR-02 | Modular monolith `core` gồm 7 module nghiệp vụ, chạy nhiều vai trò (api, worker) từ cùng một binary. Authentication và Authorization là hai service riêng. | Hệ quả: Cần kiểm tra ranh giới module tự động trong CI.<br>Lý do: Ít thành phần vận hành, transaction nội bộ module, vẫn tách được khi cần scale. Phần bảo mật tách để cô lập bí mật và ranh giới tin cậy.<br>Phương án đã loại: Microservices đầy đủ (chi phí giao dịch phân tán, quan sát) | SAD §4.2 |
| <a id="ADR-03"></a>ADR-03 | API Gateway là điểm vào duy nhất: TLS, định tuyến, rate limit, xác minh token, chặn thô theo scope; không chứa nghiệp vụ. | Hệ quả: Gateway là SPOF nên chạy ≥ 2 bản sao; cách xác minh token đổi được bằng cấu hình (jwks, introspect, hybrid), xem Mục 6.8.<br>Lý do: Gom bề mặt tấn công, quan sát và giới hạn tại một chỗ.<br>Phương án đã loại: Client gọi thẳng service; logic nghiệp vụ trong gateway | SAD §4.2 |
| <a id="ADR-04"></a>ADR-04 | Authentication Service là OIDC Relying Party với IdP của tổ chức, cấp JWT nội bộ ngắn hạn (ký bất đối xứng) cho người dùng; API token dạng opaque chỉ lưu hash; máy dùng OAuth2 client credentials. | Hệ quả: Phải xoay khoá ký và công bố JWKS; dạng token (jwt hoặc opaque) đổi được bằng cấu hình (Mục 6.8).<br>Lý do: PBOM không lưu mật khẩu; JWT kiểm được ở Gateway mà không gọi service; token máy thu hồi ≤ 1 phút.<br>Phương án đã loại: Tự quản lý tài khoản và mật khẩu; session server-side thuần | SAD §4.2 |
| <a id="ADR-05"></a>ADR-05 | Authorization Service là nguồn sự thật cho vai trò và thành viên dự án; policy viết thành mã (Rego) được nhúng vào Gateway (ràng buộc thô) và vào `core` (ràng buộc tinh) dưới dạng thư viện; dữ liệu phân quyền đồng bộ bằng snapshot và sự kiện đổi quyền. | Hệ quả: Quyền đổi có hiệu lực sau ≤ 60 giây (TTL bộ nhớ đệm + vô hiệu hoá theo sự kiện).<br>Lý do: Không gọi đồng bộ authZ trên mỗi request (tránh SPOF, độ trễ); vẫn kiểm thử được policy (negative test xuyên dự án).<br>Phương án đã loại: Gọi Authorization Service trên mọi request; hard-code role trong mã | SAD §4.2 |
| <a id="ADR-06"></a>ADR-06 | Quy tắc tách nhiệm và nguyên tắc 4 mắt (BRL-33..36) đặt trong domain Governance, ở tầng service, không ở Gateway. | Hệ quả: Cần test tự động cho mọi loại Decision.<br>Lý do: Phụ thuộc trạng thái đối tượng và người đề nghị, Gateway không biết.<br>Phương án đã loại: Kiểm ở Gateway | SAD §4.2 |
| <a id="ADR-07"></a>ADR-07 | PostgreSQL là kho chính, một cluster, mỗi module một schema; tìm kiếm bằng full-text và trigram của PostgreSQL; phân vùng theo thời gian cho `usages`, `sbom_snapshots`, `audit_logs`. | Hệ quả: Phải đo lại khi quá quy mô; đường lùi sang OpenSearch để ngỏ (RSK-04).<br>Lý do: 1 triệu Usage và tìm component ≤ 1 giây không cần công cụ tìm kiếm riêng; ít thành phần.<br>Phương án đã loại: Thêm Elasticsearch/OpenSearch, MongoDB | SAD §4.2 |
| <a id="ADR-08"></a>ADR-08 | Bất đồng bộ bằng transactional outbox và hàng đợi job dựa trên PostgreSQL; chưa thêm message broker. Broker (NATS JetStream hoặc RabbitMQ) chỉ thêm khi fan-out sự kiện vượt ngưỡng đo được. | Hệ quả: Thông lượng bị chặn bởi PostgreSQL; cần ngưỡng chuyển đổi.<br>Lý do: Giao dịch nguyên tử dữ liệu + sự kiện, ít thành phần vận hành, job idempotent và chạy bù dễ (NFR-RE-02).<br>Phương án đã loại: Broker ngay từ đầu; 2PC | SAD §4.2 |
| <a id="ADR-09"></a>ADR-09 | Connector chạy ngoài tiến trình (container riêng) theo contract gRPC discover, fetch, map, health; có manifest, giới hạn tài nguyên, egress chỉ tới đích đã khai báo. | Hệ quả: Thêm một lớp gRPC và vòng đời container.<br>Lý do: Lỗi hay treo của nguồn ngoài không ảnh hưởng lõi; thêm nguồn ≤ 5 ngày công.<br>Phương án đã loại: Plugin Go nạp cùng tiến trình | SAD §4.2 |
| <a id="ADR-10"></a>ADR-10 | Pipeline dữ liệu ngoài qua bảng staging `raw_records` (ingest, validate, normalise, publish); tỷ lệ lỗi > 1% thì dừng publish và báo. | Hệ quả: Thêm bước publish và dung lượng staging.<br>Lý do: Chống dữ liệu xấu vào KB; truy vết nguồn (NFR-IC-03).<br>Phương án đã loại: Ghi thẳng vào bảng nghiệp vụ | SAD §4.2 |
| <a id="ADR-11"></a>ADR-11 | REST + JSON, OpenAPI 3.1 contract-first, sinh stub server và client Go; lỗi theo RFC 9457; phân trang cursor; `If-Match` và `Idempotency-Key`. | Hệ quả: Cần bước sinh mã trong CI.<br>Lý do: Khớp SRS Phụ lục B; CLI dùng chung client sinh tự động.<br>Phương án đã loại: gRPC cho API công khai; GraphQL | SAD §4.2 |
| <a id="ADR-12"></a>ADR-12 | Chưa triển khai BFF nhưng để sẵn chỗ cắm (tiền tố /bff/v1, audience JWT riêng, hợp đồng OpenAPI theo từng client, xem Mục 5.6); SPA gọi API `core` qua Gateway. Tổng hợp dữ liệu bằng endpoint chuyên biệt (dashboard, blast radius) trong module. | Hệ quả: Thêm BFF sau này không phải đổi Core API hay client đang chạy; xem lại khi có client thứ hai.<br>Lý do: Một client Web duy nhất, không có mobile; thêm BFF chỉ thêm một thành phần phải vận hành.<br>Phương án đã loại: BFF riêng như mô hình nhiều client | SAD §4.2 |
| <a id="ADR-13"></a>ADR-13 | Bí mật (token Figma, webhook, SMTP, khoá ký JWT) trong OpenBao hoặc Vault qua interface trừu tượng; không đặt trong biến môi trường thô hay file tĩnh. | Hệ quả: Thêm một thành phần hạ tầng phải vận hành.<br>Lý do: NFR-SE-06; đổi nhà cung cấp không đổi mã.<br>Phương án đã loại: Kubernetes Secret thuần | SAD §4.2 |
| <a id="ADR-14"></a>ADR-14 | Audit log chỉ ghi thêm, chuỗi hash (`prev_hash`, `hash`), ghi qua outbox; người dùng ứng dụng chỉ có quyền INSERT. | Hệ quả: Hàng đợi audit phải có cảnh báo tồn đọng.<br>Lý do: NFR-AU-01, phát hiện sửa.<br>Phương án đã loại: Ghi trực tiếp từng module | SAD §4.2 |
| <a id="ADR-15"></a>ADR-15 | Quan sát chuẩn OpenTelemetry (trace, metric, log có `request_id`); không phụ thuộc nhà cung cấp backend. | Hệ quả: Cần chọn backend lưu (Mục 11).<br>Lý do: Truy vết một request qua gateway, service, worker.<br>Phương án đã loại: Log rời không liên kết | SAD §4.2 |

### 10.2 Quyết định nêu trong văn xuôi

Mục đánh dấu **[chưa chốt]** có chữ 'đề xuất', 'chờ' hoặc 'draft' trong nguồn.

| Quyết định | Tình trạng | Nguồn |
|---|---|---|
| <a id="FACT-58fc6a44f0"></a>Chế độ xác minh token là cấu hình: Cách Gateway và Core API kiểm token là một tham số cấu hình, không ghi cứng trong mã | — | SAD §6.8 |
| <a id="FACT-116d3bc3c3"></a>Cấu hình là dữ liệu: Cấu hình là dữ liệu (Policy có phiên bản), không phải mã | — | SAD §4.1 |
| <a id="FACT-781c19595b"></a>Valkey/broker khi cần: Thêm Valkey hoặc broker chỉ khi số đo cho thấy cần | — | SAD §9.3 |
| <a id="FACT-2447f2d4d1"></a>Scale PostgreSQL: scale dọc trước; replica đọc phục vụ báo cáo và dashboard khi cần | — | SAD §9.3 |
| <a id="FACT-0045744c28"></a>Triển khai container hai mức: container, single tenant, hai mức: mức A Docker Compose trên một máy, mức B Kubernetes nhiều bản sao; cùng ảnh container và cấu hình | — | SAD §9 |
| <a id="FACT-2624296d28"></a>Chưa triển khai BFF: Hiện chưa triển khai BFF (ADR-12), để sẵn chỗ cắm để thêm sau | — | SAD §5.6 |
| <a id="FACT-cd2081c7f5"></a>usages không phân vùng: usages không phân vùng ở quy mô 1 triệu dòng; đánh giá lại khi vượt 5 triệu dòng | **[chưa chốt]** | SRS §7.5 |

### 10.3 Giả định

> **[THIẾU] Giả định về tổ chức và hạ tầng.** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §1.5 “Stakeholder và mối quan tâm”; SAD §2 “Động lực kiến trúc”; SAD §2.2 “Giả định về tổ chức và hạ tầng”; SAD §2.4 “Động lực được ưu tiên khi xung đột”; SAD §3 “Bối cảnh và phạm vi hệ thống”; SAD §3.1 “Actor”; SAD §3.2 “Hệ thống ngoài”; SAD §3.3 “Sơ đồ bối cảnh”

### 10.4 Thông tin còn thiếu trong dữ liệu đã trích

| Loại | Số lượng | Ý nghĩa |
|---|---|---|
| Mã chưa biết thuộc loại phần tử nào | 270 | nguồn có hàng với mã này nhưng chưa có luật gán loại |
| Mã được nhắc tới nhưng không có mô tả | 277 | ví dụ API-P-01 được nhắc ở một yêu cầu nhưng không tài liệu nào mô tả nó |
| Yêu cầu khôi phục chưa nêu RTO/RPO | 4 | thiếu con số mục tiêu khôi phục |
| Văn xuôi chưa đọc hết | 1 | một phần văn xuôi vượt giới hạn đọc |
| Sự kiện do mô hình đề xuất bị loại | 1 | không kiểm chứng được với nguồn nên không đưa vào |

Mã được nhắc tới nhưng chưa có mô tả, theo nhóm (danh sách đầy đủ ở phụ lục A.2):

| Nhóm mã | Số mã | Ví dụ |
|---|---|---|
| ALG | 13 | ALG-01, ALG-02, ALG-03, ALG-04, ALG-05, ALG-06 |
| API | 8 | API-C, API-G, API-I, API-K, API-P, API-S |
| API-C | 9 | API-C-01, API-C-02, API-C-03, API-C-04, API-C-05, API-C-06 |
| API-G | 37 | API-G-01, API-G-02, API-G-03, API-G-04, API-G-05, API-G-06 |
| API-I | 17 | API-I-01, API-I-02, API-I-03, API-I-04, API-I-05, API-I-06 |
| API-K | 41 | API-K-01, API-K-02, API-K-03, API-K-04, API-K-05, API-K-06 |
| API-P | 33 | API-P-01, API-P-02, API-P-03, API-P-04, API-P-05, API-P-06 |
| API-S | 54 | API-S-01, API-S-02, API-S-03, API-S-04, API-S-05, API-S-06 |
| API-T | 37 | API-T-01, API-T-02, API-T-03, API-T-04, API-T-05, API-T-06 |
| API-V | 26 | API-V-01, API-V-02, API-V-03, API-V-04, API-V-05, API-V-06 |
| FR | 2 | FR-LCM, FR-SEL |

Các nhóm mã chưa biết loại (số hàng): AC (8), AD (10), AS (8), BO (7), C (8), DC (9), DS (3), EIF (19), EXT (9), OA (12), OI (18), POC (7), QA (11), QT (8), R (16), REF (18), RSK (10), SG (8), SK (4), SM (12), TDD (12), TH (12), UC (41).

### 10.5 Mâu thuẫn giữa các nguồn

Không ghi nhận mâu thuẫn.

### 10.6 Rủi ro

> **[THIẾU] Danh mục rủi ro (khả năng, tác động, giảm thiểu), nợ kỹ thuật chủ động chấp nhận.** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §12 “Truy vết, rủi ro và vấn đề mở”; SAD §12.2 “Rủi ro kiến trúc”; SAD §12.3 “Nợ kỹ thuật chủ động chấp nhận và điều kiện tách dịch vụ”; SAD §12.4 “Vấn đề mở cần chốt”; SRS §2 “Rà soát vấn đề mở của URD”; SRS §8.5 “Nhóm D: Rủi ro và quản trị”; SRS §10.6 “Rủi ro và vấn đề còn lại sau SRS”; URD §13 “Vấn đề mở”

### 10.7 Tổng hợp mục THIẾU của toàn tài liệu

| Mục | Còn thiếu | Lý do | Cần ai xác nhận |
|---|---|---|---|
| 1.1 Mục tiêu | Mục tiêu của hệ thống | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 1.3 Phi mục tiêu | Phi mục tiêu (những gì hệ thống cố ý không làm) | Chưa có phần tử phi mục tiêu trong dữ liệu đã trích, nên tài liệu không tự suy diễn. Cần BA / chủ sản phẩm xác nhận; tài liệu nguồn có đoạn 'ngoài phạm vi' (trích nguyên văn bên dưới, chưa được đưa vào dữ liệu thiết kế). | BA / chủ sản phẩm |
| 1.5 Bối cảnh hệ thống | Actor, hệ thống ngoài, phụ thuộc bên ngoài | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 2.3 Phụ thuộc giữa các thành phần | Sơ đồ và quan hệ phụ thuộc giữa các thành phần | Chưa có quan hệ phụ thuộc dạng có cấu trúc; quy tắc phụ thuộc chỉ có dạng chữ trong bảng module (nếu nguồn có). | — |
| 2.4 Phân bổ triển khai | Thành phần nào chạy ở đâu (môi trường, mạng, mức triển khai) | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 3.3 Luồng xử lý, thuật toán, bảng quyết định | Luồng chính/phụ/lỗi, thuật toán, bảng quyết định, bất biến | chưa có trong dữ liệu đã trích từ tài liệu nguồn; 13 thuật toán (mã ALG-…) được nhắc tới nhưng chưa có mô tả (xem phụ lục A.2). | — |
| 4.2 Giao diện được nhắc tới nhưng chưa có hợp đồng | Hợp đồng của từng endpoint (đường dẫn, phương thức, đầu vào/ra, mã lỗi, xác thực, phiên bản, tương thích ngược, bên tiêu thụ) | Nguồn mới nêu mã giao diện, chưa có hợp đồng chi tiết trong dữ liệu đã trích. | — |
| 5.4 Thông tin dữ liệu còn thiếu | Kiểu dữ liệu vật lý, độ dài, nullability, giá trị mặc định | 261/261 thuộc tính chưa có kiểu vật lý. | — |
| 5.4 Thông tin dữ liệu còn thiếu | Khoá ngoại, chỉ mục, view, phân vùng, phân loại dữ liệu, retention, ownership | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 6.2 Chính sách chịu lỗi nêu trong nguồn | Ranh giới transaction, isolation, nhất quán, saga/bù trừ, retry/backoff, timeout, idempotency, fallback, DLQ | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 7.2 Thông tin di trú còn thiếu | Schema trước/sau, các bước expand → migrate → contract, backfill, khoá bảng, downtime, tương thích, rollback | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 8.2 Thông tin còn thiếu | Mục tiêu khôi phục (RTO/RPO) | 4 yêu cầu nhắc đến khôi phục nhưng chưa nêu RTO/RPO: Khả dụng mức B, NFR-CO-07, NFR-RE, NFR-RE-04. | — |
| 8.2 Thông tin còn thiếu | Mô hình tải (trung bình, đỉnh, payload, tăng trưởng), capacity, SLO, concurrency, cấu hình sẵn sàng cao | chưa có trong dữ liệu đã trích từ tài liệu nguồn; các con số quy mô chỉ xuất hiện trong chỉ tiêu ở trên. | — |
| 8.2 Thông tin còn thiếu | Kiến trúc triển khai (môi trường, mạng, mức triển khai) và kế hoạch phát hành | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 8.2 Thông tin còn thiếu | Quan sát: log, metric, trace, cảnh báo | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 9.2 Thông tin còn thiếu | Ranh giới tin cậy và ma trận phân quyền (vai trò × thao tác) | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 9.2 Thông tin còn thiếu | Mô hình mối đe doạ và biện pháp kiểm soát | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 9.2 Thông tin còn thiếu | Vòng đời credential, bảo vệ dữ liệu, kiểm toán | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 10.3 Giả định | Giả định về tổ chức và hạ tầng | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 10.6 Rủi ro | Danh mục rủi ro (khả năng, tác động, giảm thiểu), nợ kỹ thuật chủ động chấp nhận | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |
| 11.2 Test case, kịch bản và nghĩa vụ thiết kế | Test case, kịch bản kiểm thử, kết quả mong đợi, nghĩa vụ thiết kế (design obligation) | chưa có trong dữ liệu đã trích từ tài liệu nguồn. | — |

<a id="ch-11"></a>
## 11. Kiểm chứng và truy vết

### 11.1 Tiêu chí nghiệm thu gắn yêu cầu

| Yêu cầu | Tiêu chí nghiệm thu (nguyên văn nguồn) | Nguồn |
|---|---|---|
| [FR-ADM-01](#FR-ADM-01) | [T] Lần đăng nhập đầu tạo User theo `idp_subject`; đổi nhóm IdP đổi vai trò ở lần đăng nhập kế; tài khoản vô hiệu ở IdP bị từ chối ≤ 15 phút; phiên hết hạn sau 8 giờ hoặc 30 phút idle (NFR-SE-01) | SRS §5.1, URD §7.1 |
| [FR-ADM-02](#FR-ADM-02) | [T] Bộ kiểm thử xuyên dự án: 0 truy cập trái phép; đối tượng không được thấy trả 404 `not_found`; gán trùng bộ (user, scope, project, role) trả 409 `duplicate` (NFR-SE-02, NFR-IC-01) | SRS §5.1, URD §7.1 |
| [FR-ADM-03](#FR-ADM-03) | [T] Giá trị token chỉ có trong phản hồi tạo; cơ sở dữ liệu chỉ có `token_hash`; `expires_at` quá 365 ngày trả 422 `expiry_too_long`; thu hồi có hiệu lực ≤ 1 phút (NFR-SE-04) | SRS §5.1, URD §7.1 |
| [FR-ADM-04](#FR-ADM-04) | [T, D] Kích hoạt khi chưa mô phỏng trong 7 ngày trả 409 `simulation_required`; `rules` sai schema trả 422 `rules_schema_invalid`; bản mới hiệu lực ≤ 5 phút; bản `Active` cũ thành `Superseded` (NFR-MA-03) | SRS §5.1, URD §7.1 |
| [FR-AST-01](#FR-AST-01) | [T] Repo 5.000 tệp (≤ 2 GB) quét ≤ 3 phút trên runner 4 vCPU, 8 GB; tệp trùng `.pbomignore` không đi vào kết quả; chỉ gửi metadata và hash, không gửi nội dung tệp (NFR-PE-07, NFR-SE-05) | SRS §5.12, URD §7.12 |
| [FR-AST-02](#FR-AST-02) | [T] Bộ mẫu mỗi nguồn khai báo gián tiếp cho ít nhất một AssetUsage với `detected_by` tương ứng | SRS §5.12, URD §7.12 |
| [FR-AST-03](#FR-AST-03) | [T] Font mẫu TTF, OTF, WOFF, WOFF2 trả đủ family, vendor, `fsType` (NFR-CO-02) | SRS §5.12, URD §7.12 |
| [FR-AST-04](#FR-AST-04) | [T] Font có nameID 13 khớp điều khoản chuẩn được gán `license_term_code`; không khớp thì Unknown, sinh rule `unknown_source` nếu không thuộc Asset Catalog Approved | SRS §5.12, URD §7.12 |
| [FR-AST-05](#FR-AST-05) | [T] Ảnh mẫu có XMP hoặc IPTC cho đủ tác giả và license; SVG có `<metadata>` được đọc | SRS §5.12, URD §7.12 |
| [FR-AST-06](#FR-AST-06) | [T] Ảnh preview watermark sinh AssetFinding rule `watermark_preview`; nguồn stock nhận diện được gắn `provider` | SRS §5.12, URD §7.12 |
| [FR-AST-07](#FR-AST-07) | [T] `sha256` bắt buộc, trùng thì chỉ tạo AssetUsage; `confirm_due_at` = hiện tại cộng 90 ngày; `in_house` thiếu `creator` trả 422; `client_provided` thiếu chứng từ sinh `ownership_unproven` | SRS §5.12, URD §7.12 |
| [FR-AST-08](#FR-AST-08) | [T] Hai tệp cùng SHA-256 dùng chung một Asset; ảnh gần giống chỉ xuất hiện ở gợi ý, không tự gộp | SRS §5.12, URD §7.12 |
| [FR-AST-09](#FR-AST-09) | [T] Token Figma lưu ở kho bí mật, không hiển thị lại; lỗi Figma thử 3 lần rồi hoãn (NFR-SE-06, NFR-RE-03) | SRS §5.12, URD §7.12 |
| [FR-AST-10](#FR-AST-10) | [I] Điều khoản seed ở `Draft`, chỉ `Active` khi LEG duyệt; người tạo không tự duyệt (OI-08, NFR-SE-03) | SRS §5.12, URD §7.12 |
| [FR-AST-11](#FR-AST-11) | [T] Rule đánh giá lại ≤ 5 phút sau khi Entitlement hoặc AssetUsage đổi; AssetFinding duy nhất theo `(asset_usage_id, rule)`; ân hạn hạ `fail` xuống `warn` (NFR-PE-08) | SRS §5.12, URD §7.12 |
| [FR-AST-12](#FR-AST-12) | [T] Entitlement thiếu licensee, `license_term_code`, chứng từ hay `valid_until`/`perpetual` bị chặn; không xoá chứng từ khi còn AssetUsage liên kết; thu hồi đóng bằng `Revoked` | SRS §5.12, URD §7.12 |
| [FR-AST-13](#FR-AST-13) | [T] Mỗi mốc gửi đúng một lần (`reminded_milestones`); gia hạn còn hơn 60 ngày đưa Entitlement về Active và xoá mốc đã nhắc | SRS §5.12, URD §7.12 |
| [FR-AST-14](#FR-AST-14) | [T] `usage_counter` vượt 80% sinh rule `limit_warning`, vượt 100% sinh `limit_exceeded`; giảm `limits` dưới mức đã dùng không bị chặn mà sinh rule | SRS §5.12, URD §7.12 |
| [FR-AST-15](#FR-AST-15) | [T] Credits sinh được ở 3 định dạng, song ngữ, hiển thị đúng ở 360 px; `credits/verify` so `content_hash`, thiếu ghi công sinh `attribution_missing`; exit code 20 khi thiếu (NFR-UX-03) | SRS §5.12, URD §7.12 |
| [FR-AST-16](#FR-AST-16) | [T] Ring chỉ nhận Approved, Conditional, Banned (và Retired); `Conditional` bắt buộc `conditions`; ARB hoặc LEG đổi ring; `next_review_at` ≤ 90 ngày | SRS §5.12, URD §7.12 |
| [FR-AST-17](#FR-AST-17) | [T] Entitlement có licensee = TECHWISE và không chuyển nhượng được là mục bắt buộc; đóng checklist khi còn mục bắt buộc trả 409 `handover_incomplete` | SRS §5.12, URD §7.12 |
| [FR-AST-18](#FR-AST-18) | [T] `violations[].group` ∈ Components, Assets; AssetFinding mức fail ở prod làm gate fail (BRL-40); `asset-compliance` hiển thị trạng thái ân hạn | SRS §5.12, URD §7.12 |
| [FR-AST-19](#FR-AST-19) | [T] Đóng bắt buộc `resolution`; `Risk_accepted` bắt buộc `decision_id` Approved còn hạn; người tạo Decision không tự duyệt | SRS §5.12, URD §7.12 |
| [FR-CAT-01](#FR-CAT-01) | [T] Tìm mục theo `layer`, `ring`, tên; danh sách layer lấy từ API-T-26; sửa trường biên tập tăng `revision` và ghi audit | SRS §5.7, URD §7.7 |
| [FR-CAT-02](#FR-CAT-02) | [T, D] Ring chỉ nhận bốn giá trị; Radar và bảng cùng dữ liệu; lịch sử ring và phiên bản tối thiểu tra được ở API-T-05 | SRS §5.7, URD §7.7 |
| [FR-CAT-03](#FR-CAT-03) | [D] Chi tiết mục có license, CVE mở, EOL, health score, số dự án dùng lấy từ dữ liệu sống, không nhập tay; `rationale` và `alternatives` sửa được qua API-T-04 | SRS §5.7, URD §7.7 |
| [FR-CAT-04](#FR-CAT-04) | [T] Gợi ý chỉ xuất hiện khi line có Vulnerability Published từ High trở lên đã có bản vá và `min_recommended_versions` thấp hơn; gợi ý làm căn cứ tạo CR `change=version` | SRS §5.7, URD §7.7 |
| [FR-CAT-05](#FR-CAT-05) | [T] Người tạo CR không tự duyệt (403 `separation_of_duties`); PATCH mục không đổi được ring; CR duyệt ghi `arb_decision`, `decision_id` và tăng `revision` (NFR-SE-03) | SRS §5.7, URD §7.7 |
| [FR-CAT-06](#FR-CAT-06) | [T] Rà soát đặt `next_review_at` ≤ 90 ngày; mục quá hạn hiển thị "Cần rà soát" và nhắc owner | SRS §5.7, URD §7.7 |
| [FR-CAT-07](#FR-CAT-07) | [T] Baseline có `purl_base`, `min_version` cho 100% mục; ánh xạ được sang `go.mod`, `package.json`, `pom`, `Dockerfile`; trùng `(catalog_revision, format, layer_key)` trả bản cũ (NFR-CO-05) | SRS §5.7, URD §7.7 |
| [FR-CAT-08](#FR-CAT-08) | [D] Kết quả so sánh gắn được vào StackProposal qua `comparison_refs`. Endpoint lưu Comparison sheet riêng chưa có: xem Mục 10.6 | SRS §5.7, URD §7.7 |
| [FR-CI-01](#FR-CI-01) | [T] Kết quả `result` ∈ pass, warn, fail; mỗi vi phạm có `code`, `level`, `target_ref`, `link`; evaluate snapshot 1.000 component p95 ≤ 5 giây; Evaluation `Completed` bất biến (NFR-PE-01) | SRS §5.10, URD §7.10 |
| [FR-CI-02](#FR-CI-02) | [T] Từng điều kiện của BRL-40 cho fail và BRL-41 cho warn có ca kiểm thử dương và âm; mỗi vi phạm nêu rule áp dụng (NFR-MA-04) | SRS §5.10, URD §7.10 |
| [FR-CI-03](#FR-CI-03) | [T] `gate_failure_mode` đổi được bởi Owner hoặc SEC không cần deploy; ghi đè gate chỉ qua Decision Approved còn hạn, đúng dự án và đúng đối tượng vi phạm | SRS §5.10, URD §7.10 |
| [FR-CI-04](#FR-CI-04) | [D] Ba template chạy `pbom scan`, `submit`, `gate` trên pipeline mẫu; token là biến môi trường, không nhúng giá trị (NFR-CO-02) | SRS §5.10, URD §7.10 |
| [FR-CI-05](#FR-CI-05) | [T] Kết quả nhập không tạo SbomSnapshot và không đóng Usage nào; bản ghi không ánh xạ được bị từ chối với `reject_code=mapping` (NFR-CO-07) | SRS §5.10, URD §7.10 |
| [FR-GOV-01](#FR-GOV-01) | [T] Thiếu trường bắt buộc theo `type` bị chặn ở API-G-05; `expires_at` quá 12 tháng trả 422 `expiry_too_long`; người tạo tự duyệt trả 403 `separation_of_duties`; Approved khoá nội dung (NFR-SE-05) | SRS §5.9, URD §7.9 |
| [FR-GOV-02](#FR-GOV-02) | [T] Decision `Expired` hoặc `Revoked` đưa Finding gắn Decision về Open, SLA tính lại (OI-15); Alert `decision_expiring` trước 14 ngày; gia hạn tạo Decision mới | SRS §5.9, URD §7.9 |
| [FR-GOV-03](#FR-GOV-03) | [T, I] Role ứng dụng không có UPDATE, DELETE trên `audit_logs` (test bị từ chối); chuỗi băm kiểm được ở API-G-43; lưu ≥ 3 năm (NFR-AU-01) | SRS §5.9, URD §7.9 |
| [FR-GOV-04](#FR-GOV-04) | [T] Critical, KEV gửi SEC ngay, Medium và Low gộp digest 08:30 GMT+7; cùng (sự kiện, đối tượng, người nhận) chỉ gửi một lần trong 24 giờ; không tắt được Critical, KEV, SLA breached; email luôn là kênh dự phòng (NFR-RE-03) | SRS §5.9, URD §7.9 |
| [FR-GOV-05](#FR-GOV-05) | [T] Tại 75% sinh Alert `sla_reminder` cho TL và assignee; quá 100% sinh `sla_breached`, `sla_state=breached`, chuyển cấp PM và SEC; mỗi mốc gửi một lần | SRS §5.9, URD §7.9 |
| [FR-GOV-06](#FR-GOV-06) | [T] Báo cáo `monthly_project` và `monthly_company` sinh đúng lịch, ghi `data_as_of`, gửi email; lọc theo quyền dữ liệu nguồn (NFR-UX-02) | SRS §5.9, URD §7.9 |
| [FR-GOV-07](#FR-GOV-07) | [T, A] `compliance_package` chứa SBOM, VEX, báo cáo license, tóm tắt CVE và SLA; sinh ≤ 5 phút với 1.000 component (NFR-PE-04) | SRS §5.9, URD §7.9 |
| [FR-INV-01](#FR-INV-01) | [T] Lưu trữ rồi khôi phục giữ nguyên dữ liệu; thêm Environment `prod` thứ hai trả 409; `sla_profile_id` lỏng hơn mặc định trả 422 `sla_profile_looser`; không có API xoá (NFR-IC-04) | SRS §5.2, URD §7.2 |
| [FR-INV-02](#FR-INV-02) | [T] SBOM 2.000 component cộng evaluate p95 ≤ 30 giây; nộp trùng hash trả `meta.deduplicated=true`, không tạo snapshot; vượt 50 MB hoặc 20.000 component trả 413; phiên bản thấp trả 400 `unsupported_spec_version` (NFR-PE-01, NFR-CO-01) | SRS §5.2, URD §7.2 |
| [FR-INV-03](#FR-INV-03) | [T] Exit code đúng Phụ lục B mục 5 (0, 2, 3, 10, 20, 30); chạy trên Linux, macOS, Windows; thử lại 3 lần khi 429 hoặc 5xx; quá 30 giây không phản hồi thì áp `gate_failure_mode` (NFR-CO-02, NFR-RE-01) | SRS §5.2, URD §7.2 |
| [FR-INV-04](#FR-INV-04) | [T] `confirm_due_at` = lần xác nhận cộng 30 ngày; quá hạn chuyển Stale và sinh nhắc; xác nhận Usage Stale đưa về Active; môi trường `prod` thiếu khai báo sinh nhắc, không chặn gate (OI-07) | SRS §5.2, URD §7.2 |
| [FR-INV-05](#FR-INV-05) | [T, D] Usage gián tiếp có ≥ 1 đường tới phụ thuộc trực tiếp (tối đa 10 đường mỗi Finding); đồ thị của snapshot `Current` khớp số node và cạnh của SBOM mẫu | SRS §5.2, URD §7.2 |
| [FR-INV-06](#FR-INV-06) | [T] Ngữ cảnh cấp Usage ghi đè cấp Service; snapshot sau kế thừa; đổi ngữ cảnh tính lại Priority ≤ 5 phút (NFR-PE-03); Usage gián tiếp lấy ngữ cảnh rủi ro cao nhất của các cha | SRS §5.2, URD §7.2 |
| [FR-INV-07](#FR-INV-07) | [T, D] Diff hai snapshot 2.000 component ≤ 2 giây; kết quả liệt kê đủ bốn loại thay đổi trên cặp SBOM mẫu (NFR-PE-03) | SRS §5.2, URD §7.2 |
| [FR-INV-08](#FR-INV-08) | [T] Import 5.000 dòng ≤ 60 giây; lỗi báo theo dòng trong `errors[]`; nhập lại cùng tệp không tạo trùng (NFR-PE-03, AC-G4) | SRS §5.2, URD §7.2 |
| [FR-KB-01](#FR-KB-01) | [T, A] `freshness=Fresh` khi `last_success_at` trong 60 phút; chạy lại cùng dữ liệu không tạo trùng; độ trễ CVE vào KB p95 ≤ 2 giờ (NFR-PE-05, NFR-CO-06) | SRS §5.3, URD §7.3 |
| [FR-KB-02](#FR-KB-02) | [T, A] NVD ≤ 2 giờ, KEV ≤ 6 giờ, EPSS ≤ 24 giờ tính từ `last_success_at`; nguồn lỗi giữ dữ liệu last-known-good, không xoá (NFR-PE-05) | SRS §5.3, URD §7.3 |
| [FR-KB-03](#FR-KB-03) | [T] Component có `eol_slug` có ReleaseLine cập nhật ≤ 24 giờ; slug không có dữ liệu giữ trạng thái `Unknown` ("Chưa rõ EOL") | SRS §5.3, URD §7.3 |
| [FR-KB-04](#FR-KB-04) | [T] Component có `repo_url` có `last_release_at`, `releases_12m`, `upstream_archived` cập nhật ≤ 7 ngày; các trường đồng bộ chỉ đọc qua API-K-28 | SRS §5.3, URD §7.3 |
| [FR-KB-05](#FR-KB-05) | [T] Mục mới tạo Vulnerability `Unclassified` và Alert `unclassified_advisory` cho SEC; phân loại tay chuyển `Published`, tạo AffectedRange `source=vendor` và kích hoạt matching | SRS §5.3, URD §7.3 |
| [FR-KB-06](#FR-KB-06) | [T] Một Vulnerability cho mọi alias của cùng lỗi; tra cứu được bằng `id`, `primary_id` hoặc alias; mã hiển thị ưu tiên CVE | SRS §5.3, URD §7.3 |
| [FR-KB-07](#FR-KB-07) | [T] Mọi lần chạy có ImportJob; nguồn `Stale` hoặc lỗi quá 24 giờ sinh Alert `stale_source` cho SEC và nhãn "dữ liệu cũ" (NFR-RE-04) | SRS §5.3, URD §7.3 |
| [FR-KB-08](#FR-KB-08) | UC-41 | SRS §2.1 |
| [FR-KB-09](#FR-KB-09) | UC-41 | SRS §2.1 |
| [FR-KB-10](#FR-KB-10) | UC-10, UC-41 | SRS §2.1 |
| [FR-LCM-01](#FR-LCM-01) | [D] Mỗi Usage có release line, `eol_date`, `eos_date`, số ngày còn lại; `Unknown` hiển thị "Chưa rõ EOL" | SRS §5.5, URD §7.5 |
| [FR-LCM-02](#FR-LCM-02) | [T] Mỗi mốc gửi đúng một lần cho mỗi cặp (release line, dự án); sau EOL nhắc hằng tuần; mốc đổi bằng Policy `eol` (NFR-PE-06) | SRS §5.5, URD §7.5 |
| [FR-LCM-03](#FR-LCM-03) | [T] `eol_date`, `eos_date`, `lts` đặt tay ưu tiên hơn nguồn đồng bộ; ghi audit; chỉ SEC, ARB | SRS §5.5, URD §7.5 |
| [FR-LCM-04](#FR-LCM-04) | [T] Score dưới 40, archived hoặc không release 12 tháng sinh cảnh báo Medium "Dấu hiệu ngừng bảo trì" | SRS §5.5, URD §7.5 |
| [FR-LCM-05](#FR-LCM-05) | [T] Go mặc định chỉ hỗ trợ 2 minor mới nhất; vi phạm sinh Alert `version_lag`; quy tắc sửa theo component không cần deploy | SRS §5.5, URD §7.5 |
| [FR-LCM-06](#FR-LCM-06) | [T] `deprecated=true` kèm `deprecated_message` hiển thị ở phiên bản; thay thế lấy từ `alternatives` của CatalogEntry. Đặt cờ chỉ qua pipeline metadata, chưa có endpoint đặt tay: xem Mục 10.6 | SRS §5.5, URD §7.5 |
| [FR-LIC-01](#FR-LIC-01) | [T] OR lấy license ít hạn chế nhất, AND lấy hạn chế nhất; NOASSERTION thuộc Unknown và action `block`; bộ test ma trận ALG-05 đạt | SRS §5.6, URD §7.6 |
| [FR-LIC-02](#FR-LIC-02) | [T] Dự án `mixed` lấy kết quả nghiêm ngặt nhất; mỗi action có `rule_ref` (`policy_version`, `matrix_cell`); dry-run không tạo Finding (NFR-MA-04) | SRS §5.6, URD §7.6 |
| [FR-LIC-03](#FR-LIC-03) | [T] Đổi nhóm license hạn chế hơn giữa hai phiên bản liền kề tạo drift và phiên bản ranh giới; Alert `license_drift` gửi SEC, ARB, owner dự án đang dùng | SRS §5.6, URD §7.6 |
| [FR-LIC-04](#FR-LIC-04) | [I] License source-available có `terms_text_ref`, `change_date`, `additional_use_grant` đúng phiên bản; phiên bản mới có Change Date hoặc Use Grant khác sinh yêu cầu rà soát | SRS §5.6, URD §7.6 |
| [FR-LIC-05](#FR-LIC-05) | [T] Báo cáo `license_report` và `notice` tạo được ở PDF, XLSX hoặc CSV, ghi `data_as_of`; chỉ chứa dự án người yêu cầu có quyền | SRS §5.6, URD §7.6 |
| [FR-SEL-01](#FR-SEL-01) | [T, D] Mỗi lần lưu chạy kiểm tra ring, ngoài Catalog, ràng buộc, CVE, EOL và trả `meta.checks[]`; StackProposal `Draft` sửa được, `Finalized` khoá `layers` | SRS §5.8, URD §7.8 |
| [FR-SEL-02](#FR-SEL-02) | [D] So sánh dưới 2 hoặc trên 4 ứng viên trả 422; đủ 6 tiêu chí (security, lifecycle, license, health, adoption, ring) kèm công thức hiển thị | SRS §5.8, URD §7.8 |
| [FR-SEL-03](#FR-SEL-03) | [T] Ứng viên ring Hold, Banned hoặc license bị cấm có `flags[].level=fail` (đỏ); không chọn làm lựa chọn chính khi chưa có Decision | SRS §5.8, URD §7.8 |
| [FR-SEL-04](#FR-SEL-04) | [T] Báo cáo chỉ sinh cho đề xuất `Finalized`; `valid_until` = ngày xuất cộng 30 ngày; PDF ≤ 2 phút (NFR-PE-04) | SRS §5.8, URD §7.8 |
| [FR-SEL-05](#FR-SEL-05) | [T] Chốt đề xuất có thành phần ngoài Catalog cho môi trường prod khi chưa có CR `Applied` hoặc Decision Approved còn hạn bị từ chối (409); tạo CR `add` từ đề xuất qua API-T-33 | SRS §5.8, URD §7.8 |
| [FR-SEL-06](#FR-SEL-06) | [T] Chốt với `planned_usage` tạo Usage `origin=planned`, `scope=direct` cho từng `chosen_version`, dùng lại Usage cùng service, môi trường, phiên bản | SRS §5.8, URD §7.8 |
| [FR-UI-01](#FR-UI-01) | [D] Số liệu khớp API-G-24; p95 ≤ 2 giây với 40 dự án, 10.000 Finding; vai trò khác SEC, ARB, LEG, ADM nhận 403 | SRS §5.11, URD §7.11 |
| [FR-UI-02](#FR-UI-02) | [D] Đủ 6 tab, dữ liệu lấy từ API tương ứng; người không thuộc dự án nhận 404 (AC-G1) | SRS §5.11, URD §7.11 |
| [FR-UI-03](#FR-UI-03) | [D] Hiển thị đủ sáu thành phần; dự án ngoài phạm vi quyền bị lọc (BRL-52) | SRS §5.11, URD §7.11 |
| [FR-UI-04](#FR-UI-04) | [D] Danh sách chỉ gồm Finding có `assignee_id` là người gọi, sắp theo `sla_due_at`; hành động chính ≤ 3 bước (NFR-UX-01) | SRS §5.11, URD §7.11 |
| [FR-UI-05](#FR-UI-05) | [D] Radar và bảng cùng một dữ liệu; lọc theo layer và ring | SRS §5.11, URD §7.11 |
| [FR-UI-06](#FR-UI-06) | [T, A] p95 ≤ 3 giây với 500 dự án, 1 triệu Usage; Usage `Closed` không tính; SEC, ARB thấy toàn công ty, vai trò khác chỉ thấy dự án được phép (NFR-PE-04) | SRS §5.11, URD §7.11 |
| [FR-UI-07](#FR-UI-07) | [D] Bộ lọc "chưa có entitlement" và "sắp hết hạn" trả đúng tập trên dữ liệu mẫu | SRS §5.11, URD §7.12 |
| [FR-UI-08](#FR-UI-08) | [D] PROC thấy Entitlement toàn công ty và chỉ tên dự án liên quan; hạn dùng, dự án dùng, chứng từ hiển thị đủ | SRS §5.11, URD §7.12 |
| [FR-UI-09](#FR-UI-09) | [D] Hiển thị metadata, vị trí dùng (`AssetUsage`), AssetFinding và lịch sử khai báo của một Asset | SRS §5.11, URD §7.12 |
| [FR-UI-10](#FR-UI-10) | [D] Lọc theo `vietnamese_support` và `usable_in_monetized` trả đúng mục ring Approved hoặc Conditional | SRS §5.11, URD §7.12 |
| [FR-VUL-01](#FR-VUL-01) | [T, A] Re-matching toàn bộ ≤ 15 phút với 500 dự án, 1 triệu Usage; Finding duy nhất theo `dedupe_key`; khoảng phiên bản không phân tích được cho kết quả Unknown, không coi là an toàn (NFR-PE-02) | SRS §5.4, URD §7.4 |
| [FR-VUL-02](#FR-VUL-02) | [T] 100% Finding có `priority_inputs` và `severity` gốc; P1 khi KEV hoặc Critical ở prod; bộ test bảng quy tắc ALG-02 đạt (NFR-MA-04) | SRS §5.4, URD §7.4 |
| [FR-VUL-03](#FR-VUL-03) | [T] `sla_due_at` = `sla_started_at` + giờ theo severity; KEV luôn áp SLA Critical; `sla_state=stopped` khi Fixed, Not_affected, Risk_accepted; Low không có SLA (NFR-PE-06) | SRS §5.4, URD §7.4 |
| [FR-VUL-04](#FR-VUL-04) | [T] Not_affected thiếu `justification` trả 422; `detail` dưới 20 ký tự trả 422; Not_affected của Critical hoặc KEV chờ SEC duyệt qua API-V-13; chuyển trạng thái sai trả 409 `invalid_state_transition` | SRS §5.4, URD §7.4 |
| [FR-VUL-05](#FR-VUL-05) | [T] Cùng khoá `context_hash` thì Finding mới tự áp VEX đã Issued; đổi ngữ cảnh làm `context_hash` đổi và Finding mở lại (Open) | SRS §5.4, URD §7.4 |
| [FR-VUL-06](#FR-VUL-06) | [D] Chi tiết Finding có phiên bản vá gần nhất và loại nâng cấp; nâng major kèm cảnh báo | SRS §5.4, URD §7.4 |
| [FR-VUL-07](#FR-VUL-07) | [T] Snapshot mới ngoài AffectedRange chuyển Fixed với `closed_reason=fixed_auto` và `closed_at`; phiên bản dính lỗi xuất hiện lại mở lại Finding (`reopen_count` tăng) | SRS §5.4, URD §7.4 |
| [FR-VUL-08](#FR-VUL-08) | [T] VEX xuất hợp lệ với công cụ xác thực OpenVEX và CycloneDX; chỉ gồm VexStatement `Issued` của dự án người gọi có quyền | SRS §5.4, URD §7.4 |

96/136 yêu cầu có tiêu chí nghiệm thu trong dữ liệu đã trích.

Chưa có tiêu chí nghiệm thu: NFR-AU-01, NFR-CO-01, NFR-CO-02, NFR-CO-03, NFR-CO-04, NFR-CO-05, NFR-CO-06, NFR-CO-07, NFR-IC-01, NFR-IC-02, NFR-IC-03, NFR-IC-04, NFR-IC-05, NFR-MA-01, NFR-MA-02, NFR-MA-03, NFR-MA-04, NFR-MA-05, NFR-MA-06, NFR-PE-01, NFR-PE-02, NFR-PE-03, NFR-PE-04, NFR-PE-05, NFR-PE-06, NFR-PE-07, NFR-PE-08, NFR-RE-01, NFR-RE-02, NFR-RE-03, NFR-RE-04, NFR-SE-01, NFR-SE-02, NFR-SE-03, NFR-SE-04, NFR-SE-05, NFR-SE-06, NFR-UX-01, NFR-UX-02, NFR-UX-03.

### 11.2 Test case, kịch bản và nghĩa vụ thiết kế

> **[THIẾU] Test case, kịch bản kiểm thử, kết quả mong đợi, nghĩa vụ thiết kế (design obligation).** chưa có trong dữ liệu đã trích từ tài liệu nguồn.
>
> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): SAD §10.6 “Kiểm thử”

### 11.3 Yêu cầu chức năng → phần tử thiết kế

| Mã | Quy tắc nghiệp vụ | Thực thể | Phi chức năng | Yêu cầu liên quan | Kiến trúc, ràng buộc, quyết định | Khác (* = chưa có mô tả) | Nguồn |
|---|---|---|---|---|---|---|---|
| [FR-ADM-01](#FR-ADM-01) | [BRL-52](#BRL-52) | [ENT-01](#ENT-01), [ENT-02](#ENT-02), [ENT-24](#ENT-24) | [NFR-SE-01](#NFR-SE-01) | — | — | API-P-01 *, API-P-05 * | SRS |
| [FR-ADM-02](#FR-ADM-02) | [BRL-52](#BRL-52) | [ENT-01](#ENT-01), [ENT-02](#ENT-02), [ENT-03](#ENT-03) | [NFR-IC-01](#NFR-IC-01), [NFR-SE-02](#NFR-SE-02) | — | — | API-P-03 *, API-P-04 *, API-P-06 * | SRS |
| [FR-ADM-03](#FR-ADM-03) | [BRL-53](#BRL-53) | [ENT-06](#ENT-06) | [NFR-SE-04](#NFR-SE-04) | — | — | API-P-02 *, API-P-22 * | SRS |
| [FR-ADM-04](#FR-ADM-04) | [BRL-11](#BRL-11), [BRL-22](#BRL-22), [BRL-23](#BRL-23), [BRL-24](#BRL-24), [BRL-40](#BRL-40), [BRL-41](#BRL-41) | [ENT-24](#ENT-24) | [NFR-MA-03](#NFR-MA-03) | — | — | ALG-05 *, ALG-08 *, API-P-30 * | SRS |
| [FR-AST-01](#FR-AST-01) | [BRL-44](#BRL-44) | [ENT-33](#ENT-33), [ENT-36](#ENT-36), [ENT-39](#ENT-39) | [NFR-PE-07](#NFR-PE-07), [NFR-SE-05](#NFR-SE-05) | — | — | ALG-07 * | SRS |
| [FR-AST-02](#FR-AST-02) | — | [ENT-33](#ENT-33), [ENT-36](#ENT-36) | — | — | — | — | SRS |
| [FR-AST-03](#FR-AST-03) | — | [ENT-33](#ENT-33) | [NFR-CO-02](#NFR-CO-02) | — | — | — | SRS |
| [FR-AST-04](#FR-AST-04) | [BRL-45](#BRL-45) | [ENT-33](#ENT-33), [ENT-34](#ENT-34) | — | — | — | — | SRS |
| [FR-AST-05](#FR-AST-05) | — | [ENT-33](#ENT-33) | — | — | — | — | SRS |
| [FR-AST-06](#FR-AST-06) | — | [ENT-33](#ENT-33), [ENT-37](#ENT-37) | — | — | — | — | SRS |
| [FR-AST-07](#FR-AST-07) | [BRL-51](#BRL-51) | [ENT-33](#ENT-33), [ENT-36](#ENT-36) | [NFR-IC-05](#NFR-IC-05) | — | — | API-S-10 *, API-S-12 *, API-S-18 *, API-S-19 * | SRS |
| [FR-AST-08](#FR-AST-08) | [BRL-44](#BRL-44) | [ENT-33](#ENT-33) | — | — | — | API-S-14 * | SRS |
| [FR-AST-09](#FR-AST-09) | — | [ENT-33](#ENT-33), [ENT-42](#ENT-42) | [NFR-RE-03](#NFR-RE-03), [NFR-SE-06](#NFR-SE-06) | — | — | API-S-48 *, API-S-55 *, API-S-56 * | SRS |
| [FR-AST-10](#FR-AST-10) | [BRL-34](#BRL-34) | [ENT-34](#ENT-34), [ENT-43](#ENT-43) | [NFR-SE-03](#NFR-SE-03) | — | — | — | SRS |
| [FR-AST-11](#FR-AST-11) | — | [ENT-35](#ENT-35), [ENT-36](#ENT-36), [ENT-37](#ENT-37) | [NFR-PE-08](#NFR-PE-08) | — | — | API-S-26 *, API-S-28 * | SRS |
| [FR-AST-12](#FR-AST-12) | [BRL-46](#BRL-46), [BRL-47](#BRL-47) | [ENT-35](#ENT-35), [ENT-43](#ENT-43) | — | — | — | API-S-15 * | SRS |
| [FR-AST-13](#FR-AST-13) | [BRL-48](#BRL-48) | [ENT-26](#ENT-26), [ENT-35](#ENT-35) | — | — | — | — | SRS |
| [FR-AST-14](#FR-AST-14) | [BRL-46](#BRL-46) | [ENT-35](#ENT-35), [ENT-37](#ENT-37) | — | — | — | — | SRS |
| [FR-AST-15](#FR-AST-15) | [BRL-49](#BRL-49) | [ENT-37](#ENT-37), [ENT-40](#ENT-40) | [NFR-UX-03](#NFR-UX-03) | — | — | API-S-38 * | SRS |
| [FR-AST-16](#FR-AST-16) | [BRL-45](#BRL-45) | [ENT-23](#ENT-23), [ENT-38](#ENT-38) | — | — | — | API-S-52 * | SRS |
| [FR-AST-17](#FR-AST-17) | [BRL-50](#BRL-50) | [ENT-35](#ENT-35), [ENT-41](#ENT-41) | — | — | — | API-S-43 * | SRS |
| [FR-AST-18](#FR-AST-18) | [BRL-40](#BRL-40), [BRL-41](#BRL-41) | [ENT-25](#ENT-25), [ENT-37](#ENT-37) | — | — | — | — | SRS |
| [FR-AST-19](#FR-AST-19) | [BRL-42](#BRL-42) | [ENT-23](#ENT-23), [ENT-35](#ENT-35), [ENT-37](#ENT-37) | — | — | — | — | SRS |
| [FR-CAT-01](#FR-CAT-01) | [BRL-29](#BRL-29) | [ENT-11](#ENT-11), [ENT-28](#ENT-28) | — | — | — | API-K-25 *, API-T-01 *, API-T-26 * | SRS |
| [FR-CAT-02](#FR-CAT-02) | [BRL-27](#BRL-27) | [ENT-28](#ENT-28) | — | — | — | API-T-05 *, API-T-06 * | SRS |
| [FR-CAT-03](#FR-CAT-03) | [BRL-17](#BRL-17), [BRL-20](#BRL-20) | [ENT-13](#ENT-13), [ENT-14](#ENT-14), [ENT-20](#ENT-20), [ENT-28](#ENT-28) | — | — | — | API-T-03 *, API-T-09 * | SRS |
| [FR-CAT-04](#FR-CAT-04) | — | [ENT-14](#ENT-14), [ENT-15](#ENT-15), [ENT-28](#ENT-28) | — | — | — | API-T-10 *, API-T-22 * | SRS |
| [FR-CAT-05](#FR-CAT-05) | [BRL-28](#BRL-28), [BRL-30](#BRL-30) | [ENT-23](#ENT-23), [ENT-28](#ENT-28), [ENT-29](#ENT-29) | [NFR-SE-03](#NFR-SE-03) | — | — | — | SRS |
| [FR-CAT-06](#FR-CAT-06) | [BRL-29](#BRL-29) | [ENT-26](#ENT-26), [ENT-28](#ENT-28) | — | — | — | API-T-07 *, API-T-08 * | SRS |
| [FR-CAT-07](#FR-CAT-07) | — | [ENT-28](#ENT-28), [ENT-31](#ENT-31) | [NFR-CO-05](#NFR-CO-05) | — | — | API-T-15 * | SRS |
| [FR-CAT-08](#FR-CAT-08) | [BRL-32](#BRL-32) | [ENT-30](#ENT-30) | — | — | — | API-T-25 *, API-T-27 *, API-T-28 *, API-T-31 * | SRS |
| [FR-CI-01](#FR-CI-01) | [BRL-40](#BRL-40), [BRL-41](#BRL-41), [BRL-42](#BRL-42), [BRL-43](#BRL-43) | [ENT-21](#ENT-21), [ENT-25](#ENT-25) | [NFR-PE-01](#NFR-PE-01) | — | — | API-C-06 *, API-C-07 * | SRS |
| [FR-CI-02](#FR-CI-02) | [BRL-19](#BRL-19), [BRL-25](#BRL-25), [BRL-27](#BRL-27), [BRL-40](#BRL-40), [BRL-41](#BRL-41) | [ENT-24](#ENT-24), [ENT-25](#ENT-25) | [NFR-MA-04](#NFR-MA-04) | — | — | API-C-05 *, API-P-31 *, API-P-37 * | URD, SRS |
| [FR-CI-03](#FR-CI-03) | [BRL-42](#BRL-42), [BRL-43](#BRL-43) | [ENT-03](#ENT-03), [ENT-23](#ENT-23), [ENT-24](#ENT-24) | — | — | — | API-C-03 *, API-C-04 *, API-G-06 * | SRS |
| [FR-CI-04](#FR-CI-04) | [BRL-53](#BRL-53) | — | [NFR-CO-02](#NFR-CO-02) | — | — | API-C-08 *, API-C-09 * | SRS |
| [FR-CI-05](#FR-CI-05) | [BRL-03](#BRL-03), [BRL-08](#BRL-08) | [ENT-09](#ENT-09), [ENT-18](#ENT-18), [ENT-21](#ENT-21) | [NFR-CO-07](#NFR-CO-07) | — | — | API-I-17 * | SRS, URD |
| [FR-GOV-01](#FR-GOV-01) | [BRL-14](#BRL-14), [BRL-33](#BRL-33), [BRL-34](#BRL-34), [BRL-35](#BRL-35) | [ENT-23](#ENT-23), [ENT-43](#ENT-43) | [NFR-SE-05](#NFR-SE-05) | — | — | ALG-11 *, API-G-01 *, API-G-05 * | SRS |
| [FR-GOV-02](#FR-GOV-02) | [BRL-36](#BRL-36) | [ENT-21](#ENT-21), [ENT-23](#ENT-23), [ENT-26](#ENT-26) | — | — | — | API-G-08 *, API-G-09 *, API-G-11 * | SRS |
| [FR-GOV-03](#FR-GOV-03) | [BRL-37](#BRL-37) | [ENT-44](#ENT-44) | [NFR-AU-01](#NFR-AU-01) | — | — | API-G-40 *, API-G-43 *, API-P-42 * | SRS |
| [FR-GOV-04](#FR-GOV-04) | [BRL-38](#BRL-38), [BRL-39](#BRL-39) | [ENT-26](#ENT-26), [ENT-27](#ENT-27), [ENT-45](#ENT-45), [ENT-46](#ENT-46) | [NFR-RE-03](#NFR-RE-03) | — | — | API-G-28 * | SRS |
| [FR-GOV-05](#FR-GOV-05) | [BRL-16](#BRL-16) | [ENT-21](#ENT-21), [ENT-26](#ENT-26) | — | — | — | API-G-17 * | SRS |
| [FR-GOV-06](#FR-GOV-06) | [BRL-52](#BRL-52) | [ENT-24](#ENT-24), [ENT-32](#ENT-32) | [NFR-UX-02](#NFR-UX-02) | — | — | API-T-37 * | SRS |
| [FR-GOV-07](#FR-GOV-07) | [BRL-24](#BRL-24) | [ENT-17](#ENT-17), [ENT-22](#ENT-22), [ENT-32](#ENT-32) | [NFR-PE-04](#NFR-PE-04) | — | — | API-I-06 *, API-V-16 * | SRS |
| [FR-INV-01](#FR-INV-01) | [BRL-52](#BRL-52) | [ENT-03](#ENT-03), [ENT-04](#ENT-04), [ENT-05](#ENT-05) | [NFR-IC-04](#NFR-IC-04) | — | — | API-P-09 * | SRS |
| [FR-INV-02](#FR-INV-02) | [BRL-01](#BRL-01), [BRL-02](#BRL-02), [BRL-03](#BRL-03), [BRL-53](#BRL-53) | [ENT-17](#ENT-17), [ENT-18](#ENT-18), [ENT-25](#ENT-25) | [NFR-CO-01](#NFR-CO-01), [NFR-CO-07](#NFR-CO-07), [NFR-PE-01](#NFR-PE-01) | — | — | API-I-01 * | SRS, URD |
| [FR-INV-03](#FR-INV-03) | — | [ENT-17](#ENT-17), [ENT-25](#ENT-25) | [NFR-CO-02](#NFR-CO-02), [NFR-RE-01](#NFR-RE-01) | — | — | API-C-01 *, API-C-02 *, API-S-01 * | SRS |
| [FR-INV-04](#FR-INV-04) | [BRL-01](#BRL-01), [BRL-04](#BRL-04) | [ENT-18](#ENT-18) | — | — | — | API-I-07 * | SRS |
| [FR-INV-05](#FR-INV-05) | [BRL-06](#BRL-06) | [ENT-17](#ENT-17), [ENT-18](#ENT-18) | — | — | — | API-I-04 *, API-I-09 *, API-I-18 * | SRS |
| [FR-INV-06](#FR-INV-06) | [BRL-05](#BRL-05), [BRL-06](#BRL-06) | [ENT-05](#ENT-05), [ENT-18](#ENT-18), [ENT-19](#ENT-19) | [NFR-PE-03](#NFR-PE-03) | — | — | API-I-13 *, API-P-20 * | SRS |
| [FR-INV-07](#FR-INV-07) | — | [ENT-17](#ENT-17), [ENT-18](#ENT-18), [ENT-20](#ENT-20) | [NFR-PE-03](#NFR-PE-03) | — | — | ALG-06 *, API-I-03 *, API-I-05 * | SRS |
| [FR-INV-08](#FR-INV-08) | [BRL-01](#BRL-01), [BRL-04](#BRL-04) | [ENT-18](#ENT-18) | [NFR-PE-03](#NFR-PE-03) | — | — | API-I-16 * | SRS |
| [FR-KB-01](#FR-KB-01) | — | [ENT-07](#ENT-07), [ENT-08](#ENT-08), [ENT-09](#ENT-09), [ENT-10](#ENT-10), [ENT-14](#ENT-14), [ENT-15](#ENT-15) | [NFR-CO-06](#NFR-CO-06), [NFR-PE-05](#NFR-PE-05) | — | — | ALG-12 *, ALG-13 *, API-K-03 *, API-K-09 *, API-K-13 *, API-K-36 *, API-K-37 * | SRS |
| [FR-KB-02](#FR-KB-02) | — | [ENT-08](#ENT-08), [ENT-09](#ENT-09), [ENT-10](#ENT-10), [ENT-14](#ENT-14) | [NFR-PE-05](#NFR-PE-05) | — | — | API-K-05 *, API-K-11 *, API-K-14 * | SRS |
| [FR-KB-03](#FR-KB-03) | [BRL-17](#BRL-17) | [ENT-08](#ENT-08), [ENT-11](#ENT-11), [ENT-13](#ENT-13) | — | — | — | ALG-09 *, API-K-34 *, API-K-35 * | SRS |
| [FR-KB-04](#FR-KB-04) | — | [ENT-08](#ENT-08), [ENT-11](#ENT-11), [ENT-13](#ENT-13) | — | — | — | ALG-10 *, API-K-21 *, API-K-27 *, API-K-28 * | SRS |
| [FR-KB-05](#FR-KB-05) | [BRL-07](#BRL-07) | [ENT-08](#ENT-08), [ENT-14](#ENT-14), [ENT-15](#ENT-15), [ENT-26](#ENT-26) | — | — | — | API-K-04 *, API-K-38 *, API-K-39 * | SRS |
| [FR-KB-06](#FR-KB-06) | [BRL-07](#BRL-07) | [ENT-14](#ENT-14) | — | — | — | — | SRS |
| [FR-KB-07](#FR-KB-07) | — | [ENT-08](#ENT-08), [ENT-09](#ENT-09), [ENT-26](#ENT-26) | [NFR-RE-04](#NFR-RE-04) | — | — | API-G-15 * | SRS |
| [FR-KB-08](#FR-KB-08) | — | — | [NFR-MA-05](#NFR-MA-05) | — | — | — | SRS |
| [FR-KB-09](#FR-KB-09) | — | — | — | — | — | — | SRS §2.1 |
| [FR-KB-10](#FR-KB-10) | — | — | — | — | — | — | SRS §2.1 |
| [FR-LCM-01](#FR-LCM-01) | [BRL-17](#BRL-17) | [ENT-13](#ENT-13), [ENT-18](#ENT-18) | — | — | — | API-T-02 *, API-V-21 *, API-V-22 * | SRS |
| [FR-LCM-02](#FR-LCM-02) | [BRL-18](#BRL-18), [BRL-19](#BRL-19) | [ENT-13](#ENT-13), [ENT-24](#ENT-24), [ENT-26](#ENT-26) | [NFR-PE-06](#NFR-PE-06) | — | — | API-P-32 * | SRS |
| [FR-LCM-03](#FR-LCM-03) | [BRL-17](#BRL-17) | [ENT-13](#ENT-13) | — | — | — | — | SRS |
| [FR-LCM-04](#FR-LCM-04) | [BRL-20](#BRL-20) | [ENT-11](#ENT-11), [ENT-13](#ENT-13), [ENT-26](#ENT-26) | — | — | — | API-V-24 * | SRS |
| [FR-LCM-05](#FR-LCM-05) | [BRL-21](#BRL-21) | [ENT-18](#ENT-18), [ENT-26](#ENT-26), [ENT-28](#ENT-28) | — | — | — | API-T-04 *, API-V-23 * | SRS |
| [FR-LCM-06](#FR-LCM-06) | — | [ENT-12](#ENT-12), [ENT-28](#ENT-28) | — | — | — | API-K-32 *, API-K-33 *, API-T-24 * | SRS |
| [FR-LIC-01](#FR-LIC-01) | [BRL-22](#BRL-22), [BRL-23](#BRL-23) | [ENT-16](#ENT-16), [ENT-20](#ENT-20) | — | — | — | API-K-40 *, API-K-41 *, API-K-43 *, API-V-25 * | SRS |
| [FR-LIC-02](#FR-LIC-02) | [BRL-24](#BRL-24) | [ENT-20](#ENT-20), [ENT-21](#ENT-21), [ENT-24](#ENT-24) | [NFR-MA-04](#NFR-MA-04) | — | — | API-K-42 *, API-V-29 * | SRS |
| [FR-LIC-03](#FR-LIC-03) | [BRL-25](#BRL-25), [BRL-26](#BRL-26) | [ENT-20](#ENT-20), [ENT-21](#ENT-21), [ENT-26](#ENT-26) | — | — | — | API-V-26 * | SRS |
| [FR-LIC-04](#FR-LIC-04) | [BRL-26](#BRL-26) | [ENT-20](#ENT-20), [ENT-43](#ENT-43) | — | — | — | — | SRS |
| [FR-LIC-05](#FR-LIC-05) | [BRL-24](#BRL-24) | [ENT-20](#ENT-20), [ENT-32](#ENT-32) | — | — | — | API-T-35 *, API-T-36 *, API-T-38 *, API-T-39 * | SRS |
| [FR-SEL-01](#FR-SEL-01) | [BRL-27](#BRL-27), [BRL-30](#BRL-30), [BRL-32](#BRL-32) | [ENT-28](#ENT-28), [ENT-30](#ENT-30) | — | — | — | API-T-23 *, API-T-34 * | SRS |
| [FR-SEL-02](#FR-SEL-02) | [BRL-32](#BRL-32) | [ENT-28](#ENT-28) | — | — | — | — | SRS |
| [FR-SEL-03](#FR-SEL-03) | [BRL-24](#BRL-24), [BRL-32](#BRL-32) | [ENT-28](#ENT-28), [ENT-30](#ENT-30) | — | — | — | — | SRS |
| [FR-SEL-04](#FR-SEL-04) | [BRL-31](#BRL-31) | [ENT-30](#ENT-30), [ENT-32](#ENT-32) | [NFR-PE-04](#NFR-PE-04) | — | — | API-T-41 * | SRS |
| [FR-SEL-05](#FR-SEL-05) | [BRL-30](#BRL-30) | [ENT-23](#ENT-23), [ENT-29](#ENT-29), [ENT-30](#ENT-30) | — | — | — | API-T-32 *, API-T-33 * | SRS |
| [FR-SEL-06](#FR-SEL-06) | [BRL-27](#BRL-27) | [ENT-18](#ENT-18), [ENT-30](#ENT-30) | — | — | — | — | SRS |
| [FR-UI-01](#FR-UI-01) | [BRL-10](#BRL-10), [BRL-52](#BRL-52) | [ENT-13](#ENT-13), [ENT-17](#ENT-17), [ENT-21](#ENT-21), [ENT-23](#ENT-23) | — | — | — | API-G-24 * | SRS |
| [FR-UI-02](#FR-UI-02) | [BRL-52](#BRL-52) | [ENT-03](#ENT-03), [ENT-17](#ENT-17), [ENT-18](#ENT-18), [ENT-21](#ENT-21), [ENT-23](#ENT-23) | — | — | — | API-G-02 *, API-G-26 *, API-I-08 *, API-P-11 * | SRS |
| [FR-UI-03](#FR-UI-03) | [BRL-17](#BRL-17), [BRL-52](#BRL-52) | [ENT-11](#ENT-11), [ENT-12](#ENT-12), [ENT-13](#ENT-13), [ENT-20](#ENT-20), [ENT-28](#ENT-28) | — | — | — | API-V-20 *, API-V-27 * | SRS |
| [FR-UI-04](#FR-UI-04) | [BRL-11](#BRL-11) | [ENT-21](#ENT-21) | [NFR-UX-01](#NFR-UX-01) | — | — | API-G-25 *, API-V-09 * | SRS |
| [FR-UI-05](#FR-UI-05) | [BRL-27](#BRL-27) | [ENT-28](#ENT-28) | — | — | — | — | SRS |
| [FR-UI-06](#FR-UI-06) | [BRL-52](#BRL-52) | [ENT-11](#ENT-11), [ENT-14](#ENT-14), [ENT-18](#ENT-18) | [NFR-PE-04](#NFR-PE-04) | — | — | API-V-19 * | SRS |
| [FR-UI-07](#FR-UI-07) | [BRL-52](#BRL-52) | [ENT-33](#ENT-33), [ENT-35](#ENT-35), [ENT-36](#ENT-36), [ENT-37](#ENT-37) | — | — | — | API-S-09 *, API-S-13 *, API-S-17 *, API-S-34 *, API-S-51 * | SRS |
| [FR-UI-08](#FR-UI-08) | [BRL-46](#BRL-46), [BRL-47](#BRL-47), [BRL-52](#BRL-52) | [ENT-35](#ENT-35), [ENT-43](#ENT-43) | — | — | — | API-S-25 *, API-S-27 *, API-S-30 *, API-S-32 *, API-S-33 * | SRS |
| [FR-UI-09](#FR-UI-09) | [BRL-44](#BRL-44) | [ENT-33](#ENT-33), [ENT-36](#ENT-36), [ENT-37](#ENT-37) | — | — | — | API-S-05 *, API-S-11 *, API-S-16 * | SRS |
| [FR-UI-10](#FR-UI-10) | [BRL-45](#BRL-45) | [ENT-34](#ENT-34), [ENT-38](#ENT-38) | — | — | — | API-S-20 *, API-S-37 * | SRS |
| [FR-VUL-01](#FR-VUL-01) | [BRL-05](#BRL-05), [BRL-06](#BRL-06), [BRL-07](#BRL-07), [BRL-08](#BRL-08), [BRL-15](#BRL-15) | [ENT-14](#ENT-14), [ENT-15](#ENT-15), [ENT-18](#ENT-18), [ENT-21](#ENT-21) | [NFR-PE-02](#NFR-PE-02) | — | — | ALG-04 *, API-V-01 *, API-V-02 * | SRS |
| [FR-VUL-02](#FR-VUL-02) | [BRL-09](#BRL-09), [BRL-10](#BRL-10) | [ENT-19](#ENT-19), [ENT-21](#ENT-21) | [NFR-MA-04](#NFR-MA-04) | — | — | API-V-17 * | SRS |
| [FR-VUL-03](#FR-VUL-03) | [BRL-11](#BRL-11), [BRL-16](#BRL-16) | [ENT-21](#ENT-21), [ENT-24](#ENT-24) | [NFR-PE-06](#NFR-PE-06) | — | — | — | SRS |
| [FR-VUL-04](#FR-VUL-04) | [BRL-12](#BRL-12), [BRL-13](#BRL-13), [BRL-14](#BRL-14) | [ENT-21](#ENT-21), [ENT-22](#ENT-22) | — | — | — | API-V-03 *, API-V-08 *, API-V-13 * | SRS |
| [FR-VUL-05](#FR-VUL-05) | [BRL-12](#BRL-12) | [ENT-19](#ENT-19), [ENT-21](#ENT-21), [ENT-22](#ENT-22) | — | — | — | API-I-14 *, API-V-04 *, API-V-12 *, API-V-14 * | SRS |
| [FR-VUL-06](#FR-VUL-06) | — | [ENT-12](#ENT-12), [ENT-15](#ENT-15), [ENT-21](#ENT-21) | — | — | — | — | SRS |
| [FR-VUL-07](#FR-VUL-07) | [BRL-15](#BRL-15) | [ENT-17](#ENT-17), [ENT-21](#ENT-21) | — | — | — | API-V-07 * | SRS |
| [FR-VUL-08](#FR-VUL-08) | [BRL-12](#BRL-12) | [ENT-22](#ENT-22) | — | — | — | — | SRS |

### 11.4 Yêu cầu phi chức năng → phần tử thiết kế

| Mã | Quy tắc nghiệp vụ | Thực thể | Phi chức năng | Yêu cầu liên quan | Kiến trúc, ràng buộc, quyết định | Khác (* = chưa có mô tả) | Nguồn |
|---|---|---|---|---|---|---|---|
| [NFR-AU-01](#NFR-AU-01) | [BRL-37](#BRL-37) | [ENT-44](#ENT-44) | [NFR-CO-07](#NFR-CO-07) | [FR-GOV-03](#FR-GOV-03) | [ADR-14](#ADR-14), [INT-08](#INT-08) | — | URD, SRS, SAD |
| [NFR-CO-01](#NFR-CO-01) | — | [ENT-17](#ENT-17) | — | [FR-INV-02](#FR-INV-02) | — | — | SRS |
| [NFR-CO-02](#NFR-CO-02) | — | — | — | [FR-AST-03](#FR-AST-03), [FR-CI-04](#FR-CI-04), [FR-INV-03](#FR-INV-03) | [CLI là binary Go không cần root](#FACT-57542dc1c3) | — | SRS |
| [NFR-CO-03](#NFR-CO-03) | — | — | — | — | — | — | SRS §6.2, URD §8.2 |
| [NFR-CO-04](#NFR-CO-04) | — | — | — | — | — | — | SRS §6.2, URD §8.2 |
| [NFR-CO-05](#NFR-CO-05) | — | [ENT-31](#ENT-31) | [CI bắt buộc](#FACT-b0bf4a0273) | [FR-CAT-07](#FR-CAT-07) | [Ghim phụ thuộc](#FACT-0cdb51f711) | — | SRS, SAD |
| [NFR-CO-06](#NFR-CO-06) | — | [ENT-07](#ENT-07), [ENT-09](#ENT-09), [ENT-10](#ENT-10) | [NFR-PE-05](#NFR-PE-05) | [FR-KB-01](#FR-KB-01) | [INT-06](#INT-06) | — | URD, SRS |
| [NFR-CO-07](#NFR-CO-07) | — | [ENT-45](#ENT-45) | [NFR-AU-01](#NFR-AU-01) | [FR-CI-05](#FR-CI-05), [FR-INV-02](#FR-INV-02) | — | — | URD, SRS |
| [NFR-IC-01](#NFR-IC-01) | — | [ENT-01](#ENT-01), [ENT-02](#ENT-02) | — | [FR-ADM-02](#FR-ADM-02) | — | — | SRS |
| [NFR-IC-02](#NFR-IC-02) | — | — | — | — | [INT-01](#INT-01), [INT-02](#INT-02), [INT-04](#INT-04) | — | SRS |
| [NFR-IC-03](#NFR-IC-03) | — | [ENT-18](#ENT-18), [ENT-36](#ENT-36) | — | — | [ADR-10](#ADR-10) | — | SRS, SAD |
| [NFR-IC-04](#NFR-IC-04) | — | — | — | [FR-INV-01](#FR-INV-01) | [INT-09](#INT-09) | — | SRS |
| [NFR-IC-05](#NFR-IC-05) | [BRL-04](#BRL-04) | — | — | [FR-AST-07](#FR-AST-07) | — | — | SRS |
| [NFR-MA-01](#NFR-MA-01) | — | — | — | — | — | — | SRS §6.6, URD §8.6 |
| [NFR-MA-02](#NFR-MA-02) | — | — | — | — | — | — | SRS §6.6, URD §8.6 |
| [NFR-MA-03](#NFR-MA-03) | — | [ENT-24](#ENT-24) | — | [FR-ADM-04](#FR-ADM-04) | — | — | SRS |
| [NFR-MA-04](#NFR-MA-04) | — | [ENT-21](#ENT-21), [ENT-25](#ENT-25) | — | [FR-CI-02](#FR-CI-02), [FR-LIC-02](#FR-LIC-02), [FR-VUL-02](#FR-VUL-02) | — | — | SRS |
| [NFR-MA-05](#NFR-MA-05) | — | [ENT-07](#ENT-07) | — | [Cập nhật connector khi đổi schema](#FACT-873d2e6032), [FR-KB-08](#FR-KB-08) | — | — | SRS, URD |
| [NFR-MA-06](#NFR-MA-06) | — | — | — | — | [Trường mở rộng jsonb](#FACT-91507b2cbc) | — | SAD |
| [NFR-PE-01](#NFR-PE-01) | — | — | [Ngân sách POST /sbom](#FACT-7320d119a1) | [FR-CI-01](#FR-CI-01), [FR-INV-02](#FR-INV-02) | — | — | SRS, SAD |
| [NFR-PE-02](#NFR-PE-02) | — | — | [NFR-PE-03](#NFR-PE-03) | [FR-VUL-01](#FR-VUL-01) | — | — | SRS |
| [NFR-PE-03](#NFR-PE-03) | — | — | [NFR-PE-02](#NFR-PE-02) | [FR-INV-06](#FR-INV-06), [FR-INV-07](#FR-INV-07), [FR-INV-08](#FR-INV-08) | — | — | SRS |
| [NFR-PE-04](#NFR-PE-04) | — | — | — | [FR-GOV-07](#FR-GOV-07), [FR-SEL-04](#FR-SEL-04), [FR-UI-06](#FR-UI-06) | — | — | SRS |
| [NFR-PE-05](#NFR-PE-05) | — | [ENT-08](#ENT-08) | [Độ tươi dữ liệu ngoài](#FACT-e04513f453), [NFR-CO-06](#NFR-CO-06) | [FR-KB-01](#FR-KB-01), [FR-KB-02](#FR-KB-02) | — | — | SRS, URD, SAD |
| [NFR-PE-06](#NFR-PE-06) | — | — | — | [FR-LCM-02](#FR-LCM-02), [FR-VUL-03](#FR-VUL-03) | — | — | SRS |
| [NFR-PE-07](#NFR-PE-07) | — | — | — | [FR-AST-01](#FR-AST-01) | — | — | SRS |
| [NFR-PE-08](#NFR-PE-08) | — | — | — | [FR-AST-11](#FR-AST-11) | — | — | SRS |
| [NFR-RE-01](#NFR-RE-01) | [BRL-43](#BRL-43), [Xử lý lỗi nguồn ngoài](#FACT-95f031570e) | [ENT-03](#ENT-03) | — | [FR-INV-03](#FR-INV-03) | — | — | URD, SRS |
| [NFR-RE-02](#NFR-RE-02) | [BRL-38](#BRL-38) | — | — | — | [ADR-08](#ADR-08) | — | SRS, SAD |
| [NFR-RE-03](#NFR-RE-03) | [Email dự phòng và cảnh báo tồn đọng](#FACT-e573d8cece) | — | — | [FR-AST-09](#FR-AST-09), [FR-GOV-04](#FR-GOV-04) | — | — | SRS, SAD |
| [NFR-RE-04](#NFR-RE-04) | [Xử lý lỗi nguồn ngoài](#FACT-95f031570e) | — | — | [FR-KB-07](#FR-KB-07) | — | — | SRS |
| [NFR-SE-01](#NFR-SE-01) | — | — | — | [FR-ADM-01](#FR-ADM-01) | — | — | SRS |
| [NFR-SE-02](#NFR-SE-02) | [BRL-52](#BRL-52) | — | — | [FR-ADM-02](#FR-ADM-02) | — | — | SRS |
| [NFR-SE-03](#NFR-SE-03) | [BRL-13](#BRL-13), [BRL-28](#BRL-28), [BRL-35](#BRL-35) | — | — | [FR-AST-10](#FR-AST-10), [FR-CAT-05](#FR-CAT-05) | — | — | SRS |
| [NFR-SE-04](#NFR-SE-04) | [BRL-53](#BRL-53) | [ENT-06](#ENT-06) | — | [FR-ADM-03](#FR-ADM-03) | — | — | SRS |
| [NFR-SE-05](#NFR-SE-05) | — | [ENT-23](#ENT-23), [ENT-43](#ENT-43) | — | [FR-AST-01](#FR-AST-01), [FR-GOV-01](#FR-GOV-01) | [Thời hạn lưu trữ dữ liệu](#FACT-e9a3db0731), [INT-05](#INT-05) | — | SRS, URD |
| [NFR-SE-06](#NFR-SE-06) | — | — | — | [FR-AST-09](#FR-AST-09) | [ADR-13](#ADR-13) | — | SRS, SAD |
| [NFR-UX-01](#NFR-UX-01) | — | — | — | [FR-UI-04](#FR-UI-04) | — | — | SRS |
| [NFR-UX-02](#NFR-UX-02) | — | [ENT-32](#ENT-32) | — | [FR-GOV-06](#FR-GOV-06) | — | — | SRS |
| [NFR-UX-03](#NFR-UX-03) | — | [ENT-26](#ENT-26), [ENT-40](#ENT-40) | — | [FR-AST-15](#FR-AST-15) | — | — | SRS |

### 11.5 Nhóm yêu cầu → thành phần kiến trúc (bảng truy vết của SAD)

| Nhóm yêu cầu | Thành phần kiến trúc | Nguồn |
|---|---|---|
| <a id="FR-ADM"></a>FR-ADM | [SVC-01](#SVC-01), [SVC-02](#SVC-02), [M-04](#M-04) (Policy), [M-07](#M-07) | SAD §12.1 |
| <a id="FR-AST"></a>FR-AST | [M-06](#M-06) | SAD §12.1 |
| <a id="FR-CAT"></a>FR-CAT | [M-05](#M-05) | SAD §12.1 |
| <a id="FR-CI"></a>FR-CI | [M-04](#M-04), CLI ([CT-02](#CT-02)) | SAD §12.1 |
| <a id="FR-GOV"></a>FR-GOV | [M-07](#M-07), [WRK-04](#WRK-04), [WRK-05](#WRK-05) | SAD §12.1 |
| <a id="FR-INV"></a>FR-INV | [M-01](#M-01), [WRK-01](#WRK-01) | SAD §12.1 |
| <a id="FR-KB"></a>FR-KB | [M-02](#M-02), [WRK-03](#WRK-03), CN-nn | SAD §12.1 |
| <a id="FR-LIC"></a>FR-LIC | [M-03](#M-03), [M-04](#M-04) | SAD §12.1 |
| <a id="FR-UI"></a>FR-UI | [CT-01](#CT-01) | SAD §12.1 |
| <a id="FR-VUL"></a>FR-VUL | [M-03](#M-03), [WRK-02](#WRK-02) | SAD §12.1 |
| <a id="NFR-AU"></a>NFR-AU | [ADR-14](#ADR-14), §10.5 | SAD §12.1 |
| <a id="NFR-CO"></a>NFR-CO | [ADR-09](#ADR-09), [ADR-11](#ADR-11), §8.2 | SAD §12.1 |
| <a id="NFR-IC"></a>NFR-IC | [ADR-07](#ADR-07), [ADR-10](#ADR-10), §7 | SAD §12.1 |
| <a id="NFR-MA"></a>NFR-MA | [ADR-02](#ADR-02), [ADR-09](#ADR-09), AD-08 | SAD §12.1 |
| <a id="NFR-PE"></a>NFR-PE | QA-01, QA-02, [ADR-07](#ADR-07), [ADR-08](#ADR-08), §8.1..8.3, §10.4 | SAD §12.1 |
| <a id="NFR-RE"></a>NFR-RE | §10.3, §8.7, §9 | SAD §12.1 |
| <a id="NFR-SE"></a>NFR-SE | [ADR-04](#ADR-04)..06, [ADR-13](#ADR-13), §6, §10.1 | SAD §12.1 |
| <a id="NFR-UX"></a>NFR-UX | [CT-01](#CT-01) | SAD §12.1 |

### 11.6 Độ phủ truy vết

| Nhóm | Tổng | Có quy tắc nghiệp vụ | Có thực thể | Có kiến trúc/ràng buộc | Có tiêu chí nghiệm thu | Có test case |
|---|---|---|---|---|---|---|
| Yêu cầu chức năng | 96 | 77 | 92 | 0 | 96 | 0 |
| Yêu cầu phi chức năng | 40 | 9 | 16 | 11 | 0 | 0 |

*Tính từ các quan hệ truy vết trong dữ liệu đã trích. 'Có test case' bằng 0 vì chưa có test case nào được trích (xem 11.2).*

<a id="ch-A"></a>
## A. Phụ lục

### A.1 Bảng tra mã

Các mã (FR-…, BRL-…, ENT-…, NFR-…, ADR-…, M-…, SVC-…) lấy nguyên từ tài liệu nguồn. Bảng này cho biết mã đó là gì và nằm ở phần nào của tài liệu này.

| Mã | Tên hoặc nội dung | Phần |
|---|---|---|
| [ADR-01](#ADR-01) | Golang cho toàn bộ backend, worker và CLI; monorepo, một toolchain. | 10 |
| [ADR-02](#ADR-02) | Modular monolith `core` gồm 7 module nghiệp vụ, chạy nhiều vai trò (api, worker) từ cùng một binary. Authenti… | 10 |
| [ADR-03](#ADR-03) | API Gateway là điểm vào duy nhất: TLS, định tuyến, rate limit, xác minh token, chặn thô theo scope; không chứ… | 10 |
| [ADR-04](#ADR-04) | Authentication Service là OIDC Relying Party với IdP của tổ chức, cấp JWT nội bộ ngắn hạn (ký bất đối xứng) c… | 10 |
| [ADR-05](#ADR-05) | Authorization Service là nguồn sự thật cho vai trò và thành viên dự án; policy viết thành mã (Rego) được nhún… | 10 |
| [ADR-06](#ADR-06) | Quy tắc tách nhiệm và nguyên tắc 4 mắt (BRL-33..36) đặt trong domain Governance, ở tầng service, không ở Gate… | 10 |
| [ADR-07](#ADR-07) | PostgreSQL là kho chính, một cluster, mỗi module một schema; tìm kiếm bằng full-text và trigram của PostgreSQ… | 10 |
| [ADR-08](#ADR-08) | Bất đồng bộ bằng transactional outbox và hàng đợi job dựa trên PostgreSQL; chưa thêm message broker. Broker (… | 10 |
| [ADR-09](#ADR-09) | Connector chạy ngoài tiến trình (container riêng) theo contract gRPC discover, fetch, map, health; có manifes… | 10 |
| [ADR-10](#ADR-10) | Pipeline dữ liệu ngoài qua bảng staging `raw_records` (ingest, validate, normalise, publish); tỷ lệ lỗi > 1%… | 10 |
| [ADR-11](#ADR-11) | REST + JSON, OpenAPI 3.1 contract-first, sinh stub server và client Go; lỗi theo RFC 9457; phân trang cursor;… | 10 |
| [ADR-12](#ADR-12) | Chưa triển khai BFF nhưng để sẵn chỗ cắm (tiền tố /bff/v1, audience JWT riêng, hợp đồng OpenAPI theo từng cli… | 10 |
| [ADR-13](#ADR-13) | Bí mật (token Figma, webhook, SMTP, khoá ký JWT) trong OpenBao hoặc Vault qua interface trừu tượng; không đặt… | 10 |
| [ADR-14](#ADR-14) | Audit log chỉ ghi thêm, chuỗi hash (`prev_hash`, `hash`), ghi qua outbox; người dùng ứng dụng chỉ có quyền IN… | 10 |
| [ADR-15](#ADR-15) | Quan sát chuẩn OpenTelemetry (trace, metric, log có `request_id`); không phụ thuộc nhà cung cấp backend. | 10 |
| [BRL-01](#BRL-01) | Component định danh bằng purl chuẩn hoá | 3 |
| [BRL-02](#BRL-02) | Snapshot xác định bởi (service, môi trường, hash SBOM); nộp trùng idempotent | 3 |
| [BRL-03](#BRL-03) | Snapshot mới nhất là nguồn sự thật cho Usage SBOM | 3 |
| [BRL-04](#BRL-04) | Usage khai báo tay xác nhận lại 30 ngày, quá hạn Stale | 3 |
| [BRL-05](#BRL-05) | Ngữ cảnh Service kế thừa cho Usage | 3 |
| [BRL-06](#BRL-06) | Phụ thuộc gián tiếp kế thừa môi trường và ngữ cảnh | 3 |
| [BRL-07](#BRL-07) | Vulnerability hợp nhất alias | 3 |
| [BRL-08](#BRL-08) | Finding khi phiên bản trong affected range; duy nhất | 3 |
| [BRL-09](#BRL-09) | Severity theo thứ tự CVSS v4.0, v3.1, nguồn | 3 |
| [BRL-10](#BRL-10) | Priority P1 đến P4 | 3 |
| [BRL-11](#BRL-11) | SLA theo severity; KEV luôn SLA Critical | 3 |
| [BRL-12](#BRL-12) | Not affected cần 1 trong 5 lý do VEX, mô tả ≥ 20 ký tự | 3 |
| [BRL-13](#BRL-13) | Not affected của Critical hoặc KEV cần SEC duyệt | 3 |
| [BRL-14](#BRL-14) | Risk accepted phải gắn Decision Approved còn hạn | 3 |
| [BRL-15](#BRL-15) | Tự Fixed khi snapshot mới hết; tự mở lại khi tái xuất hiện | 3 |
| [BRL-16](#BRL-16) | 75% SLA nhắc, 100% breached và chuyển cấp | 3 |
| [BRL-17](#BRL-17) | Thứ tự nguồn ngày EOL | 3 |
| [BRL-18](#BRL-18) | Cảnh báo EOL T-180, T-90, T-30, T-0, rồi hằng tuần | 3 |
| [BRL-19](#BRL-19) | EOL ở prod là vi phạm gate trừ khi có Decision | 3 |
| [BRL-20](#BRL-20) | Health dưới 40 hoặc ngừng bảo trì cảnh báo Medium | 3 |
| [BRL-21](#BRL-21) | Version lag theo component cảnh báo Medium | 3 |
| [BRL-22](#BRL-22) | License chuẩn hoá SPDX; OR chọn ít hạn chế, AND chọn hạn chế nhất | 3 |
| [BRL-23](#BRL-23) | Không xác định license: NOASSERTION, nhóm Unknown, block | 3 |
| [BRL-24](#BRL-24) | Action license tra theo ma trận | 3 |
| [BRL-25](#BRL-25) | Phiên bản đổi nhóm license là ranh giới; gate chặn | 3 |
| [BRL-26](#BRL-26) | License source-available lưu văn bản điều khoản | 3 |
| [BRL-27](#BRL-27) | Ring Adopt, Trial, Hold, Banned | 3 |
| [BRL-28](#BRL-28) | Đổi ring qua Change Request do ARB duyệt | 3 |
| [BRL-29](#BRL-29) | `next_review_at` ≤ 90 ngày, quá hạn Cần rà soát | 3 |
| [BRL-30](#BRL-30) | Công nghệ ngoài Catalog chỉ ở môi trường không phải prod | 3 |
| [BRL-31](#BRL-31) | Review Report hiệu lực 30 ngày | 3 |
| [BRL-32](#BRL-32) | Ứng viên vi phạm ràng buộc bị đánh dấu đỏ | 3 |
| [BRL-33](#BRL-33) | Decision bắt buộc trường, hạn ≤ 12 tháng | 3 |
| [BRL-34](#BRL-34) | Thẩm quyền duyệt theo loại Decision | 3 |
| [BRL-35](#BRL-35) | Người tạo không được duyệt | 3 |
| [BRL-36](#BRL-36) | Hết hạn hoặc thu hồi quay về vi phạm, nhắc trước 14 ngày | 3 |
| [BRL-37](#BRL-37) | Audit chỉ thêm, không sửa | 3 |
| [BRL-38](#BRL-38) | Routing thông báo, không gửi trùng | 3 |
| [BRL-39](#BRL-39) | Không tắt Critical, KEV, SLA breached; email luôn gửi | 3 |
| [BRL-40](#BRL-40) | Gate fail | 3 |
| [BRL-41](#BRL-41) | Gate warn | 3 |
| [BRL-42](#BRL-42) | Ghi đè gate chỉ qua Decision còn hạn | 3 |
| [BRL-43](#BRL-43) | Không phản hồi 30 giây: fail-open hoặc fail-closed | 3 |
| [BRL-44](#BRL-44) | Asset định danh theo SHA-256 | 3 |
| [BRL-45](#BRL-45) | Không rõ license và ngoài Asset Catalog: `unknown_source` | 3 |
| [BRL-46](#BRL-46) | Tài sản thương mại hợp lệ khi có Entitlement | 3 |
| [BRL-47](#BRL-47) | Entitlement bắt buộc licensee, điều khoản, chứng từ, hạn | 3 |
| [BRL-48](#BRL-48) | Cảnh báo Entitlement T-60, T-30, T-7, T-0 | 3 |
| [BRL-49](#BRL-49) | Nghĩa vụ ghi công đạt khi có trong Credits | 3 |
| [BRL-50](#BRL-50) | Bàn giao: Entitlement TECHWISE không chuyển nhượng xử lý trước | 3 |
| [BRL-51](#BRL-51) | Tài sản tự tạo khai người tạo; của khách cần văn bản xác nhận | 3 |
| [BRL-52](#BRL-52) | Dữ liệu dự án chỉ hiển thị cho thành viên và vai trò hệ thống | 3 |
| [BRL-53](#BRL-53) | API token gắn một dự án, scope tối thiểu, hạn ≤ 365 ngày | 3 |
| [CT-01](#CT-01) | Web SPA | 2 |
| [CT-02](#CT-02) | CLI `pbom` | 2 |
| [CT-03](#CT-03) | API Gateway | 2 |
| [ENT-01](#ENT-01) | User | 5 |
| [ENT-02](#ENT-02) | RoleAssignment | 5 |
| [ENT-03](#ENT-03) | Project | 5 |
| [ENT-04](#ENT-04) | Environment | 5 |
| [ENT-05](#ENT-05) | Service | 5 |
| [ENT-06](#ENT-06) | ApiToken | 5 |
| [ENT-07](#ENT-07) | Connector | 5 |
| [ENT-08](#ENT-08) | DataSource | 5 |
| [ENT-09](#ENT-09) | ImportJob | 5 |
| [ENT-10](#ENT-10) | RawRecord | 5 |
| [ENT-11](#ENT-11) | Component | 5 |
| [ENT-12](#ENT-12) | ComponentVersion | 5 |
| [ENT-13](#ENT-13) | ReleaseLine | 5 |
| [ENT-14](#ENT-14) | Vulnerability | 5 |
| [ENT-15](#ENT-15) | AffectedRange | 5 |
| [ENT-16](#ENT-16) | License | 5 |
| [ENT-17](#ENT-17) | SbomSnapshot | 5 |
| [ENT-18](#ENT-18) | Usage | 5 |
| [ENT-19](#ENT-19) | UsageContext | 5 |
| [ENT-20](#ENT-20) | LicenseObservation | 5 |
| [ENT-21](#ENT-21) | Finding | 5 |
| [ENT-22](#ENT-22) | VexStatement | 5 |
| [ENT-23](#ENT-23) | Decision | 5 |
| [ENT-24](#ENT-24) | Policy | 5 |
| [ENT-25](#ENT-25) | Evaluation | 5 |
| [ENT-26](#ENT-26) | Alert | 5 |
| [ENT-27](#ENT-27) | NotificationPreference | 5 |
| [ENT-28](#ENT-28) | CatalogEntry | 5 |
| [ENT-29](#ENT-29) | CatalogChangeRequest | 5 |
| [ENT-30](#ENT-30) | StackProposal | 5 |
| [ENT-31](#ENT-31) | Baseline | 5 |
| [ENT-32](#ENT-32) | Report | 5 |
| [ENT-33](#ENT-33) | Asset | 5 |
| [ENT-34](#ENT-34) | AssetLicenseTerm | 5 |
| [ENT-35](#ENT-35) | Entitlement | 5 |
| [ENT-36](#ENT-36) | AssetUsage | 5 |
| [ENT-37](#ENT-37) | AssetFinding | 5 |
| [ENT-38](#ENT-38) | AssetCatalogEntry | 5 |
| [ENT-39](#ENT-39) | AssetScanRun | 5 |
| [ENT-40](#ENT-40) | CreditsDocument | 5 |
| [ENT-41](#ENT-41) | HandoverChecklist | 5 |
| [ENT-42](#ENT-42) | FigmaConnection | 5 |
| [ENT-43](#ENT-43) | Evidence | 5 |
| [ENT-44](#ENT-44) | AuditLog | 5 |
| [ENT-45](#ENT-45) | WebhookSubscription | 5 |
| [ENT-46](#ENT-46) | EventDelivery | 5 |
| [ERR-conflict](#ERR-conflict) | conflict | 6 |
| [ERR-decision_not_approved](#ERR-decision_not_approved) | decision_not_approved | 6 |
| [ERR-duplicate](#ERR-duplicate) | duplicate | 6 |
| [ERR-expiry_too_long](#ERR-expiry_too_long) | expiry_too_long | 6 |
| [ERR-forbidden](#ERR-forbidden) | forbidden | 6 |
| [ERR-gone](#ERR-gone) | gone | 6 |
| [ERR-handover_incomplete](#ERR-handover_incomplete) | handover_incomplete | 6 |
| [ERR-idempotency_key_reuse](#ERR-idempotency_key_reuse) | idempotency_key_reuse | 6 |
| [ERR-insufficient_scope](#ERR-insufficient_scope) | insufficient_scope | 6 |
| [ERR-internal_error](#ERR-internal_error) | internal_error | 6 |
| [ERR-invalid_client](#ERR-invalid_client) | invalid_client | 6 |
| [ERR-invalid_purl](#ERR-invalid_purl) | invalid_purl | 6 |
| [ERR-invalid_request](#ERR-invalid_request) | invalid_request | 6 |
| [ERR-invalid_sbom](#ERR-invalid_sbom) | invalid_sbom | 6 |
| [ERR-invalid_scope](#ERR-invalid_scope) | invalid_scope | 6 |
| [ERR-invalid_state_transition](#ERR-invalid_state_transition) | invalid_state_transition | 6 |
| [ERR-not_found](#ERR-not_found) | not_found | 6 |
| [ERR-payload_too_large](#ERR-payload_too_large) | payload_too_large | 6 |
| [ERR-precondition_failed](#ERR-precondition_failed) | precondition_failed | 6 |
| [ERR-precondition_required](#ERR-precondition_required) | precondition_required | 6 |
| [ERR-project_archived](#ERR-project_archived) | project_archived | 6 |
| [ERR-rate_limited](#ERR-rate_limited) | rate_limited | 6 |
| [ERR-rules_schema_invalid](#ERR-rules_schema_invalid) | rules_schema_invalid | 6 |
| [ERR-sbom_too_large](#ERR-sbom_too_large) | sbom_too_large | 6 |
| [ERR-sbom_too_many_components](#ERR-sbom_too_many_components) | sbom_too_many_components | 6 |
| [ERR-separation_of_duties](#ERR-separation_of_duties) | separation_of_duties | 6 |
| [ERR-service_unavailable](#ERR-service_unavailable) | service_unavailable | 6 |
| [ERR-simulation_required](#ERR-simulation_required) | simulation_required | 6 |
| [ERR-sla_profile_looser](#ERR-sla_profile_looser) | sla_profile_looser | 6 |
| [ERR-timeout](#ERR-timeout) | timeout | 6 |
| [ERR-token_expired](#ERR-token_expired) | token_expired | 6 |
| [ERR-token_revoked](#ERR-token_revoked) | token_revoked | 6 |
| [ERR-unauthenticated](#ERR-unauthenticated) | unauthenticated | 6 |
| [ERR-unsupported_media_type](#ERR-unsupported_media_type) | unsupported_media_type | 6 |
| [ERR-unsupported_spec_version](#ERR-unsupported_spec_version) | unsupported_spec_version | 6 |
| [ERR-validation_failed](#ERR-validation_failed) | validation_failed | 6 |
| [FR-ADM](#FR-ADM) | SVC-01, SVC-02, M-04 (Policy), M-07 | 11 |
| [FR-ADM-01](#FR-ADM-01) | Hệ thống phải xác thực người dùng bằng SSO OIDC và ánh xạ nhóm IdP sang vai trò hệ thống theo `Policy(type=id… | 1 |
| [FR-ADM-02](#FR-ADM-02) | Hệ thống phải phân quyền theo vai trò hệ thống (SEC, ARB, LEG, PROC, ADM) và vai trò dự án (Viewer, Contribut… | 1 |
| [FR-ADM-03](#FR-ADM-03) | Hệ thống phải cấp, hiển thị một lần, hết hạn và thu hồi API token theo dự án với scope tối thiểu, chỉ lưu has… | 1 |
| [FR-ADM-04](#FR-ADM-04) | Hệ thống phải cho SEC cấu hình SLA, gate, EOL, limits và LEG cấu hình ma trận license, asset_rule, retention… | 1 |
| [FR-AST](#FR-AST) | M-06 | 11 |
| [FR-AST-01](#FR-AST-01) | Hệ thống phải quét font, ảnh, SVG, Lottie, audio, video trong repo và thư mục build; bỏ qua theo `.pbomignore… | 1 |
| [FR-AST-02](#FR-AST-02) | Hệ thống phải phát hiện khai báo font gián tiếp (CSS, link Google hay Adobe Fonts, package, pubspec, manifest… | 1 |
| [FR-AST-03](#FR-AST-03) | Hệ thống phải đọc metadata font (bảng `name`, `OS/2.achVendID`, `fsType`) | 1 |
| [FR-AST-04](#FR-AST-04) | Hệ thống phải nhận diện license font từ nameID 13 và 14; không khớp thì Commercial hoặc Unknown | 1 |
| [FR-AST-05](#FR-AST-05) | Hệ thống phải đọc metadata ảnh XMP, IPTC, EXIF và `<metadata>` của SVG | 1 |
| [FR-AST-06](#FR-AST-06) | Hệ thống nên nhận diện nguồn stock, marketplace theo tên file và metadata; chặn ảnh preview có watermark | 1 |
| [FR-AST-07](#FR-AST-07) | Hệ thống phải cho khai báo tay tài sản không quét được; nhắc xác nhận lại mỗi 90 ngày | 1 |
| [FR-AST-08](#FR-AST-08) | Hệ thống nên khử trùng lặp theo SHA-256, gộp biến thể font; pHash chỉ gợi ý | 1 |
| [FR-AST-09](#FR-AST-09) | Hệ thống có thể tích hợp Figma REST API để đọc `fontFamily` và thư viện ngoài | 1 |
| [FR-AST-10](#FR-AST-10) | Hệ thống phải có thư viện AssetLicenseTerm chuẩn hoá do LEG duyệt; cho thêm điều khoản riêng | 1 |
| [FR-AST-11](#FR-AST-11) | Hệ thống phải có rule engine sinh AssetFinding khi nộp scan, khi Entitlement đổi và theo job hằng ngày | 1 |
| [FR-AST-12](#FR-AST-12) | Hệ thống phải có kho Entitlement (CRUD, chứng từ, liên kết tài sản, phạm vi, dự án) | 1 |
| [FR-AST-13](#FR-AST-13) | Hệ thống phải cảnh báo Entitlement thuê bao tại T-60, T-30, T-7, T-0 | 1 |
| [FR-AST-14](#FR-AST-14) | Hệ thống nên đếm và so sánh giới hạn định lượng; vượt 80% cảnh báo, vượt 100% `limit_exceeded` | 1 |
| [FR-AST-15](#FR-AST-15) | Hệ thống nên sinh Credits, Attribution (MD, HTML, JSON) và kiểm tra bản phát hành chứa bản mới nhất | 1 |
| [FR-AST-16](#FR-AST-16) | Hệ thống nên có Asset Catalog với ring Approved, Conditional, Banned và `next_review_at` | 1 |
| [FR-AST-17](#FR-AST-17) | Hệ thống nên sinh handover checklist và xuất PDF, XLSX phụ lục biên bản | 1 |
| [FR-AST-18](#FR-AST-18) | Hệ thống phải đưa tài sản vào policy gate chung, tách nhóm "Components" và "Assets" | 1 |
| [FR-AST-19](#FR-AST-19) | Hệ thống phải triage AssetFinding: liên kết Entitlement, thay hoặc xoá, false positive, Risk accepted qua Dec… | 1 |
| [FR-CAT](#FR-CAT) | M-05 | 11 |
| [FR-CAT-01](#FR-CAT-01) | Hệ thống phải quản lý CatalogEntry theo layer (ngôn ngữ, framework, CSDL, cache, message bus, gateway, secret… | 1 |
| [FR-CAT-02](#FR-CAT-02) | Hệ thống phải có ring Adopt, Trial, Hold, Banned; xem dạng Radar và bảng | 1 |
| [FR-CAT-03](#FR-CAT-03) | Hệ thống phải tự hiển thị license, CVE mở theo line, EOL từng line, health score, số dự án dùng; trường biên… | 1 |
| [FR-CAT-04](#FR-CAT-04) | Hệ thống nên gợi ý cập nhật phiên bản tối thiểu khi có bản vá CVE từ High trở lên | 1 |
| [FR-CAT-05](#FR-CAT-05) | Hệ thống phải đổi ring và phiên bản tối thiểu qua Change Request do ARB duyệt và lưu audit | 1 |
| [FR-CAT-06](#FR-CAT-06) | Hệ thống nên có `next_review_at` (mặc định 90 ngày) và nhắc khi quá hạn | 1 |
| [FR-CAT-07](#FR-CAT-07) | Hệ thống nên xuất version-pin baseline (JSON, YAML) theo layer cho CI | 1 |
| [FR-CAT-08](#FR-CAT-08) | Hệ thống nên lưu Comparison sheet gắn layer để tái sử dụng | 1 |
| [FR-CI](#FR-CI) | M-04, CLI (CT-02) | 11 |
| [FR-CI-01](#FR-CI-01) | Hệ thống phải có `POST /api/v1/evaluate` (hoặc kèm submit) trả pass, warn hoặc fail và danh sách vi phạm có l… | 1 |
| [FR-CI-02](#FR-CI-02) | Hệ thống phải áp rule fail và warn mặc định theo BRL-40, BRL-41 | 1 |
| [FR-CI-03](#FR-CI-03) | Hệ thống nên cấu hình rule theo dự án; ghi đè tạm thời phải có Decision | 1 |
| [FR-CI-04](#FR-CI-04) | Hệ thống phải có template pipeline GitLab CI, GitHub Actions, Jenkins | 1 |
| [FR-CI-05](#FR-CI-05) | Hệ thống nên import kết quả Trivy, Grype, OSV-Scanner, npm audit như nguồn Finding bổ sung | 1 |
| [FR-GOV](#FR-GOV) | M-07, WRK-04, WRK-05 | 11 |
| [FR-GOV-01](#FR-GOV-01) | Hệ thống phải có Decision registry (ADR, Risk acceptance, License exception, Catalog change) với trường bắt b… | 1 |
| [FR-GOV-02](#FR-GOV-02) | Hệ thống phải trả đối tượng về trạng thái vi phạm khi Decision hết hạn hoặc thu hồi; nhắc trước 14 ngày | 1 |
| [FR-GOV-03](#FR-GOV-03) | Hệ thống phải có audit log chỉ ghi thêm cho mọi thay đổi trạng thái (ai, khi nào, trước, sau) | 1 |
| [FR-GOV-04](#FR-GOV-04) | Hệ thống phải thông báo qua email và webhook (Teams, Slack, Telegram); routing theo owner và severity; digest… | 1 |
| [FR-GOV-05](#FR-GOV-05) | Hệ thống phải nhắc tại 75% SLA và chuyển cấp xử lý tới PM và SEC khi vi phạm SLA | 1 |
| [FR-GOV-06](#FR-GOV-06) | Hệ thống phải tự tạo báo cáo hằng tháng (PDF, XLSX) theo dự án và toàn công ty, gửi email | 1 |
| [FR-GOV-07](#FR-GOV-07) | Hệ thống nên xuất gói tuân thủ khách hàng: SBOM, VEX, license, tóm tắt CVE và SLA | 1 |
| [FR-INV](#FR-INV) | M-01, WRK-01 | 11 |
| [FR-INV-01](#FR-INV-01) | Hệ thống phải tạo, sửa, lưu trữ Project, Environment, Service; gán owner (TL) và SLA profile cho dự án; không… | 1 |
| [FR-INV-02](#FR-INV-02) | Hệ thống phải có `POST /api/v1/sbom` nhận CycloneDX ≥ 1.5 hoặc SPDX ≥ 2.3 (JSON) kèm project, service, enviro… | 1 |
| [FR-INV-03](#FR-INV-03) | Hệ thống phải có CLI `pbom` (`scan`, `submit`, `gate` của URD; thêm `credits`, `baseline`, `login`, `whoami`… | 1 |
| [FR-INV-04](#FR-INV-04) | Hệ thống phải cho khai báo tay Usage hạ tầng, nhắc xác nhận lại mỗi 30 ngày, quá hạn chuyển Stale; Usage Stal… | 1 |
| [FR-INV-05](#FR-INV-05) | Hệ thống phải lưu cây phụ thuộc của snapshot và hiển thị đường phụ thuộc từ trực tiếp tới gián tiếp | 1 |
| [FR-INV-06](#FR-INV-06) | Hệ thống phải cho khai báo ngữ cảnh sử dụng (`modified_source`, `distributed_to_third_party`, `network_expose… | 1 |
| [FR-INV-07](#FR-INV-07) | Hệ thống nên so sánh hai snapshot: thêm, bớt, đổi phiên bản, đổi license | 1 |
| [FR-INV-08](#FR-INV-08) | Hệ thống nên nhập hàng loạt Usage từ Excel hoặc CSV và báo lỗi theo dòng | 1 |
| [FR-KB](#FR-KB) | M-02, WRK-03, CN-nn | 11 |
| [FR-KB-01](#FR-KB-01) | Hệ thống phải đồng bộ tăng dần OSV, GitHub Advisory, Go vulndb mỗi 1 giờ qua Connector | 1 |
| [FR-KB-02](#FR-KB-02) | Hệ thống phải đồng bộ NVD (CVE và CPE) mỗi 2 giờ, CISA KEV mỗi 6 giờ, EPSS hằng ngày | 1 |
| [FR-KB-03](#FR-KB-03) | Hệ thống phải đồng bộ endoflife.date hằng ngày cho slug đã ánh xạ (`eol_slug`) | 1 |
| [FR-KB-04](#FR-KB-04) | Hệ thống phải thu thập metadata repo (release gần nhất, archived, license, số release 12 tháng) hằng tuần | 1 |
| [FR-KB-05](#FR-KB-05) | Hệ thống phải đăng ký nguồn advisory vendor (RSS, Atom, URL) theo Component; mục mới tạo advisory chưa phân l… | 1 |
| [FR-KB-06](#FR-KB-06) | Hệ thống phải hợp nhất alias CVE, GHSA, GO-ID, OSV thành một Vulnerability | 1 |
| [FR-KB-07](#FR-KB-07) | Hệ thống phải lưu lịch sử đồng bộ và cảnh báo SEC khi một nguồn lỗi quá 24 giờ | 1 |
| [FR-KB-08](#FR-KB-08) | Hệ thống phải cho ADM đăng ký, bật/tắt, gỡ phiên bản Connector; chỉ bật sau khi qua kiểm thử conformance (NFR… | 1 |
| [FR-KB-09](#FR-KB-09) | Hệ thống phải cho ADM/SEC tạo DataSource từ Connector: endpoint, tham chiếu bí mật, lịch, priority; thử kết n… | 1 |
| [FR-KB-10](#FR-KB-10) | Hệ thống phải cho SEC chạy đồng bộ thủ công, nhập file thủ công (CSV/JSON/XLSX) cho một DataSource, xem rejec… | 1 |
| [FR-LCM-01](#FR-LCM-01) | Hệ thống phải hiển thị release line, ngày EOL, EOS và số ngày còn lại cho mỗi Usage | 1 |
| [FR-LCM-02](#FR-LCM-02) | Hệ thống phải cảnh báo EOL tại T-180, T-90, T-30, T-0 và hằng tuần sau EOL (cấu hình được) | 1 |
| [FR-LCM-03](#FR-LCM-03) | Hệ thống phải cho khai báo EOL tay trong Catalog khi endoflife.date không có | 1 |
| [FR-LCM-04](#FR-LCM-04) | Hệ thống nên tính health score (0 đến 100) và cảnh báo khi không có release quá 12 tháng hoặc repo archived | 1 |
| [FR-LCM-05](#FR-LCM-05) | Hệ thống nên có version lag rule cấu hình theo component; vi phạm cảnh báo Medium | 1 |
| [FR-LCM-06](#FR-LCM-06) | Hệ thống nên đánh dấu thành phần hoặc phiên bản deprecated kèm thay thế đề xuất | 1 |
| [FR-LIC](#FR-LIC) | M-03, M-04 | 11 |
| [FR-LIC-01](#FR-LIC-01) | Hệ thống phải chuẩn hoá license về SPDX ID; biểu thức OR chọn phương án ít hạn chế nhất; không xác định thì N… | 1 |
| [FR-LIC-02](#FR-LIC-02) | Hệ thống phải đánh giá theo ma trận (nhóm license, mô hình phân phối, có sửa mã), cấu hình được | 1 |
| [FR-LIC-03](#FR-LIC-03) | Hệ thống phải phát hiện license drift giữa các phiên bản, cảnh báo SEC, ARB và mọi dự án đang dùng, đánh dấu… | 1 |
| [FR-LIC-04](#FR-LIC-04) | Hệ thống nên đính kèm văn bản điều khoản theo phiên bản (BSL Additional Use Grant, Change Date) và nhắc rà so… | 1 |
| [FR-LIC-05](#FR-LIC-05) | Hệ thống nên xuất báo cáo license và NOTICE cho bản phân phối | 1 |
| [FR-SEL-01](#FR-SEL-01) | Hệ thống phải có wizard tạo đề xuất stack cho dự án mới theo mô hình phân phối và ràng buộc khách hàng | 1 |
| [FR-SEL-02](#FR-SEL-02) | Hệ thống nên so sánh 2 đến 4 ứng viên theo tiêu chí chuẩn | 1 |
| [FR-SEL-03](#FR-SEL-03) | Hệ thống nên tự đánh dấu ứng viên vi phạm ràng buộc dự án | 1 |
| [FR-SEL-04](#FR-SEL-04) | Hệ thống nên xuất Tech Stack Review Report (DOCX, PDF, Markdown) có dấu "valid until" | 1 |
| [FR-SEL-05](#FR-SEL-05) | Hệ thống phải chuyển công nghệ ngoài Catalog thành Change Request; production chỉ dùng sau khi ARB duyệt hoặc… | 1 |
| [FR-SEL-06](#FR-SEL-06) | Hệ thống nên chốt đề xuất thành Usage "planned" để giám sát từ trước khi có code | 1 |
| [FR-UI](#FR-UI) | CT-01 | 11 |
| [FR-UI-01](#FR-UI-01) | Hệ thống phải có màn hình Company overview cho SEC, ARB: Finding mở, % SLA, top 10 rủi ro, EOL 90 ngày, licen… | 1 |
| [FR-UI-02](#FR-UI-02) | Hệ thống phải có Project detail với tab Stack, Findings, Lifecycle, License, Decisions, Snapshots | 1 |
| [FR-UI-03](#FR-UI-03) | Hệ thống phải có Component detail: line, CVE theo phiên bản, EOL, license history, dự án đang dùng, Catalog e… | 1 |
| [FR-UI-04](#FR-UI-04) | Hệ thống phải có My work: Finding được giao, hạn SLA, hành động đề xuất | 1 |
| [FR-UI-05](#FR-UI-05) | Hệ thống nên có Tech Radar: radar và bảng Catalog | 1 |
| [FR-UI-06](#FR-UI-06) | Hệ thống phải có blast radius search "ai đang dùng X phiên bản Y" trả ≤ 3 giây | 1 |
| [FR-UI-07](#FR-UI-07) | Hệ thống phải có tab Assets của dự án: danh sách theo loại, nguồn, license, entitlement, finding; lọc "chưa c… | 1 |
| [FR-UI-08](#FR-UI-08) | Hệ thống phải có màn hình Entitlements cho PROC, SEC: kho license, hạn dùng, dự án dùng, chứng từ | 1 |
| [FR-UI-09](#FR-UI-09) | Hệ thống phải có Asset detail: metadata đã đọc, đường dẫn file, dự án dùng, lịch sử khai báo, ảnh preview | 1 |
| [FR-UI-10](#FR-UI-10) | Hệ thống nên có Asset Catalog cho DES: tìm và lọc "hỗ trợ tiếng Việt", "dùng được cho app thu phí" | 1 |
| [FR-VUL](#FR-VUL) | M-03, WRK-02 | 11 |
| [FR-VUL-01](#FR-VUL-01) | Hệ thống phải tự đối chiếu (matching) để tạo hoặc cập nhật Finding khi có Vulnerability mới hoặc Usage mới | 1 |
| [FR-VUL-02](#FR-VUL-02) | Hệ thống phải tính Priority từ CVSS, EPSS, KEV, độ quan trọng môi trường, network_exposed, trực tiếp hoặc giá… | 1 |
| [FR-VUL-03](#FR-VUL-03) | Hệ thống phải tính `sla_due_at` theo SLA profile (Critical 48 giờ, High 7 ngày, Medium 30 ngày); bộ đếm chạy… | 1 |
| [FR-VUL-04](#FR-VUL-04) | Hệ thống phải có workflow triage Open, Triaging, Affected hoặc Fixed hoặc Not affected kèm justification VEX… | 1 |
| [FR-VUL-05](#FR-VUL-05) | Hệ thống phải ghi nhớ quyết định Not affected theo (vulnerability, component, ngữ cảnh) và tự mở lại khi ngữ… | 1 |
| [FR-VUL-06](#FR-VUL-06) | Hệ thống nên hiển thị fix recommendation: phiên bản vá gần nhất, loại nâng cấp (patch, minor, major), cảnh bá… | 1 |
| [FR-VUL-07](#FR-VUL-07) | Hệ thống phải tự đóng Finding (Fixed) khi snapshot mới không còn phiên bản bị ảnh hưởng; lưu `fixed_in_versio… | 1 |
| [FR-VUL-08](#FR-VUL-08) | Hệ thống nên xuất VEX (CycloneDX VEX, OpenVEX) theo dự án | 1 |
| [INT-01](#INT-01) | Component duy nhất theo purl chuẩn hoá (bỏ version và qualifier); thành phần không có purl dùng `pkg:generic`… | 5 |
| [INT-02](#INT-02) | SbomSnapshot duy nhất theo (service, môi trường, hash nội dung); nộp trùng hash là idempotent | 5 |
| [INT-03](#INT-03) | Snapshot mới nhất của (service, môi trường) là nguồn sự thật cho Usage nguồn SBOM; Usage không còn thì chuyển… | 5 |
| [INT-04](#INT-04) | Finding duy nhất theo (Vulnerability, Usage); đổi trạng thái không tạo Finding mới | 5 |
| [INT-05](#INT-05) | Mọi Decision có `expires_at` không quá 12 tháng kể từ ngày duyệt; gia hạn tạo Decision mới | 5 |
| [INT-06](#INT-06) | Resource sinh từ dữ liệu ngoài (Vulnerability, ComponentVersion, ReleaseLine, License) có lineage về RawRecor… | 5 |
| [INT-07](#INT-07) | Entitlement bắt buộc licensee, license_term_code, proof (Evidence) và valid_until hoặc vĩnh viễn | 5 |
| [INT-08](#INT-08) | AuditLog chỉ thêm, không sửa, không xoá; lưu ≥ 3 năm | 5 |
| [INT-09](#INT-09) | Không xoá vật lý thực thể nghiệp vụ; đóng bằng trạng thái (Closed, Retired, Revoked, Archived) để giữ lịch sử | 5 |
| [M-01](#M-01) | Inventory (`inventory`) | 2 |
| [M-02](#M-02) | Knowledge Base (`kb`) | 2 |
| [M-03](#M-03) | Risk (`risk`) | 2 |
| [M-04](#M-04) | Gate và Policy (`gate`) | 2 |
| [M-05](#M-05) | Catalog và Selection (`catalog`) | 2 |
| [M-06](#M-06) | Asset Compliance (`asset`) | 2 |
| [M-07](#M-07) | Governance (`gov`) | 2 |
| [NFR-AU](#NFR-AU) | ADR-14, §10.5 | 11 |
| [NFR-AU-01](#NFR-AU-01) | Audit log | 8 |
| [NFR-CO](#NFR-CO) | ADR-09, ADR-11, §8.2 | 11 |
| [NFR-CO-01](#NFR-CO-01) | Định dạng SBOM và định danh | 8 |
| [NFR-CO-02](#NFR-CO-02) | CLI và pipeline | 8 |
| [NFR-CO-03](#NFR-CO-03) | Trao đổi dữ liệu và tích hợp | 8 |
| [NFR-CO-04](#NFR-CO-04) | Nguồn dữ liệu ngoài | 8 |
| [NFR-CO-05](#NFR-CO-05) | Xuất baseline | 8 |
| [NFR-CO-06](#NFR-CO-06) | Import và đồng bộ dữ liệu ngoài qua connector, pipeline 4 bước: thu nhận, kiểm tra, chuẩn hoá, công bố | 8 |
| [NFR-CO-07](#NFR-CO-07) | Hệ thống mở, sẵn sàng tích hợp: REST versioned có OpenAPI, webhook ký số có thử lại và dead-letter, inbound A… | 8 |
| [NFR-IC](#NFR-IC) | ADR-07, ADR-10, §7 | 11 |
| [NFR-IC-01](#NFR-IC-01) | Định danh người dùng | 5 |
| [NFR-IC-02](#NFR-IC-02) | Duy nhất và idempotent | 5 |
| [NFR-IC-03](#NFR-IC-03) | Truy vết nguồn dữ liệu | 5 |
| [NFR-IC-04](#NFR-IC-04) | Toàn vẹn tham chiếu | 5 |
| [NFR-IC-05](#NFR-IC-05) | Chất lượng dữ liệu khai báo tay | 5 |
| [NFR-MA](#NFR-MA) | ADR-02, ADR-09, AD-08 | 11 |
| [NFR-MA-01](#NFR-MA-01) | Mở rộng hệ sinh thái | 8 |
| [NFR-MA-02](#NFR-MA-02) | Kiểm thử và tự tuân thủ | 8 |
| [NFR-MA-03](#NFR-MA-03) | Cấu hình không cần deploy | 8 |
| [NFR-MA-04](#NFR-MA-04) | Giải thích được | 8 |
| [NFR-MA-05](#NFR-MA-05) | Kiến trúc plugin, connector (discover, fetch, map, health) | 8 |
| [NFR-MA-06](#NFR-MA-06) | Mô hình dữ liệu trung lập, mở rộng được | 8 |
| [NFR-PE](#NFR-PE) | QA-01, QA-02, ADR-07, ADR-08, §8.1..8.3, §10.4 | 11 |
| [NFR-PE-01](#NFR-PE-01) | Nộp SBOM và đánh giá gate | 8 |
| [NFR-PE-02](#NFR-PE-02) | Matching và tính lại theo lô | 8 |
| [NFR-PE-03](#NFR-PE-03) | Thao tác tương tác | 8 |
| [NFR-PE-04](#NFR-PE-04) | Truy vấn nặng và xuất báo cáo | 8 |
| [NFR-PE-05](#NFR-PE-05) | Độ tươi dữ liệu và thời gian cảnh báo | 8 |
| [NFR-PE-06](#NFR-PE-06) | Job định kỳ | 8 |
| [NFR-PE-07](#NFR-PE-07) | Quét tài sản | 8 |
| [NFR-PE-08](#NFR-PE-08) | Đánh giá lại rule tài sản | 8 |
| [NFR-RE](#NFR-RE) | §10.3, §8.7, §9 | 11 |
| [NFR-RE-01](#NFR-RE-01) | Chịu lỗi khi gọi dịch vụ | 8 |
| [NFR-RE-02](#NFR-RE-02) | Job an toàn | 8 |
| [NFR-RE-03](#NFR-RE-03) | Kênh và hệ thống ngoài lỗi | 8 |
| [NFR-RE-04](#NFR-RE-04) | Khả dụng và suy giảm có kiểm soát | 8 |
| [NFR-SE](#NFR-SE) | ADR-04..06, ADR-13, §6, §10.1 | 11 |
| [NFR-SE-01](#NFR-SE-01) | Xác thực | 9 |
| [NFR-SE-02](#NFR-SE-02) | Phân quyền | 9 |
| [NFR-SE-03](#NFR-SE-03) | Tách biệt nhiệm vụ | 9 |
| [NFR-SE-04](#NFR-SE-04) | API token | 9 |
| [NFR-SE-05](#NFR-SE-05) | Bảo vệ dữ liệu | 9 |
| [NFR-SE-06](#NFR-SE-06) | Bí mật tích hợp | 9 |
| [NFR-UX](#NFR-UX) | CT-01 | 11 |
| [NFR-UX-01](#NFR-UX-01) | Giao diện dễ dùng | 8 |
| [NFR-UX-02](#NFR-UX-02) | Biểu mẫu và báo cáo | 8 |
| [NFR-UX-03](#NFR-UX-03) | Thông báo và Credits | 8 |
| [SVC-01](#SVC-01) | Authentication Service | 2 |
| [SVC-02](#SVC-02) | Authorization Service | 2 |
| [SVC-03](#SVC-03) | Core API | 2 |
| [WRK-01](#WRK-01) | Ingest worker | 2 |
| [WRK-02](#WRK-02) | Matcher | 2 |
| [WRK-03](#WRK-03) | Scheduler | 2 |
| [WRK-04](#WRK-04) | Notifier | 2 |
| [WRK-05](#WRK-05) | Report generator | 2 |

### A.2 Danh sách đầy đủ thông tin còn thiếu

| Mã được nhắc | Loại | Nhắc ở | Vấn đề |
|---|---|---|---|
| AC-G1 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AC' has no configured class |
| AC-G2 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AC' has no configured class |
| AC-G3 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AC' has no configured class |
| AC-G4 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AC' has no configured class |
| AC-G5 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AC' has no configured class |
| AC-G6 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AC' has no configured class |
| AC-G7 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AC' has no configured class |
| AC-G8 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AC' has no configured class |
| AD-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AD' has no configured class |
| AD-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AD' has no configured class |
| AD-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AD' has no configured class |
| AD-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AD' has no configured class |
| AD-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AD' has no configured class |
| AD-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AD' has no configured class |
| AD-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AD' has no configured class |
| AD-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AD' has no configured class |
| AD-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AD' has no configured class |
| AD-10 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AD' has no configured class |
| ALG-01 | Mã được nhắc tới nhưng không có mô tả | WRK-02 | ALG-01 is referenced by WRK-02 but none of the supplied documents defines it |
| ALG-02 | Mã được nhắc tới nhưng không có mô tả | WRK-02 | ALG-02 is referenced by WRK-02 but none of the supplied documents defines it |
| ALG-03 | Mã được nhắc tới nhưng không có mô tả | WRK-02 | ALG-03 is referenced by WRK-02 but none of the supplied documents defines it |
| ALG-04 | Mã được nhắc tới nhưng không có mô tả | FR-VUL-01 | ALG-04 is referenced by FR-VUL-01 but none of the supplied documents defines it |
| ALG-05 | Mã được nhắc tới nhưng không có mô tả | FR-ADM-04 | ALG-05 is referenced by FR-ADM-04 but none of the supplied documents defines it |
| ALG-06 | Mã được nhắc tới nhưng không có mô tả | FR-INV-07 | ALG-06 is referenced by FR-INV-07 but none of the supplied documents defines it |
| ALG-07 | Mã được nhắc tới nhưng không có mô tả | FR-AST-01 | ALG-07 is referenced by FR-AST-01 but none of the supplied documents defines it |
| ALG-08 | Mã được nhắc tới nhưng không có mô tả | FR-ADM-04 | ALG-08 is referenced by FR-ADM-04 but none of the supplied documents defines it |
| ALG-09 | Mã được nhắc tới nhưng không có mô tả | FR-KB-03 | ALG-09 is referenced by FR-KB-03 but none of the supplied documents defines it |
| ALG-10 | Mã được nhắc tới nhưng không có mô tả | FR-KB-04 | ALG-10 is referenced by FR-KB-04 but none of the supplied documents defines it |
| ALG-11 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-01 | ALG-11 is referenced by FR-GOV-01 but none of the supplied documents defines it |
| ALG-12 | Mã được nhắc tới nhưng không có mô tả | FR-KB-01 | ALG-12 is referenced by FR-KB-01 but none of the supplied documents defines it |
| ALG-13 | Mã được nhắc tới nhưng không có mô tả | FR-KB-01 | ALG-13 is referenced by FR-KB-01 but none of the supplied documents defines it |
| API-C-01 | Mã được nhắc tới nhưng không có mô tả | FR-INV-03 | API-C-01 is referenced by FR-INV-03 but none of the supplied documents defines it |
| API-C-02 | Mã được nhắc tới nhưng không có mô tả | FR-INV-03 | API-C-02 is referenced by FR-INV-03 but none of the supplied documents defines it |
| API-C-03 | Mã được nhắc tới nhưng không có mô tả | FR-CI-03 | API-C-03 is referenced by FR-CI-03 but none of the supplied documents defines it |
| API-C-04 | Mã được nhắc tới nhưng không có mô tả | FR-CI-03 | API-C-04 is referenced by FR-CI-03 but none of the supplied documents defines it |
| API-C-05 | Mã được nhắc tới nhưng không có mô tả | FR-CI-02 | API-C-05 is referenced by FR-CI-02 but none of the supplied documents defines it |
| API-C-06 | Mã được nhắc tới nhưng không có mô tả | FR-CI-01 | API-C-06 is referenced by FR-CI-01 but none of the supplied documents defines it |
| API-C-07 | Mã được nhắc tới nhưng không có mô tả | FR-CI-01 | API-C-07 is referenced by FR-CI-01 but none of the supplied documents defines it |
| API-C-08 | Mã được nhắc tới nhưng không có mô tả | FR-CI-04 | API-C-08 is referenced by FR-CI-04 but none of the supplied documents defines it |
| API-C-09 | Mã được nhắc tới nhưng không có mô tả | FR-CI-04 | API-C-09 is referenced by FR-CI-04 but none of the supplied documents defines it |
| API-C | Mã được nhắc tới nhưng không có mô tả | M-04 | API-C is referenced by M-04 but none of the supplied documents defines it |
| API-G-01 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-01 | API-G-01 is referenced by FR-GOV-01 but none of the supplied documents defines it |
| API-G-02 | Mã được nhắc tới nhưng không có mô tả | FR-UI-02 | API-G-02 is referenced by FR-UI-02 but none of the supplied documents defines it |
| API-G-03 | Mã được nhắc tới nhưng không có mô tả | ENT-23 | API-G-03 is referenced by ENT-23 but none of the supplied documents defines it |
| API-G-04 | Mã được nhắc tới nhưng không có mô tả | ENT-23 | API-G-04 is referenced by ENT-23 but none of the supplied documents defines it |
| API-G-05 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-01 | API-G-05 is referenced by FR-GOV-01 but none of the supplied documents defines it |
| API-G-06 | Mã được nhắc tới nhưng không có mô tả | FR-CI-03 | API-G-06 is referenced by FR-CI-03 but none of the supplied documents defines it |
| API-G-08 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-02 | API-G-08 is referenced by FR-GOV-02 but none of the supplied documents defines it |
| API-G-09 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-02 | API-G-09 is referenced by FR-GOV-02 but none of the supplied documents defines it |
| API-G-10 | Mã được nhắc tới nhưng không có mô tả | ENT-23 | API-G-10 is referenced by ENT-23 but none of the supplied documents defines it |
| API-G-11 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-02 | API-G-11 is referenced by FR-GOV-02 but none of the supplied documents defines it |
| API-G-12 | Mã được nhắc tới nhưng không có mô tả | ENT-43 | API-G-12 is referenced by ENT-43 but none of the supplied documents defines it |
| API-G-13 | Mã được nhắc tới nhưng không có mô tả | ENT-43 | API-G-13 is referenced by ENT-43 but none of the supplied documents defines it |
| API-G-14 | Mã được nhắc tới nhưng không có mô tả | ENT-43 | API-G-14 is referenced by ENT-43 but none of the supplied documents defines it |
| API-G-15 | Mã được nhắc tới nhưng không có mô tả | FR-KB-07 | API-G-15 is referenced by FR-KB-07 but none of the supplied documents defines it |
| API-G-16 | Mã được nhắc tới nhưng không có mô tả | ENT-26 | API-G-16 is referenced by ENT-26 but none of the supplied documents defines it |
| API-G-17 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-05 | API-G-17 is referenced by FR-GOV-05 but none of the supplied documents defines it |
| API-G-20 | Mã được nhắc tới nhưng không có mô tả | ENT-27 | API-G-20 is referenced by ENT-27 but none of the supplied documents defines it |
| API-G-21 | Mã được nhắc tới nhưng không có mô tả | ENT-27 | API-G-21 is referenced by ENT-27 but none of the supplied documents defines it |
| API-G-22 | Mã được nhắc tới nhưng không có mô tả | ENT-26 | API-G-22 is referenced by ENT-26 but none of the supplied documents defines it |
| API-G-23 | Mã được nhắc tới nhưng không có mô tả | ENT-26 | API-G-23 is referenced by ENT-26 but none of the supplied documents defines it |
| API-G-24 | Mã được nhắc tới nhưng không có mô tả | FR-UI-01 | API-G-24 is referenced by FR-UI-01 but none of the supplied documents defines it |
| API-G-25 | Mã được nhắc tới nhưng không có mô tả | FR-UI-04 | API-G-25 is referenced by FR-UI-04 but none of the supplied documents defines it |
| API-G-26 | Mã được nhắc tới nhưng không có mô tả | FR-UI-02 | API-G-26 is referenced by FR-UI-02 but none of the supplied documents defines it |
| API-G-28 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-04 | API-G-28 is referenced by FR-GOV-04 but none of the supplied documents defines it |
| API-G-29 | Mã được nhắc tới nhưng không có mô tả | ENT-45 | API-G-29 is referenced by ENT-45 but none of the supplied documents defines it |
| API-G-30 | Mã được nhắc tới nhưng không có mô tả | ENT-45 | API-G-30 is referenced by ENT-45 but none of the supplied documents defines it |
| API-G-31 | Mã được nhắc tới nhưng không có mô tả | ENT-45 | API-G-31 is referenced by ENT-45 but none of the supplied documents defines it |
| API-G-35 | Mã được nhắc tới nhưng không có mô tả | ENT-45 | API-G-35 is referenced by ENT-45 but none of the supplied documents defines it |
| API-G-36 | Mã được nhắc tới nhưng không có mô tả | ENT-45 | API-G-36 is referenced by ENT-45 but none of the supplied documents defines it |
| API-G-37 | Mã được nhắc tới nhưng không có mô tả | ENT-46 | API-G-37 is referenced by ENT-46 but none of the supplied documents defines it |
| API-G-38 | Mã được nhắc tới nhưng không có mô tả | ENT-46 | API-G-38 is referenced by ENT-46 but none of the supplied documents defines it |
| API-G-39 | Mã được nhắc tới nhưng không có mô tả | ENT-46 | API-G-39 is referenced by ENT-46 but none of the supplied documents defines it |
| API-G-40 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-03 | API-G-40 is referenced by FR-GOV-03 but none of the supplied documents defines it |
| API-G-41 | Mã được nhắc tới nhưng không có mô tả | ENT-44 | API-G-41 is referenced by ENT-44 but none of the supplied documents defines it |
| API-G-42 | Mã được nhắc tới nhưng không có mô tả | ENT-32 | API-G-42 is referenced by ENT-32 but none of the supplied documents defines it |
| API-G-43 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-03 | API-G-43 is referenced by FR-GOV-03 but none of the supplied documents defines it |
| API-G-44 | Mã được nhắc tới nhưng không có mô tả | ENT-44 | API-G-44 is referenced by ENT-44 but none of the supplied documents defines it |
| API-G | Mã được nhắc tới nhưng không có mô tả | M-07 | API-G is referenced by M-07 but none of the supplied documents defines it |
| API-I-01 | Mã được nhắc tới nhưng không có mô tả | FR-INV-02 | API-I-01 is referenced by FR-INV-02 but none of the supplied documents defines it |
| API-I-02 | Mã được nhắc tới nhưng không có mô tả | ENT-17 | API-I-02 is referenced by ENT-17 but none of the supplied documents defines it |
| API-I-03 | Mã được nhắc tới nhưng không có mô tả | FR-INV-07 | API-I-03 is referenced by FR-INV-07 but none of the supplied documents defines it |
| API-I-04 | Mã được nhắc tới nhưng không có mô tả | FR-INV-05 | API-I-04 is referenced by FR-INV-05 but none of the supplied documents defines it |
| API-I-05 | Mã được nhắc tới nhưng không có mô tả | FR-INV-07 | API-I-05 is referenced by FR-INV-07 but none of the supplied documents defines it |
| API-I-06 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-07 | API-I-06 is referenced by FR-GOV-07 but none of the supplied documents defines it |
| API-I-07 | Mã được nhắc tới nhưng không có mô tả | FR-INV-04 | API-I-07 is referenced by FR-INV-04 but none of the supplied documents defines it |
| API-I-08 | Mã được nhắc tới nhưng không có mô tả | FR-UI-02 | API-I-08 is referenced by FR-UI-02 but none of the supplied documents defines it |
| API-I-09 | Mã được nhắc tới nhưng không có mô tả | FR-INV-05 | API-I-09 is referenced by FR-INV-05 but none of the supplied documents defines it |
| API-I-10 | Mã được nhắc tới nhưng không có mô tả | ENT-18 | API-I-10 is referenced by ENT-18 but none of the supplied documents defines it |
| API-I-11 | Mã được nhắc tới nhưng không có mô tả | BRL-04 | API-I-11 is referenced by BRL-04 but none of the supplied documents defines it |
| API-I-13 | Mã được nhắc tới nhưng không có mô tả | FR-INV-06 | API-I-13 is referenced by FR-INV-06 but none of the supplied documents defines it |
| API-I-14 | Mã được nhắc tới nhưng không có mô tả | FR-VUL-05 | API-I-14 is referenced by FR-VUL-05 but none of the supplied documents defines it |
| API-I-15 | Mã được nhắc tới nhưng không có mô tả | ENT-19 | API-I-15 is referenced by ENT-19 but none of the supplied documents defines it |
| API-I-16 | Mã được nhắc tới nhưng không có mô tả | FR-INV-08 | API-I-16 is referenced by FR-INV-08 but none of the supplied documents defines it |
| API-I-17 | Mã được nhắc tới nhưng không có mô tả | FR-CI-05 | API-I-17 is referenced by FR-CI-05 but none of the supplied documents defines it |
| API-I-18 | Mã được nhắc tới nhưng không có mô tả | FR-INV-05 | API-I-18 is referenced by FR-INV-05 but none of the supplied documents defines it |
| API-I | Mã được nhắc tới nhưng không có mô tả | M-01 | API-I is referenced by M-01 but none of the supplied documents defines it |
| API-K-01 | Mã được nhắc tới nhưng không có mô tả | ENT-07 | API-K-01 is referenced by ENT-07 but none of the supplied documents defines it |
| API-K-02 | Mã được nhắc tới nhưng không có mô tả | ENT-07 | API-K-02 is referenced by ENT-07 but none of the supplied documents defines it |
| API-K-03 | Mã được nhắc tới nhưng không có mô tả | FR-KB-01 | API-K-03 is referenced by FR-KB-01 but none of the supplied documents defines it |
| API-K-04 | Mã được nhắc tới nhưng không có mô tả | FR-KB-05 | API-K-04 is referenced by FR-KB-05 but none of the supplied documents defines it |
| API-K-05 | Mã được nhắc tới nhưng không có mô tả | FR-KB-02 | API-K-05 is referenced by FR-KB-02 but none of the supplied documents defines it |
| API-K-06 | Mã được nhắc tới nhưng không có mô tả | ENT-08 | API-K-06 is referenced by ENT-08 but none of the supplied documents defines it |
| API-K-07 | Mã được nhắc tới nhưng không có mô tả | ENT-07 | API-K-07 is referenced by ENT-07 but none of the supplied documents defines it |
| API-K-08 | Mã được nhắc tới nhưng không có mô tả | ENT-08 | API-K-08 is referenced by ENT-08 but none of the supplied documents defines it |
| API-K-09 | Mã được nhắc tới nhưng không có mô tả | FR-KB-01 | API-K-09 is referenced by FR-KB-01 but none of the supplied documents defines it |
| API-K-11 | Mã được nhắc tới nhưng không có mô tả | FR-KB-02 | API-K-11 is referenced by FR-KB-02 but none of the supplied documents defines it |
| API-K-12 | Mã được nhắc tới nhưng không có mô tả | ENT-09 | API-K-12 is referenced by ENT-09 but none of the supplied documents defines it |
| API-K-13 | Mã được nhắc tới nhưng không có mô tả | FR-KB-01 | API-K-13 is referenced by FR-KB-01 but none of the supplied documents defines it |
| API-K-14 | Mã được nhắc tới nhưng không có mô tả | FR-KB-02 | API-K-14 is referenced by FR-KB-02 but none of the supplied documents defines it |
| API-K-16 | Mã được nhắc tới nhưng không có mô tả | ENT-10 | API-K-16 is referenced by ENT-10 but none of the supplied documents defines it |
| API-K-17 | Mã được nhắc tới nhưng không có mô tả | ENT-09 | API-K-17 is referenced by ENT-09 but none of the supplied documents defines it |
| API-K-19 | Mã được nhắc tới nhưng không có mô tả | ENT-10 | API-K-19 is referenced by ENT-10 but none of the supplied documents defines it |
| API-K-20 | Mã được nhắc tới nhưng không có mô tả | ENT-09 | API-K-20 is referenced by ENT-09 but none of the supplied documents defines it |
| API-K-21 | Mã được nhắc tới nhưng không có mô tả | FR-KB-04 | API-K-21 is referenced by FR-KB-04 but none of the supplied documents defines it |
| API-K-23 | Mã được nhắc tới nhưng không có mô tả | ENT-10 | API-K-23 is referenced by ENT-10 but none of the supplied documents defines it |
| API-K-24 | Mã được nhắc tới nhưng không có mô tả | ENT-07 | API-K-24 is referenced by ENT-07 but none of the supplied documents defines it |
| API-K-25 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-01 | API-K-25 is referenced by FR-CAT-01 but none of the supplied documents defines it |
| API-K-26 | Mã được nhắc tới nhưng không có mô tả | ENT-11 | API-K-26 is referenced by ENT-11 but none of the supplied documents defines it |
| API-K-27 | Mã được nhắc tới nhưng không có mô tả | FR-KB-04 | API-K-27 is referenced by FR-KB-04 but none of the supplied documents defines it |
| API-K-28 | Mã được nhắc tới nhưng không có mô tả | FR-KB-04 | API-K-28 is referenced by FR-KB-04 but none of the supplied documents defines it |
| API-K-29 | Mã được nhắc tới nhưng không có mô tả | ENT-11 | API-K-29 is referenced by ENT-11 but none of the supplied documents defines it |
| API-K-30 | Mã được nhắc tới nhưng không có mô tả | ENT-11 | API-K-30 is referenced by ENT-11 but none of the supplied documents defines it |
| API-K-31 | Mã được nhắc tới nhưng không có mô tả | ENT-11 | API-K-31 is referenced by ENT-11 but none of the supplied documents defines it |
| API-K-32 | Mã được nhắc tới nhưng không có mô tả | FR-LCM-06 | API-K-32 is referenced by FR-LCM-06 but none of the supplied documents defines it |
| API-K-33 | Mã được nhắc tới nhưng không có mô tả | FR-LCM-06 | API-K-33 is referenced by FR-LCM-06 but none of the supplied documents defines it |
| API-K-34 | Mã được nhắc tới nhưng không có mô tả | FR-KB-03 | API-K-34 is referenced by FR-KB-03 but none of the supplied documents defines it |
| API-K-35 | Mã được nhắc tới nhưng không có mô tả | FR-KB-03 | API-K-35 is referenced by FR-KB-03 but none of the supplied documents defines it |
| API-K-36 | Mã được nhắc tới nhưng không có mô tả | FR-KB-01 | API-K-36 is referenced by FR-KB-01 but none of the supplied documents defines it |
| API-K-37 | Mã được nhắc tới nhưng không có mô tả | FR-KB-01 | API-K-37 is referenced by FR-KB-01 but none of the supplied documents defines it |
| API-K-38 | Mã được nhắc tới nhưng không có mô tả | FR-KB-05 | API-K-38 is referenced by FR-KB-05 but none of the supplied documents defines it |
| API-K-39 | Mã được nhắc tới nhưng không có mô tả | FR-KB-05 | API-K-39 is referenced by FR-KB-05 but none of the supplied documents defines it |
| API-K-40 | Mã được nhắc tới nhưng không có mô tả | FR-LIC-01 | API-K-40 is referenced by FR-LIC-01 but none of the supplied documents defines it |
| API-K-41 | Mã được nhắc tới nhưng không có mô tả | FR-LIC-01 | API-K-41 is referenced by FR-LIC-01 but none of the supplied documents defines it |
| API-K-42 | Mã được nhắc tới nhưng không có mô tả | FR-LIC-02 | API-K-42 is referenced by FR-LIC-02 but none of the supplied documents defines it |
| API-K-43 | Mã được nhắc tới nhưng không có mô tả | FR-LIC-01 | API-K-43 is referenced by FR-LIC-01 but none of the supplied documents defines it |
| API-K-44 | Mã được nhắc tới nhưng không có mô tả | ENT-20 | API-K-44 is referenced by ENT-20 but none of the supplied documents defines it |
| API-K-45 | Mã được nhắc tới nhưng không có mô tả | ENT-20 | API-K-45 is referenced by ENT-20 but none of the supplied documents defines it |
| API-K | Mã được nhắc tới nhưng không có mô tả | M-02 | API-K is referenced by M-02 but none of the supplied documents defines it |
| API-P-01 | Mã được nhắc tới nhưng không có mô tả | FR-ADM-01 | API-P-01 is referenced by FR-ADM-01 but none of the supplied documents defines it |
| API-P-02 | Mã được nhắc tới nhưng không có mô tả | FR-ADM-03 | API-P-02 is referenced by FR-ADM-03 but none of the supplied documents defines it |
| API-P-03 | Mã được nhắc tới nhưng không có mô tả | FR-ADM-02 | API-P-03 is referenced by FR-ADM-02 but none of the supplied documents defines it |
| API-P-04 | Mã được nhắc tới nhưng không có mô tả | FR-ADM-02 | API-P-04 is referenced by FR-ADM-02 but none of the supplied documents defines it |
| API-P-05 | Mã được nhắc tới nhưng không có mô tả | FR-ADM-01 | API-P-05 is referenced by FR-ADM-01 but none of the supplied documents defines it |
| API-P-06 | Mã được nhắc tới nhưng không có mô tả | FR-ADM-02 | API-P-06 is referenced by FR-ADM-02 but none of the supplied documents defines it |
| API-P-07 | Mã được nhắc tới nhưng không có mô tả | ENT-02 | API-P-07 is referenced by ENT-02 but none of the supplied documents defines it |
| API-P-08 | Mã được nhắc tới nhưng không có mô tả | ENT-02 | API-P-08 is referenced by ENT-02 but none of the supplied documents defines it |
| API-P-09 | Mã được nhắc tới nhưng không có mô tả | FR-INV-01 | API-P-09 is referenced by FR-INV-01 but none of the supplied documents defines it |
| API-P-10 | Mã được nhắc tới nhưng không có mô tả | ENT-03 | API-P-10 is referenced by ENT-03 but none of the supplied documents defines it |
| API-P-11 | Mã được nhắc tới nhưng không có mô tả | FR-UI-02 | API-P-11 is referenced by FR-UI-02 but none of the supplied documents defines it |
| API-P-12 | Mã được nhắc tới nhưng không có mô tả | ENT-03 | API-P-12 is referenced by ENT-03 but none of the supplied documents defines it |
| API-P-15 | Mã được nhắc tới nhưng không có mô tả | ENT-04 | API-P-15 is referenced by ENT-04 but none of the supplied documents defines it |
| API-P-16 | Mã được nhắc tới nhưng không có mô tả | ENT-04 | API-P-16 is referenced by ENT-04 but none of the supplied documents defines it |
| API-P-17 | Mã được nhắc tới nhưng không có mô tả | ENT-04 | API-P-17 is referenced by ENT-04 but none of the supplied documents defines it |
| API-P-18 | Mã được nhắc tới nhưng không có mô tả | ENT-05 | API-P-18 is referenced by ENT-05 but none of the supplied documents defines it |
| API-P-19 | Mã được nhắc tới nhưng không có mô tả | ENT-05 | API-P-19 is referenced by ENT-05 but none of the supplied documents defines it |
| API-P-20 | Mã được nhắc tới nhưng không có mô tả | FR-INV-06 | API-P-20 is referenced by FR-INV-06 but none of the supplied documents defines it |
| API-P-21 | Mã được nhắc tới nhưng không có mô tả | ENT-05 | API-P-21 is referenced by ENT-05 but none of the supplied documents defines it |
| API-P-22 | Mã được nhắc tới nhưng không có mô tả | FR-ADM-03 | API-P-22 is referenced by FR-ADM-03 but none of the supplied documents defines it |
| API-P-23 | Mã được nhắc tới nhưng không có mô tả | ENT-06 | API-P-23 is referenced by ENT-06 but none of the supplied documents defines it |
| API-P-26 | Mã được nhắc tới nhưng không có mô tả | ENT-06 | API-P-26 is referenced by ENT-06 but none of the supplied documents defines it |
| API-P-27 | Mã được nhắc tới nhưng không có mô tả | ENT-06 | API-P-27 is referenced by ENT-06 but none of the supplied documents defines it |
| API-P-30 | Mã được nhắc tới nhưng không có mô tả | FR-ADM-04 | API-P-30 is referenced by FR-ADM-04 but none of the supplied documents defines it |
| API-P-31 | Mã được nhắc tới nhưng không có mô tả | FR-CI-02 | API-P-31 is referenced by FR-CI-02 but none of the supplied documents defines it |
| API-P-32 | Mã được nhắc tới nhưng không có mô tả | FR-LCM-02 | API-P-32 is referenced by FR-LCM-02 but none of the supplied documents defines it |
| API-P-33 | Mã được nhắc tới nhưng không có mô tả | ENT-24 | API-P-33 is referenced by ENT-24 but none of the supplied documents defines it |
| API-P-34 | Mã được nhắc tới nhưng không có mô tả | ENT-24 | API-P-34 is referenced by ENT-24 but none of the supplied documents defines it |
| API-P-35 | Mã được nhắc tới nhưng không có mô tả | ENT-24 | API-P-35 is referenced by ENT-24 but none of the supplied documents defines it |
| API-P-36 | Mã được nhắc tới nhưng không có mô tả | ENT-24 | API-P-36 is referenced by ENT-24 but none of the supplied documents defines it |
| API-P-37 | Mã được nhắc tới nhưng không có mô tả | FR-CI-02 | API-P-37 is referenced by FR-CI-02 but none of the supplied documents defines it |
| API-P-38 | Mã được nhắc tới nhưng không có mô tả | ENT-24 | API-P-38 is referenced by ENT-24 but none of the supplied documents defines it |
| API-P-42 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-03 | API-P-42 is referenced by FR-GOV-03 but none of the supplied documents defines it |
| API-P | Mã được nhắc tới nhưng không có mô tả | M-01 | API-P is referenced by M-01 but none of the supplied documents defines it |
| API-S-01 | Mã được nhắc tới nhưng không có mô tả | FR-INV-03 | API-S-01 is referenced by FR-INV-03 but none of the supplied documents defines it |
| API-S-02 | Mã được nhắc tới nhưng không có mô tả | ENT-39 | API-S-02 is referenced by ENT-39 but none of the supplied documents defines it |
| API-S-03 | Mã được nhắc tới nhưng không có mô tả | ENT-39 | API-S-03 is referenced by ENT-39 but none of the supplied documents defines it |
| API-S-04 | Mã được nhắc tới nhưng không có mô tả | ENT-39 | API-S-04 is referenced by ENT-39 but none of the supplied documents defines it |
| API-S-05 | Mã được nhắc tới nhưng không có mô tả | FR-UI-09 | API-S-05 is referenced by FR-UI-09 but none of the supplied documents defines it |
| API-S-06 | Mã được nhắc tới nhưng không có mô tả | ENT-39 | API-S-06 is referenced by ENT-39 but none of the supplied documents defines it |
| API-S-07 | Mã được nhắc tới nhưng không có mô tả | ENT-03 | API-S-07 is referenced by ENT-03 but none of the supplied documents defines it |
| API-S-08 | Mã được nhắc tới nhưng không có mô tả | ENT-03 | API-S-08 is referenced by ENT-03 but none of the supplied documents defines it |
| API-S-09 | Mã được nhắc tới nhưng không có mô tả | FR-UI-07 | API-S-09 is referenced by FR-UI-07 but none of the supplied documents defines it |
| API-S-10 | Mã được nhắc tới nhưng không có mô tả | FR-AST-07 | API-S-10 is referenced by FR-AST-07 but none of the supplied documents defines it |
| API-S-11 | Mã được nhắc tới nhưng không có mô tả | FR-UI-09 | API-S-11 is referenced by FR-UI-09 but none of the supplied documents defines it |
| API-S-12 | Mã được nhắc tới nhưng không có mô tả | FR-AST-07 | API-S-12 is referenced by FR-AST-07 but none of the supplied documents defines it |
| API-S-13 | Mã được nhắc tới nhưng không có mô tả | FR-UI-07 | API-S-13 is referenced by FR-UI-07 but none of the supplied documents defines it |
| API-S-14 | Mã được nhắc tới nhưng không có mô tả | FR-AST-08 | API-S-14 is referenced by FR-AST-08 but none of the supplied documents defines it |
| API-S-15 | Mã được nhắc tới nhưng không có mô tả | FR-AST-12 | API-S-15 is referenced by FR-AST-12 but none of the supplied documents defines it |
| API-S-16 | Mã được nhắc tới nhưng không có mô tả | FR-UI-09 | API-S-16 is referenced by FR-UI-09 but none of the supplied documents defines it |
| API-S-17 | Mã được nhắc tới nhưng không có mô tả | FR-UI-07 | API-S-17 is referenced by FR-UI-07 but none of the supplied documents defines it |
| API-S-18 | Mã được nhắc tới nhưng không có mô tả | FR-AST-07 | API-S-18 is referenced by FR-AST-07 but none of the supplied documents defines it |
| API-S-19 | Mã được nhắc tới nhưng không có mô tả | FR-AST-07 | API-S-19 is referenced by FR-AST-07 but none of the supplied documents defines it |
| API-S-20 | Mã được nhắc tới nhưng không có mô tả | FR-UI-10 | API-S-20 is referenced by FR-UI-10 but none of the supplied documents defines it |
| API-S-21 | Mã được nhắc tới nhưng không có mô tả | ENT-34 | API-S-21 is referenced by ENT-34 but none of the supplied documents defines it |
| API-S-22 | Mã được nhắc tới nhưng không có mô tả | ENT-34 | API-S-22 is referenced by ENT-34 but none of the supplied documents defines it |
| API-S-23 | Mã được nhắc tới nhưng không có mô tả | ENT-34 | API-S-23 is referenced by ENT-34 but none of the supplied documents defines it |
| API-S-24 | Mã được nhắc tới nhưng không có mô tả | ENT-34 | API-S-24 is referenced by ENT-34 but none of the supplied documents defines it |
| API-S-25 | Mã được nhắc tới nhưng không có mô tả | FR-UI-08 | API-S-25 is referenced by FR-UI-08 but none of the supplied documents defines it |
| API-S-26 | Mã được nhắc tới nhưng không có mô tả | FR-AST-11 | API-S-26 is referenced by FR-AST-11 but none of the supplied documents defines it |
| API-S-27 | Mã được nhắc tới nhưng không có mô tả | FR-UI-08 | API-S-27 is referenced by FR-UI-08 but none of the supplied documents defines it |
| API-S-28 | Mã được nhắc tới nhưng không có mô tả | FR-AST-11 | API-S-28 is referenced by FR-AST-11 but none of the supplied documents defines it |
| API-S-29 | Mã được nhắc tới nhưng không có mô tả | ENT-35 | API-S-29 is referenced by ENT-35 but none of the supplied documents defines it |
| API-S-30 | Mã được nhắc tới nhưng không có mô tả | FR-UI-08 | API-S-30 is referenced by FR-UI-08 but none of the supplied documents defines it |
| API-S-31 | Mã được nhắc tới nhưng không có mô tả | ENT-35 | API-S-31 is referenced by ENT-35 but none of the supplied documents defines it |
| API-S-32 | Mã được nhắc tới nhưng không có mô tả | FR-UI-08 | API-S-32 is referenced by FR-UI-08 but none of the supplied documents defines it |
| API-S-33 | Mã được nhắc tới nhưng không có mô tả | FR-UI-08 | API-S-33 is referenced by FR-UI-08 but none of the supplied documents defines it |
| API-S-34 | Mã được nhắc tới nhưng không có mô tả | FR-UI-07 | API-S-34 is referenced by FR-UI-07 but none of the supplied documents defines it |
| API-S-35 | Mã được nhắc tới nhưng không có mô tả | ENT-37 | API-S-35 is referenced by ENT-37 but none of the supplied documents defines it |
| API-S-36 | Mã được nhắc tới nhưng không có mô tả | ENT-37 | API-S-36 is referenced by ENT-37 but none of the supplied documents defines it |
| API-S-37 | Mã được nhắc tới nhưng không có mô tả | FR-UI-10 | API-S-37 is referenced by FR-UI-10 but none of the supplied documents defines it |
| API-S-38 | Mã được nhắc tới nhưng không có mô tả | FR-AST-15 | API-S-38 is referenced by FR-AST-15 but none of the supplied documents defines it |
| API-S-39 | Mã được nhắc tới nhưng không có mô tả | ENT-40 | API-S-39 is referenced by ENT-40 but none of the supplied documents defines it |
| API-S-40 | Mã được nhắc tới nhưng không có mô tả | ENT-40 | API-S-40 is referenced by ENT-40 but none of the supplied documents defines it |
| API-S-41 | Mã được nhắc tới nhưng không có mô tả | ENT-40 | API-S-41 is referenced by ENT-40 but none of the supplied documents defines it |
| API-S-42 | Mã được nhắc tới nhưng không có mô tả | ENT-40 | API-S-42 is referenced by ENT-40 but none of the supplied documents defines it |
| API-S-43 | Mã được nhắc tới nhưng không có mô tả | FR-AST-17 | API-S-43 is referenced by FR-AST-17 but none of the supplied documents defines it |
| API-S-44 | Mã được nhắc tới nhưng không có mô tả | ENT-41 | API-S-44 is referenced by ENT-41 but none of the supplied documents defines it |
| API-S-45 | Mã được nhắc tới nhưng không có mô tả | ENT-41 | API-S-45 is referenced by ENT-41 but none of the supplied documents defines it |
| API-S-46 | Mã được nhắc tới nhưng không có mô tả | ENT-41 | API-S-46 is referenced by ENT-41 but none of the supplied documents defines it |
| API-S-47 | Mã được nhắc tới nhưng không có mô tả | ENT-41 | API-S-47 is referenced by ENT-41 but none of the supplied documents defines it |
| API-S-48 | Mã được nhắc tới nhưng không có mô tả | FR-AST-09 | API-S-48 is referenced by FR-AST-09 but none of the supplied documents defines it |
| API-S-49 | Mã được nhắc tới nhưng không có mô tả | ENT-42 | API-S-49 is referenced by ENT-42 but none of the supplied documents defines it |
| API-S-50 | Mã được nhắc tới nhưng không có mô tả | ENT-42 | API-S-50 is referenced by ENT-42 but none of the supplied documents defines it |
| API-S-51 | Mã được nhắc tới nhưng không có mô tả | FR-UI-07 | API-S-51 is referenced by FR-UI-07 but none of the supplied documents defines it |
| API-S-52 | Mã được nhắc tới nhưng không có mô tả | FR-AST-16 | API-S-52 is referenced by FR-AST-16 but none of the supplied documents defines it |
| API-S-55 | Mã được nhắc tới nhưng không có mô tả | FR-AST-09 | API-S-55 is referenced by FR-AST-09 but none of the supplied documents defines it |
| API-S-56 | Mã được nhắc tới nhưng không có mô tả | FR-AST-09 | API-S-56 is referenced by FR-AST-09 but none of the supplied documents defines it |
| API-S | Mã được nhắc tới nhưng không có mô tả | M-06 | API-S is referenced by M-06 but none of the supplied documents defines it |
| API-T-01 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-01 | API-T-01 is referenced by FR-CAT-01 but none of the supplied documents defines it |
| API-T-02 | Mã được nhắc tới nhưng không có mô tả | FR-LCM-01 | API-T-02 is referenced by FR-LCM-01 but none of the supplied documents defines it |
| API-T-03 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-03 | API-T-03 is referenced by FR-CAT-03 but none of the supplied documents defines it |
| API-T-04 | Mã được nhắc tới nhưng không có mô tả | FR-LCM-05 | API-T-04 is referenced by FR-LCM-05 but none of the supplied documents defines it |
| API-T-05 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-02 | API-T-05 is referenced by FR-CAT-02 but none of the supplied documents defines it |
| API-T-06 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-02 | API-T-06 is referenced by FR-CAT-02 but none of the supplied documents defines it |
| API-T-07 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-06 | API-T-07 is referenced by FR-CAT-06 but none of the supplied documents defines it |
| API-T-08 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-06 | API-T-08 is referenced by FR-CAT-06 but none of the supplied documents defines it |
| API-T-09 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-03 | API-T-09 is referenced by FR-CAT-03 but none of the supplied documents defines it |
| API-T-10 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-04 | API-T-10 is referenced by FR-CAT-04 but none of the supplied documents defines it |
| API-T-11 | Mã được nhắc tới nhưng không có mô tả | ENT-29 | API-T-11 is referenced by ENT-29 but none of the supplied documents defines it |
| API-T-12 | Mã được nhắc tới nhưng không có mô tả | ENT-29 | API-T-12 is referenced by ENT-29 but none of the supplied documents defines it |
| API-T-13 | Mã được nhắc tới nhưng không có mô tả | ENT-28 | API-T-13 is referenced by ENT-28 but none of the supplied documents defines it |
| API-T-14 | Mã được nhắc tới nhưng không có mô tả | ENT-29 | API-T-14 is referenced by ENT-29 but none of the supplied documents defines it |
| API-T-15 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-07 | API-T-15 is referenced by FR-CAT-07 but none of the supplied documents defines it |
| API-T-18 | Mã được nhắc tới nhưng không có mô tả | ENT-31 | API-T-18 is referenced by ENT-31 but none of the supplied documents defines it |
| API-T-19 | Mã được nhắc tới nhưng không có mô tả | ENT-31 | API-T-19 is referenced by ENT-31 but none of the supplied documents defines it |
| API-T-22 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-04 | API-T-22 is referenced by FR-CAT-04 but none of the supplied documents defines it |
| API-T-23 | Mã được nhắc tới nhưng không có mô tả | FR-SEL-01 | API-T-23 is referenced by FR-SEL-01 but none of the supplied documents defines it |
| API-T-24 | Mã được nhắc tới nhưng không có mô tả | FR-LCM-06 | API-T-24 is referenced by FR-LCM-06 but none of the supplied documents defines it |
| API-T-25 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-08 | API-T-25 is referenced by FR-CAT-08 but none of the supplied documents defines it |
| API-T-26 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-01 | API-T-26 is referenced by FR-CAT-01 but none of the supplied documents defines it |
| API-T-27 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-08 | API-T-27 is referenced by FR-CAT-08 but none of the supplied documents defines it |
| API-T-28 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-08 | API-T-28 is referenced by FR-CAT-08 but none of the supplied documents defines it |
| API-T-29 | Mã được nhắc tới nhưng không có mô tả | ENT-30 | API-T-29 is referenced by ENT-30 but none of the supplied documents defines it |
| API-T-30 | Mã được nhắc tới nhưng không có mô tả | ENT-30 | API-T-30 is referenced by ENT-30 but none of the supplied documents defines it |
| API-T-31 | Mã được nhắc tới nhưng không có mô tả | FR-CAT-08 | API-T-31 is referenced by FR-CAT-08 but none of the supplied documents defines it |
| API-T-32 | Mã được nhắc tới nhưng không có mô tả | FR-SEL-05 | API-T-32 is referenced by FR-SEL-05 but none of the supplied documents defines it |
| API-T-33 | Mã được nhắc tới nhưng không có mô tả | FR-SEL-05 | API-T-33 is referenced by FR-SEL-05 but none of the supplied documents defines it |
| API-T-34 | Mã được nhắc tới nhưng không có mô tả | FR-SEL-01 | API-T-34 is referenced by FR-SEL-01 but none of the supplied documents defines it |
| API-T-35 | Mã được nhắc tới nhưng không có mô tả | FR-LIC-05 | API-T-35 is referenced by FR-LIC-05 but none of the supplied documents defines it |
| API-T-36 | Mã được nhắc tới nhưng không có mô tả | FR-LIC-05 | API-T-36 is referenced by FR-LIC-05 but none of the supplied documents defines it |
| API-T-37 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-06 | API-T-37 is referenced by FR-GOV-06 but none of the supplied documents defines it |
| API-T-38 | Mã được nhắc tới nhưng không có mô tả | FR-LIC-05 | API-T-38 is referenced by FR-LIC-05 but none of the supplied documents defines it |
| API-T-39 | Mã được nhắc tới nhưng không có mô tả | FR-LIC-05 | API-T-39 is referenced by FR-LIC-05 but none of the supplied documents defines it |
| API-T-40 | Mã được nhắc tới nhưng không có mô tả | ENT-32 | API-T-40 is referenced by ENT-32 but none of the supplied documents defines it |
| API-T-41 | Mã được nhắc tới nhưng không có mô tả | FR-SEL-04 | API-T-41 is referenced by FR-SEL-04 but none of the supplied documents defines it |
| API-T | Mã được nhắc tới nhưng không có mô tả | M-05 | API-T is referenced by M-05 but none of the supplied documents defines it |
| API-V-01 | Mã được nhắc tới nhưng không có mô tả | FR-VUL-01 | API-V-01 is referenced by FR-VUL-01 but none of the supplied documents defines it |
| API-V-02 | Mã được nhắc tới nhưng không có mô tả | FR-VUL-01 | API-V-02 is referenced by FR-VUL-01 but none of the supplied documents defines it |
| API-V-03 | Mã được nhắc tới nhưng không có mô tả | FR-VUL-04 | API-V-03 is referenced by FR-VUL-04 but none of the supplied documents defines it |
| API-V-04 | Mã được nhắc tới nhưng không có mô tả | FR-VUL-05 | API-V-04 is referenced by FR-VUL-05 but none of the supplied documents defines it |
| API-V-05 | Mã được nhắc tới nhưng không có mô tả | ENT-22 | API-V-05 is referenced by ENT-22 but none of the supplied documents defines it |
| API-V-06 | Mã được nhắc tới nhưng không có mô tả | BRL-14 | API-V-06 is referenced by BRL-14 but none of the supplied documents defines it |
| API-V-07 | Mã được nhắc tới nhưng không có mô tả | FR-VUL-07 | API-V-07 is referenced by FR-VUL-07 but none of the supplied documents defines it |
| API-V-08 | Mã được nhắc tới nhưng không có mô tả | FR-VUL-04 | API-V-08 is referenced by FR-VUL-04 but none of the supplied documents defines it |
| API-V-09 | Mã được nhắc tới nhưng không có mô tả | FR-UI-04 | API-V-09 is referenced by FR-UI-04 but none of the supplied documents defines it |
| API-V-11 | Mã được nhắc tới nhưng không có mô tả | ENT-21 | API-V-11 is referenced by ENT-21 but none of the supplied documents defines it |
| API-V-12 | Mã được nhắc tới nhưng không có mô tả | FR-VUL-05 | API-V-12 is referenced by FR-VUL-05 but none of the supplied documents defines it |
| API-V-13 | Mã được nhắc tới nhưng không có mô tả | FR-VUL-04 | API-V-13 is referenced by FR-VUL-04 but none of the supplied documents defines it |
| API-V-14 | Mã được nhắc tới nhưng không có mô tả | FR-VUL-05 | API-V-14 is referenced by FR-VUL-05 but none of the supplied documents defines it |
| API-V-16 | Mã được nhắc tới nhưng không có mô tả | FR-GOV-07 | API-V-16 is referenced by FR-GOV-07 but none of the supplied documents defines it |
| API-V-17 | Mã được nhắc tới nhưng không có mô tả | FR-VUL-02 | API-V-17 is referenced by FR-VUL-02 but none of the supplied documents defines it |
| API-V-18 | Mã được nhắc tới nhưng không có mô tả | ENT-21 | API-V-18 is referenced by ENT-21 but none of the supplied documents defines it |
| API-V-19 | Mã được nhắc tới nhưng không có mô tả | FR-UI-06 | API-V-19 is referenced by FR-UI-06 but none of the supplied documents defines it |
| API-V-20 | Mã được nhắc tới nhưng không có mô tả | FR-UI-03 | API-V-20 is referenced by FR-UI-03 but none of the supplied documents defines it |
| API-V-21 | Mã được nhắc tới nhưng không có mô tả | FR-LCM-01 | API-V-21 is referenced by FR-LCM-01 but none of the supplied documents defines it |
| API-V-22 | Mã được nhắc tới nhưng không có mô tả | FR-LCM-01 | API-V-22 is referenced by FR-LCM-01 but none of the supplied documents defines it |
| API-V-23 | Mã được nhắc tới nhưng không có mô tả | FR-LCM-05 | API-V-23 is referenced by FR-LCM-05 but none of the supplied documents defines it |
| API-V-24 | Mã được nhắc tới nhưng không có mô tả | FR-LCM-04 | API-V-24 is referenced by FR-LCM-04 but none of the supplied documents defines it |
| API-V-25 | Mã được nhắc tới nhưng không có mô tả | FR-LIC-01 | API-V-25 is referenced by FR-LIC-01 but none of the supplied documents defines it |
| API-V-26 | Mã được nhắc tới nhưng không có mô tả | FR-LIC-03 | API-V-26 is referenced by FR-LIC-03 but none of the supplied documents defines it |
| API-V-27 | Mã được nhắc tới nhưng không có mô tả | FR-UI-03 | API-V-27 is referenced by FR-UI-03 but none of the supplied documents defines it |
| API-V-29 | Mã được nhắc tới nhưng không có mô tả | FR-LIC-02 | API-V-29 is referenced by FR-LIC-02 but none of the supplied documents defines it |
| API-V | Mã được nhắc tới nhưng không có mô tả | M-03 | API-V is referenced by M-03 but none of the supplied documents defines it |
| AS-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AS' has no configured class |
| AS-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AS' has no configured class |
| AS-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AS' has no configured class |
| AS-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AS' has no configured class |
| AS-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AS' has no configured class |
| AS-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AS' has no configured class |
| AS-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AS' has no configured class |
| AS-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'AS' has no configured class |
| BO-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'BO' has no configured class |
| BO-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'BO' has no configured class |
| BO-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'BO' has no configured class |
| BO-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'BO' has no configured class |
| BO-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'BO' has no configured class |
| BO-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'BO' has no configured class |
| BO-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'BO' has no configured class |
| C-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'C' has no configured class |
| C-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'C' has no configured class |
| C-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'C' has no configured class |
| C-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'C' has no configured class |
| C-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'C' has no configured class |
| C-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'C' has no configured class |
| C-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'C' has no configured class |
| C-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'C' has no configured class |
| DC-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'DC' has no configured class |
| DC-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'DC' has no configured class |
| DC-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'DC' has no configured class |
| DC-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'DC' has no configured class |
| DC-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'DC' has no configured class |
| DC-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'DC' has no configured class |
| DC-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'DC' has no configured class |
| DC-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'DC' has no configured class |
| DC-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'DC' has no configured class |
| DS-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'DS' has no configured class |
| DS-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'DS' has no configured class |
| DS-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'DS' has no configured class |
| EIF-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-10 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-11 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-12 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-13 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-14 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-15 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-16 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-17 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-18 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EIF-19 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EIF' has no configured class |
| EXT-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EXT' has no configured class |
| EXT-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EXT' has no configured class |
| EXT-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EXT' has no configured class |
| EXT-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EXT' has no configured class |
| EXT-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EXT' has no configured class |
| EXT-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EXT' has no configured class |
| EXT-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EXT' has no configured class |
| EXT-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EXT' has no configured class |
| EXT-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'EXT' has no configured class |
| “Khả dụng mức B” | Yêu cầu khôi phục chưa nêu RTO/RPO | — | QualityTarget “Khả dụng mức B”: MISSING_RECOVERY_OBJECTIVE |
| FR-LCM | Mã được nhắc tới nhưng không có mô tả | FR-VUL | FR-LCM is referenced by FR-VUL but none of the supplied documents defines it |
| FR-SEL | Mã được nhắc tới nhưng không có mô tả | FR-CAT | FR-SEL is referenced by FR-CAT but none of the supplied documents defines it |
| NFR-CO-07 | Yêu cầu khôi phục chưa nêu RTO/RPO | — | QualityTarget NFR-CO-07: MISSING_RECOVERY_OBJECTIVE |
| NFR-RE-04 | Yêu cầu khôi phục chưa nêu RTO/RPO | — | QualityTarget NFR-RE-04: MISSING_RECOVERY_OBJECTIVE |
| NFR-RE | Yêu cầu khôi phục chưa nêu RTO/RPO | — | QualityTarget NFR-RE: MISSING_RECOVERY_OBJECTIVE |
| OA-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OA' has no configured class |
| OA-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OA' has no configured class |
| OA-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OA' has no configured class |
| OA-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OA' has no configured class |
| OA-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OA' has no configured class |
| OA-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OA' has no configured class |
| OA-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OA' has no configured class |
| OA-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OA' has no configured class |
| OA-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OA' has no configured class |
| OA-10 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OA' has no configured class |
| OA-11 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OA' has no configured class |
| OA-12 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OA' has no configured class |
| OI-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-10 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-11 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-12 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-13 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-14 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-15 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-16 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-17 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| OI-18 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'OI' has no configured class |
| POC-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'POC' has no configured class |
| POC-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'POC' has no configured class |
| POC-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'POC' has no configured class |
| POC-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'POC' has no configured class |
| POC-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'POC' has no configured class |
| POC-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'POC' has no configured class |
| POC-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'POC' has no configured class |
| QA-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QA' has no configured class |
| QA-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QA' has no configured class |
| QA-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QA' has no configured class |
| QA-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QA' has no configured class |
| QA-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QA' has no configured class |
| QA-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QA' has no configured class |
| QA-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QA' has no configured class |
| QA-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QA' has no configured class |
| QA-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QA' has no configured class |
| QA-10 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QA' has no configured class |
| QA-11 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QA' has no configured class |
| QT-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QT' has no configured class |
| QT-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QT' has no configured class |
| QT-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QT' has no configured class |
| QT-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QT' has no configured class |
| QT-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QT' has no configured class |
| QT-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QT' has no configured class |
| QT-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QT' has no configured class |
| QT-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'QT' has no configured class |
| R-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-10 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-11 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-12 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-13 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-14 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-15 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| R-16 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'R' has no configured class |
| REF-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-10 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-11 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-A | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-B | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-C | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-D | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-E | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-F | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| REF-G | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'REF' has no configured class |
| RSK-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'RSK' has no configured class |
| RSK-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'RSK' has no configured class |
| RSK-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'RSK' has no configured class |
| RSK-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'RSK' has no configured class |
| RSK-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'RSK' has no configured class |
| RSK-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'RSK' has no configured class |
| RSK-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'RSK' has no configured class |
| RSK-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'RSK' has no configured class |
| RSK-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'RSK' has no configured class |
| RSK-10 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'RSK' has no configured class |
| SAD | Văn xuôi chưa đọc hết | — | 2 candidate line(s) beyond prose_limit=40 were not read |
| SG-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SG' has no configured class |
| SG-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SG' has no configured class |
| SG-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SG' has no configured class |
| SG-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SG' has no configured class |
| SG-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SG' has no configured class |
| SG-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SG' has no configured class |
| SG-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SG' has no configured class |
| SG-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SG' has no configured class |
| SK-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SK' has no configured class |
| SK-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SK' has no configured class |
| SK-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SK' has no configured class |
| SK-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SK' has no configured class |
| SM-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SM' has no configured class |
| SM-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SM' has no configured class |
| SM-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SM' has no configured class |
| SM-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SM' has no configured class |
| SM-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SM' has no configured class |
| SM-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SM' has no configured class |
| SM-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SM' has no configured class |
| SM-10 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SM' has no configured class |
| SM-11 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SM' has no configured class |
| SM-12 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SM' has no configured class |
| SM-13 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SM' has no configured class |
| SM-14 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'SM' has no configured class |
| TDD-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TDD' has no configured class |
| TDD-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TDD' has no configured class |
| TDD-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TDD' has no configured class |
| TDD-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TDD' has no configured class |
| TDD-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TDD' has no configured class |
| TDD-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TDD' has no configured class |
| TDD-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TDD' has no configured class |
| TDD-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TDD' has no configured class |
| TDD-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TDD' has no configured class |
| TDD-10 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TDD' has no configured class |
| TDD-11 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TDD' has no configured class |
| TDD-12 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TDD' has no configured class |
| TH-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TH' has no configured class |
| TH-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TH' has no configured class |
| TH-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TH' has no configured class |
| TH-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TH' has no configured class |
| TH-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TH' has no configured class |
| TH-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TH' has no configured class |
| TH-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TH' has no configured class |
| TH-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TH' has no configured class |
| TH-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TH' has no configured class |
| TH-10 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TH' has no configured class |
| TH-11 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TH' has no configured class |
| TH-12 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'TH' has no configured class |
| UC-01 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-02 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-03 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-04 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-05 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-06 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-07 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-08 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-09 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-10 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-11 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-12 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-13 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-14 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-15 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-16 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-17 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-18 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-19 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-20 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-21 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-22 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-23 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-24 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-25 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-26 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-27 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-28 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-29 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-30 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-31 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-32 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-33 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-34 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-35 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-36 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-37 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-38 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-39 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-40 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| UC-41 | Mã chưa biết thuộc loại phần tử nào | — | id prefix 'UC' has no configured class |
| line:1316 | Sự kiện do mô hình đề xuất bị loại | — | model proposal failed verification: QUOTE_NOT_IN_LINE, CLASS_NOT_ALLOWED |

### A.3 Phần tử chưa xếp vào phần nào

Mọi phần tử đều đã có chỗ trong các phần 1–11.
