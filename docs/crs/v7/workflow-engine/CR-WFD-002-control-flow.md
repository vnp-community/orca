# CR-WFD-002 — Điều khiển luồng: rẽ nhánh thật, trạng thái `skipped`, workflow con, lặp có giới hạn, ngôn ngữ biểu thức

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WFD-002 |
| **Tên** | Guard `when` trên bước, trạng thái `skipped`, ngôn ngữ biểu thức có kiểu, bước `sub_workflow`, `foreach` có giới hạn (P2) |
| **Loại** | Feature |
| **Priority** | 🔴 P0 (guard, biểu thức, `skipped`); 🟡 P2 (`foreach`) |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-08 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-WFD-001 (lược đồ), CR-WFD-003 (ngữ cảnh dữ liệu) |
| **Mở khoá** | CR-WFD-004 (cổng cần rẽ nhánh theo quyết định), CR-WFD-005 (`sub_workflow`) |
| **Tác động** | `workflow-service` (`domain/condition.go` thay thế, `domain/expression/*` mới, `usecase/wave_dispatcher.go`, migration trạng thái bước), proto, frontend |
| **Thay thế một phần** | Bước `condition` hiện có (giữ để tương thích, đánh dấu deprecated) |

---

## 1. Bối cảnh và vấn đề

1. **`condition` không rẽ nhánh.** Bước chỉ trả `{result: bool}` với status `completed` (`WSI/domain/condition.go:55-59`); dispatcher không có tham chiếu nào tới kết quả này (`wave_dispatcher.go`), nên bước sau chạy bất kể giá trị. Không có if/else, không bỏ qua nhánh.
2. **Biểu thức nghèo.** Ngữ pháp `ident ==|!= (ident|'string')` nối bằng `&&`/`||`; không ngoặc, không `!`, không số, không `in` (`condition.go:8-35`). Ngữ cảnh là `map[string]string` do người dùng tự điền trong config của bước, không tự có output các bước trước (muốn dùng phải viết `"context": {"x": "{{outputs.a.b}}"}`).
3. **Không có `skipped`.** CHECK ở migration 0004 chỉ cho `pending, running, completed, failed`; không biểu diễn được "bước không chạy vì điều kiện".
4. **Không có workflow con.** Không có loại bước gọi workflow khác; `parallel` chỉ là danh sách phẳng, `dependsOn` của bước con bị bỏ qua (`step.go:76-79`).
5. **Không có lặp.** Muốn xử lý một danh sách phải nhân bản bước bằng tay.

## 2. Giải pháp đề xuất

### 2.1 Guard `when`

Mọi bước có thể khai báo `when` (biểu thức bool, README mục 5.3 cho ngữ cảnh). Quy tắc đánh giá, ngay trước khi bước vào hàng đợi:

| Tình huống | Kết quả |
|---|---|
| Không có `when` | chạy |
| `when` = true | chạy |
| `when` = false | bước `skipped` (kèm `reason=guard_false`) |
| `when` lỗi lúc chạy (tham chiếu thiếu, sai kiểu) | bước `failed` với `WORKFLOW_EXPRESSION_INVALID`, theo `onError` của bước (CR-WFD-006) |
| Một phụ thuộc `skipped` | mặc định bước này cũng `skipped` (`reason=dependency_skipped`) **trừ khi** khai `runIfDependenciesSkipped: true` |
| Một phụ thuộc `failed` với `onError=continue` | bước chạy; `steps.<id>.status` = `failed` đọc được trong `when` |

Điều này cho phép rẽ nhánh bằng hai bước có `when` đối lập, và "hội tụ" bằng bước có `runIfDependenciesSkipped`. Không thêm bước "if/else" riêng; giữ mô hình một khối dựng.

### 2.2 Ngôn ngữ biểu thức (O1)

Ngữ pháp tối thiểu, tự viết, thuần hàm, không I/O, không vòng lặp, không gọi hàm do người dùng định nghĩa:

```
expr    := or
or      := and ( "||" and )*
and     := not ( "&&" not )*
not     := "!" not | cmp
cmp     := value ( ("=="|"!="|"<"|"<="|">"|">=") value | "in" list | "exists" )?
value   := literal | path | "(" expr ")" | call
call    := "len(" path ")" | "now()"
path    := ident ( "." ident | "[" int "]" )*
literal := string | number | "true" | "false" | "null"
```

- Kiểu: `string, number, boolean, null, json`. So sánh khác kiểu là lỗi, không ép kiểu ngầm. `exists` đúng khi đường dẫn có mặt và khác `null`.
- Giới hạn: độ dài ≤ 2 KiB, độ sâu AST ≤ 32, ≤ 256 nút, thời gian đánh giá ≤ 5 ms (hết hạn → lỗi).
- Phân tích **lúc lưu** (CR-WFD-001 2.2) và đánh giá **lúc chạy**; cùng một cài đặt `internal/domain/expression` dùng cho cả hai.
- `{{…}}` hiện có vẫn hoạt động trong `config` (tương thích) nhưng `when` chỉ nhận ngữ pháp trên, không nhận `{{…}}`.
- Nếu về sau cần mạnh hơn: thay bằng CEL là quyết định riêng (O1), giữ nguyên hợp đồng ngữ cảnh.

### 2.3 Trạng thái `skipped` và thay đổi lập lịch

- Migration: mở rộng CHECK của `step_executions.status` thêm `skipped`, `waiting`, `cancelled`; thêm cột `skip_reason varchar(32)`.
- `BuildWaves` giữ nguyên (chỉ phụ thuộc tĩnh). Trước mỗi bước trong wave, dispatcher gọi `evaluateGuard`; bước `skipped` ghi vào DB và phát sự kiện `step.skipped`, **không** tính là lỗi, không chạy executor.
- Execution `completed` khi mọi bước ở trạng thái cuối (`completed|skipped|failed(onError=continue)|cancelled`) và không còn lỗi chưa được xử lý.

### 2.4 Bước `sub_workflow`

```jsonc
{ "id": "child", "type": "sub_workflow",
  "config": { "templateId": "<uuid>", "version": 3, "inputs": { "k": "{{expr}}" }, "waitForCompletion": true } }
```

| Quy tắc | Nội dung |
|---|---|
| Ghim | `version` bắt buộc ghim khi `schemaVersion=2` (không có "mới nhất" ngầm) để lượt chạy cha tái lập được |
| Dữ liệu | `inputs` ánh xạ sang input của con theo CR-WFD-003; output của con xuất hiện như `steps.child.outputs.*` theo `outputs[]` của con |
| Độ sâu | ≤ 5 tầng; **phát hiện vòng gọi** (A→B→A) lúc lưu bằng cách theo `templateId` đã ghim, và lúc chạy bằng bộ đếm tầng |
| Vòng đời | Con là một execution riêng có `parent_execution_id`, `parent_step_id`; huỷ cha huỷ con (CR-WFD-006); con `failed` → bước cha `failed` theo `onError` |
| Quyền | Người chạy phải có quyền chạy template con (CR-WFD-010) |
| `waitForCompletion=false` | P2; bước cha `completed` ngay khi con được tạo, output chỉ có `executionId` |

### 2.5 `foreach` (P2)

```jsonc
{ "id": "each", "type": "foreach",
  "config": { "items": "inputs.list", "maxItems": 50, "maxParallel": 5, "body": "<templateId>@<version>" } }
```

Mỗi phần tử chạy một execution con của `body` với `item`, `index`. `maxItems` bắt buộc (≤ 200), `maxParallel` ≤ 10; vượt `maxItems` là lỗi, không cắt ngầm. Output gộp thành mảng theo thứ tự `index`. Chỉ làm sau khi `sub_workflow` ổn định (O5).

### 2.6 Bước `condition` cũ

Giữ nguyên hành vi (trả `{result}`), đánh dấu `deprecated`; `ValidateDefinition` cảnh báo "kết quả không điều khiển luồng, dùng `when`". Template v1 không bị đổi nghĩa (CR-WFD-011).

## 3. Tiêu chí chấp nhận

1. Hai bước có `when` đối lập: đúng một bước `completed`, bước kia `skipped`, execution `completed`.
2. Bước phụ thuộc vào bước `skipped` tự `skipped` trừ khi có `runIfDependenciesSkipped`.
3. Biểu thức tham chiếu output của bước không phải tổ tiên bị từ chối lúc lưu.
4. `when` không hợp lệ về kiểu (`"a" < 3`) bị từ chối lúc lưu; lỗi lúc chạy tạo bước `failed`, không làm sập dispatcher.
5. `sub_workflow` A→B→A bị từ chối lúc lưu; vòng chỉ lộ lúc chạy dừng ở độ sâu 5 với lỗi rõ.
6. Huỷ execution cha làm execution con sang `cancelled`.
7. Test bảng cho ngữ pháp (mỗi toán tử, mỗi lỗi) và fuzz không gây panic hay vượt thời gian.

## 4. Tác động và rủi ro

| Mục | Ghi chú |
|---|---|
| Migration | Mở rộng CHECK trạng thái (dialect MySQL không có CHECK ngang nhau trước 8.0.16; ràng buộc ở tầng domain) |
| Tương thích | Client cũ không biết `skipped`: UI phải hiển thị mặc định như "bỏ qua" (CR-WFD-009); sự kiện `step.skipped` là loại mới |
| Rủi ro | Ngữ pháp tự viết dễ có lỗ hổng; giới hạn kích thước/thời gian và fuzz là bắt buộc. Nếu nhóm không muốn tự duy trì, đổi sang CEL (O1) trước khi triển khai |
| Rủi ro | Độ sâu `sub_workflow` làm nhân đôi chi phí lập lịch; cần hạn mức số execution đồng thời (CR-WFD-010) |

## 5. Chưa kiểm chứng / câu hỏi mở

- Chi phí đánh giá biểu thức trong wave 200 bước chưa đo.
- Có cần `else` dạng cú pháp (ngoài hai bước `when` đối lập) hay không: mặc định là không.
- `foreach` cần tập con của CR-WFD-003 (kiểu `json` mảng) đã xong.
