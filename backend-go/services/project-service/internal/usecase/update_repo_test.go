package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestUpdateRepo_FieldMaskSemantics(t *testing.T) {
	cases := []struct {
		name            string
		existing        domain.Repo
		in              UpdateRepoInput
		wantURL         string
		wantDisplayName string
	}{
		{
			name:            "empty url and display name leave both unchanged",
			existing:        domain.Repo{ID: "r1", ProjectID: "p1", URL: "https://old", DisplayName: "Old"},
			in:              UpdateRepoInput{RepoID: "r1"},
			wantURL:         "https://old",
			wantDisplayName: "Old",
		},
		{
			name:            "non-empty url overwrites, display name unchanged",
			existing:        domain.Repo{ID: "r1", ProjectID: "p1", URL: "https://old", DisplayName: "Old"},
			in:              UpdateRepoInput{RepoID: "r1", URL: "https://new"},
			wantURL:         "https://new",
			wantDisplayName: "Old",
		},
		{
			name:            "both fields overwrite",
			existing:        domain.Repo{ID: "r1", ProjectID: "p1", URL: "https://old", DisplayName: "Old"},
			in:              UpdateRepoInput{RepoID: "r1", URL: "https://new", DisplayName: "New"},
			wantURL:         "https://new",
			wantDisplayName: "New",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepoRepository()
			repo.repos["r1"] = tc.existing
			uc := NewUpdateRepo(repo, ownerMembership("p1", "u1"), &fakeOPAClient{repoDecide: repoRegoDecide})
			ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

			got, err := uc.Execute(ctx, tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.URL != tc.wantURL || got.DisplayName != tc.wantDisplayName {
				t.Errorf("got {URL:%q DisplayName:%q}, want {URL:%q DisplayName:%q}",
					got.URL, got.DisplayName, tc.wantURL, tc.wantDisplayName)
			}
		})
	}
}

func TestUpdateRepo_HookSettingsExplicitPresenceSemantics(t *testing.T) {
	emptyHookSettings := ""
	newHookSettings := `{"scripts":{"setup":"pnpm install"}}`

	cases := []struct {
		name             string
		existing         domain.Repo
		in               UpdateRepoInput
		wantHookSettings string
	}{
		{
			name:             "nil HookSettings leaves the existing value unchanged",
			existing:         domain.Repo{ID: "r1", ProjectID: "p1", URL: "https://old", HookSettings: `{"scripts":{"setup":"old"}}`},
			in:               UpdateRepoInput{RepoID: "r1"},
			wantHookSettings: `{"scripts":{"setup":"old"}}`,
		},
		{
			name:             "pointer to empty string explicitly clears hook_settings",
			existing:         domain.Repo{ID: "r1", ProjectID: "p1", URL: "https://old", HookSettings: `{"scripts":{"setup":"old"}}`},
			in:               UpdateRepoInput{RepoID: "r1", HookSettings: &emptyHookSettings},
			wantHookSettings: "",
		},
		{
			name:             "pointer to a value sets hook_settings",
			existing:         domain.Repo{ID: "r1", ProjectID: "p1", URL: "https://old"},
			in:               UpdateRepoInput{RepoID: "r1", HookSettings: &newHookSettings},
			wantHookSettings: newHookSettings,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepoRepository()
			repo.repos["r1"] = tc.existing
			uc := NewUpdateRepo(repo, ownerMembership("p1", "u1"), &fakeOPAClient{repoDecide: repoRegoDecide})
			ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")

			got, err := uc.Execute(ctx, tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.HookSettings != tc.wantHookSettings {
				t.Errorf("got HookSettings=%q, want %q", got.HookSettings, tc.wantHookSettings)
			}
		})
	}
}

func TestUpdateRepo_NotFound(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewUpdateRepo(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, UpdateRepoInput{RepoID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}

func TestUpdateRepo_RequiresRepoID(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewUpdateRepo(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, UpdateRepoInput{RepoID: ""})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REPO_ID_REQUIRED")
}

func TestUpdateRepo_OwnerAllowedMemberDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "https://old"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewUpdateRepo(repo, membership, &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, UpdateRepoInput{RepoID: "r1", URL: "https://new"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.repos["r1"].URL != "https://old" {
		t.Error("expected repo to remain unchanged")
	}
}
