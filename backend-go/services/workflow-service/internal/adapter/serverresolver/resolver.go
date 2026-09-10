// Package serverresolver implements usecase.ServerResolver — turning a
// step's Target string (see domain.AgentStepConfig.Target's doc comment
// for the four accepted shapes) into a concrete infra-fleet-service
// connectionId, resolving through project-service and infra-fleet-service
// as needed. BUG-WF-02 found no server-resolution logic anywhere: every
// agent-type step's Target/ConnectionID passed straight through to the
// relay call unresolved.
package serverresolver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// resolveTimeout bounds each Resolve call — workflow-service.md §8's
// intra-cluster default for a synchronous backend-to-backend hop.
const resolveTimeout = 5 * time.Second

type resolver struct {
	projects projectv1.ProjectServiceClient
	infra    infrafleetv1.InfraFleetServiceClient
	// fleetTagCounters round-robins fleet:tag:<tag> targets — per-replica,
	// not globally coordinated; a perfectly even global distribution is not
	// a correctness requirement here.
	fleetTagCounters sync.Map // map[string]*atomic.Uint64
}

// New builds a usecase.ServerResolver against already-dialed
// project-service and infra-fleet-service clients — see cmd/server/main.go
// for the real dial, and this package's tests for fakes.
func New(projects projectv1.ProjectServiceClient, infra infrafleetv1.InfraFleetServiceClient) usecase.ServerResolver {
	return &resolver{projects: projects, infra: infra}
}

func (r *resolver) Resolve(ctx context.Context, tenantID, target string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()
	// project-service/infra-fleet-service's own usecases pull tenant scope
	// from inbound gRPC metadata (via common/grpcmw's interceptor), same
	// convention infrafleetclient.withTenantMetadata follows for the Relay
	// path — this outbound hop must forward it explicitly, never trust
	// callee-side re-derivation.
	ctx = metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID)

	switch {
	case target == "":
		return "", nil // execute locally — unchanged default
	case strings.HasPrefix(target, "connection:"):
		return strings.TrimPrefix(target, "connection:"), nil
	case strings.HasPrefix(target, "server:"):
		devServerID := strings.TrimPrefix(target, "server:")
		return r.resolveDevServer(ctx, devServerID)
	case strings.HasPrefix(target, "project:"):
		projectID := strings.TrimPrefix(target, "project:")
		proj, err := r.projects.GetProject(ctx, &projectv1.GetProjectRequest{Id: projectID})
		if err != nil {
			return "", fmt.Errorf("serverresolver: resolve project %s: %w", projectID, err)
		}
		devServerID := proj.GetProject().GetDevServerId()
		if devServerID == "" {
			return "", fmt.Errorf("serverresolver: project %s has no dev_server_id bound", projectID)
		}
		return r.resolveDevServer(ctx, devServerID)
	case strings.HasPrefix(target, "fleet:tag:"):
		return r.resolveFleetTag(ctx, tenantID, strings.TrimPrefix(target, "fleet:tag:"))
	default:
		return target, nil // legacy bare connectionId
	}
}

// resolveDevServer turns a dev_server_id into its currently-active
// connectionId — shared by the "server:" and "project:" Target shapes.
func (r *resolver) resolveDevServer(ctx context.Context, devServerID string) (string, error) {
	resp, err := r.infra.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: devServerID})
	if err != nil {
		return "", fmt.Errorf("serverresolver: resolve dev server %s: %w", devServerID, err)
	}
	// A target explicitly named a dev server — an unresolved connection
	// here must fail loudly, not silently fall back to
	// ServerResolver.Resolve's empty-string "execute locally" convention
	// (which only applies to an unset Target, see its doc comment).
	if !resp.GetConnected() || resp.GetConnectionId() == "" {
		return "", fmt.Errorf("serverresolver: dev server %s has no active connection", devServerID)
	}
	return resp.GetConnectionId(), nil
}

func (r *resolver) resolveFleetTag(ctx context.Context, tenantID, tag string) (string, error) {
	resp, err := r.infra.ListDevServersByTag(ctx, &infrafleetv1.ListDevServersByTagRequest{Tag: tag, HealthyOnly: true})
	if err != nil {
		return "", fmt.Errorf("serverresolver: list dev servers tagged %q: %w", tag, err)
	}
	servers := resp.GetDevServers()
	if len(servers) == 0 {
		return "", fmt.Errorf("serverresolver: no healthy dev server tagged %q", tag)
	}
	counterAny, _ := r.fleetTagCounters.LoadOrStore(tag, new(atomic.Uint64))
	counter := counterAny.(*atomic.Uint64)
	chosen := servers[counter.Add(1)%uint64(len(servers))]
	return r.resolveDevServer(ctx, chosen.GetId())
}
