package collab

import (
	"sync"
	"time"
)

type serializedDebounceIntervalWorker struct {
	fn     func()
	jobs   chan struct{}
	done   chan struct{}
	closed bool
	mu     sync.Mutex
	wg     sync.WaitGroup
}

func newSerializedDebounceIntervalWorker(fn func(), delay time.Duration, tickerDelay time.Duration) *serializedDebounceIntervalWorker {
	var worker = &serializedDebounceIntervalWorker{
		fn:   fn,
		jobs: make(chan struct{}, 1),
		done: make(chan struct{}),
	}
	worker.wg.Go(func() {
		var ticker = time.NewTicker(tickerDelay)
		defer ticker.Stop()
		var timer *time.Timer
		var timerC <-chan time.Time
		for {
			select {
			case <-worker.done:
				if timer != nil {
					timer.Stop()
				}
				return
			case <-worker.jobs:
				if timer != nil {
					timer.Stop()
				}
				timer = time.NewTimer(delay)
				timerC = timer.C
			case <-ticker.C:
				if timer != nil {
					timer.Stop()
				}
				timerC = nil
				timer = nil
				fn()
			case <-timerC:
				timerC = nil
				timer = nil
				fn()
			}
		}
	})
	return worker
}

func (w *serializedDebounceIntervalWorker) do() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return false
	}
	select {
	case w.jobs <- struct{}{}:
	default:
		select {
		case <-w.jobs:
		default:
		}
		w.jobs <- struct{}{}
	}
	return true
}

func (w *serializedDebounceIntervalWorker) close() {
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.done)
	}
	w.mu.Unlock()
	w.wg.Wait()
}
