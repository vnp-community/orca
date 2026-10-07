package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCodeIntelChannelInventory(t *testing.T) {
	r := NewRegistry()
	registerCodeIntelChannels(r, ChannelDeps{})

	channels := r.Channels()
	if len(channels) != 46 {
		t.Fatalf("expected 46 channels registered, got %d", len(channels))
	}

	unaryCount := 0
	streamCount := 0
	for _, ch := range channels {
		switch ch.Kind {
		case ChannelUnary:
			unaryCount++
		case ChannelStream:
			streamCount++
		default:
			t.Errorf("channel %s has unexpected kind %s", ch.Name, ch.Kind)
		}
	}

	if unaryCount != 45 {
		t.Errorf("expected 45 unary channels, got %d", unaryCount)
	}
	if streamCount != 1 {
		t.Errorf("expected 1 stream channel, got %d", streamCount)
	}

	// Verify all placeholder channels return CODEINTEL_UNAVAILABLE: channel not wired
	ctx := context.Background()
	id := Identity{TenantID: "t-1", UserID: "u-1"}

	for _, spec := range codeIntelChannelCatalog {
		if spec.Stream {
			h, ok := r.StreamHandlerFor(spec.Name)
			if !ok {
				t.Errorf("stream channel %s not found in registry", spec.Name)
				continue
			}
			_, err := h(ctx, id, nil)
			if err == nil || !strings.Contains(err.Error(), "CODEINTEL_UNAVAILABLE") {
				t.Errorf("expected CODEINTEL_UNAVAILABLE for stream %s, got %v", spec.Name, err)
			}
		} else {
			_, err := r.Dispatch(ctx, id, spec.Name, []json.RawMessage{json.RawMessage(`{}`)})
			if err == nil || !strings.Contains(err.Error(), "CODEINTEL_UNAVAILABLE") {
				t.Errorf("expected CODEINTEL_UNAVAILABLE for unary %s, got %v", spec.Name, err)
			}
		}
	}
}

func TestCodeIntelPlaceholders_OnlyFillMissing(t *testing.T) {
	r := NewRegistry()

	// Pre-register codeIntel.reindex with custom implementation
	r.Register("codeIntel.reindex", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return "already-registered-custom", nil
	})

	registerCodeIntelChannels(r, ChannelDeps{})

	ctx := context.Background()
	id := Identity{TenantID: "t-1", UserID: "u-1"}

	// codeIntel.reindex should retain its pre-registered handler
	res, err := r.Dispatch(ctx, id, "codeIntel.reindex", []json.RawMessage{})
	if err != nil {
		t.Fatalf("unexpected error for pre-registered status: %v", err)
	}
	if res != "already-registered-custom" {
		t.Errorf("expected 'already-registered-custom', got %v", res)
	}

	// Another channel (e.g. codeIntel.erd) should have the placeholder
	_, err = r.Dispatch(ctx, id, "codeIntel.erd", []json.RawMessage{})
	if err == nil || !strings.Contains(err.Error(), "CODEINTEL_UNAVAILABLE: channel not wired") {
		t.Errorf("expected placeholder error for codeIntel.erd, got %v", err)
	}
}
