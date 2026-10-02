package mcpserver

import (
	"strings"
	"sync"
)

// LocalSignalBus is an in-process SignalBus (single replica, or the shared
// "NATS" of two-replica unit tests). Handlers run synchronously on the
// publisher's goroutine in a fresh goroutine each, like an async broker.
type LocalSignalBus struct {
	mu   sync.Mutex
	next int
	subs map[int]localSub
	wg   sync.WaitGroup
}

type localSub struct {
	pattern string
	fn      func(string, []byte)
}

func NewLocalSignalBus() *LocalSignalBus { return &LocalSignalBus{subs: map[int]localSub{}} }

func (b *LocalSignalBus) Publish(subject string, data []byte) error {
	b.mu.Lock()
	var fns []func(string, []byte)
	for _, s := range b.subs {
		if subjectMatches(s.pattern, subject) {
			fns = append(fns, s.fn)
		}
	}
	b.mu.Unlock()
	for _, fn := range fns {
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			fn(subject, append([]byte(nil), data...))
		}()
	}
	return nil
}

func (b *LocalSignalBus) Subscribe(pattern string, fn func(string, []byte)) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	id := b.next
	b.subs[id] = localSub{pattern, fn}
	return func() {
		b.mu.Lock()
		delete(b.subs, id)
		b.mu.Unlock()
	}, nil
}

// Wait blocks until every delivered handler returned (tests).
func (b *LocalSignalBus) Wait() { b.wg.Wait() }

// subjectMatches implements NATS wildcards: "*" one token, ">" the rest.
func subjectMatches(pattern, subject string) bool {
	p, s := strings.Split(pattern, "."), strings.Split(subject, ".")
	for i, tok := range p {
		if tok == ">" {
			return len(s) > i
		}
		if i >= len(s) || (tok != "*" && tok != s[i]) {
			return false
		}
	}
	return len(p) == len(s)
}

var _ SignalBus = (*LocalSignalBus)(nil)
