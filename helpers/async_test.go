package helpers

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSerializedDebounceWorker(t *testing.T) {
	t.Run("correct", func(t *testing.T) {
		var counter atomic.Int32
		var worker = NewSerializedDebounceWorker(func(i int) {
			counter.Add(int32(i))
		}, 100*time.Millisecond)
		for i := range 100 {
			worker.Do(i)
		}
		time.Sleep(200 * time.Millisecond)
		for i := range 100 {
			worker.Do(i)
		}
		worker.Close()
		if counter.Load() != 99 {
			t.Errorf("expected counter is 99")
		}
	})
}

func TestSerializedDebounceWorkerConcurrentDoClose(t *testing.T) {
	var worker = NewSerializedDebounceWorker(func(int) {}, time.Millisecond)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for j := range 2000 {
				worker.Do(j)
			}
		})
	}
	wg.Wait()
	var done = make(chan struct{})
	go func() { worker.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung: Do drain race")
	}
}
