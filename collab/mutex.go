package collab

import (
	"sync"
)

type mutexSource struct {
	mu     sync.Mutex
	owners int
}

type mutexStore struct {
	mu      sync.Mutex
	sources map[string]*mutexSource
}

func newMutexStore() *mutexStore {
	return &mutexStore{sources: make(map[string]*mutexSource, 1024)}
}

func (s *mutexStore) lock(name string) {
	s.mu.Lock()
	var source, ok = s.sources[name]
	if !ok {
		source = &mutexSource{}
		s.sources[name] = source
	}
	source.owners++
	s.mu.Unlock()
	source.mu.Lock()
}

func (s *mutexStore) unlock(name string) {
	s.mu.Lock()
	var source, ok = s.sources[name]
	if !ok {
		s.mu.Unlock()
		panic("collab: mutex not exists for key " + name)
	}
	source.owners--
	if source.owners == 0 && s.sources[name] == source {
		delete(s.sources, name)
	}
	s.mu.Unlock()
	source.mu.Unlock()
}
