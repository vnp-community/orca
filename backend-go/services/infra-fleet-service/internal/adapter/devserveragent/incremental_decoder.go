package devserveragent

import "encoding/binary"

// IncrementalDecoder accumulates bytes fed via Feed and emits complete
// frames as soon as they're available — needed for transports that don't
// already deliver whole messages the way a WebSocket does (see frame.go's
// DecodeFrame doc comment). adapter/sshrelay's SSH exec-channel stdio
// transport is the one consumer: stdio delivers arbitrary-sized chunks with
// no message boundary, so a single-shot DecodeFrame isn't safe there.
//
// Mirrors relay-protocol.ts's FrameDecoder: an oversized declared length is
// skipped (resync past just the header, not the whole buffer) rather than
// treated as fatal, so one corrupt frame can't kill the whole stream —
// matches Transport.ReadFrame's contract that only a real transport failure
// is an error, never a malformed frame.
type IncrementalDecoder struct {
	buf []byte
}

// Feed appends data to the internal buffer and returns every complete,
// well-formed frame now available, in wire order.
func (d *IncrementalDecoder) Feed(data []byte) []DecodedFrame {
	d.buf = append(d.buf, data...)

	var out []DecodedFrame
	for {
		if len(d.buf) < HeaderLength {
			return out
		}
		length := binary.BigEndian.Uint32(d.buf[9:13])
		if length > MaxMessageSize {
			// Corrupt/oversized declared length — drop just the header and
			// keep scanning, rather than discarding everything buffered so
			// far (mirrors FrameDecoder's lenient resync behavior).
			d.buf = d.buf[HeaderLength:]
			continue
		}
		total := HeaderLength + int(length)
		if len(d.buf) < total {
			return out // incomplete frame — wait for more data
		}
		frame, err := DecodeFrame(d.buf[:total])
		d.buf = d.buf[total:]
		if err != nil {
			continue // shouldn't happen given the checks above, but stay lenient
		}
		out = append(out, frame)
	}
}
