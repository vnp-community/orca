# PP06 — Painpoints: DevOps Engineer

| Thuộc tính | Giá trị |
|-----------|---------|
| **Actor ID** | PP06 |
| **Actor** | DevOps Engineer |
| **Đại diện** | DevOps Engineer quản lý CI/CD và automation pipeline |
| **Quote** | *"Tôi cần tích hợp AI agent vào CI/CD pipeline nhưng không có CLI interface hay headless mode nào phù hợp."* |
| **Tham chiếu giải pháp** | [SOL06](../solutions/SOL06-devops-engineer.md) |

---

## Bối cảnh

DevOps Engineer chịu trách nhiệm tự động hóa toàn bộ SDLC — từ CI builds, automated testing, đến deployment. Khi team bắt đầu dùng AI agent trong development workflow, DevOps cần integrate agent vào pipeline nhưng không có tooling phù hợp.

---

## Danh sách Painpoints

### PP06-01: Không Có CLI / Headless Mode Cho CI/CD Integration

**Mức độ nghiêm trọng:** 🔴 Critical  
**Tần suất:** Mỗi khi cần automate  

**Mô tả:**
Orca là desktop GUI app, không thể chạy trong CI/CD environment (GitHub Actions, GitLab CI, Jenkins) vì không có display. Không có CLI interface để script agent workflows. DevOps không thể automate gì cả.

**Biểu hiện cụ thể:**
- `orca` binary không tồn tại hoặc không hoạt động headless
- Không có REST API để trigger agent từ pipeline
- Phải trigger agent thủ công → không scale
- CI/CD pipeline không thể wait for agent completion

---

### PP06-02: Không Có Metrics / Observability Cho Agent Runs

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** Hằng ngày  

**Mô tả:**
DevOps cần monitor agent runs như monitor service: success rate, duration, resource usage, failure reasons. Hiện tại không có metrics được expose — không biết agent runs có healthy không.

**Biểu hiện cụ thể:**
- Không có dashboard agent run metrics
- Không alert khi agent failure rate tăng cao
- Không biết average agent session duration để capacity plan
- Không có distributed tracing khi agent gọi nhiều services

---

### PP06-03: Không Thể Chạy Orca Trên Headless Linux Server

**Mức độ nghiêm trọng:** 🔴 Critical  
**Tần suất:** Mỗi khi deploy lên server  

**Mô tả:**
Server CI/CD thường là Linux không có display (headless). Electron app cần display để chạy. Không có cách chạy Orca trên headless Linux server mà không cần virtual display (Xvfb hack).

**Biểu hiện cụ thể:**
- `electron: error while loading shared libraries: libnss3.so`
- Phải dùng Xvfb workaround — fragile và tốn resource
- Docker container không có display → fail
- Không thể deploy Orca vào Kubernetes cluster

---

### PP06-04: Thiếu Retention và Cleanup Policy Cho Worktrees

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** Khi scale lên nhiều agent  

**Mô tả:**
Khi nhiều agent chạy tự động, số lượng worktrees tích lũy nhanh chóng. Không có policy tự động cleanup worktree cũ (theo age, status, hoặc disk space). Disk sẽ bị fill up.

**Biểu hiện cụ thể:**
- 50+ worktrees cũ tích lũy sau 1 tuần auto-run
- Disk usage tăng không kiểm soát
- Không có scheduled cleanup job
- Manual cleanup tốn thời gian và dễ xóa nhầm

---

### PP06-05: Không Có Access Control / Multi-user Support

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** Khi team scale  

**Mô tả:**
Orca là single-user desktop app. Khi nhiều developer cùng team muốn dùng shared agent infrastructure, không có cơ chế access control, quota per user, hay audit log cho AI usage.

**Biểu hiện cụ thể:**
- Không có user-level quota cho AI tokens
- Không có audit log ai dùng agent làm gì
- Không có role-based access (dev vs lead vs admin)
- Shared account cho team → không biết ai dùng bao nhiêu

---

## Tổng hợp Impact

| Painpoint | Mức độ | Impact | Tần suất |
|-----------|--------|--------|---------|
| PP06-01: Không có CLI/headless | 🔴 Critical | Không thể automate | Blocker |
| PP06-02: Không có observability | 🟠 High | Blind operation | Hằng ngày |
| PP06-03: Không chạy được headless Linux | 🔴 Critical | Không deploy được | Blocker |
| PP06-04: Thiếu cleanup policy | 🟡 Medium | Disk fill up | 1-2 tuần |
| PP06-05: Không có access control | 🟡 Medium | Governance issue | Khi scale |

---

*Tham chiếu: URD §2.2 (DevOps Engineer), PRD §3.8 (Orca CLI), SRS FR-9.2*
