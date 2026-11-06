package collab

import (
	"sync"
	"sync/atomic"
)

type notifier struct {
	mu      sync.RWMutex
	subs    map[*updateSub]struct{}
	version *atomic.Uint64
}

func newNotifier(version *atomic.Uint64) *notifier {
	return &notifier{
		subs:    make(map[*updateSub]struct{}),
		version: version,
	}
}

func (u *notifier) subscribe(fn func(version uint64)) (func(), bool) {
	var s = &updateSub{
		fn:   fn,
		wake: make(chan struct{}, 1),
		done: make(chan struct{}),
	}
	u.mu.Lock()
	if u.subs == nil {
		u.mu.Unlock()
		return func() {}, false
	}
	u.subs[s] = struct{}{}
	u.mu.Unlock()
	go s.loop(u.version)
	return func() {
		u.mu.Lock()
		delete(u.subs, s)
		u.mu.Unlock()
		s.stop()
	}, true
}

func (u *notifier) notify() {
	u.mu.RLock()
	defer u.mu.RUnlock()
	for s := range u.subs {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

func (u *notifier) close() {
	u.mu.Lock()
	var subs = u.subs
	u.subs = nil
	u.mu.Unlock()
	for s := range subs {
		s.stop()
	}
}

type updateSub struct {
	fn     func(version uint64)
	wake   chan struct{}
	done   chan struct{}
	closed atomic.Bool
}

func (s *updateSub) loop(version *atomic.Uint64) {
	var last uint64
	for {
		select {
		case <-s.wake:
			select {
			case <-s.done:
				return
			default:
			}
			var v = version.Load()
			if v > last {
				last = v
				s.fn(v)
			}
		case <-s.done:
			return
		}
	}
}

func (s *updateSub) stop() {
	if s.closed.CompareAndSwap(false, true) {
		close(s.done)
	}
}
