# BE-CV-TASK-037-01: Re-verify `structuralFacts`, git log, CODEOWNERS và dựng fixture phát hiện

**From Solution:** BE-CV-SOL-037-structure-findings-and-dismissals
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/testdata/findings/PROVENANCE.txt` (mới); `testdata/findings/structuralFacts-{layerImports,cycles,importInDegree,fileSizes,unusedExports}.json` (mới); `testdata/findings/git-log-90d.txt`, `git-shortlog.txt`, `CODEOWNERS.sample` (mới)
**Depends on:** `AG-CV-SOL-037-structural-facts` (tệp vàng G1 nếu có); BE-CV-SOL-030 (chữ ký `Log`)
**Status:** [ ] TODO

---

## Context

Số liệu của CR-037 (6 tệp vi phạm lớp, 93 vòng, 13/1 231 hàm chết, 886 commit/90 ngày) đo bằng CLI ngày 2026-10-05, chưa qua agent; hợp đồng agent §4.9 cũng ghi "đo một lần, chưa chạy lại". Task này khoá fixture trước khi viết phát hiện. Chỉ đọc.

## Việc cần làm

1. Đọc lại bốn tệp: `backend-go/services/infra-fleet-service/internal/usecase/{ports.go,get_terminal_agent_status.go}`, `backend-go/services/ai-provider-service/internal/usecase/{ports.go,test_connection.go}` (dòng import `internal/adapter/eventbus`), và ghi đúng/lệch vào PR. Kiểm `mcp-service/internal/usecase/usecasetest/harness.go` vẫn import `adapter/policyengine`.
2. `find . -iname 'CODEOWNERS*'` (loại `node_modules`): xác nhận vẫn không có; nếu có thay đổi thì ghi vào PR và cập nhật Q5 của solution.
3. Đọc `agent/src/relay/agent-git-handler.ts`: xác nhận `log`, `shortlog` nằm trong `ALLOWED_GIT_SUBCOMMANDS`, `SHELL_METACHARACTERS`, và chỗ đóng stdin của tiến trình con (`child.stdin?.end()`); ghi dòng số.
4. Fixture `structuralFacts-*.json` dựng đúng phong bì và `data` theo hợp đồng §4.9: `layerImports` có 3 hàng infra-fleet + 2 hàng ai-provider (cùng thư mục `adapter/eventbus`, nhiều tệp đích) + 1 hàng `usecasetest`; `cycles` 3 vòng (tệp đầu lặp ở cuối); `fileSizes`, `importInDegree` cho ≥ 8 tệp; `unusedExports` có `NewFleetDefinitionStore` hai bản + một hàm trong `_test.go` (để thử loại trừ). Nếu có tệp vàng G1 thì dùng, không tự dựng.
5. `git-log-90d.txt`: bản rút gọn đúng định dạng `--name-only` với dấu NUL và `%an`, gồm một commit chạm 150 tệp, tệp nhiễu (`pnpm-lock.yaml`, `i18n/locales/vi.json`), tên có dấu cách và Unicode dạng C-quote (`"\303\251.go"`). **Tác giả giả** (không email thật).
6. `CODEOWNERS.sample` có luật chồng, `**`, thư mục, dòng một cột, section GitLab `[Docs]`.
7. `PROVENANCE.txt` nêu nguồn/độ tin cậy từng fixture.

## Kiểm thử

- Không test Go. Kiểm tay: JSON hợp lệ; `grep -rn "@" testdata/findings/git-log-90d.txt` không có địa chỉ email thật.

## Tiêu chí hoàn thành

- [ ] Bảng đối chiếu đúng/lệch trong PR; `CODEOWNERS` vẫn vắng (hoặc cập nhật Q5).
- [ ] Mọi fixture có `PROVENANCE`; không dữ liệu cá nhân.

## Rủi ro và lưu ý

- Fixture tự dựng có thể lệch agent thật; thay bằng tệp vàng G1 khi có.
