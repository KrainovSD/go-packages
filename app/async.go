package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	onPanic func(err error, stack []byte, original any)
	mu      sync.RWMutex
	stopped bool
	logger  *slog.Logger
}

func NewBackgroundWorker(capacity int, workers int, onPanic func(err error, stack []byte, original any), logger *slog.Logger) *BackgroundWorker {
	var bg = &BackgroundWorker{
		tasks:   make(chan backgroundTask, capacity),
		onPanic: onPanic,
		logger:  logger,
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

func (bg *BackgroundWorker) handlePanic(value any) {
	var stack = debug.Stack()
	var err error
	switch e := value.(type) {
	case error:
		err = e
	case string:
		err = errors.New(e)
	default:
		err = fmt.Errorf("%v", e)
	}
	if bg.onPanic != nil {
		bg.onPanic(err, stack, value)
	} else {
		bg.logger.LogAttrs(context.Background(), slog.LevelError, "BGWORKER PANIC", slog.String("err", err.Error()), slog.String("stack", string(stack)))
	}
}

func (bg *BackgroundWorker) run(task backgroundTask) {
	defer func() {
		if value := recover(); value != nil {
			bg.handlePanic(value)
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
				bg.handlePanic(value)
			}
		}()
		fn()
	})
}
