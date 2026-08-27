package sshconn

import (
	"fmt"
	"io"
	"net"
	"sync"
)

// Tunnel is a live local:remote forward — the "process/tunnel handle"
// domain.PortForward's doc comment names. Close stops the listener and
// every in-flight forwarded connection.
type Tunnel struct {
	listener net.Listener
	done     chan struct{}
	closeOne sync.Once

	mu    sync.Mutex
	conns map[net.Conn]struct{} // both sides (local+remote) of every live forward
}

// Forward opens a local TCP listener on 127.0.0.1:localPort and, for every
// accepted connection, dials remotePort on conn's target via the SSH
// connection's own direct-tcpip channel type (client.Dial), then copies
// bytes both directions until either side closes.
func (conn *Connection) Forward(localPort, remotePort int) (*Tunnel, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return nil, fmt.Errorf("sshconn: binding local port %d: %w", localPort, err)
	}
	t := &Tunnel{listener: listener, done: make(chan struct{}), conns: make(map[net.Conn]struct{})}
	go t.acceptLoop(conn, remotePort)
	return t, nil
}

func (t *Tunnel) acceptLoop(conn *Connection, remotePort int) {
	for {
		local, err := t.listener.Accept()
		if err != nil {
			return // listener closed — Close() was called
		}
		go t.serveOne(conn, remotePort, local)
	}
}

func (t *Tunnel) track(c net.Conn) {
	t.mu.Lock()
	t.conns[c] = struct{}{}
	t.mu.Unlock()
}

func (t *Tunnel) untrack(c net.Conn) {
	t.mu.Lock()
	delete(t.conns, c)
	t.mu.Unlock()
}

func (t *Tunnel) serveOne(conn *Connection, remotePort int, local net.Conn) {
	t.track(local)
	defer t.untrack(local)

	remote, err := conn.client.Dial("tcp", fmt.Sprintf("localhost:%d", remotePort))
	if err != nil {
		_ = local.Close()
		return
	}
	t.track(remote)
	defer t.untrack(remote)

	go func() {
		_, _ = io.Copy(remote, local)
		_ = remote.Close()
	}()
	_, _ = io.Copy(local, remote)
	_ = local.Close()
}

// Close stops accepting new connections, closes the listener, and closes
// every currently tracked in-flight local/remote socket — unblocking their
// io.Copy calls immediately rather than waiting for either peer to hang up
// on its own, so Close() doesn't leak goroutines behind a slow/idle forward.
func (t *Tunnel) Close() error {
	var listenerErr error
	t.closeOne.Do(func() {
		close(t.done)
		listenerErr = t.listener.Close()

		t.mu.Lock()
		conns := make([]net.Conn, 0, len(t.conns))
		for c := range t.conns {
			conns = append(conns, c)
		}
		t.mu.Unlock()
		for _, c := range conns {
			_ = c.Close()
		}
	})
	return listenerErr
}
