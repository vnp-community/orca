# Pilot metrics for Request 7ef3c8e7-e066-44cc-83b2-0742b14f054f

## Analysis and plan runs

| run | kind | engine | status | seconds | turns | tool calls | KG calls | cost USD |
|---|---|---|---|---|---|---|---|---|
| 4d0fc2d9 | solution | native | succeeded | 475 | 62 | 61 | 19 | 3.25 |
| 505348b8 | plan | openspec | succeeded | 982 | 149 | 148 | 39 | 10.37 |

Total reported by the agent for these runs: **$13.61**. Task runs report no cost yet.

## Tasks

| phase | task | status | attempts | failures |
|---|---|---|---|---|
| `case_ai_config` sang MySQL với ba | Viết test cấu hình: mặc định, lưu/đọc lại, từ chối theo từng | done | 2 | retryable |
| `case_ai_config` sang MySQL với ba | Thay `domain.AIConfig` bằng `CaseAiChatConfig{designConfig,  | done | 1 |  |
| `case_ai_config` sang MySQL với ba | Thêm repository MySQL đọc/ghi `case_ai_config` theo khoá `(u | done | 1 |  |
| `case_ai_config` sang MySQL với ba | Viết lại `GetConfig`/`SaveConfig` và handler theo hợp đồng U | done | 1 |  |
| `case_ai_config` sang MySQL với ba | Thêm migration `V025__case_ai_config.sql` và khai báo bảng ` | done | 2 | retryable |
| Cổng AI có bản fake và chat trả ch | Viết test chat: marker đúng, không marker, cờ tắt, cấu hình  | done | 1 |  |
| Cổng AI có bản fake và chat trả ch | Thêm cờ `AI_ENABLED` vào config và cổng 503 cho nhóm route ` | done | 1 |  |
| Cổng AI có bản fake và chat trả ch | Thêm `FakeGate` trả chuỗi/JSON dựng sẵn (marker đúng, thiếu  | done | 1 |  |
| Cổng AI có bản fake và chat trả ch | Viết lại `Chat` trả chuỗi markdown theo hợp đồng UI (bind `{ | done | 1 |  |
| Cổng AI có bản fake và chat trả ch | Khai báo interface `Gate` (`Chat`, `ResolveModel`, `AppendTu | done | 1 |  |
| Thư viện prompt, marker và schema  | Thêm `CaseTransformSchema` và `ValidateTransform` (tên ≤ 255 | done | 1 |  |
| Thư viện prompt, marker và schema  | Thêm `ParseCaseBlocks(s) []Block{Title,Body}` chịu được khoả | done | 1 |  |
| Thư viện prompt, marker và schema  | Viết test bảng cho `aiprompt` (prompt, tách khối, kiểm tra s | done | 1 |  |
| Thư viện prompt, marker và schema  | Thêm package `aiprompt` với hằng marker và `CaseChatSystemPr | done | 1 |  |
| Nghiệm thu đầu-cuối với mock-ext v | Cập nhật tài liệu tính năng F24 và dòng FR-SYS-07 của SRS ch | open | 2 | needs_info, needs_info |
| Nghiệm thu đầu-cuối với mock-ext v | Thêm ca e2e drawer AI đến Transform đến lưu vào module, rồi  | blocked | 0 |  |
| Nghiệm thu đầu-cuối với mock-ext v | Thêm test tích hợp `test_f3_case_ai.py` với mock-ext: chat ≥ | open | 0 |  |
| Lưu hàng loạt: giới hạn vào cổng,  | Ghi chi tiết thay đổi cho từng case bằng `audit.RecordAdded` | done | 1 |  |
| Lưu hàng loạt: giới hạn vào cổng,  | Viết test lưu hàng loạt: 21 khối ⇒ 400 và số case không đổi, | done | 1 |  |
| Lưu hàng loạt: giới hạn vào cổng,  | Thêm `AddMany`: một `ev.Do`, một `AllocNums(projectID, n)`,  | done | 1 |  |
| Lưu hàng loạt: giới hạn vào cổng,  | Thêm hàm repository tạo lô case + blob + custom values bằng  | done | 1 |  |
| Lưu hàng loạt: giới hạn vào cổng,  | Viết lại `BatchSave`: transform các khối (song song ≤ 4) ngo | done | 1 |  |
| Lưu hàng loạt: giới hạn vào cổng,  | Chặn > 20 khối hoặc > 100 case bằng 400 ngay sau `ParseCaseB | done | 1 |  |
| `Transform` theo JSON schema với đ | Viết test transform: JSON hợp lệ, hỏng rồi đúng, hỏng cả hai | done | 1 |  |
| `Transform` theo JSON schema với đ | Thử lại đúng một lần kèm chỉ dẫn "chỉ trả JSON", vẫn sai thì | done | 1 |  |
| `Transform` theo JSON schema với đ | Gọi JSON mode theo `CaseTransformSchema` và map sang `AiCase | done | 1 |  |

23 of 26 task(s) done or in review; 4 failed attempt(s) recorded.