package wscompat

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRegistryChannelsSortedAndTyped(t *testing.T) {
	r := NewRegistry()
	r.Register("b.unary", func(context.Context, Identity, []json.RawMessage) (any, error) { return nil, nil })
	r.RegisterStream("a.stream", func(context.Context, Identity, []json.RawMessage) (<-chan PushEvent, error) { return nil, nil })
	r.RegisterStreamChannel("c.sc", func(context.Context, Identity, []json.RawMessage) (any, <-chan PushEvent, error) {
		return nil, nil, nil
	})
	r.RegisterBinaryStreamHandler("d.bin", nil)
	got := r.Channels()
	want := []ChannelInfo{{"a.stream", ChannelStream}, {"b.unary", ChannelUnary}, {"c.sc", ChannelStreamChannel}, {"d.bin", ChannelBinaryStream}}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d] got %v want %v", i, got[i], want[i])
		}
	}
}
