package helpers

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestKeyedMutexSameKeyMutualExclusion(t *testing.T) {
	var km = NewKeyedMutex[string]()
	var inSection atomic.Int32
	var violated atomic.Bool
	var wg sync.WaitGroup
	for range 1024 {
		wg.Go(func() {
			km.Lock("room-1")
			if inSection.Add(1) != 1 {
				violated.Store(true)
			}
			time.Sleep(100 * time.Microsecond)
			inSection.Add(-1)
			km.Unlock("room-1")
		})
	}
	wg.Wait()
	if violated.Load() {
		t.Error("expected no concurrent entries into critical section")
	}
	if len(km.mus) != 0 {
		t.Errorf("expected empty mutexes map, got %d entries", len(km.mus))
	}
}

func TestKeyedMutexDifferentKeysNotBlocked(t *testing.T) {
	var km = NewKeyedMutex[int]()
	km.Lock(1)
	var locked = make(chan struct{})
	go func() {
		km.Lock(2)
		close(locked)
	}()
	select {
	case <-locked:
	case <-time.After(time.Second):
		t.Fatal("lock of key 2 is blocked by held lock of key 1")
	}
	km.Unlock(2)
	km.Unlock(1)
	if len(km.mus) != 0 {
		t.Errorf("expected empty mutexes map, got %d entries", len(km.mus))
	}
}

func TestKeyedMutexWaiterKeepsEntryAlive(t *testing.T) {
	var km = NewKeyedMutex[string]()
	km.Lock("doc-1")
	var acquired = make(chan struct{})
	go func() {
		km.Lock("doc-1")
		km.Unlock("doc-1")
		close(acquired)
	}()
	var deadline = time.After(time.Second)
	for {
		km.mu.Lock()
		var raw, ok = km.mus["doc-1"]
		var used int
		if ok {
			used = raw.used
		}
		km.mu.Unlock()
		if used == 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("waiter did not register within deadline")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	km.Unlock("doc-1")
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("waiter did not acquire after unlock")
	}
	if len(km.mus) != 0 {
		t.Errorf("expected empty mutexes map, got %d entries", len(km.mus))
	}
}

func TestKeyedMutexSequentialReuse(t *testing.T) {
	var km = NewKeyedMutex[string]()
	for i := range 100 {
		var key = fmt.Sprintf("key-%d", i%5)
		km.Lock(key)
		if len(km.mus) != 1 {
			t.Errorf("expected 1 entry while locked, got %d", len(km.mus))
		}
		km.Unlock(key)
	}
	if len(km.mus) != 0 {
		t.Errorf("expected empty mutexes map, got %d entries", len(km.mus))
	}
}

func TestKeyedMutexConcurrentMixedKeys(t *testing.T) {
	var km = NewKeyedMutex[int]()
	var inSection [4]atomic.Int32
	var violated atomic.Bool
	var wg sync.WaitGroup
	for i := range 512 {
		wg.Go(func() {
			for j := range 300 {
				var key = (i + j) % 4
				km.Lock(key)
				if inSection[key].Add(1) != 1 {
					violated.Store(true)
				}
				inSection[key].Add(-1)
				km.Unlock(key)
			}
		})
	}
	wg.Wait()
	if violated.Load() {
		t.Error("expected no concurrent entries into critical section of same key")
	}
	if len(km.mus) != 0 {
		t.Errorf("expected empty mutexes map, got %d entries", len(km.mus))
	}
}

func TestKeyedMutexUnlockMissingKeyPanics(t *testing.T) {
	var km = NewKeyedMutex[string]()
	defer func() {
		if recover() == nil {
			t.Error("expected panic on unlock of missing key")
		}
	}()
	km.Unlock("missing")
}
