# SOLUTION-FE-BIGFILE-006 — Tách `pty-connection.ts` (7,600 dòng)

**Bug:** `../BUG-FE-BIGFILE-006-pty-connection.md`
**Trạng thái:** 📝 Proposed
**Thứ tự thực hiện:** #9 (gần cuối — xem lý do trong `SOLUTION-FE-BIGFILE-001`
mục 3: không có ranh giới export sẵn + vừa dính bug race-condition gần đây)
**Chiến lược:** Barrel/facade cho phần đã tách; **thiết kế lại nội bộ** cho
`connectPanePty` (không có shortcut barrel đơn giản vì chỉ có 1 export)

---

## Khác biệt so với các file khác trong đợt này

`pty-connection.ts` chỉ có 2 export: `STARTUP_CWD_FALLBACK_NOTICE` (const,
dòng 250) và `connectPanePty` (dòng 943 → hết file, ~6,650 dòng cho 1 hàm).
**Không có ranh giới sẵn** như `BrowserPane.tsx`/`TaskPage.tsx` — không thể
chỉ "di chuyển nguyên khối theo export có sẵn". Cần đọc + thiết kế nội bộ
hàm trước khi tách được.

## Bước 0 — Bắt buộc trước khi tách bất kỳ dòng nào: tăng test coverage

File này vừa là trung tâm của investigation `BUG-FE-PTY-001` (race condition
giữa local tab và host-session mirror tab, dẫn tới PTY bị đóng nhầm — xem
memory session `bug-fe-pty-001-investigation.md`). Việc tách 1 hàm 6,650 dòng
KHÔNG có test che phủ tốt có rủi ro regression cao hơn nhiều so với các file
khác trong đợt này.

**Trước khi tách**, xác nhận/bổ sung test cho tối thiểu:
1. Luồng spawn mới (local/SSH/remote-runtime) thành công.
2. Luồng retry khi `SSH_SESSION_EXPIRED` (đã fix trong investigation gần
   đây — đảm bảo có test regression cho fix đó nếu tách vô tình đổi hành vi).
3. Luồng reattach/daemon reattach.
4. Race giữa local pane và host-session mirror pane cho cùng 1 leaf (nếu
   logic liên quan còn nằm trong file này — cần xác nhận khi đọc).

## Bước 1 — Đọc và phân vùng nội bộ `connectPanePty`

Không đoán trước cấu trúc chi tiết trong solution doc này (rủi ro đưa ra kế
hoạch sai) — thực hiện đọc trực tiếp toàn bộ hàm, xác định các nhánh theo
loại transport. Dựa trên tên các transport class đã biết trong cùng thư mục
(`remote-runtime-pty-transport.ts` — xem `BUG-FE-BIGFILE-001` bảng #99), khả
năng cao `connectPanePty` có ít nhất 3–4 nhánh lớn:

- Local PTY connect (không remote)
- SSH PTY connect
- Remote-runtime PTY connect (devServer/host-mirrored — liên quan trực tiếp
  investigation gần đây)
- Retry/cold-start/reattach logic dùng chung giữa các nhánh trên

## Bước 2 — Thiết kế mục tiêu (strategy pattern, KHÔNG barrel đơn giản)

```ts
// pty-connection.ts (SAU khi tách) — hàm điều phối ngắn gọn
export { STARTUP_CWD_FALLBACK_NOTICE } from './pty-connection-shared'

export async function connectPanePty(opts: ConnectPanePtyOptions): Promise<...> {
  const strategy = resolveConnectStrategy(opts) // local | ssh | remote-runtime
  return strategy.connect(opts)
}
```

```ts
// pty-connection-local.ts
export const localConnectStrategy: ConnectPtyStrategy = { connect: async (opts) => { /* ... */ } }

// pty-connection-ssh.ts
export const sshConnectStrategy: ConnectPtyStrategy = { connect: async (opts) => { /* ... */ } }

// pty-connection-remote-runtime.ts
export const remoteRuntimeConnectStrategy: ConnectPtyStrategy = { connect: async (opts) => { /* ... */ } }

// pty-connection-shared.ts
export const STARTUP_CWD_FALLBACK_NOTICE = /* ... */
export type ConnectPtyStrategy = { connect(opts: ConnectPanePtyOptions): Promise<...> }
// + các helper/retry logic dùng chung giữa 3 strategy trên
```

**Lưu ý:** đây là thiết kế MỤC TIÊU, không phải bước thực hiện máy móc như
các file khác — người thực hiện cần điều chỉnh theo cấu trúc thực tế phát
hiện ở Bước 1. Nếu 3 nhánh transport chia sẻ quá nhiều state/closure để tách
sạch thành strategy riêng, cân nhắc phương án nhẹ hơn: giữ `connectPanePty`
là 1 hàm, nhưng trích các khối logic lớn (vd toàn bộ nhánh "remote-runtime
connect", ~2,000+ dòng ước tính) thành 1 hàm con `connectRemoteRuntimePty()`
export riêng, gọi từ `connectPanePty` — vẫn giảm đáng kể độ phức tạp đọc dù
không tách hoàn toàn theo strategy pattern.

## Bước 3 — Tách từng nhánh, xác nhận xanh sau MỖI nhánh

Không tách 3+ nhánh trong 1 commit. Thứ tự đề xuất: nhánh ít rủi ro nhất
trước (local) → SSH → remote-runtime (rủi ro cao nhất, vừa có bug gần đây) →
cuối cùng mới rút gọn `connectPanePty` thành hàm điều phối.

## Xác minh (sau MỖI nhánh, không chỉ cuối cùng)

- `pnpm run typecheck`, `pnpm run lint`
- Toàn bộ test đã bổ sung ở Bước 0 + test hiện có
- `gitnexus impact({target: "connectPanePty", direction: "upstream"})` +
  `gitnexus detect_changes({scope: "all"})`
- **Test thủ công trên môi trường thật nếu có thể** (tương tự cách
  investigation `BUG-FE-PTY-001` đã làm — deploy lên môi trường dev, thử tạo
  terminal devServer thật) — vì đây là logic đã có lịch sử race-condition
  khó phát hiện chỉ bằng unit test.
- `node scripts/find-frontend-bigfiles.mjs`

## Rủi ro

**Cao nhất trong toàn bộ 10 file** — không phải vì độ lớn (7,600 dòng, đứng
thứ 5) mà vì: (1) không có ranh giới export sẵn, cần thiết kế lại thay vì chỉ
di chuyển, (2) đã có lịch sử bug race-condition nghiêm trọng, khó phát hiện
qua test thông thường (cần test trên môi trường thật với timing thực). Đây là
lý do đặt thứ tự #9 (gần cuối) — chỉ nên làm sau khi đã có kinh nghiệm từ các
file rủi ro thấp hơn.
