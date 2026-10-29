package collab

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KrainovSD/go-packages/helpers"
)

func TestMutexStoreLockUnlock(t *testing.T) {
	var s = newMutexStore()
	if err := helpers.LimitWork(context.Background(), 1*time.Second, func(ctx context.Context) error {
		s.lock("a")
		s.unlock("a")
		return nil
	}); err != nil {
		t.Fatalf("mutex not released")
	}
}

func TestMutexStoreUnlockUnknownPanics(t *testing.T) {
	var s = newMutexStore()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on unlock of unknown key")
		}
	}()
	s.unlock("missing")
}

func TestMutexStoreMutualExclusion(t *testing.T) {
	var s = newMutexStore()
	var active, maxActive atomic.Int32
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			for range 200 {
				s.lock("a")
				var cur = active.Add(1)
				for {
					var prev = maxActive.Load()
					if cur <= prev || maxActive.CompareAndSwap(prev, cur) {
						break
					}
				}
				time.Sleep(time.Microsecond)
				active.Add(-1)
				s.unlock("a")
			}
		})
	}
	waitTimeout(t, 5*time.Second, wg.Wait)
	if got := maxActive.Load(); got > 1 {
		t.Fatalf("mutual exclusion violated: max concurrent holders = %d", got)
	}
}

func TestMutexStoreContentionNoDeadlock(t *testing.T) {
	var s = newMutexStore()
	var wg sync.WaitGroup
	wg.Go(func() {
		s.lock("a")
		time.Sleep(20 * time.Millisecond)
		s.unlock("a")
	})
	time.Sleep(5 * time.Millisecond)
	wg.Go(func() {
		s.lock("a")
		s.unlock("a")
	})
	waitTimeout(t, time.Second, wg.Wait)
}

func TestMutexStoreManyNamesManyGoroutines(t *testing.T) {
	var s = newMutexStore()
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for range 300 {
				var name = fmt.Sprintf("k%d", n%4)
				s.lock(name)
				s.unlock(name)
			}
		}(i)
	}
	waitTimeout(t, 2*time.Second, wg.Wait)
}
