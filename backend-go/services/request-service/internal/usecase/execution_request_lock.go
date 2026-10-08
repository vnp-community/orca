package usecase

import "sync"

// keyedLock serialises work per Request inside one process. Across replicas the execution loop relies on
// idempotent task-service calls (a second Execute on a running task is ALREADY_IN_PROGRESS), so this only
// removes the common case of two triggers racing in the same instance.
type keyedLock struct {
	mu    sync.Mutex
	locks map[string]*keyedLockEntry
}

type keyedLockEntry struct {
	mu   sync.Mutex
	refs int
}

// Lock blocks until key is free and returns the unlock function.
func (k *keyedLock) Lock(key string) func() {
	k.mu.Lock()
	if k.locks == nil {
		k.locks = make(map[string]*keyedLockEntry)
	}
	e := k.locks[key]
	if e == nil {
		e = &keyedLockEntry{}
		k.locks[key] = e
	}
	e.refs++
	k.mu.Unlock()

	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		k.mu.Lock()
		if e.refs--; e.refs == 0 {
			delete(k.locks, key)
		}
		k.mu.Unlock()
	}
}
