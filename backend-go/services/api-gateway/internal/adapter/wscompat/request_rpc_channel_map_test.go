package wscompat

// Every public RPC of the Request domain is either exposed as a WS channel or
// excluded with a reason. A missing entry fails TestRequestRPCChannelParity, so a
// new RPC cannot ship without a conscious gateway decision (BUG-E2E-001).

type rpcDisposition struct {
	channel string // WS channel that serves the RPC; empty when excluded
	exclude string // reason the RPC has no channel
}

func viaChannel(name string) rpcDisposition { return rpcDisposition{channel: name} }

// internalRPC: invoked by services (task/workflow outcome, gates), never by a browser.
func internalRPC(why string) rpcDisposition { return rpcDisposition{exclude: "internal: " + why} }

// deferredRPC: the RPC exists but its UI/contract belongs to a later CR; removing the
// entry and adding the channel is the only way to ship it.
func deferredRPC(cr string) rpcDisposition { return rpcDisposition{exclude: "deferred: " + cr} }

var requestServiceRPCChannels = map[string]rpcDisposition{
	"GetRequest":             viaChannel("request.get"),
	"ListRequests":           viaChannel("request.list"),
	"ListBacklog":            viaChannel("backlog.requests"),
	"CreateRequest":          viaChannel("request.create"),
	"ClassifyRequest":        viaChannel("request.classify"),
	"ConfirmRequestType":     viaChannel("request.confirmType"),
	"ChangeRequestType":      viaChannel("request.changeType"),
	"SetRequestBaseRef":      viaChannel("request.setBaseRef"),
	"ListRequestTypeHistory": viaChannel("request.typeHistory"),
	"ReturnToBacklog":        viaChannel("request.returnToBacklog"),
	"ReopenRequest":          viaChannel("request.reopen"),
	"CancelRequest":          viaChannel("request.cancel"),
	"SpawnChildRequest":      viaChannel("request.spawnChild"),
	"ListRequestLinks":       viaChannel("request.links"),
	"ListRequestActivity":    viaChannel("request.activity"),
	"GetExecutionSummary":    viaChannel("request.executionSummary"),
	// Outcome Review (CR-OR-001 to 022).
	"EstimateOutcomeReview":          viaChannel("request.review.estimate"),
	"GenerateOutcomeReview":          viaChannel("request.review.generate"),
	"GetOutcomeReview":               viaChannel("request.review.get"),
	"ListOutcomeReviews":             viaChannel("request.review.list"),
	"ListOutcomeReviewRuns":          viaChannel("request.review.runs"),
	"GetReviewArtifact":              viaChannel("request.review.artifact"),
	"ListReviewArtifactRevisions":    viaChannel("request.review.revisions"),
	"RegenerateReviewArtifacts":      viaChannel("request.review.regenerate"),
	"RestoreReviewArtifactRevision":  viaChannel("request.review.restore"),
	"CompareReviewArtifactRevisions": viaChannel("request.review.compare"),
	"DecideOutcomeReviewItem":        viaChannel("request.review.decide"),
	"ApproveOutcomeReview":           viaChannel("request.review.approve"),
	"ListOutcomeReviewDecisions":     viaChannel("request.review.decisions"),
	"LinkOutcomeReviewChild":         viaChannel("request.review.linkChild"),
	"GetReviewDrawingProfile":        viaChannel("request.review.profile.get"),
	"SaveReviewDrawingProfile":       viaChannel("request.review.profile.save"),
	"ResetReviewDrawingProfile":      viaChannel("request.review.profile.reset"),
	"GetRequestFlow":                 viaChannel("request.flow"),
	"LookupRequestBySource":          internalRPC("source dedupe runs inside CreateRequest"),
	"GetRequestFlowSettings":         viaChannel("request.flowStatus"),
	"SetRequestFlowSettings":         viaChannel("request.flowSet"),
	"GenerateSolution":               viaChannel("solution.generate"),
	"ListSolutions":                  viaChannel("solution.list"),
	"ChooseSolutionOption":           viaChannel("solution.choose"),
	"GetProjectEngineSettings":       viaChannel("request.engineSettings"),
	"SetProjectEngineSettings":       viaChannel("request.engineSettingsSet"),
	"GeneratePlan":                   viaChannel("request.generatePlan"),
	"GetPlanProposal":                viaChannel("request.planProposal"),
	"GetPlanBundleFile":              viaChannel("request.planBundleFile"),
	"CleanupRequestProposal":         viaChannel("request.cleanupProposal"),
	"CommitPlan":                     viaChannel("request.generatePlan"), // mode=commit
	"RetryPlanGeneration":            viaChannel("request.retryPlan"),
	"ListPlanRevisions":              viaChannel("request.planRevisions"),
	"StartPhase":                     viaChannel("request.startPhase"),
	"ReportTaskOutcome":              internalRPC("task-service reports phase outcomes"),
	"RecordRequestCheck":             internalRPC("verification runners record checks"),
	"ListRequestChecks":              viaChannel("request.checks"),
	"GetTestSpecs":                   viaChannel("request.testSpecs"),
	"StartTestSpecs":                 viaChannel("request.testSpecsStart"),
	"GetTestSpecDocument":            viaChannel("request.testSpecDocument"),
	"GetTestScripts":                 viaChannel("request.testScripts"),
	"StartTestScripts":               viaChannel("request.testScriptsStart"),
	"GetTestScriptReport":            viaChannel("request.testScriptReport"),
	"GetTestScriptFile":              viaChannel("request.testScriptFile"),

	"GetRequestReadiness":              deferredRPC("CR-REQ-028 readiness panel"),
	"RequestClarification":             viaChannel("clarification.request"),
	"ListClarifications":               viaChannel("clarification.list"),
	"GetClarification":                 viaChannel("clarification.get"),
	"AnswerClarification":              viaChannel("clarification.answer"),
	"CancelClarification":              viaChannel("clarification.cancel"),
	"ListPendingClarificationsForUser": deferredRPC("CR-REQ-028 personal inbox"),
	"WaiveReadiness":                   deferredRPC("CR-REQ-028 readiness waiver"),
	"ListDecisions":                    viaChannel("decision.list"),
	"GetDecision":                      deferredRPC("CR-REQ-028 decision log UI"),
	"ConfirmDecision":                  viaChannel("decision.confirm"),
	"EditRequestContent":               deferredRPC("CR-REQ-027 artifact model"),
	"ListRequestRevisions":             deferredRPC("CR-REQ-027 artifact model"),
	"GetRequestRevision":               deferredRPC("CR-REQ-027 artifact model"),
	"GetRequestCoverage":               deferredRPC("CR-REQ-027 artifact model"),
	"GetArtifactGraph":                 deferredRPC("CR-REQ-027 artifact model"),
	"ExportArtifactProjection":         deferredRPC("CR-REQ-027 artifact model"),
	"ResolveArtifactRef":               deferredRPC("CR-REQ-027 artifact model"),
	"RequestImpactAssessment":          deferredRPC("CR-REQ-030 impact and risk"),
	"GetImpactAssessment":              deferredRPC("CR-REQ-030 impact and risk"),
	"GetImpactGraph":                   deferredRPC("CR-REQ-030 impact and risk"),
	"CompareImpact":                    deferredRPC("CR-REQ-030 impact and risk"),
	"ListImpactFindings":               deferredRPC("CR-REQ-030 impact and risk"),
	"GetImpactEvidence":                deferredRPC("CR-REQ-030 impact and risk"),
	"GetPlanRiskHeatmap":               deferredRPC("CR-REQ-030 impact and risk"),
	"GetImpactDrift":                   deferredRPC("CR-REQ-030 impact and risk"),
	"AcceptRisk":                       deferredRPC("CR-REQ-030 impact and risk"),
	"OverrideRiskGate":                 deferredRPC("CR-REQ-030 impact and risk"),
	"GetRiskPolicy":                    deferredRPC("CR-REQ-030 impact and risk"),
	"SetRiskPolicy":                    deferredRPC("CR-REQ-030 impact and risk"),
	"CheckReadiness":                   deferredRPC("CR-REQ-029 execution contract"),
	"GetReadinessReport":               deferredRPC("CR-REQ-029 execution contract"),
	"ListReadiness":                    deferredRPC("CR-REQ-029 execution contract"),
	"ListContextSources":               deferredRPC("CR-REQ-031 context sources"),
	"UpsertContextSource":              deferredRPC("CR-REQ-031 context sources"),
	"SetContextSourceStatus":           deferredRPC("CR-REQ-031 context sources"),
	"PreviewContextPack":               deferredRPC("CR-REQ-031 context sources"),
	"GetEvidence":                      deferredRPC("CR-REQ-031 context sources"),
	"ListEvidence":                     deferredRPC("CR-REQ-031 context sources"),
	"EraseRequest":                     deferredRPC("CR-REQ-035 compliance, admin only"),
	"ExportRequest":                    deferredRPC("CR-REQ-035 compliance, admin only"),
	"ExportTenantRequests":             deferredRPC("CR-REQ-035 compliance, admin only"),
}

var approvalServiceRPCChannels = map[string]rpcDisposition{
	"RequestApproval":    internalRPC("opened by request-service gates"),
	"Approve":            viaChannel("approval.approve"),
	"Reject":             viaChannel("approval.reject"),
	"Cancel":             viaChannel("approval.cancel"),
	"GetApproval":        viaChannel("approval.get"),
	"ListApprovals":      viaChannel("approval.list"),
	"ListPendingForUser": viaChannel("approval.listPending"),
	"ExtendApproval":     deferredRPC("CR-REQ-010 deadline extension"),
}

var approvalPolicyAdminRPCChannels = map[string]rpcDisposition{
	"ListApprovalPolicies": deferredRPC("CR-REQ-010 policy admin UI"),
	"UpsertApprovalPolicy": deferredRPC("CR-REQ-010 policy admin UI"),
	"DeleteApprovalPolicy": deferredRPC("CR-REQ-010 policy admin UI"),
}
