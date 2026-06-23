package app

import (
	"context"
	"sync"
)

type hookOnPreStartup = func(ctx context.Context) (func(ctx context.Context), error)
type hookOnPostStartup = func(ctx context.Context, wg *sync.WaitGroup) error
type hookOnPreShutdown = func(ctx context.Context)
type hookOnPostShutdown = func(ctx context.Context)
type hooksCleanup = func(ctx context.Context)

type Hooks struct {
	cleanup        []hooksCleanup
	onPreStartup   []hookOnPreStartup
	onPostStartup  []hookOnPostStartup
	onPreShutdown  []hookOnPreShutdown
	onPostShutdown []hookOnPostShutdown
}

func newHooks() *Hooks {
	return &Hooks{
		cleanup:        make([]hooksCleanup, 0, 10),
		onPreStartup:   make([]hookOnPreStartup, 0, 10),
		onPostStartup:  make([]hookOnPostStartup, 0, 5),
		onPreShutdown:  make([]hookOnPreShutdown, 0, 5),
		onPostShutdown: make([]hookOnPostShutdown, 0, 5),
	}
}

func (h *Hooks) OnPreStartup(handler func(ctx context.Context) (func(ctx context.Context), error)) {
	if handler == nil {
		return
	}
	h.onPreStartup = append(h.onPreStartup, handler)
}

func (h *Hooks) OnPostStartup(handler func(ctx context.Context, wg *sync.WaitGroup) error) {
	if handler == nil {
		return
	}
	h.onPostStartup = append(h.onPostStartup, handler)
}

func (h *Hooks) OnPreShutdown(handler func(ctx context.Context)) {
	if handler == nil {
		return
	}
	h.onPreShutdown = append(h.onPreShutdown, handler)
}

func (h *Hooks) OnPostShutdown(handler func(ctx context.Context)) {
	if handler == nil {
		return
	}
	h.onPostShutdown = append(h.onPostShutdown, handler)
}
