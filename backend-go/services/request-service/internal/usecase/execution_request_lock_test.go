package usecase

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestKeyedLock_SerialisesPerKey_NotAcrossKeys(t *testing.T) {
	var k keyedLock
	var inside, maxInside atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := k.Lock("same")
			n := inside.Add(1)
			for {
				cur := maxInside.Load()
				if n <= cur || maxInside.CompareAndSwap(cur, n) {
					break
				}
			}
			inside.Add(-1)
			unlock()
		}()
	}
	wg.Wait()
	if maxInside.Load() != 1 {
		t.Fatalf("two holders of the same key at once: %d", maxInside.Load())
	}
	unlockA := k.Lock("a")
	done := make(chan struct{})
	go func() { k.Lock("b")(); close(done) }()
	<-done // would deadlock if keys shared a lock
	unlockA()
	if len(k.locks) != 0 {
		t.Fatalf("entries must be released: %d", len(k.locks))
	}
}
