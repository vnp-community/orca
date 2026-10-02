package wscompat

import "sort"

// ChannelKind says which of the four registration maps a channel lives in.
type ChannelKind string

const (
	ChannelUnary         ChannelKind = "unary"
	ChannelStream        ChannelKind = "stream"
	ChannelStreamChannel ChannelKind = "streamChannel"
	ChannelBinaryStream  ChannelKind = "binaryStream"
)

// ChannelInfo describes one registered channel.
type ChannelInfo struct {
	Name string
	Kind ChannelKind
}

// Channels returns a name-sorted snapshot of every registered channel. The
// maps are unsynchronised by design (registration happens in the composition
// root before Serve), so call it only after registration is complete.
func (r *Registry) Channels() []ChannelInfo {
	out := make([]ChannelInfo, 0, len(r.handlers)+len(r.streamHandlers)+len(r.streamChannelHandlers)+len(r.binaryStreamHandlers))
	for n := range r.handlers {
		out = append(out, ChannelInfo{n, ChannelUnary})
	}
	for n := range r.streamHandlers {
		out = append(out, ChannelInfo{n, ChannelStream})
	}
	for n := range r.streamChannelHandlers {
		out = append(out, ChannelInfo{n, ChannelStreamChannel})
	}
	for n := range r.binaryStreamHandlers {
		out = append(out, ChannelInfo{n, ChannelBinaryStream})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
