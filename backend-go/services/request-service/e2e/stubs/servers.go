package stubs

import (
	"context"
	"net"
	"sync"

	"github.com/stablyai/orca-go/common/grpcmw"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// DevServerID is the single dev server every project resolves to.
const DevServerID = "stub-dev-server"

// InfraFleet answers the calls RelayClassifier makes: ResolveConnection says "not connected" so the
// resolver falls back to the project's dev server, GetFleetHealth says it is reachable, Relay* run the script.
type InfraFleet struct {
	infrafleetv1.UnimplementedInfraFleetServiceServer
	Script *AgentScript
}

func (f *InfraFleet) ResolveConnection(context.Context, *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
	return &infrafleetv1.ResolveConnectionResponse{Connected: false}, nil
}

func (f *InfraFleet) GetFleetHealth(context.Context, *infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error) {
	return &infrafleetv1.GetFleetHealthResponse{Statuses: []*infrafleetv1.DevServerHealth{{DevServerId: DevServerID, Reachable: true}}}, nil
}

func (f *InfraFleet) Relay(_ context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
	out, err := f.Script.Answer("", in.GetMethod(), in.GetParamsJson())
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &infrafleetv1.RelayResponse{ResultJson: out}, nil
}

func (f *InfraFleet) RelayByDevServer(_ context.Context, in *infrafleetv1.RelayByDevServerRequest) (*infrafleetv1.RelayResponse, error) {
	out, err := f.Script.Answer(in.GetDevServerId(), in.GetMethod(), in.GetParamsJson())
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &infrafleetv1.RelayResponse{ResultJson: out}, nil
}

// Project binds every project to the stub dev server and answers the membership lookups of the
// project-role check (CR-REQ-035) from what the scenarios register with SetMember.
type Project struct {
	projectv1.UnimplementedProjectServiceServer
	mu      sync.Mutex
	members map[[3]string]projectv1.ProjectRole
}

func NewProject() *Project {
	return &Project{members: map[[3]string]projectv1.ProjectRole{}}
}

// SetMember makes userID an owner or member of the project; unregistered users are not members.
func (p *Project) SetMember(tenantID, projectID, userID string, role projectv1.ProjectRole) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.members[[3]string{tenantID, projectID, userID}] = role
}

func (p *Project) ListMembers(ctx context.Context, in *projectv1.ListMembersRequest) (*projectv1.ListMembersResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	first := func(key string) string {
		if v := md.Get(key); len(v) > 0 {
			return v[0]
		}
		return ""
	}
	tenantID, userID := first(grpcmw.MetadataTenantID), first(grpcmw.MetadataUserID)
	p.mu.Lock()
	defer p.mu.Unlock()
	role, ok := p.members[[3]string{tenantID, in.GetProjectId(), userID}]
	if !ok {
		return &projectv1.ListMembersResponse{}, nil
	}
	return &projectv1.ListMembersResponse{Members: []*projectv1.Member{{UserId: userID, Role: role}}}, nil
}

func (*Project) ListRepos(context.Context, *projectv1.ListReposRequest) (*projectv1.ListReposResponse, error) {
	return &projectv1.ListReposResponse{Repos: []*projectv1.Repo{{Id: "stub-repo", DevServerId: DevServerID}}}, nil
}

// AuditEntry is one AppendAuditEntry call as received.
type AuditEntry struct {
	TenantID, ActorID, ActorType, Action, Target, TargetType, TargetID, Outcome, MetadataJSON string
}

// Auth captures audit entries and lists the configured admins per tenant.
type Auth struct {
	authv1.UnimplementedAuthServiceServer
	mu      sync.Mutex
	entries []AuditEntry
	// Admins maps tenant id to its admin user ids (approval recipients).
	Admins map[string][]string
}

func NewAuth() *Auth { return &Auth{Admins: map[string][]string{}} }

func (a *Auth) AppendAuditEntry(_ context.Context, in *authv1.AppendAuditEntryRequest) (*emptypb.Empty, error) {
	a.mu.Lock()
	a.entries = append(a.entries, AuditEntry{in.GetTenantId(), in.GetActorId(), in.GetActorType(), in.GetAction(), in.GetTarget(),
		in.GetTargetType(), in.GetTargetId(), in.GetOutcome(), in.GetMetadataJson()})
	a.mu.Unlock()
	return &emptypb.Empty{}, nil
}

// Entries returns the captured entries of one tenant, oldest first.
func (a *Auth) Entries(tenantID string) []AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []AuditEntry
	for _, e := range a.entries {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out
}

func (a *Auth) SetAdmins(tenantID string, ids ...string) {
	a.mu.Lock()
	a.Admins[tenantID] = ids
	a.mu.Unlock()
}

func (a *Auth) ListUsers(_ context.Context, in *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var users []*authv1.User
	for _, id := range a.Admins[in.GetTenantId()] {
		users = append(users, &authv1.User{Id: id, TenantId: in.GetTenantId(), Role: authv1.Role_ROLE_ADMIN, IsActive: true})
	}
	return &authv1.ListUsersResponse{Users: users}, nil
}

// Servers hosts the three stubs on one gRPC listener, which is how both the in-test stack and the T2 container use them.
type Servers struct {
	Script  *AgentScript
	Auth    *Auth
	Project *Project
	srv     *grpc.Server
}

func NewServers() *Servers {
	return &Servers{Script: NewAgentScript(), Auth: NewAuth(), Project: NewProject()}
}

// Serve registers the stubs on lis and blocks until Stop.
func (s *Servers) Serve(lis net.Listener) error {
	s.srv = grpc.NewServer()
	infrafleetv1.RegisterInfraFleetServiceServer(s.srv, &InfraFleet{Script: s.Script})
	projectv1.RegisterProjectServiceServer(s.srv, s.Project)
	authv1.RegisterAuthServiceServer(s.srv, s.Auth)
	return s.srv.Serve(lis)
}

func (s *Servers) Stop() {
	if s.srv != nil {
		s.srv.Stop()
	}
}
