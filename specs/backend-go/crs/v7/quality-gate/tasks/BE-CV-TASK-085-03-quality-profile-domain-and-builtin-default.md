# BE-CV-TASK-085-03: Domain `QualityProfileDefinition`: validate chặt và profile dựng sẵn `orca-default`

**From Solution:** BE-CV-SOL-085-quality-gate-evaluator-and-profiles
**Priority:** P0
**Service:** `code-intel-service`
**File:** `internal/domain/quality_profile.go`, `quality_profile_builtin.go`, `quality_gate_errors.go` (mới)
**Depends on:** BE-CV-TASK-085-02
**Status:** [ ] TODO

## Context
Schema v1 và ràng buộc: CR-085 §2.1, hợp đồng ui-api §4.7 (`QualityProfile`). `orca-default`: CR §2.5 — **giá trị khởi điểm chưa hiệu chỉnh**.

## Việc cần làm
1. `DecodeProfileDefinition([]byte)`: `DisallowUnknownFields`, `json.Valid`, UTF-8, không `\u0000`, ≤ 64 KiB, `schemaVersion==1`, `checks ≤ 32`, `maxErrors/maxWarnings/maxFailed ∈ [0,10000]`, `diffCoverage* ∈ [0,1]` hoặc null, `category` ∈ enum `QualityFinding.category`, `name` khớp `^[a-z0-9-]{1,64}$`, `mode=block` ⇒ `CODEINTEL_INVALID_PARAMS` (`field=mode`, `reason=block_mode_not_enabled`).
2. `checks[].whenChangedPaths` (≤ 16 glob, không chứa `..`/tuyệt đối). Đề xuất L9: nếu không được duyệt thì bỏ trường và giữ nguyên logic bảng ca.
3. Lỗi `CODEINTEL_PROFILE_INVALID` kèm `data.field`.
4. `BuiltinOrcaDefault()` (lint, typecheck, unit luôn bắt buộc; go-vet/go-lint khi chạm `backend-go/**`; proto khi chạm `.proto`; repo-rules; structure; coverage tắt) với hằng ghi chú "uncalibrated".
5. `DefinitionDigest()` = sha256 JSON chuẩn hoá (khoá sắp).

## Kiểm thử
- Bảng ca hợp lệ/không hợp lệ (khoá lạ, ngưỡng âm, 33 checks, NUL, 65 KiB); fuzz ngắn `DecodeProfileDefinition`; digest ổn định khi đổi thứ tự khoá.

## Tiêu chí hoàn thành
- [ ] mọi ràng buộc CR có test; [ ] builtin qua chính `Validate`; [ ] không import adapter/proto-gen vào domain nếu tránh được.

## Rủi ro
- Tên `checks[].profile` phải khớp catalog của AG-CV-SOL-081 (chưa chốt); sai ⇒ `unknown` khi đánh giá.
