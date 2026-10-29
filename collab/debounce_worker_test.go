package collab

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDebounceWorkerCoalesces(t *testing.T) {
	var count atomic.Int32
	var w = newSerializedDebounceIntervalWorker(func() { count.Add(1) }, 50*time.Millisecond, time.Hour)
	defer w.close()
	for range 10 {
		w.do()
	}
	time.Sleep(150 * time.Millisecond)
	if got := count.Load(); got != 1 {
		t.Fatalf("expected 1 call after coalescing, got %d", got)
	}
}

func TestDebounceWorkerFiresAfterDelay(t *testing.T) {
	var fired = make(chan struct{}, 1)
	var w = newSerializedDebounceIntervalWorker(func() { fired <- struct{}{} }, 40*time.Millisecond, time.Hour)
	defer w.close()
	w.do()
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("fn did not fire after the debounce delay")
	}
}

func TestDebounceWorkerResetsOnNewJob(t *testing.T) {
	var count atomic.Int32
	var w = newSerializedDebounceIntervalWorker(func() { count.Add(1) }, 60*time.Millisecond, time.Hour)
	defer w.close()
	w.do()
	time.Sleep(30 * time.Millisecond)
	w.do()
	time.Sleep(40 * time.Millisecond)
	if got := count.Load(); got != 0 {
		t.Fatalf("debounce timer was not reset: got %d calls before the full delay", got)
	}
	time.Sleep(80 * time.Millisecond)
	if got := count.Load(); got != 1 {
		t.Fatalf("expected exactly 1 call after the reset delay, got %d", got)
	}
}

func TestDebounceWorkerPeriodicTicker(t *testing.T) {
	var count atomic.Int32
	var w = newSerializedDebounceIntervalWorker(func() { count.Add(1) }, time.Hour, 40*time.Millisecond)
	defer w.close()
	time.Sleep(150 * time.Millisecond)
	if got := count.Load(); got < 2 {
		t.Fatalf("expected periodic calls from ticker, got %d", got)
	}
}

func TestDebounceWorkerDoAfterCloseReturnsFalse(t *testing.T) {
	var w = newSerializedDebounceIntervalWorker(func() {}, 10*time.Millisecond, time.Hour)
	w.close()
	if w.do() {
		t.Fatal("do() after close must return false")
	}
}

func TestDebounceWorkerCloseIsIdempotent(t *testing.T) {
	var w = newSerializedDebounceIntervalWorker(func() {}, 10*time.Millisecond, time.Hour)
	w.close()
	w.close()
}

func TestDebounceWorkerCloseWaitsForInFlight(t *testing.T) {
	var entered = make(chan struct{})
	var release = make(chan struct{})
	var w = newSerializedDebounceIntervalWorker(func() {
		close(entered)
		<-release
	}, 10*time.Millisecond, time.Hour)
	w.do()
	<-entered
	var closed = make(chan struct{})
	go func() { w.close(); close(closed) }()
	select {
	case <-closed:
		t.Fatal("close must not complete while fn is still running")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close did not complete after fn finished")
	}
}

func TestDebounceWorkerConcurrentDoClose(t *testing.T) {
	var w = newSerializedDebounceIntervalWorker(func() {}, time.Millisecond, time.Hour)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 2000 {
				w.do()
			}
		})
	}
	wg.Wait()
	var done = make(chan struct{})
	go func() { w.close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("close hung: do drain race")
	}
}
