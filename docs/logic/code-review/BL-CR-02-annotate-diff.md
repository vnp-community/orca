# BL-CR-02 — Annotate Dòng Code trong Diff

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-CR-02 |
| **Tên** | Annotate Dòng Code trong Diff |
| **Nhóm** | Code Review |
| **Actors** | Maya (Tech Lead), Alex |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F08 Annotate AI Diffs |
| **SRS** | FR-6.2 |

---

## Mô tả nghiệp vụ

Người dùng click vào bất kỳ dòng code nào trong diff viewer để thêm comment inline — comment sau đó được gửi về agent với đầy đủ context.

---

## Luồng chính

```
1. Người dùng đang xem diff (BL-CR-01)
2. Click vào line number bất kỳ
3. Comment box mở ngay dưới dòng đó
4. Người dùng nhập comment (hỗ trợ markdown)
5. Click "Add Comment" → comment được lưu vào review buffer
6. Comment hiển thị inline trong diff với indicator
7. Người dùng tiếp tục review và thêm comment khác
8. Khi xong: "Send all to Agent" (BL-CR-03)
```

---

## Comment Data Structure

```typescript
interface DiffComment {
  file: string;          // "src/auth.ts"
  line: number;          // 42
  side: 'old' | 'new';   // dòng cũ hay mới
  originalCode: string;  // nội dung dòng đó
  comment: string;       // comment của reviewer
  timestamp: Date;
}
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-CR-05 | Comment có thể attach vào cả dòng cũ và dòng mới |
| BR-CR-06 | Multi-line selection: comment attach vào range |
| BR-CR-07 | Comment không bị mất khi scroll hoặc switch file |
| BR-CR-08 | Xóa comment phải có confirm nếu comment đã được gửi về agent |
