package grpcclient

import (
	"context"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/grpcmw"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	projectRoleTTL  = 30 * time.Second
	maxProjectPage  = 200
	maxProjectPages = 10
)

type roleKey struct{ tenant, project, user string }

type cachedRole struct {
	role    string
	expires time.Time
}

type cachedProjects struct {
	ids     []string
	expires time.Time
}

// ProjectRoleResolver asks project-service for the caller's own membership, as the caller. A removed
// member keeps the cached role for up to the TTL (accepted in CR-REQ-035; shorten it to tighten).
type ProjectRoleResolver struct {
	client projectv1.ProjectServiceClient
	ttl    time.Duration
	now    func() time.Time

	mu       sync.Mutex
	roles    map[roleKey]cachedRole
	projects map[roleKey]cachedProjects
}

func NewProjectRoleResolver(client projectv1.ProjectServiceClient) *ProjectRoleResolver {
	return &ProjectRoleResolver{client: client, ttl: projectRoleTTL, now: time.Now, roles: map[roleKey]cachedRole{}, projects: map[roleKey]cachedProjects{}}
}

func asUser(ctx context.Context, tenantID, userID string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID)
}

// RoleOf returns "owner", "member" or "" (not a member). A project-service refusal or missing project means
// not a member; any other failure is an error, never a silent "no role" and never an allow.
func (r *ProjectRoleResolver) RoleOf(ctx context.Context, tenantID, projectID, userID string) (string, error) {
	k := roleKey{tenantID, projectID, userID}
	if role, ok := r.cachedRole(k); ok {
		return role, nil
	}
	resp, err := r.client.ListMembers(asUser(ctx, tenantID, userID), &projectv1.ListMembersRequest{ProjectId: projectID})
	role := ""
	switch {
	case err == nil:
		for _, m := range resp.GetMembers() {
			if m.GetUserId() == userID {
				role = roleName(m.GetRole())
				break
			}
		}
	case isNotMember(err):
	default:
		return "", err
	}
	r.mu.Lock()
	r.sweepLocked()
	r.roles[k] = cachedRole{role: role, expires: r.now().Add(r.ttl)}
	r.mu.Unlock()
	return role, nil
}

// ProjectsOf lists the projects the user belongs to (project-service already scopes ListProjects by membership).
func (r *ProjectRoleResolver) ProjectsOf(ctx context.Context, tenantID, userID string) ([]string, error) {
	k := roleKey{tenant: tenantID, user: userID}
	r.mu.Lock()
	if c, ok := r.projects[k]; ok && r.now().Before(c.expires) {
		r.mu.Unlock()
		return append([]string{}, c.ids...), nil
	}
	r.mu.Unlock()
	var ids []string
	token := ""
	for page := 0; page < maxProjectPages; page++ {
		resp, err := r.client.ListProjects(asUser(ctx, tenantID, userID), &projectv1.ListProjectsRequest{TenantId: tenantID, PageToken: token, PageSize: maxProjectPage})
		if err != nil {
			return nil, err
		}
		for _, p := range resp.GetProjects() {
			ids = append(ids, p.GetId())
		}
		if token = resp.GetNextPageToken(); token == "" {
			break
		}
	}
	r.mu.Lock()
	r.sweepLocked()
	r.projects[k] = cachedProjects{ids: ids, expires: r.now().Add(r.ttl)}
	r.mu.Unlock()
	return append([]string{}, ids...), nil
}

func (r *ProjectRoleResolver) cachedRole(k roleKey) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.roles[k]
	if !ok || !r.now().Before(c.expires) {
		return "", false
	}
	return c.role, true
}

// sweepLocked drops expired entries on write so the maps do not grow without bound.
func (r *ProjectRoleResolver) sweepLocked() {
	now := r.now()
	for k, c := range r.roles {
		if !now.Before(c.expires) {
			delete(r.roles, k)
		}
	}
	for k, c := range r.projects {
		if !now.Before(c.expires) {
			delete(r.projects, k)
		}
	}
}

func roleName(r projectv1.ProjectRole) string {
	switch r {
	case projectv1.ProjectRole_PROJECT_ROLE_OWNER:
		return "owner"
	case projectv1.ProjectRole_PROJECT_ROLE_MEMBER:
		return "member"
	}
	return ""
}

func isNotMember(err error) bool {
	c := status.Code(err)
	return c == codes.PermissionDenied || c == codes.NotFound
}
