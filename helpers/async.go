package helpers

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var AsyncFilterError = errors.New("async filter error")

func AsyncCollect[T any](maxWorker int, workLen int, work func(i int) (T, error)) ([]T, error) {
	if maxWorker == 0 {
		maxWorker = GetMaxThreads()
	}
	if maxWorker > workLen {
		maxWorker = workLen
	}
	var resultsChan = make(chan T, maxWorker*2)
	var errorsChan = make(chan error, 1)
	var indexes = make(chan int, workLen)
	var wgWorkers sync.WaitGroup
	var wgProcessors sync.WaitGroup
	var results = make([]T, 0, workLen)
	var err error

	for range maxWorker {
		wgWorkers.Go(func() {
			var err error
			for i := range indexes {
				var result T
				if result, err = work(i); err != nil {
					if err == AsyncFilterError {
						continue
					}
					errorsChan <- err
					continue
				}
				resultsChan <- result
			}
		})
	}

	go func() {
		for i := range workLen {
			indexes <- i
		}
		close(indexes)
		wgWorkers.Wait()
		close(resultsChan)
		close(errorsChan)
	}()

	wgProcessors.Go(func() {
		for asyncErr := range errorsChan {
			if err == nil {
				err = fmt.Errorf("async error")
			}
			err = fmt.Errorf("%s: %w", asyncErr.Error(), err)
		}
	})
	wgProcessors.Go(func() {
		for result := range resultsChan {
			results = append(results, result)
		}
	})
	wgProcessors.Wait()

	return results, err

}

func AsyncWork(maxWorker int, workLen int, work func(i int) error) error {
	if maxWorker == 0 {
		maxWorker = GetMaxThreads()
	}
	if maxWorker > workLen {
		maxWorker = workLen
	}
	var errorsChan = make(chan error, 1)
	var indexes = make(chan int, workLen)
	var wgWorkers sync.WaitGroup
	var err error
	for range maxWorker {
		wgWorkers.Go(func() {
			var err error
			for i := range indexes {
				if err = work(i); err != nil {
					if err == AsyncFilterError {
						continue
					}
					errorsChan <- err
					continue
				}
			}
		})
	}
	go func() {
		for i := range workLen {
			indexes <- i
		}
		close(indexes)
		wgWorkers.Wait()
		close(errorsChan)
	}()
	for asyncErr := range errorsChan {
		if err == nil {
			err = fmt.Errorf("async error")
		}
		err = fmt.Errorf("%s: %w", asyncErr.Error(), err)
	}

	return err
}

type SerializedDebounceWorker[T any] struct {
	fn     func(T)
	jobs   chan T
	done   chan struct{}
	closed bool
	mu     sync.Mutex
	wg     sync.WaitGroup
}

func NewSerializedDebounceWorker[T any](fn func(T), delay time.Duration) *SerializedDebounceWorker[T] {
	var worker = &SerializedDebounceWorker[T]{
		fn:   fn,
		jobs: make(chan T, 1),
		done: make(chan struct{}),
	}
	worker.wg.Go(func() {
		var timer *time.Timer
		var timerC <-chan time.Time
		var arg T
		var hasArg bool
		for {
			select {
			case <-worker.done:
				if timer != nil {
					timer.Stop()
				}
				return
			case arg = <-worker.jobs:
				hasArg = true
				if timer != nil {
					timer.Stop()
				}
				timer = time.NewTimer(delay)
				timerC = timer.C
			case <-timerC:
				timerC = nil
				timer = nil
				if hasArg {
					fn(arg)
					hasArg = false
				}
			}
		}
	})
	return worker
}

func (w *SerializedDebounceWorker[T]) Do(arg T) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return false
	}
	select {
	case w.jobs <- arg:
	default:
		select {
		case <-w.jobs:
		default:
		}
		w.jobs <- arg
	}
	return true
}

func (w *SerializedDebounceWorker[T]) Close() {
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.done)
	}
	w.mu.Unlock()
	w.wg.Wait()
}
