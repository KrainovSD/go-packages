package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"
)

type backgroundTask struct {
	Fn      func(ctx context.Context)
	Ctx     context.Context
	Timeout time.Duration
}

type BackgroundWorker struct {
	tasks   chan backgroundTask
	workers sync.WaitGroup
	onPanic func(any)
	mu      sync.RWMutex
	stopped bool
}

func NewBackgroundWorker(capacity int, workers int, onPanic func(any)) *BackgroundWorker {
	if onPanic == nil {
		onPanic = func(epanic any) {
			var stack = debug.Stack()
			var err error
			switch e := epanic.(type) {
			case error:
				err = e
			case string:
				err = errors.New(e)
			default:
				err = fmt.Errorf("%v", e)
			}
			log.Printf("BGWORKER PANIC: %v\n%s", err, stack)
		}
	}
	var bg = &BackgroundWorker{
		tasks:   make(chan backgroundTask, capacity),
		onPanic: onPanic,
	}
	for range workers {
		bg.workers.Go(func() {
			for task := range bg.tasks {
				bg.run(task)
			}
		})
	}
	return bg

}

func (bg *BackgroundWorker) run(task backgroundTask) {
	defer func() {
		if value := recover(); value != nil {
			if bg.onPanic != nil {
				bg.onPanic(value)
			}
		}
	}()
	var ctx = context.WithoutCancel(task.Ctx)
	var timeout = 5 * time.Minute
	if task.Timeout != 0 {
		timeout = task.Timeout
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()
	task.Fn(ctx)
}

func (bg *BackgroundWorker) Stop() {
	bg.mu.Lock()
	if !bg.stopped {
		bg.stopped = true
		close(bg.tasks)
	}
	bg.mu.Unlock()
	bg.workers.Wait()
}

// Send task to limited queue
func (bg *BackgroundWorker) Do(ctx context.Context, fn func(ctx context.Context), timeout time.Duration) bool {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	if bg.stopped {
		return false
	}
	bg.tasks <- backgroundTask{
		Ctx:     ctx,
		Fn:      fn,
		Timeout: timeout,
	}
	return true
}

// Send task to immediately execute
func (bg *BackgroundWorker) Go(fn func()) {
	bg.workers.Go(func() {
		defer func() {
			if value := recover(); value != nil {
				if bg.onPanic != nil {
					bg.onPanic(value)
				}
			}
		}()
		fn()
	})
}
