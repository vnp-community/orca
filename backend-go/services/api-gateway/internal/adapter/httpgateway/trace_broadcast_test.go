package httpgateway

import (
	"sync"
	"testing"
	"time"
)

func TestTraceBroadcast_PublishReachesAllSubscribers(t *testing.T) {
	b := NewTraceBroadcast()
	ch1, unsub1 := b.Subscribe()
	defer unsub1()
	ch2, unsub2 := b.Subscribe()
	defer unsub2()

	msg := []byte(`{"id":"1"}`)
	b.Publish(msg)

	for i, ch := range []<-chan []byte{ch1, ch2} {
		select {
		case got := <-ch:
			if string(got) != string(msg) {
				t.Errorf("subscriber %d: got %q, want %q", i, got, msg)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: did not receive published message", i)
		}
	}
}

func TestTraceBroadcast_SlowSubscriberDropsWithoutBlockingOthers(t *testing.T) {
	b := NewTraceBroadcast()
	slowCh, unsubSlow := b.Subscribe() // never drained — buffer of 16 fills up
	defer unsubSlow()
	fastCh, unsubFast := b.Subscribe()
	defer unsubFast()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 32; i++ { // more than the 16-slot buffer
			b.Publish([]byte("msg"))
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publish() blocked/deadlocked on a slow subscriber")
	}

	select {
	case <-fastCh:
	case <-time.After(time.Second):
		t.Fatal("fast subscriber never received a message despite the slow one filling up")
	}

	_ = slowCh // intentionally never drained
}

func TestTraceBroadcast_UnsubscribeRemovesFromRegistry(t *testing.T) {
	b := NewTraceBroadcast()
	for i := 0; i < 5; i++ {
		_, unsubscribe := b.Subscribe()
		b.mu.Lock()
		n := len(b.subs)
		b.mu.Unlock()
		if n != 1 {
			t.Fatalf("iteration %d: expected 1 subscriber before unsubscribe, got %d", i, n)
		}
		unsubscribe()
		b.mu.Lock()
		n = len(b.subs)
		b.mu.Unlock()
		if n != 0 {
			t.Fatalf("iteration %d: expected 0 subscribers after unsubscribe, got %d", i, n)
		}
	}
}

func TestTraceBroadcast_UnsubscribeIsIdempotent(t *testing.T) {
	b := NewTraceBroadcast()
	_, unsubscribe := b.Subscribe()

	unsubscribe()
	unsubscribe() // must not panic (double-close)
}

func TestTraceBroadcast_ConcurrentSubscribePublish(t *testing.T) {
	b := NewTraceBroadcast()
	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, unsubscribe := b.Subscribe()
			defer unsubscribe()
			for j := 0; j < 10; j++ {
				select {
				case <-ch:
				default:
				}
			}
		}()
	}

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				b.Publish([]byte("x"))
			}
		}()
	}

	wg.Wait()
}
