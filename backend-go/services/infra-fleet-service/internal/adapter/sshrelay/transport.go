package sshrelay

import (
	"context"

	"golang.org/x/crypto/ssh"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/devserveragent"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
)

// sshExecTransport implements devserveragent.Transport over one SSH exec
// channel's stdin/stdout — the launched `node agent.js --stdio` process's
// pipes. Unlike a WebSocket, this stream delivers arbitrary-sized chunks
// with no message boundary, so reads go through
// devserveragent.IncrementalDecoder rather than the WS transport's
// single-shot DecodeFrame.
type sshExecTransport struct {
	conn    *sshconn.Connection // closed alongside the session — one dedicated SSH connection per relay-ssh session, no pooling
	session *ssh.Session
	stdin   sshWriter
	stdout  sshReader

	decoder devserveragent.IncrementalDecoder
	pending []devserveragent.DecodedFrame
}

// sshWriter/sshReader narrow *ssh.Session's StdinPipe()/StdoutPipe() return
// types to just what this file uses — golang.org/x/crypto/ssh returns
// io.WriteCloser/io.Reader for these, named here only for readability.
type (
	sshWriter interface {
		Write(p []byte) (int, error)
	}
	sshReader interface {
		Read(p []byte) (int, error)
	}
)

func newSSHExecTransport(conn *sshconn.Connection, session *ssh.Session, stdin sshWriter, stdout sshReader) *sshExecTransport {
	return &sshExecTransport{conn: conn, session: session, stdin: stdin, stdout: stdout}
}

// readResult carries one stdout.Read outcome (plus the buffer it read into)
// across the goroutine ReadFrame spawns to make an otherwise
// non-cancelable blocking Read respect ctx.
type readResult struct {
	buf []byte
	n   int
	err error
}

// ReadFrame returns the next complete frame, feeding the incremental
// decoder from stdout as needed. io.Reader has no native context-cancellation
// support, so each underlying Read runs in its own goroutine — a standard,
// accepted trade-off for wrapping blocking I/O with ctx: the goroutine
// outlives ReadFrame if ctx cancels mid-read and the pipe never unblocks on
// its own (the process/session teardown that follows a cancellation closes
// the pipe and unblocks it in practice). Each goroutine gets its OWN
// buffer, allocated fresh per call rather than reused across calls — a
// shared buffer would let an abandoned (ctx-cancelled) read's eventual,
// late completion race with a subsequent call's own Read into the same
// backing array; a fresh allocation per call costs little for this
// interactive, low-throughput JSON-RPC-over-SSH use case and removes the
// hazard outright instead of documenting around it.
func (t *sshExecTransport) ReadFrame(ctx context.Context) (devserveragent.DecodedFrame, error) {
	for len(t.pending) == 0 {
		resultCh := make(chan readResult, 1)
		go func() {
			buf := make([]byte, 32*1024)
			n, err := t.stdout.Read(buf)
			resultCh <- readResult{buf: buf, n: n, err: err}
		}()

		select {
		case <-ctx.Done():
			return devserveragent.DecodedFrame{}, ctx.Err()
		case res := <-resultCh:
			if res.err != nil {
				return devserveragent.DecodedFrame{}, res.err
			}
			t.pending = t.decoder.Feed(res.buf[:res.n])
		}
	}

	frame := t.pending[0]
	t.pending = t.pending[1:]
	return frame, nil
}

func (t *sshExecTransport) WriteFrame(_ context.Context, frame []byte) error {
	_, err := t.stdin.Write(frame)
	return err
}

// Close tears down both the exec session and its dedicated SSH connection —
// see conn's doc comment on why this transport owns the whole connection,
// not just the session.
func (t *sshExecTransport) Close(_ string) error {
	_ = t.session.Close()
	return t.conn.Close()
}
