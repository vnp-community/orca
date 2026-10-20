package usecase

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReviewDrawingProfiles keeps and resolves the drawing profile of a tenant and of its projects. Saving a profile draws nothing and
// costs nothing: the review shows which sections now differ, and a person decides what to draw again.
type ReviewDrawingProfiles struct {
	Repo  ReviewDrawingProfileRepository
	Audit ReviewAuditor
	Log   *slog.Logger
}

var _ DrawingProfiles = (*ReviewDrawingProfiles)(nil)

func (s *ReviewDrawingProfiles) logger() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// ProfileView is the layers as stored and the profile in force with the source of every field.
type ProfileView struct {
	Tenant    *StoredDrawingProfile
	Project   *StoredDrawingProfile
	Effective domain.ResolvedProfile
}

func (s *ReviewDrawingProfiles) layer(ctx context.Context, projectKey string) (*StoredDrawingProfile, *domain.DrawingProfile, error) {
	stored, ok, err := s.Repo.Get(ctx, projectKey)
	if err != nil || !ok {
		return nil, nil, err
	}
	p, perr := domain.ParseDrawingProfile(stored.Spec)
	if perr != nil {
		// A stored layer that no longer passes the rules (they changed after it was saved) is left out, so drawing keeps working with
		// the defaults instead of failing every review. The layer is still returned so a person can see and replace it.
		s.logger().WarnContext(ctx, "a stored drawing profile no longer passes validation and is ignored", slog.String("scope", projectKey), slog.Any("error", perr))
		return &stored, nil, nil
	}
	return &stored, &p, nil
}

// Get returns both layers and the effective profile for a project; with an empty project it returns the tenant layer alone.
func (s *ReviewDrawingProfiles) Get(ctx context.Context, projectID string) (ProfileView, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return ProfileView{}, domain.ErrRequestTenantRequired()
	}
	var view ProfileView
	var layers []domain.Layer
	t, tp, err := s.layer(ctx, "")
	if err != nil {
		return ProfileView{}, err
	}
	view.Tenant = t
	if tp != nil {
		layers = append(layers, domain.Layer{Scope: domain.SourceTenant, Profile: *tp})
	}
	if projectID != "" {
		p, pp, perr := s.layer(ctx, projectID)
		if perr != nil {
			return ProfileView{}, perr
		}
		view.Project = p
		if pp != nil {
			layers = append(layers, domain.Layer{Scope: domain.SourceProject, Profile: *pp})
		}
	}
	view.Effective = domain.ResolveDrawingProfile(layers...)
	return view, nil
}

// Resolve is the profile a project draws with. It satisfies DrawingProfiles.
func (s *ReviewDrawingProfiles) Resolve(ctx context.Context, projectID string) (domain.ResolvedProfile, error) {
	v, err := s.Get(ctx, projectID)
	return v.Effective, err
}

// Save validates the layer and stores it under the version the person saw (0 creates it). The error of a profile that does not pass names
// the field. The audit entry holds the digests of the old and the new layer, never the profile.
func (s *ReviewDrawingProfiles) Save(ctx context.Context, projectID string, spec []byte, expectedVersion int64, by string) (StoredDrawingProfile, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return StoredDrawingProfile{}, domain.ErrRequestTenantRequired()
	}
	profile, err := domain.ParseDrawingProfile(spec)
	if err != nil {
		return StoredDrawingProfile{}, err
	}
	digest, err := profile.SpecDigest()
	if err != nil {
		return StoredDrawingProfile{}, err
	}
	canonical, err := json.Marshal(profile)
	if err != nil {
		return StoredDrawingProfile{}, err
	}
	var oldDigest string
	if cur, ok, gerr := s.Repo.Get(ctx, projectID); gerr == nil && ok {
		oldDigest = cur.SpecDigest
	}
	saved, err := s.Repo.Save(ctx, StoredDrawingProfile{ProjectKey: projectID, Spec: canonical, SpecDigest: digest, UpdatedBy: by}, expectedVersion)
	if err != nil {
		return StoredDrawingProfile{}, err
	}
	s.audit(ctx, "save", projectID, oldDigest, digest)
	return saved, nil
}

// Reset removes the layer, so the layers below apply again.
func (s *ReviewDrawingProfiles) Reset(ctx context.Context, projectID string, expectedVersion int64) error {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.ErrRequestTenantRequired()
	}
	var oldDigest string
	if cur, ok, err := s.Repo.Get(ctx, projectID); err == nil && ok {
		oldDigest = cur.SpecDigest
	}
	if err := s.Repo.Delete(ctx, projectID, expectedVersion); err != nil {
		return err
	}
	s.audit(ctx, "reset", projectID, oldDigest, "")
	return nil
}

func (s *ReviewDrawingProfiles) audit(ctx context.Context, what, projectID, oldDigest, newDigest string) {
	if s.Audit == nil {
		return
	}
	scope := "tenant"
	if projectID != "" {
		scope = "project"
	}
	_ = s.Audit.Record(ctx, AuditEvent{
		Action: domain.AuditReviewProfile, TargetType: "review_drawing_profile", TargetID: projectID, Outcome: "allowed",
		Metadata: map[string]any{"what": what, "scope": scope, "old_digest": oldDigest, "new_digest": newDigest},
	})
}
