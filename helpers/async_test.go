package helpers

import (
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
