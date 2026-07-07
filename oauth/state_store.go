package oauth

import "sync"

type StateStore struct {
	store map[string]string
	mutex sync.RWMutex
}

var stateStore = StateStore{
	store: map[string]string{},
	mutex: sync.RWMutex{},
}

func (s *StateStore) Get(key string) string {
	s.mutex.Lock()
	result := s.store[key]
	delete(s.store, key)
	s.mutex.Unlock()

	return result
}
func (s *StateStore) Set(key string, value string) {
	s.mutex.Lock()
	s.store[key] = value
	s.mutex.Unlock()
}
