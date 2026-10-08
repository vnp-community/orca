# RPC-CATALOG: RPC của `request-service` (proto-first)

> Cập nhật 2026-10-08. Nguồn: `backend-go/proto/orca/request/v1/*.proto` (package `orca.request.v1`). Tổng 89 RPC trong 4 service. Agent triển khai chỉ **điền handler** (server đã nhúng `Unimplemented*Server`); không sửa `.proto`. Cần bổ sung thì ghi vào `IMPLEMENTATION-NOTES.md` và báo người điều phối (IMPLEMENTATION-PLAN mục 6).

## Quy ước
- Mỗi RPC có request/response riêng (`buf lint --path orca/request` sạch). Chỉ thêm, không đổi số trường.
- `RequestService` gom mọi RPC miền Request để bảng `RpcCatalog` của CR-REQ-035 chỉ có ba tiền tố; `ApprovalPolicyAdminService` tách theo CR-REQ-010 (nhóm `admin`, cần thêm vào catalog).
- Message cần `Request` nằm trong `request.proto`; file miền (`solution`, `plan`, `clarification`, `decision`, `artifact`, `impact`, `execution_readiness`, `context`, `compliance`) không import `request.proto` (tránh vòng import), nên trạng thái Request trả về dạng chuỗi.
- Bất đồng bộ (D3): `ClassifyRequest` trả `run_id`; `GeneratePlan` (PROPOSE) trả `run_id`, kết quả đọc bằng `GetPlanProposal`; `GenerateSolution` trả `run_id`, đọc bằng `ListSolutions.runs`.
- Nội bộ (không qua gateway): `ReportTaskOutcome`, `LookupRequestBySource` (bọc `internalcaller.Guard`).

## Bảng RPC

| Service | RPC | Request | Response | CR sở hữu | Đợt | Trạng thái |
|---|---|---|---|---|---|---|
| RequestService | `GetRequest` | `GetRequestRequest` | `GetRequestResponse` | CR-REQ-001/002 | R1a | Real (R1a) |
| RequestService | `ListRequests` | `ListRequestsRequest` | `ListRequestsResponse` | CR-REQ-001/002 | R1a | Real (R1a) |
| RequestService | `ListBacklog` | `ListBacklogRequest` | `ListBacklogResponse` | CR-REQ-015 | R3 | Unimplemented |
| RequestService | `CreateRequest` | `CreateRequestRequest` | `CreateRequestResponse` | CR-REQ-004 | R1b | Unimplemented |
| RequestService | `ClassifyRequest` | `ClassifyRequestRequest` | `ClassifyRequestResponse` | CR-REQ-005 | R1b | Unimplemented |
| RequestService | `ConfirmRequestType` | `ConfirmRequestTypeRequest` | `ConfirmRequestTypeResponse` | CR-REQ-005 | R1b | Unimplemented |
| RequestService | `ChangeRequestType` | `ChangeRequestTypeRequest` | `ChangeRequestTypeResponse` | CR-REQ-005 | R1b | Unimplemented |
| RequestService | `ListRequestTypeHistory` | `ListRequestTypeHistoryRequest` | `ListRequestTypeHistoryResponse` | CR-REQ-005 | R1b | Unimplemented |
| RequestService | `ReturnToBacklog` | `ReturnToBacklogRequest` | `ReturnToBacklogResponse` | CR-REQ-006 | R1b | Unimplemented |
| RequestService | `ReopenRequest` | `ReopenRequestRequest` | `ReopenRequestResponse` | CR-REQ-006 | R1b | Unimplemented |
| RequestService | `CancelRequest` | `CancelRequestRequest` | `CancelRequestResponse` | CR-REQ-006 | R1b | Unimplemented |
| RequestService | `SpawnChildRequest` | `SpawnChildRequestRequest` | `SpawnChildRequestResponse` | CR-REQ-006 | R1b | Unimplemented |
| RequestService | `ListRequestLinks` | `ListRequestLinksRequest` | `ListRequestLinksResponse` | CR-REQ-006 | R1b | Unimplemented |
| RequestService | `GetRequestFlow` | `GetRequestFlowRequest` | `GetRequestFlowResponse` | CR-REQ-003 | R1b | Unimplemented |
| RequestService | `LookupRequestBySource` | `LookupRequestBySourceRequest` | `LookupRequestBySourceResponse` | CR-REQ-024 | R5 | Unimplemented |
| RequestService | `GetRequestFlowSettings` | `GetRequestFlowSettingsRequest` | `GetRequestFlowSettingsResponse` | CR-REQ-025 | R5 | Unimplemented |
| RequestService | `SetRequestFlowSettings` | `SetRequestFlowSettingsRequest` | `SetRequestFlowSettingsResponse` | CR-REQ-025 | R5 | Unimplemented |
| RequestService | `GenerateSolution` | `GenerateSolutionRequest` | `GenerateSolutionResponse` | CR-REQ-007 | R2 | Unimplemented |
| RequestService | `ListSolutions` | `ListSolutionsRequest` | `ListSolutionsResponse` | CR-REQ-007 | R2 | Unimplemented |
| RequestService | `ChooseSolutionOption` | `ChooseSolutionOptionRequest` | `ChooseSolutionOptionResponse` | CR-REQ-007 | R2 | Unimplemented |
| RequestService | `GetProjectEngineSettings` | `GetProjectEngineSettingsRequest` | `GetProjectEngineSettingsResponse` | CR-REQ-026 | R2 | Unimplemented |
| RequestService | `SetProjectEngineSettings` | `SetProjectEngineSettingsRequest` | `SetProjectEngineSettingsResponse` | CR-REQ-026 | R2 | Unimplemented |
| RequestService | `GeneratePlan` | `GeneratePlanRequest` | `GeneratePlanResponse` | CR-REQ-012 | R3 | Unimplemented |
| RequestService | `GetPlanProposal` | `GetPlanProposalRequest` | `GetPlanProposalResponse` | CR-REQ-012 | R3 | Unimplemented |
| RequestService | `CommitPlan` | `CommitPlanRequest` | `CommitPlanResponse` | CR-REQ-012 | R3 | Unimplemented |
| RequestService | `StartPhase` | `StartPhaseRequest` | `StartPhaseResponse` | CR-REQ-013 | R3 | Unimplemented |
| RequestService | `ReportTaskOutcome` | `ReportTaskOutcomeRequest` | `ReportTaskOutcomeResponse` | CR-REQ-013 | R3 | Unimplemented |
| RequestService | `RecordRequestCheck` | `RecordRequestCheckRequest` | `RecordRequestCheckResponse` | CR-REQ-014 | R3 | Unimplemented |
| RequestService | `ListRequestChecks` | `ListRequestChecksRequest` | `ListRequestChecksResponse` | CR-REQ-014 | R3 | Unimplemented |
| RequestService | `GetRequestReadiness` | `GetRequestReadinessRequest` | `GetRequestReadinessResponse` | CR-REQ-028 | R4 | Unimplemented |
| RequestService | `RequestClarification` | `RequestClarificationRequest` | `RequestClarificationResponse` | CR-REQ-028 | R4 | Unimplemented |
| RequestService | `ListClarifications` | `ListClarificationsRequest` | `ListClarificationsResponse` | CR-REQ-028 | R4 | Unimplemented |
| RequestService | `GetClarification` | `GetClarificationRequest` | `GetClarificationResponse` | CR-REQ-028 | R4 | Unimplemented |
| RequestService | `AnswerClarification` | `AnswerClarificationRequest` | `AnswerClarificationResponse` | CR-REQ-028 | R4 | Unimplemented |
| RequestService | `CancelClarification` | `CancelClarificationRequest` | `CancelClarificationResponse` | CR-REQ-028 | R4 | Unimplemented |
| RequestService | `ListPendingClarificationsForUser` | `ListPendingClarificationsForUserRequest` | `ListPendingClarificationsForUserResponse` | CR-REQ-028 | R4 | Unimplemented |
| RequestService | `WaiveReadiness` | `WaiveReadinessRequest` | `WaiveReadinessResponse` | CR-REQ-028 | R4 | Unimplemented |
| RequestService | `ListDecisions` | `ListDecisionsRequest` | `ListDecisionsResponse` | CR-REQ-028 | R4 | Unimplemented |
| RequestService | `GetDecision` | `GetDecisionRequest` | `GetDecisionResponse` | CR-REQ-028 | R4 | Unimplemented |
| RequestService | `ConfirmDecision` | `ConfirmDecisionRequest` | `ConfirmDecisionResponse` | CR-REQ-028 | R4 | Unimplemented |
| RequestService | `EditRequestContent` | `EditRequestContentRequest` | `EditRequestContentResponse` | CR-REQ-027 | R4 | Unimplemented |
| RequestService | `ListRequestRevisions` | `ListRequestRevisionsRequest` | `ListRequestRevisionsResponse` | CR-REQ-027 | R4 | Unimplemented |
| RequestService | `GetRequestRevision` | `GetRequestRevisionRequest` | `GetRequestRevisionResponse` | CR-REQ-027 | R4 | Unimplemented |
| RequestService | `GetRequestCoverage` | `GetRequestCoverageRequest` | `GetRequestCoverageResponse` | CR-REQ-027 | R4 | Unimplemented |
| RequestService | `GetArtifactGraph` | `GetArtifactGraphRequest` | `GetArtifactGraphResponse` | CR-REQ-027 | R4 | Unimplemented |
| RequestService | `ExportArtifactProjection` | `ExportArtifactProjectionRequest` | `ExportArtifactProjectionResponse` | CR-REQ-027 | R4 | Unimplemented |
| RequestService | `ResolveArtifactRef` | `ResolveArtifactRefRequest` | `ResolveArtifactRefResponse` | CR-REQ-027 | R4 | Unimplemented |
| RequestService | `RequestImpactAssessment` | `RequestImpactAssessmentRequest` | `RequestImpactAssessmentResponse` | CR-REQ-030 | R4 | Unimplemented |
| RequestService | `GetImpactAssessment` | `GetImpactAssessmentRequest` | `GetImpactAssessmentResponse` | CR-REQ-030 | R4 | Unimplemented |
| RequestService | `GetImpactGraph` | `GetImpactGraphRequest` | `GetImpactGraphResponse` | CR-REQ-030 | R4 | Unimplemented |
| RequestService | `CompareImpact` | `CompareImpactRequest` | `CompareImpactResponse` | CR-REQ-030 | R4 | Unimplemented |
| RequestService | `ListImpactFindings` | `ListImpactFindingsRequest` | `ListImpactFindingsResponse` | CR-REQ-030 | R4 | Unimplemented |
| RequestService | `GetImpactEvidence` | `GetImpactEvidenceRequest` | `GetImpactEvidenceResponse` | CR-REQ-030 | R4 | Unimplemented |
| RequestService | `GetPlanRiskHeatmap` | `GetPlanRiskHeatmapRequest` | `GetPlanRiskHeatmapResponse` | CR-REQ-030 | R4 | Unimplemented |
| RequestService | `GetImpactDrift` | `GetImpactDriftRequest` | `GetImpactDriftResponse` | CR-REQ-030 | R4 | Unimplemented |
| RequestService | `AcceptRisk` | `AcceptRiskRequest` | `AcceptRiskResponse` | CR-REQ-030 | R4 | Unimplemented |
| RequestService | `OverrideRiskGate` | `OverrideRiskGateRequest` | `OverrideRiskGateResponse` | CR-REQ-030 | R4 | Unimplemented |
| RequestService | `GetRiskPolicy` | `GetRiskPolicyRequest` | `GetRiskPolicyResponse` | CR-REQ-030 | R4 | Unimplemented |
| RequestService | `SetRiskPolicy` | `SetRiskPolicyRequest` | `SetRiskPolicyResponse` | CR-REQ-030 | R4 | Unimplemented |
| RequestService | `CheckReadiness` | `CheckReadinessRequest` | `CheckReadinessResponse` | CR-REQ-029 | R4 | Unimplemented |
| RequestService | `GetReadinessReport` | `GetReadinessReportRequest` | `GetReadinessReportResponse` | CR-REQ-029 | R4 | Unimplemented |
| RequestService | `ListReadiness` | `ListReadinessRequest` | `ListReadinessResponse` | CR-REQ-029 | R4 | Unimplemented |
| RequestService | `ListContextSources` | `ListContextSourcesRequest` | `ListContextSourcesResponse` | CR-REQ-031 | R6 | Unimplemented |
| RequestService | `UpsertContextSource` | `UpsertContextSourceRequest` | `UpsertContextSourceResponse` | CR-REQ-031 | R6 | Unimplemented |
| RequestService | `SetContextSourceStatus` | `SetContextSourceStatusRequest` | `SetContextSourceStatusResponse` | CR-REQ-031 | R6 | Unimplemented |
| RequestService | `PreviewContextPack` | `PreviewContextPackRequest` | `PreviewContextPackResponse` | CR-REQ-031 | R6 | Unimplemented |
| RequestService | `GetEvidence` | `GetEvidenceRequest` | `GetEvidenceResponse` | CR-REQ-031 | R6 | Unimplemented |
| RequestService | `ListEvidence` | `ListEvidenceRequest` | `ListEvidenceResponse` | CR-REQ-031 | R6 | Unimplemented |
| RequestService | `EraseRequest` | `EraseRequestRequest` | `EraseRequestResponse` | CR-REQ-035 | R6 | Unimplemented |
| RequestService | `ExportRequest` | `ExportRequestRequest` | `ExportRequestResponse` | CR-REQ-035 | R6 | Unimplemented |
| RequestService | `ExportTenantRequests` | `ExportTenantRequestsRequest` | `stream ExportTenantRequestsResponse` | CR-REQ-035 | R6 | Unimplemented |
| ApprovalService | `RequestApproval` | `RequestApprovalRequest` | `RequestApprovalResponse` | CR-REQ-009 | R2 | Unimplemented |
| ApprovalService | `Approve` | `ApproveRequest` | `ApproveResponse` | CR-REQ-009 | R2 | Unimplemented |
| ApprovalService | `Reject` | `RejectRequest` | `RejectResponse` | CR-REQ-009 | R2 | Unimplemented |
| ApprovalService | `Cancel` | `ApprovalServiceCancelRequest` | `ApprovalServiceCancelResponse` | CR-REQ-009 | R2 | Unimplemented |
| ApprovalService | `GetApproval` | `GetApprovalRequest` | `GetApprovalResponse` | CR-REQ-009 | R2 | Unimplemented |
| ApprovalService | `ListApprovals` | `ListApprovalsRequest` | `ListApprovalsResponse` | CR-REQ-009 | R2 | Unimplemented |
| ApprovalService | `ListPendingForUser` | `ListPendingForUserRequest` | `ListPendingForUserResponse` | CR-REQ-009 | R2 | Unimplemented |
| ApprovalService | `ExtendApproval` | `ExtendApprovalRequest` | `ExtendApprovalResponse` | CR-REQ-010 | R2 | Unimplemented |
| ApprovalPolicyAdminService | `ListApprovalPolicies` | `ListApprovalPoliciesRequest` | `ListApprovalPoliciesResponse` | CR-REQ-010 | R2 | Unimplemented |
| ApprovalPolicyAdminService | `UpsertApprovalPolicy` | `UpsertApprovalPolicyRequest` | `UpsertApprovalPolicyResponse` | CR-REQ-010 | R2 | Unimplemented |
| ApprovalPolicyAdminService | `DeleteApprovalPolicy` | `DeleteApprovalPolicyRequest` | `DeleteApprovalPolicyResponse` | CR-REQ-010 | R2 | Unimplemented |
| AiBudgetAdminService | `ListAiBudgets` | `ListAiBudgetsRequest` | `ListAiBudgetsResponse` | CR-REQ-034 | R6 | Unimplemented |
| AiBudgetAdminService | `UpsertAiBudget` | `UpsertAiBudgetRequest` | `UpsertAiBudgetResponse` | CR-REQ-034 | R6 | Unimplemented |
| AiBudgetAdminService | `DeleteAiBudget` | `DeleteAiBudgetRequest` | `DeleteAiBudgetResponse` | CR-REQ-034 | R6 | Unimplemented |
| AiBudgetAdminService | `ListAiStepPolicies` | `ListAiStepPoliciesRequest` | `ListAiStepPoliciesResponse` | CR-REQ-034 | R6 | Unimplemented |
| AiBudgetAdminService | `UpsertAiStepPolicy` | `UpsertAiStepPolicyRequest` | `UpsertAiStepPolicyResponse` | CR-REQ-034 | R6 | Unimplemented |
| AiBudgetAdminService | `GetAiTenantSettings` | `GetAiTenantSettingsRequest` | `GetAiTenantSettingsResponse` | CR-REQ-034 | R6 | Unimplemented |
| AiBudgetAdminService | `SetAiTenantSettings` | `SetAiTenantSettingsRequest` | `SetAiTenantSettingsResponse` | CR-REQ-034 | R6 | Unimplemented |

`ListBacklog` có handler trả `Unimplemented` có chủ đích (test `TestServer_ListBacklogStaysUnimplemented`), CR-REQ-015 thay.

## Ngoài `orca.request.v1` (không thuộc proto-first này)
`task-service`: `CreatePlanTree` (012), `ListExecutionStates` (015), `SetTaskSpec`/`GetTaskSpecs`/`LockTaskSpecs` (027), `ListExecutionRecords` (029). `mcp-service`: `CallExternalTool`, `ReadExternalResource` (031). `infra-fleet-service`: `GetDevServerCapabilities` (033). Mỗi cái do task của CR đó thêm vào proto của service tương ứng.

## Quyết định lệch so với CR (xem thêm IMPLEMENTATION-NOTES)
1. `ApprovalService`: tách response dùng chung để lint sạch (`RequestApprovalResponse`, `ApproveResponse`, `RejectResponse`, `ApprovalServiceCancelRequest/Response`, `GetApprovalResponse`). Message cũ (`CancelApprovalRequest`, `DecideApprovalResponse`) giữ lại, không dùng. `buf breaking` báo `RPC_SAME_REQUEST_TYPE`/`RPC_SAME_RESPONSE_TYPE` cho 6 RPC; `buf.yaml` có `ignore_only` giới hạn ở `approval.proto` (service chưa triển khai, số trường giữ nguyên).
2. `ProposeRequestClassification` không là RPC (CR-005: use case nội bộ, `ClassifyRequest` bọc `ClassifyNow`).
3. Thêm ngoài CR: `GetPlanProposal` (đọc kết quả PROPOSE bất đồng bộ), `ExtendApproval` (gia hạn), `GetEvidence`/`ListEvidence`, `ExportTenantRequests` (stream).
4. `GetRiskPolicy` trả `active` và `shadow`; `Approval` thêm `request_title/type/number` (chỉ `ListPendingForUser` điền; CONTRACT Q3).
5. `ListBacklogRequest` giữ nguyên 1 đến 10 của bản đã cài, thêm 11 đến 13; không dùng bố cục SOL-015.
