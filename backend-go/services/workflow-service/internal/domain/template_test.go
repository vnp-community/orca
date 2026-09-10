package domain

import (
	"errors"
	"testing"
)

func TestNewWorkflowTemplate_RequiresOwner(t *testing.T) {
	_, err := NewWorkflowTemplate("tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, ScopePersonal, "", "")
	if !errors.Is(err, ErrTemplateEmptyOwner) {
		t.Fatalf("expected ErrTemplateEmptyOwner, got %v", err)
	}
}

func TestNewWorkflowTemplate_AcceptsOwner(t *testing.T) {
	tmpl, err := NewWorkflowTemplate("tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.OwnerID != "owner-1" {
		t.Errorf("expected OwnerID=owner-1, got %q", tmpl.OwnerID)
	}
}

func TestNewWorkflowTemplate_InheritOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    []TemplateOption
		wantErr error
	}{
		{
			name: "valid overrides/inject/remove",
			opts: []TemplateOption{
				WithOverrides(`{"step-1":{"config":{"cmd":"echo hi"}}}`),
				WithInjectSteps(`[{"id":"extra","type":"shell"}]`),
				WithRemoveSteps(`["step-2"]`),
			},
		},
		{
			name: "empty inherit fields are valid (Inherit mode not used)",
			opts: []TemplateOption{
				WithOverrides(""),
				WithInjectSteps(""),
				WithRemoveSteps(""),
			},
		},
		{
			name:    "malformed overrides JSON",
			opts:    []TemplateOption{WithOverrides(`{not json`)},
			wantErr: ErrTemplateInvalidOverrides,
		},
		{
			name:    "malformed inject_steps JSON",
			opts:    []TemplateOption{WithInjectSteps(`[not json`)},
			wantErr: ErrTemplateInvalidInjectSteps,
		},
		{
			name:    "malformed remove_steps JSON",
			opts:    []TemplateOption{WithRemoveSteps(`[not json`)},
			wantErr: ErrTemplateInvalidRemoveSteps,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := NewWorkflowTemplate("tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, ScopePersonal, "", "owner-1", tt.opts...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			_ = tmpl
		})
	}
}

func TestNewWorkflowTemplate_DescriptionTagsClonedFrom(t *testing.T) {
	tmpl, err := NewWorkflowTemplate("tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, ScopePersonal, "", "owner-1",
		WithDescription("deploys to prod"),
		WithTags([]string{"deploy", "prod"}),
		WithClonedFrom("tmpl-parent"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Description != "deploys to prod" {
		t.Errorf("expected Description set, got %q", tmpl.Description)
	}
	if len(tmpl.Tags) != 2 {
		t.Errorf("expected 2 tags, got %v", tmpl.Tags)
	}
	if tmpl.ClonedFromTemplateID != "tmpl-parent" {
		t.Errorf("expected ClonedFromTemplateID set, got %q", tmpl.ClonedFromTemplateID)
	}
}
