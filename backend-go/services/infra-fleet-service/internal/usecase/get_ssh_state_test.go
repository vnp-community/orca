package usecase

import (
	"context"
	"testing"
)

func TestGetSshState_ThreeCases(t *testing.T) {
	cases := []struct {
		name           string
		devServerFound bool
		connFound      bool
		wantConnected  bool
	}{
		{name: "no dev server bound", devServerFound: false, wantConnected: false},
		{name: "dev server bound, no active connection", devServerFound: true, connFound: false, wantConnected: false},
		{name: "dev server bound, active connection", devServerFound: true, connFound: true, wantConnected: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			devServers := &fakeDevServerRepository{found: tc.devServerFound}
			conns := &fakeConnectionRepository{found: tc.connFound}
			uc := NewGetSshState(&fakeSshTargetRepository{}, devServers, conns)

			got, err := uc.Execute(withTenant(context.Background(), "t1"), SshStateInput{SshTargetID: "s1"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Connected != tc.wantConnected {
				t.Errorf("got Connected=%v, want %v", got.Connected, tc.wantConnected)
			}
		})
	}
}

func TestGetSshState_RequiresTenantContext(t *testing.T) {
	uc := NewGetSshState(&fakeSshTargetRepository{}, &fakeDevServerRepository{}, &fakeConnectionRepository{})
	_, err := uc.Execute(context.Background(), SshStateInput{SshTargetID: "s1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
