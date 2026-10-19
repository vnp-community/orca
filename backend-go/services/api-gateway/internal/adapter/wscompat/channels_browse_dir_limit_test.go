package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

func relayEntries(dirs, files int) []browseDirRelayEntry {
	out := make([]browseDirRelayEntry, 0, dirs+files)
	for i := 0; i < dirs; i++ {
		out = append(out, browseDirRelayEntry{Name: fmt.Sprintf("dir-%04d", i), Type: "directory"})
	}
	for i := 0; i < files; i++ {
		out = append(out, browseDirRelayEntry{Name: fmt.Sprintf("file-%04d.txt", i), Type: "file"})
	}
	return out
}

func TestBuildBrowseDirView_CapsAndReportsTruncation(t *testing.T) {
	cases := []struct {
		name          string
		opts          browseDirOptions
		wantLen       int
		wantTruncated bool
		wantTotal     int
	}{
		{"default cap is 500", browseDirOptions{}, 500, true, 1000},
		{"explicit limit", browseDirOptions{Limit: 10}, 10, true, 1000},
		{"limit above max is clamped", browseDirOptions{Limit: 99999}, 1000, false, 1000},
		{"directories only", browseDirOptions{DirectoriesOnly: true, Limit: 2000}, 400, false, 400},
		{"query is case-insensitive", browseDirOptions{Query: "DIR-0001", Limit: 2000}, 1, false, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			view := buildBrowseDirView("/x", relayEntries(400, 600), tc.opts)
			if len(view.Entries) != tc.wantLen || view.Truncated != tc.wantTruncated || view.TotalEntries != tc.wantTotal {
				t.Fatalf("len=%d truncated=%v total=%d, want %d/%v/%d", len(view.Entries), view.Truncated, view.TotalEntries, tc.wantLen, tc.wantTruncated, tc.wantTotal)
			}
		})
	}
}

func TestBuildBrowseDirView_SmallDirectoryIsUntouched(t *testing.T) {
	view := buildBrowseDirView("/x", relayEntries(1, 1), browseDirOptions{})
	if view.Truncated || len(view.Entries) != 2 || view.TotalEntries != 2 {
		t.Fatalf("unexpected view: %+v", view)
	}
	if !view.Entries[0].IsDirectory || view.Entries[1].IsDirectory {
		t.Errorf("entry kinds lost: %+v", view.Entries)
	}
}

// A $TMPDIR-sized listing must reach the browser capped, with the additive
// truncated/totalEntries fields, for both browse channels.
func TestBrowseChannels_LargeDirectoryIsTruncated(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"path": "/private/var/folders/T", "entries": relayEntries(0, 1500)})
	fake := &fakeInfraFleetClient{
		relayByDevServerFunc: func(context.Context, *infrafleetv1.RelayByDevServerRequest) (*infrafleetv1.RelayResponse, error) {
			return &infrafleetv1.RelayResponse{ResultJson: string(raw)}, nil
		},
		resolveConnectionFunc: func(context.Context, *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			return &infrafleetv1.ResolveConnectionResponse{Connected: true}, nil
		},
	}
	r := NewRegistry()
	registerDevServerChannels(r, fake)
	registerFilesBrowseServerDirChannel(r, fake)

	cases := []struct {
		name          string
		channel       string
		args          map[string]any
		wantLen       int
		wantTotal     int
		wantTruncated bool
	}{
		{"browseDir default cap", "devServer.browseDir", map[string]any{"id": "ds-1", "path": "/p"}, 500, 1500, true},
		{"browseServerDir default cap", "files.browseServerDir", map[string]any{"environmentId": "ds-1", "path": "/p"}, 500, 1500, true},
		{"browseDir query and limit", "devServer.browseDir", map[string]any{"id": "ds-1", "path": "/p", "limit": 20, "query": "file-00"}, 20, 100, true},
		{"browseServerDir directoriesOnly", "files.browseServerDir", map[string]any{"environmentId": "ds-1", "path": "/p", "directoriesOnly": true}, 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, tc.channel, argsJSON(t, tc.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			view := result.(devServerBrowseDirResultView)
			if len(view.Entries) != tc.wantLen || view.TotalEntries != tc.wantTotal || view.Truncated != tc.wantTruncated {
				t.Errorf("len=%d total=%d truncated=%v, want %d/%d/%v", len(view.Entries), view.TotalEntries, view.Truncated, tc.wantLen, tc.wantTotal, tc.wantTruncated)
			}
			out, _ := json.Marshal(view)
			for _, key := range []string{`"resolvedPath"`, `"entries"`, `"truncated"`, `"totalEntries"`} {
				if !strings.Contains(string(out), key) {
					t.Errorf("wire JSON missing %s: %.120s", key, out)
				}
			}
		})
	}
}
