# BL-CR-04 — Tạo Commit Message bằng AI

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-CR-04 |
| **Tên** | Tạo Commit Message bằng AI |
| **Nhóm** | Code Review |
| **Actors** | Alex, Maya |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F06 GitHub & Linear Integration |
| **SRS** | FR-6.3 |

---

## Mô tả nghiệp vụ

Tự động generate commit message chất lượng cao từ staged changes, tuân thủ Conventional Commits format và project convention.

---

## Luồng chính

```
1. Người dùng click "Commit" hoặc "Generate Message"
2. Hệ thống thu thập:
   a. git diff --staged (staged changes)
   b. git log --oneline -5 (recent commits, cho context)
   c. Current branch name
   d. Issue/ticket number từ branch name (nếu có)
3. Build AI prompt:
   - Staged diff + file stats
   - Recent commit style (để match convention)
   - Branch context
4. Stream AI response vào commit message field
5. Người dùng review và chỉnh sửa
6. Confirm → git commit
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-CR-13 | Commit message phải theo Conventional Commits nếu project dùng |
| BR-CR-14 | Không commit tự động — người dùng phải confirm |
| BR-CR-15 | Nếu diff quá lớn (> 50 files): chỉ dùng file stats, không full diff |
| BR-CR-16 | Issue/ticket ID từ branch name phải được auto-included |

---

## Output Format

```
fix(auth): handle null user before accessing properties

Prevents NullReferenceException when user session expires during
multi-step authentication flow.

- Add null check in validateUser()
- Update unit tests for edge case

Refs: #123
```
