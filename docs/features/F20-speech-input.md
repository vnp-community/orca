# F20 — Speech Input

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F20 |
| **Tên** | Speech Input |
| **Ưu tiên** | P3 — Nice to Have |
| **Trạng thái** | 🔬 Experimental |
| **Tham chiếu PRD** | §3.10 (Speech Input) |
| **ADR References** | — |
| **HLD References** | C3.4 |

---

## Mô tả

Nhập liệu bằng giọng nói để gửi prompt cho agent — sử dụng Sherpa-ONNX cho speech-to-text offline, không cần kết nối internet hay gửi audio lên server.

---

## Vấn đề cần giải quyết

Đôi khi gõ bàn phím không tiện lợi (họp online, tay bận). Speech input cho phép người dùng nói chuyện trực tiếp với agent — offline, bảo mật, không có latency do network.

---

## Tính năng chi tiết

### Speech-to-Text Engine
- **Sherpa-ONNX**: offline ASR engine, runs on-device
- Không gửi audio ra ngoài — hoàn toàn offline
- Native binaries cho từng platform: darwin-arm64, darwin-x64, linux-arm64, linux-x64, win-x64

### Supported Languages
- English (mặc định)
- Có thể extend với model khác

### Input Modes
- **Push-to-talk**: giữ phím để record, thả để transcribe
- **Continuous**: record liên tục cho đến khi dừng

### Integration
- Transcribed text xuất hiện trong agent prompt box
- Người dùng review và chỉnh sửa trước khi gửi

### Privacy
- Audio không rời khỏi máy tính người dùng
- Không có API call cho speech recognition
- Model chạy hoàn toàn local

---

## Tiêu chí chấp nhận

- [ ] Speech recognition hoạt động offline
- [ ] WER (Word Error Rate) < 10% với giọng rõ ràng
- [ ] Transcription xuất hiện trong < 1 giây sau khi nói xong
- [ ] Không gửi audio ra internet

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Engine** | `sherpa-onnx` v1.12.37 |
| **Platforms** | darwin-arm64, darwin-x64, linux-arm64, linux-x64, win-x64 |
| **Speech types** | `src/shared/speech-types.ts` |
| **Main module** | `src/main/speech/` |

---

## Notes

- Đây là tính năng experimental (P3), không phải core feature
- Model size: ~100MB (cần download riêng)
- Chỉ hỗ trợ English tốt trong phiên bản đầu
