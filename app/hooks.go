package app

import (
	"context"
	"time"
)

type hookOnResourceInit struct {
	Name     string
	Duration time.Duration
	Fn       func(ctx context.Context) (func(context.Context), error)
}
type hookOnPreStartup = func() error
type hookOnPostStartup = func() error
type hookOnPreShutdown = func()
type hookOnPostShutdown = func()
type hooksCleanup struct {
	Duration time.Duration
	Fn       func(context.Context)
}

type Hooks struct {
	cleanup        []hooksCleanup
	onResourceInit []hookOnResourceInit
	onPreStartup   []hookOnPreStartup
	onPostStartup  []hookOnPostStartup
	onPreShutdown  []hookOnPreShutdown
	onPostShutdown []hookOnPostShutdown
}

func newHooks() *Hooks {
	return &Hooks{}
}

func (h *Hooks) OnResourceInit(name string, duration time.Duration, fn func(ctx context.Context) (func(context.Context), error)) {
	if fn == nil {
		return
	}
	h.onResourceInit = append(h.onResourceInit, hookOnResourceInit{
		Name:     name,
		Duration: duration,
		Fn:       fn,
	})

}

func (h *Hooks) OnPreStartup(fn func() error) {
	if fn == nil {
		return
	}
	h.onPreStartup = append(h.onPreStartup, fn)
}

func (h *Hooks) OnPostStartup(fn func() error) {
	if fn == nil {
		return
	}
	h.onPostStartup = append(h.onPostStartup, fn)
}

func (h *Hooks) OnPreShutdown(fn func()) {
	if fn == nil {
		return
	}
	h.onPreShutdown = append(h.onPreShutdown, fn)
}

func (h *Hooks) OnPostShutdown(fn func()) {
	if fn == nil {
		return
	}
	h.onPostShutdown = append(h.onPostShutdown, fn)
}
