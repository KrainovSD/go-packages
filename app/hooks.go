package app

import (
	"context"
)

type hookOnPreStartup = func(startupCtx context.Context) (func(shutdownCtx context.Context), error)
type hookOnPostStartup = func(shutdownSignal context.Context) error
type hookOnPreShutdown = func(shutdownCtx context.Context)
type hookOnPostShutdown = func(shutdownCtx context.Context)
type hooksCleanup = func(shutdownCtx context.Context)

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

func (h *Hooks) OnPreStartup(handler func(startupCtx context.Context) (func(shutdownCtx context.Context), error)) {
	if handler == nil {
		return
	}
	h.onPreStartup = append(h.onPreStartup, handler)
}

func (h *Hooks) OnPostStartup(handler func(shutdownSignal context.Context) error) {
	if handler == nil {
		return
	}
	h.onPostStartup = append(h.onPostStartup, handler)
}

func (h *Hooks) OnPreShutdown(handler func(shutdownCtx context.Context)) {
	if handler == nil {
		return
	}
	h.onPreShutdown = append(h.onPreShutdown, handler)
}

func (h *Hooks) OnPostShutdown(handler func(shutdownCtx context.Context)) {
	if handler == nil {
		return
	}
	h.onPostShutdown = append(h.onPostShutdown, handler)
}
