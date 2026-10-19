package wscompat

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const channelInventoryGolden = "testdata/channel_inventory.json"

type channelInventoryEntry struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func channelKindName(k ChannelKind) string {
	switch k {
	case ChannelUnary:
		return "unary"
	case ChannelStream:
		return "stream"
	case ChannelStreamChannel:
		return "streamChannel"
	case ChannelBinaryStream:
		return "binaryStream"
	}
	return "unknown"
}

// The frontend contract test reads this file instead of keeping its own channel list,
// so a renamed or dropped channel fails in CI rather than at a user's click (BUG-E2E-001).
func TestChannelInventoryMatchesGolden(t *testing.T) {
	r := NewRegistry()
	RegisterProductionChannels(r, ChannelDeps{TaskActivityEnabled: true})
	var inv []channelInventoryEntry
	for _, c := range r.Channels() {
		inv = append(inv, channelInventoryEntry{Name: c.Name, Kind: channelKindName(c.Kind)})
	}
	got, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(channelInventoryGolden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(channelInventoryGolden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(channelInventoryGolden)
	if err != nil {
		t.Fatalf("read golden: %v (create it with UPDATE_GOLDEN=1)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("channel inventory changed; if intended run `UPDATE_GOLDEN=1 go test ./internal/adapter/wscompat -run TestChannelInventoryMatchesGolden` and update frontend callers (specs/frontend/bugs/e2e-acceptance-v1)\n%s", diffChannelInventories(want, got))
	}
}

func diffChannelInventories(want, got []byte) string {
	index := func(b []byte) map[string]string {
		var inv []channelInventoryEntry
		_ = json.Unmarshal(b, &inv)
		m := map[string]string{}
		for _, e := range inv {
			m[e.Name] = e.Kind
		}
		return m
	}
	w, g := index(want), index(got)
	var out string
	for n, k := range g {
		if wk, ok := w[n]; !ok {
			out += "  + " + n + " (" + k + ")\n"
		} else if wk != k {
			out += "  ~ " + n + " " + wk + " -> " + k + "\n"
		}
	}
	for n := range w {
		if _, ok := g[n]; !ok {
			out += "  - " + n + "\n"
		}
	}
	return out
}
