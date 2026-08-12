# SOLUTION-FE-BIGFILE-007 — `PullRequestPage.tsx`

**Bug:** `../BUG-FE-BIGFILE-007-pullrequestpage.md`
**Trạng thái:** 📝 Proposed (xem solution chính)

---

Solution cho file này được gộp chung với `GitHubItemDialog.tsx` — xem
**`SOLUTION-FE-BIGFILE-005-githubitemdialog-and-pullrequestpage.md`** (cùng
thư mục) để có kế hoạch đầy đủ.

Lý do gộp: `PullRequestPage.tsx` tự nhận trong comment dòng 1 là "duplicated
from GitHubItemDialog" — xử lý riêng lẻ sẽ nhân đôi công sức tách logic giống
nhau ở 2 nơi. Xem `SOLUTION-FE-BIGFILE-001` mục 4 để biết lý do đặt 2 bug này
xử lý cùng nhau trong trình tự chung.
