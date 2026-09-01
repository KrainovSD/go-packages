package app

import (
	"context"
	"sync"
	"time"
)

type BackgroundTask struct {
	Fn      func(ctx context.Context)
	Ctx     context.Context
	Timeout time.Duration
}

type BackgroundWorker struct {
	tasks   chan BackgroundTask
	workers sync.WaitGroup
	onPanic func(any)
	mu      sync.RWMutex
	stopped bool
}

func NewBackgroundWorker(capacity int, workers int, onPanic func(any)) *BackgroundWorker {
	var bg = &BackgroundWorker{
		tasks:   make(chan BackgroundTask, capacity),
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

func (bg *BackgroundWorker) run(task BackgroundTask) {
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

func (bg *BackgroundWorker) Do(task BackgroundTask) bool {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	if bg.stopped {
		return false
	}
	bg.tasks <- task
	return true
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
