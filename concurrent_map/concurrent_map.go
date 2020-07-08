package concurrent_map

import "sync"

type ConcurrentMap struct {
	Map map[string]interface{}
	sync.RWMutex
}

func New() *ConcurrentMap {
	m := new(ConcurrentMap)
	m.Map = make(map[string]interface{})
	return m
}

func (m *ConcurrentMap) Set(key string, value interface{}) {
	m.Lock()
	m.Map[key] = value
	m.Unlock()
}

func (m *ConcurrentMap) Get(key string) (interface{}, bool) {
	m.RLock()
	value, ok := m.Map[key]
	m.RUnlock()
	return value, ok
}
