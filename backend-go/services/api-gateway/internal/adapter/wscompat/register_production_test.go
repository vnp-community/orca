package wscompat

import "testing"

func TestRegisterProductionChannelsBuildsWithNilDeps(t *testing.T) {
	r := NewRegistry()
	RegisterProductionChannels(r, ChannelDeps{TaskActivityEnabled: true})
	if n := len(r.Channels()); n < 400 {
		t.Fatalf("expected the full inventory, got %d channels", n)
	}
	var unary, stream, sc, bin int
	for _, c := range r.Channels() {
		switch c.Kind {
		case ChannelUnary:
			unary++
		case ChannelStream:
			stream++
		case ChannelStreamChannel:
			sc++
		case ChannelBinaryStream:
			bin++
		}
	}
	t.Logf("CHANNELS total=%d unary=%d stream=%d streamChannel=%d binary=%d", unary+stream+sc+bin, unary, stream, sc, bin)
}
