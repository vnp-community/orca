# BE-CV-TASK-036-01: Re-verify hợp đồng agent `detectChanges`/`impact` và dựng fixture overlay

**From Solution:** BE-CV-SOL-036-change-overlay-pipeline
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/testdata/change-overlay/PROVENANCE.txt` (mới); `testdata/change-overlay/detect-changes-{clean,dirty,untracked,renamed,unborn,drifted,3000-files}.json` (mới); `testdata/change-overlay/impact-{low,high,ambiguous,timeout}.json` (mới)
**Depends on:** `AG-CV-SOL-070-golden-fixtures-and-parsers` (tệp vàng G1 `testdata/agent-results/*.json`); nếu chưa có, dùng bản tay dựng theo `CONTRACT-codeintel-agent-rpc.md` §4.5, §4.8 và ghi "tự dựng"
**Status:** [x] DONE

---

## Context

Solution 036 dựa vào hình dạng `detectChanges`/`impact` do hợp đồng mô tả; hợp đồng tự ghi mẫu Cypher và ngữ nghĩa `head` vắng "chưa chạy/chưa kiểm chứng". Bước này khoá fixture trước khi viết ánh xạ. Chỉ đọc; không chạy `gitnexus`.

## Việc cần làm

1. Đọc lại `agent/src/relay/git-handler-ops.ts` (`branchCompare`) và `agent/src/relay/agent-git-handler.ts` (`ALLOWED_GIT_SUBCOMMANDS`, `SHELL_METACHARACTERS`) và xác nhận vẫn không có `merge-base`/`rev-list`; ghi kết quả vào PR.
2. Kiểm `agent/src/relay/` đã có `codeintel-detect-changes.ts` hay chưa (`ls`). Chưa có ⇒ ghi "agent chưa triển khai; backend dùng tệp vàng".
3. Nếu tệp vàng G1 đã có: chép sang `testdata/change-overlay/` (không sửa tay). Nếu chưa: dựng bảy JSON `detectChanges` đúng phong bì §2.2 (`sources, headCommit, stale, truncated, totalCount, warnings, data`) và bốn JSON `impact` (`levels[0].symbols[].direct`, `via:"calls"`, `testsCovering`); ca `3000-files` sinh bằng tập lệnh Go nhỏ trong test (không commit tệp lớn): chỉ ghi tham số sinh vào `PROVENANCE.txt`.
4. Mỗi fixture có chú thích trong `PROVENANCE.txt`: nguồn (agent thật / tự dựng), commit hợp đồng, các trường cố ý đặt (ví dụ `driftedFromIndex:true`, `status:"U"`, `warnings:["unborn_head"]`).
5. Xác nhận chữ ký `RepoSourceReader` của `BE-CV-SOL-030` có `Log(ctx, RepoRef, LogQuery)` dùng được cho `commitsBehind` (xem TASK-036-04); ghi tên kiểu `LogQuery` thật.

## Kiểm thử

- Không có test Go. Kiểm tay: mỗi JSON parse được (`python3 -m json.tool`), và không chứa mã nguồn thật ngoài `SymbolRef` (quét `grep -c "func " testdata/change-overlay/*.json` = 0).

## Tiêu chí hoàn thành

- [x] 11 fixture + `PROVENANCE.txt` đủ ca của tiêu chí chấp nhận (§9 solution).
- [x] Ghi rõ fixture nào tự dựng.
- [x] Chữ ký `Log` đã ghi lại.

## Rủi ro và lưu ý

- Fixture tự dựng có thể lệch agent thật; khi tệp vàng G1 có, thay và chạy lại test (task 036-03, 036-05).
