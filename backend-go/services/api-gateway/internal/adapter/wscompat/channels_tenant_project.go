// profile.* / project.* / projectGroup.* / projectHostSetup.* channel
// wiring — see specs/backend-go/bugs/missing-v1/tasks/TASK-126..145.md.
//
// Deliberately kept in its own file, NOT appended to channels.go: this pass
// (tenant-service's profile.* RPCs + project-service's project/member/
// projectGroup/projectHostSetup RPCs) lands in parallel with other groups
// also extending channels.go/RegisterRealChannels/main.go in their own
// worktrees. registerTenantProjectChannels below is self-contained and NOT
// yet called from RegisterRealChannels — the final integration pass wires
// it in with one line (see this file's bottom doc comment) once every
// parallel group's channels_*.go has landed, to avoid clashing edits to the
// same function/file.
package wscompat

import (
	"context"
	"encoding/json"
	"time"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// rpcTimeout (8s) is already declared package-wide in channels.go — reused
// here as-is per that file's own anticipated-collision note; deduped during
// the integration pass.

// registerTenantProjectChannels wires every profile.*/project.*/
// projectGroup.*/projectHostSetup.* channel this pass gives real backend-go
// implementations to. Call from main.go's composition root (or from
// RegisterRealChannels, once the integration pass merges this in) with the
// already-dialed tenantClient/projectClient — see main.go's existing
// `tenantClient := tenantv1.NewTenantServiceClient(tenantConn)` and
// `projectClient := projectv1.NewProjectServiceClient(projectConn)`.
func registerTenantProjectChannels(r *Registry, tenantClient tenantv1.TenantServiceClient, projectClient projectv1.ProjectServiceClient) {
	registerProfileChannels(r, tenantClient)
	registerProjectChannels(r, projectClient)
	registerProjectGroupChannels(r, projectClient)
	registerProjectHostSetupChannels(r, projectClient)
}

// ── profile.* ──────────────────────────────────────────────────────────
//
// profile.getResolved: RPC + REST already exist (tenant_routes.go's
// handleGetResolvedProfile) — wiring-only. The other 5 profile.* channels
// (getUserProfile/listDepts/updateCompany/updateDept/updateUser) call the
// new RPCs TASK-127/128 added to tenant.proto/tenant-service.
func registerProfileChannels(r *Registry, client tenantv1.TenantServiceClient) {
	r.Register("profile.getResolved", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetResolvedProfile(rpcCtx, &tenantv1.GetResolvedProfileRequest{UserId: id.UserID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("profile.getUserProfile", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getArgs struct {
			UserID string `json:"userId"`
		}
		in, err := decodeArg[getArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetUserProfile(rpcCtx, &tenantv1.GetUserProfileRequest{UserId: in.UserID})
		if err != nil {
			return nil, err
		}
		return resp.GetProfile(), nil
	})

	r.Register("profile.listDepts", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			CompanyID string `json:"companyId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListDepartments(rpcCtx, &tenantv1.ListDepartmentsRequest{CompanyId: in.CompanyID})
		if err != nil {
			return nil, err
		}
		return resp.GetDepartments(), nil
	})

	r.Register("profile.updateCompany", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			SettingsJSON string `json:"settingsJson"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateCompany(rpcCtx, &tenantv1.UpdateCompanyRequest{
			Id: in.ID, Name: in.Name, SettingsJson: in.SettingsJSON,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetCompany(), nil
	})

	r.Register("profile.updateDept", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			SettingsJSON string `json:"settingsJson"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateDepartment(rpcCtx, &tenantv1.UpdateDepartmentRequest{
			Id: in.ID, Name: in.Name, SettingsJson: in.SettingsJSON,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetDepartment(), nil
	})

	r.Register("profile.updateUser", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			UserID          string `json:"userId"`
			DepartmentID    string `json:"departmentId"`
			ClearDepartment bool   `json:"clearDepartment"`
			SettingsJSON    string `json:"settingsJson"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateUserProfile(rpcCtx, &tenantv1.UpdateUserProfileRequest{
			UserId: in.UserID, DepartmentId: in.DepartmentID, ClearDepartment: in.ClearDepartment, SettingsJson: in.SettingsJSON,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetProfile(), nil
	})
}

// ── project.* ──────────────────────────────────────────────────────────
//
// create/get/list/update: RPC + REST already exist, wiring-only.
// getMembers/removeMember/updateMemberRole call the new RPCs TASK-132/133
// added to project.proto/project-service.
func registerProjectChannels(r *Registry, client projectv1.ProjectServiceClient) {
	r.Register("project.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			DevServerID   string `json:"devServerId"`
			RepoPath      string `json:"repoPath"` // NEW — wasn't decoded at all before
			DefaultBranch string `json:"defaultBranch"`
			Visibility    string `json:"visibility"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateProject(rpcCtx, &projectv1.CreateProjectRequest{
			TenantId: id.TenantID, Name: in.Name, Description: in.Description,
			DefaultBranch: in.DefaultBranch, Visibility: in.Visibility,
			DevServerId: in.DevServerID, RepoPath: in.RepoPath, // NEW
		})
		if err != nil {
			return nil, err
		}
		return resp.GetProject(), nil
	})

	r.Register("project.get", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[getArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetProject(rpcCtx, &projectv1.GetProjectRequest{Id: in.ID})
		if err != nil {
			return nil, err
		}
		return resp.GetProject(), nil
	})

	r.Register("project.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			PageToken string `json:"pageToken"`
			PageSize  int32  `json:"pageSize"`
		}
		// list's args are optional (frontend often calls with no args) —
		// decode best-effort, defaulting to zero values on missing/absent arg[0].
		var in listArgs
		if len(args) > 0 {
			in, _ = decodeArg[listArgs](args, 0)
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		// TenantId is always taken from the resolved Identity, never a
		// caller-supplied arg.
		resp, err := client.ListProjects(rpcCtx, &projectv1.ListProjectsRequest{
			TenantId: id.TenantID, PageToken: in.PageToken, PageSize: in.PageSize,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetProjects(), nil
	})

	r.Register("project.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			Description   string `json:"description"`
			DefaultBranch string `json:"defaultBranch"`
			Visibility    string `json:"visibility"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateProject(rpcCtx, &projectv1.UpdateProjectRequest{
			ProjectId: in.ID, Name: in.Name, Description: in.Description,
			DefaultBranch: in.DefaultBranch, Visibility: in.Visibility,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetProject(), nil
	})

	r.Register("project.getMembers", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getArgs struct {
			ProjectID string `json:"projectId"`
		}
		in, err := decodeArg[getArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListMembers(rpcCtx, &projectv1.ListMembersRequest{ProjectId: in.ProjectID})
		if err != nil {
			return nil, err
		}
		return resp.GetMembers(), nil
	})

	r.Register("project.removeMember", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type removeArgs struct {
			ProjectID string `json:"projectId"`
			UserID    string `json:"userId"`
		}
		in, err := decodeArg[removeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		if _, err := client.RemoveMember(rpcCtx, &projectv1.RemoveMemberRequest{
			ProjectId: in.ProjectID, UserId: in.UserID,
		}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("project.updateMemberRole", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ProjectID string `json:"projectId"`
			UserID    string `json:"userId"`
			Role      string `json:"role"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateMemberRole(rpcCtx, &projectv1.UpdateMemberRoleRequest{
			ProjectId: in.ProjectID, UserId: in.UserID, Role: toProjectRoleArg(in.Role),
		})
		if err != nil {
			return nil, err
		}
		return resp.GetMember(), nil
	})
}

// toProjectRoleArg maps the wscompat wire arg's role string ("member" |
// "owner") onto projectv1.ProjectRole — mirrors toConnectionMode's shape
// for the same kind of string-to-enum wire mapping.
func toProjectRoleArg(role string) projectv1.ProjectRole {
	switch role {
	case "owner":
		return projectv1.ProjectRole_PROJECT_ROLE_OWNER
	case "member":
		return projectv1.ProjectRole_PROJECT_ROLE_MEMBER
	default:
		return projectv1.ProjectRole_PROJECT_ROLE_UNSPECIFIED
	}
}

// ── projectGroup.* ─────────────────────────────────────────────────────
//
// create/update/delete/list: RPC + REST already exist, wiring-only.
// moveProject/scanNested/importNested call the new RPCs TASK-137/138 added
// to project.proto/project-service.
func registerProjectGroupChannels(r *Registry, client projectv1.ProjectServiceClient) {
	r.Register("projectGroup.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Name          string `json:"name"`
			ParentGroupID string `json:"parentGroupId"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateProjectGroup(rpcCtx, &projectv1.CreateProjectGroupRequest{
			Name: in.Name, ParentGroupId: in.ParentGroupID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetGroup(), nil
	})

	r.Register("projectGroup.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			GroupID string `json:"groupId"`
			Name    string `json:"name"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateProjectGroup(rpcCtx, &projectv1.UpdateProjectGroupRequest{
			GroupId: in.GroupID, Name: in.Name,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetGroup(), nil
	})

	r.Register("projectGroup.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			GroupID string `json:"groupId"`
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		if _, err := client.DeleteProjectGroup(rpcCtx, &projectv1.DeleteProjectGroupRequest{GroupId: in.GroupID}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("projectGroup.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListProjectGroups(rpcCtx, &projectv1.ListProjectGroupsRequest{})
		if err != nil {
			return nil, err
		}
		return resp.GetGroups(), nil
	})

	r.Register("projectGroup.moveProject", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type moveArgs struct {
			ProjectID           string `json:"projectId"`
			TargetParentGroupID string `json:"targetParentGroupId"`
		}
		in, err := decodeArg[moveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.MoveProject(rpcCtx, &projectv1.MoveProjectRequest{
			ProjectId: in.ProjectID, TargetParentGroupId: in.TargetParentGroupID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetGroup(), nil
	})

	r.Register("projectGroup.scanNested", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type scanArgs struct {
			DevServerID string `json:"devServerId"`
			RootPath    string `json:"rootPath"`
		}
		in, err := decodeArg[scanArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// Longer, explicit deadline: a filesystem scan over a possibly-deep
		// tree on a remote host can legitimately exceed rpcTimeout — see
		// infra-fleet-service.md §8's "Deadlines" note for Agent-bound calls.
		rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		resp, err := client.ScanNested(rpcCtx, &projectv1.ScanNestedRequest{
			DevServerId: in.DevServerID, RootPath: in.RootPath,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetCandidates(), nil
	})

	r.Register("projectGroup.importNested", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type candidateArg struct {
			Path          string `json:"path"`
			SuggestedName string `json:"suggestedName"`
			IsGitRepo     bool   `json:"isGitRepo"`
		}
		type importArgs struct {
			DevServerID   string         `json:"devServerId"`
			ParentGroupID string         `json:"parentGroupId"`
			Selected      []candidateArg `json:"selected"`
		}
		in, err := decodeArg[importArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		selected := make([]*projectv1.NestedRepoCandidate, 0, len(in.Selected))
		for _, c := range in.Selected {
			selected = append(selected, &projectv1.NestedRepoCandidate{
				Path: c.Path, SuggestedName: c.SuggestedName, IsGitRepo: c.IsGitRepo,
			})
		}
		resp, err := client.ImportNested(rpcCtx, &projectv1.ImportNestedRequest{
			DevServerId: in.DevServerID, ParentGroupId: in.ParentGroupID, Selected: selected,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}

// ── projectHostSetup.* ─────────────────────────────────────────────────
//
// All 5 channels call brand-new RPCs (TASK-141/143 added to
// project.proto/project-service) — none of these are wiring-only.
func registerProjectHostSetupChannels(r *Registry, client projectv1.ProjectServiceClient) {
	r.Register("projectHostSetup.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			DevServerID string `json:"devServerId"`
			FolderPath  string `json:"folderPath"`
			DisplayName string `json:"displayName"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateHostSetup(rpcCtx, &projectv1.CreateHostSetupRequest{
			DevServerId: in.DevServerID, FolderPath: in.FolderPath, DisplayName: in.DisplayName,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetSetup(), nil
	})

	r.Register("projectHostSetup.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListHostSetups(rpcCtx, &projectv1.ListHostSetupsRequest{})
		if err != nil {
			return nil, err
		}
		return resp.GetSetups(), nil
	})

	r.Register("projectHostSetup.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID          string `json:"id"`
			FolderPath  string `json:"folderPath"`
			DisplayName string `json:"displayName"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateHostSetup(rpcCtx, &projectv1.UpdateHostSetupRequest{
			Id: in.ID, FolderPath: in.FolderPath, DisplayName: in.DisplayName,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetSetup(), nil
	})

	r.Register("projectHostSetup.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		if _, err := client.DeleteHostSetup(rpcCtx, &projectv1.DeleteHostSetupRequest{Id: in.ID}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("projectHostSetup.setupExistingFolder", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type setupArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[setupArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// Same reasoning as projectGroup.scanNested — a remote
		// path-check-then-finalize round-trip can exceed rpcTimeout.
		rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		resp, err := client.SetupExistingFolder(rpcCtx, &projectv1.SetupExistingFolderRequest{Id: in.ID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}

// Integration-pass wiring (NOT yet applied — see this file's top doc
// comment):
//
//  1. channels.go: add `tenantClient tenantv1.TenantServiceClient` and
//     `projectClient projectv1.ProjectServiceClient` params to
//     RegisterRealChannels, and call
//     `registerTenantProjectChannels(r, tenantClient, projectClient)`
//     from its body.
//  2. main.go: change the RegisterRealChannels call site to
//     `wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, tenantClient, projectClient, rateLimiter)`
//     (tenantClient/projectClient are already dialed in main.go for
//     mountTenantRoutes/mountProjectRoutes — no new Dial call needed;
//     merge param order with whatever other parallel groups' channels_*.go
//     additions already appended to RegisterRealChannels's signature).
