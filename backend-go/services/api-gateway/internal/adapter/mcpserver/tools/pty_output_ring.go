package tools

import (
	"errors"
	"sync"
	"time"
	"unicode/utf8"
)

// errInvalidCursor: since_seq lies beyond everything the PTY ever produced
// (e.g. the replica restarted and the ring is new).
var errInvalidCursor = errors.New("INVALID_CURSOR: since_seq is beyond the output produced so far; read again without since_seq")

// ptyOutputRing is a fixed-size byte ring with a monotonically increasing
// offset ("seq"). seq counts raw bytes of one PTY from 0 and is never reused, so
// a cursor stays meaningful across overwrites: a reader that fell behind learns
// exactly how many bytes it missed. Memory is O(cap) however much the PTY prints.
type ptyOutputRing struct {
	mu      sync.Mutex
	buf     []byte
	head    int    // index of the byte with seq == baseSeq
	n       int    // bytes held
	baseSeq uint64 // seq of the oldest byte held
	nextSeq uint64 // seq the next byte gets (== total bytes received)

	exited   bool
	exitCode int32
	detached bool // the output stream ended without an exit frame
	lastIOAt time.Time
	dropped  uint64 // bytes overwritten before anyone could read them
	notify   chan struct{}
	// onDrop (optional, set before the ring is shared) is told how many bytes a
	// write overwrote; metrics only, it never affects ring contents.
	onDrop func(n uint64)
}

func newPtyOutputRing(capBytes int, now time.Time) *ptyOutputRing {
	if capBytes < 16 {
		capBytes = 16
	}
	return &ptyOutputRing{buf: make([]byte, capBytes), lastIOAt: now, notify: make(chan struct{})}
}

func (r *ptyOutputRing) wake() { close(r.notify); r.notify = make(chan struct{}) }

// Append stores p, overwriting the oldest bytes when full. It never blocks.
func (r *ptyOutputRing) Append(p []byte, now time.Time) {
	if len(p) == 0 {
		return
	}
	r.mu.Lock()
	droppedBefore := r.dropped
	defer func() {
		d := r.dropped - droppedBefore
		r.mu.Unlock()
		if d > 0 && r.onDrop != nil {
			r.onDrop(d)
		}
	}()
	c := len(r.buf)
	r.nextSeq += uint64(len(p))
	if len(p) >= c { // only the tail can survive
		copy(r.buf, p[len(p)-c:])
		r.dropped += uint64(r.n + len(p) - c)
		r.head, r.n = 0, c
		r.baseSeq = r.nextSeq - uint64(c)
	} else {
		if over := r.n + len(p) - c; over > 0 {
			r.head = (r.head + over) % c
			r.n -= over
			r.baseSeq += uint64(over)
			r.dropped += uint64(over)
		}
		w := (r.head + r.n) % c
		k := copy(r.buf[w:], p)
		copy(r.buf, p[k:])
		r.n += len(p)
	}
	r.lastIOAt = now
	r.wake()
}

// MarkExited records the exit frame.
func (r *ptyOutputRing) MarkExited(code int32, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exited, r.exitCode, r.lastIOAt = true, code, now
	r.wake()
}

// MarkDetached records that the output stream ended without an exit frame.
func (r *ptyOutputRing) MarkDetached() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.detached = true
	r.wake()
}

func (r *ptyOutputRing) byteAt(seq uint64) byte {
	return r.buf[(r.head+int(seq-r.baseSeq))%len(r.buf)]
}

// ringRead is one cursor read of raw bytes.
type ringRead struct {
	Data     []byte
	From     uint64 // seq of Data[0]
	Next     uint64 // seq after Data (before any sanitizer hold-back)
	Dropped  uint64 // bytes between the caller's cursor and From that are gone
	Behind   bool   // the caller's cursor was older than the ring
	Tail     uint64 // seq of the newest byte + 1 at read time
	Exited   bool
	ExitCode int32
	Detached bool
}

// Read returns up to max raw bytes starting at *since (nil = oldest held). A
// cursor older than the ring is moved forward and reported through Dropped. The
// slice never starts or ends inside a UTF-8 sequence unless the stream is over.
func (r *ptyOutputRing) Read(since *uint64, max int) (ringRead, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if max < utf8.UTFMax {
		max = utf8.UTFMax
	}
	out := ringRead{Tail: r.nextSeq, Exited: r.exited, ExitCode: r.exitCode, Detached: r.detached}
	from := r.baseSeq
	if since != nil {
		if *since > r.nextSeq {
			return ringRead{}, errInvalidCursor
		}
		if *since < r.baseSeq {
			out.Dropped, out.Behind = r.baseSeq-*since, true
		} else {
			from = *since
		}
	}
	// A ring that wrapped can start mid-rune; skip those continuation bytes.
	if from == r.baseSeq && r.baseSeq > 0 {
		for from < r.nextSeq && !utf8.RuneStart(r.byteAt(from)) {
			from++
			out.Dropped++
		}
	}
	end := r.nextSeq
	if end-from > uint64(max) {
		end = from + uint64(max)
	}
	// Do not end inside a rune: more bytes will complete it later. After the
	// stream is over an incomplete tail is returned as is.
	if end > from && (end < r.nextSeq || !(r.exited || r.detached)) {
		lo := end
		for lo > from && end-lo < utf8.UTFMax && !utf8.RuneStart(r.byteAt(lo-1)) {
			lo--
		}
		if lo > from { // lo-1 is a rune start
			var tmp [utf8.UTFMax]byte
			k := 0
			for s := lo - 1; s < end && k < utf8.UTFMax; s++ {
				tmp[k] = r.byteAt(s)
				k++
			}
			if !utf8.FullRune(tmp[:k]) {
				end = lo - 1
			}
		}
	}
	out.From, out.Next = from, end
	if end > from {
		out.Data = make([]byte, 0, end-from)
		for s := from; s < end; s++ {
			out.Data = append(out.Data, r.byteAt(s))
		}
	}
	return out, nil
}

// Wait blocks until output beyond seq exists, the stream is over, ctx ends or d
// elapses; it reports whether there is something new to read.
func (r *ptyOutputRing) Wait(done <-chan struct{}, seq uint64, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		r.mu.Lock()
		ready := r.nextSeq > seq || r.exited || r.detached
		ch := r.notify
		r.mu.Unlock()
		if ready {
			return true
		}
		select {
		case <-ch:
		case <-timer.C:
			return false
		case <-done:
			return false
		}
	}
}

// WaitIdle blocks until no byte arrived for idle, the stream ended, ctx ended
// or total elapsed. It returns when the output went quiet.
func (r *ptyOutputRing) WaitIdle(done <-chan struct{}, idle, total time.Duration, now func() time.Time) {
	deadline := time.NewTimer(total)
	defer deadline.Stop()
	for {
		r.mu.Lock()
		quiet := now().Sub(r.lastIOAt)
		over := r.exited || r.detached
		ch := r.notify
		r.mu.Unlock()
		if over || quiet >= idle {
			return
		}
		t := time.NewTimer(idle - quiet)
		select {
		case <-ch:
		case <-t.C:
		case <-deadline.C:
			t.Stop()
			return
		case <-done:
			t.Stop()
			return
		}
		t.Stop()
	}
}

// Tail returns the seq after the newest byte.
func (r *ptyOutputRing) Tail() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.nextSeq
}

func (r *ptyOutputRing) LastIO() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastIOAt
}

func (r *ptyOutputRing) State() (exited bool, code int32, detached bool, droppedTotal uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.exited, r.exitCode, r.detached, r.dropped
}
