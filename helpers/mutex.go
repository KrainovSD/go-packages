package helpers

import "sync"

type MutexMap struct {
	mutexes map[int]*sync.Mutex
	global  sync.Mutex
}

func CreateMutexMap(length int) MutexMap {
	return MutexMap{
		mutexes: make(map[int]*sync.Mutex, length),
		global:  sync.Mutex{},
	}
}

func (m *MutexMap) Get(key int) *sync.Mutex {
	var mutex *sync.Mutex

	m.global.Lock()
	if mutex, ok := m.mutexes[key]; ok {
		m.global.Unlock()
		return mutex
	}
	mutex = &sync.Mutex{}
	m.mutexes[key] = mutex
	m.global.Unlock()
	return mutex
}
