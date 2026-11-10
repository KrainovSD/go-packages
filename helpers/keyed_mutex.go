package helpers

import (
	"fmt"
	"sync"
)

type mutex struct {
	mu   sync.Mutex
	used int
}

type KeyedMutex[T comparable] struct {
	mu  sync.Mutex
	mus map[T]*mutex
}

func NewKeyedMutex[T comparable]() *KeyedMutex[T] {
	return &KeyedMutex[T]{
		mus: make(map[T]*mutex),
	}
}

func (m *KeyedMutex[T]) Lock(key T) {
	m.mu.Lock()
	var raw, ok = m.mus[key]
	if !ok {
		raw = &mutex{}
		m.mus[key] = raw
	}
	raw.used++
	m.mu.Unlock()
	raw.mu.Lock()
}

func (m *KeyedMutex[T]) Unlock(key T) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var raw, ok = m.mus[key]
	if !ok {
		panic(fmt.Sprintf("mutex for key %v not found", key))
	}
	raw.mu.Unlock()
	raw.used--
	if raw.used == 0 {
		delete(m.mus, key)
	}
}
