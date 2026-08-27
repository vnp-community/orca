// Command server is project-service's composition root — the only place
// allowed to know about every layer at once, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/policy"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/project-service/internal/config"

	projectgrpc "github.com/stablyai/orca-go/services/project-service/internal/adapter/grpc"
	projectgrpcclient "github.com/stablyai/orca-go/services/project-service/internal/adapter/grpcclient"
	projectopaclient "github.com/stablyai/orca-go/services/project-service/internal/adapter/opaclient"
	projectpostgres "github.com/stablyai/orca-go/services/project-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/project-service/internal/usecase"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("project-service exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := svcconfig.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := logging.New(cfg.ServiceName, version)
	slog.SetDefault(logger)

	shutdownTracing, err := tracing.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	if err != nil {
		return fmt.Errorf("initializing tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	dsn := cfg.DatabaseDSN
	if dsn == "" {
		return errors.New("DATABASE_DSN is required (or a Vault-Agent-rendered credentials file — not wired in this scaffold)")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()

	repo := projectpostgres.New(pool)
	repoRepo := projectpostgres.NewRepoRepository(pool)
	worktreeRepo := projectpostgres.NewWorktreeRepository(pool)
	projectGroupRepo := projectpostgres.NewProjectGroupRepository(pool)
	folderWorkspaceRepo := projectpostgres.NewFolderWorkspaceRepository(pool)
	hostSetupRepo := projectpostgres.NewHostSetupRepository(pool)

	// Real clients — Epic C (docs/execution-plan.md §10, 2026-08-17) closed
	// the gap these were previously stubs for. Dialed lazily (doesn't block
	// startup) — see internal/adapter/grpcclient's doc comments.
	workflowChecker, err := projectgrpcclient.NewWorkflowExecutionChecker(cfg.WorkflowServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing workflow-service: %w", err)
	}
	defer func() { _ = workflowChecker.Close() }()

	taskChecker, err := projectgrpcclient.NewTaskExecutionChecker(cfg.TaskServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing task-service: %w", err)
	}
	defer func() { _ = taskChecker.Close() }()

	devServerRelay, err := projectgrpcclient.NewDevServerRelay(cfg.InfraFleetServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing infra-fleet-service: %w", err)
	}
	defer func() { _ = devServerRelay.Close() }()

	devServerLister, err := projectgrpcclient.NewInfraFleetDevServerLister(cfg.InfraFleetServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing infra-fleet-service (dev server lister): %w", err)
	}
	defer func() { _ = devServerLister.Close() }()

	// Shared embedded-OPA evaluator (common/policy) for project-role/
	// global-admin authorization — mirrors auth-service/annotation-service/
	// task-service's own composition-root wiring. One Evaluator, pointed at
	// the same orca-authz bundle, shared by every OPA-gated usecase below.
	opa := projectopaclient.New(policy.NewEvaluator(cfg.OPABundlePath))

	createProjectUC := usecase.NewCreateProject(repo)
	getProjectUC := usecase.NewGetProject(repo, opa)
	listProjectsUC := usecase.NewListProjects(repo)
	addMemberUC := usecase.NewAddMember(repo, opa)
	listMembersUC := usecase.NewListMembers(repo, opa)
	removeMemberUC := usecase.NewRemoveMember(repo, opa)
	updateMemberRoleUC := usecase.NewUpdateMemberRole(repo, opa)
	rebindDevServerUC := usecase.NewRebindDevServer(repo, workflowChecker, taskChecker, opa)
	updateProjectUC := usecase.NewUpdateProject(repo, opa)
	deleteProjectUC := usecase.NewDeleteProject(repo, workflowChecker, taskChecker, opa)

	// repo (usecase.ProjectRepository) satisfies usecase.MembershipRepository
	// structurally — passed as the membership-lookup port to usecases whose
	// primary repository dependency is RepoRepository/WorktreeRepository
	// instead. See usecase.MembershipRepository's doc comment.
	addRepoUC := usecase.NewAddRepo(repoRepo, repo, opa)
	listReposUC := usecase.NewListRepos(repoRepo, repo, opa)
	reorderReposUC := usecase.NewReorderRepos(repoRepo, repo, opa)
	removeRepoUC := usecase.NewRemoveRepo(repoRepo, repo, opa)
	updateRepoUC := usecase.NewUpdateRepo(repoRepo, repo, opa)

	recordWorktreeCreatedUC := usecase.NewRecordWorktreeCreated(worktreeRepo)
	recordWorktreeRemovedUC := usecase.NewRecordWorktreeRemoved(worktreeRepo)
	listWorktreesUC := usecase.NewListWorktrees(worktreeRepo, repo, opa)
	setWorktreeActivationUC := usecase.NewSetWorktreeActivation(worktreeRepo)
	renameWorktreeUC := usecase.NewRenameWorktree(worktreeRepo)
	getWorktreeByIdempotencyKeyUC := usecase.NewGetWorktreeByIdempotencyKey(worktreeRepo)

	createProjectGroupUC := usecase.NewCreateProjectGroup(projectGroupRepo)
	updateProjectGroupUC := usecase.NewUpdateProjectGroup(projectGroupRepo)
	deleteProjectGroupUC := usecase.NewDeleteProjectGroup(projectGroupRepo)
	listProjectGroupsUC := usecase.NewListProjectGroups(projectGroupRepo)
	moveProjectUC := usecase.NewMoveProject(repo, projectGroupRepo, opa)
	scanNestedUC := usecase.NewScanNested(devServerRelay)
	importNestedUC := usecase.NewImportNested(projectGroupRepo)

	createHostSetupUC := usecase.NewCreateHostSetup(hostSetupRepo, devServerLister)
	listHostSetupsUC := usecase.NewListHostSetups(hostSetupRepo)
	updateHostSetupUC := usecase.NewUpdateHostSetup(hostSetupRepo)
	deleteHostSetupUC := usecase.NewDeleteHostSetup(hostSetupRepo)
	setupExistingFolderUC := usecase.NewSetupExistingFolder(hostSetupRepo, repo, repoRepo, devServerRelay)

	folderWorkspaceUC := usecase.NewFolderWorkspaceUseCase(folderWorkspaceRepo)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	projectv1.RegisterProjectServiceServer(grpcServer, projectgrpc.New(projectgrpc.Deps{
		CreateProject:   createProjectUC,
		GetProject:      getProjectUC,
		ListProjects:    listProjectsUC,
		AddMember:       addMemberUC,
		RebindDevServer: rebindDevServerUC,
		UpdateProject:   updateProjectUC,
		DeleteProject:   deleteProjectUC,

		ListMembers:      listMembersUC,
		RemoveMember:     removeMemberUC,
		UpdateMemberRole: updateMemberRoleUC,

		AddRepo:      addRepoUC,
		ListRepos:    listReposUC,
		ReorderRepos: reorderReposUC,
		RemoveRepo:   removeRepoUC,
		UpdateRepo:   updateRepoUC,

		RecordWorktreeCreated:       recordWorktreeCreatedUC,
		RecordWorktreeRemoved:       recordWorktreeRemovedUC,
		ListWorktrees:               listWorktreesUC,
		SetWorktreeActivation:       setWorktreeActivationUC,
		RenameWorktree:              renameWorktreeUC,
		GetWorktreeByIdempotencyKey: getWorktreeByIdempotencyKeyUC,

		CreateProjectGroup: createProjectGroupUC,
		UpdateProjectGroup: updateProjectGroupUC,
		DeleteProjectGroup: deleteProjectGroupUC,
		ListProjectGroups:  listProjectGroupsUC,

		FolderWorkspaces: folderWorkspaceUC,
		MoveProject:        moveProjectUC,
		ScanNested:         scanNestedUC,
		ImportNested:       importNestedUC,

		CreateHostSetup:     createHostSetupUC,
		ListHostSetups:      listHostSetupsUC,
		UpdateHostSetup:     updateHostSetupUC,
		DeleteHostSetup:     deleteHostSetupUC,
		SetupExistingFolder: setupExistingFolderUC,
	}))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

	healthSrv := health.New()
	healthSrv.Register("postgres", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pool.Ping(ctx)
	})

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: healthSrv.Handler(),
	}

	errCh := make(chan error, 2)

	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			errCh <- fmt.Errorf("listening on grpc port: %w", err)
			return
		}
		logger.Info("project-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("project-service http (health) listening", slog.Int("port", cfg.HTTPPort))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining in-flight requests")
	case err := <-errCh:
		return err
	}

	// Graceful shutdown: GracefulStop drains in-flight gRPC calls before
	// returning, matching the termination-grace-period expectation in
	// standards/production-readiness-checklist.md.
	grpcServer.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)

	return nil
}
