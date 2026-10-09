package contracttest

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/grpcmw"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/metadata"
)

// solHotfixFixTaskAndPreDeploy runs a hotfix diagnosis against the real database and the real approval engine: the fix
// task is created in task-service during analysis, the pre_deploy approval opens on it, and approving it starts execution.
func solHotfixFixTaskAndPreDeploy(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	tasks := StartFakeTaskService(t)
	admin := uuid.NewString()
	k.admins.admins = []string{admin}
	k.registry.Register(domain.SubjectPreDeploy, &usecase.TransitionSubjectHandler{
		Artifacts: &usecase.PreDeployArtifacts{Tasks: tasks.Client}, Transition: k.tr, Returner: k.ret, Requests: env.Locker,
	})
	k.writer.WithHotfixFixTasks(usecase.NewEnsureHotfixFixTask(tasks.Client, tasks.Client, domain.NewPolicyRegistry(domain.PolicyDeps{}), domain.DefaultPlanLimits))
	k.agent.reply = analysisDoc(t, "diagnosis_valid.json")

	r := k.seed(t, domain.RequestTypeHotfix, domain.RequestStatusAnalyzing)
	gen, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	if run := k.waitRun(t, gen.RunID); run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run = %+v", run)
	}
	if got := k.requestStatus(t, r.ID); got != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("request = %s", got)
	}

	fixes, err := tasks.Client.ListTasks(k.ctx, usecase.ListTasksQuery{RequestIDs: []string{r.ID}, TaskTypes: []string{"task", "bug", "feature", "epic", "plan", "phase"}})
	if err != nil || len(fixes) != 1 {
		t.Fatalf("task-service holds %+v (%v), want exactly one fix task", fixes, err)
	}
	fix := fixes[0]
	if fix.ParentID != "" || fix.Type != "bug" || fix.Status != domain.TaskStatusOpen || fix.RequestID != r.ID {
		t.Fatalf("fix task = %+v", fix)
	}
	if got := tasks.Task(fix.ID).GetOwnerId(); got != k.userID {
		t.Fatalf("fix task created by %q, want the user who ran the analysis %q", got, k.userID)
	}

	pending := k.pendingApproval(t, domain.SubjectPreDeploy, fix.ID)
	if pending.RequestID != r.ID || pending.Stage != string(domain.RequestStatusAwaitingPlanApproval) || pending.SubjectDigest == "" {
		t.Fatalf("approval = %+v", pending)
	}
	if reloaded, err := env.Requests.Get(k.ctx, r.ID); err != nil || reloaded.PlanTaskID != "" {
		t.Fatalf("a hotfix has no Plan, plan_task_id = %q (%v)", reloaded.PlanTaskID, err)
	}

	// The reporter cannot approve their own pre_deploy; the admin can, and that starts execution.
	if _, err := k.approvals.Approve(k.rpc(), &requestv1.ApproveRequest{Id: pending.ID, ExpectedDigest: pending.SubjectDigest}); err == nil {
		t.Fatal("the reporter approved their own pre_deploy")
	}
	adminCtx := metadata.NewOutgoingContext(rpcAs(k.tenantID, admin), metadata.Pairs(grpcmw.MetadataTenantID, k.tenantID, grpcmw.MetadataUserID, admin, grpcmw.MetadataRole, "admin"))
	res, err := k.approvals.Approve(adminCtx, &requestv1.ApproveRequest{Id: pending.ID, ExpectedDigest: pending.SubjectDigest})
	if err != nil || res.GetRequestStatus() != string(domain.RequestStatusExecuting) {
		t.Fatalf("approve: %+v %v", res, err)
	}

	// Running the analysis again must not give the Request a second fix task.
	again, err := usecase.NewEnsureHotfixFixTask(tasks.Client, tasks.Client, domain.NewPolicyRegistry(domain.PolicyDeps{}), domain.DefaultPlanLimits).
		Ensure(k.ctx, domain.Request{ID: r.ID, ProjectID: r.ProjectID, Type: domain.RequestTypeHotfix, Size: r.Size, Title: r.Title}, []byte(analysisDoc(t, "diagnosis_valid.json")))
	if err != nil || again.Created || again.TaskID != fix.ID {
		t.Fatalf("second Ensure = %+v (%v)", again, err)
	}
}
