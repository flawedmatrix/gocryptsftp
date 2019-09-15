package requester

import "sync"

type syncCache struct {
	m    map[string]genCacheEntry
	lock sync.RWMutex
}

func newSyncCache(startingSize int) *syncCache {
	return &syncCache{
		m: make(map[string]genCacheEntry, startingSize),
	}
}

// ClearCache empties the cache in a thread-safe way
func (s *syncCache) ClearCache() {
	keysToDelete := []string{}
	s.lock.RLock()
	for k := range s.m {
		keysToDelete = append(keysToDelete, k)
	}
	s.lock.RUnlock()
	s.lock.Lock()
	for _, k := range keysToDelete {
		delete(s.m, k)
	}
	s.lock.Unlock()
}

// Get retrieves the value associated with the key from the cache in a
// thread-safe way
func (s *syncCache) Get(key string) (genCacheEntry, bool) {
	s.lock.RLock()
	cacheEntry, found := s.m[key]
	s.lock.RUnlock()
	return cacheEntry, found
}

// GetOrInsert tries to retrieve the value associated with the key from the
// cache. It returns true if the value was successfully retrieved from the
// cache. If the key did not previously exist, then it stores the provided value
// into the cache and returns false.
func (s *syncCache) GetOrInsert(key string, value genCacheEntry) (genCacheEntry, bool) {
	s.lock.Lock()
	cacheEntry, loaded := s.m[key]
	if !loaded {
		s.m[key] = value
	}
	s.lock.Unlock()
	return cacheEntry, loaded
}

// Set sets the the value associated with the key in the cache in a thread-safe
// way
func (s *syncCache) Set(key string, value genCacheEntry) {
	s.lock.Lock()
	s.m[key] = value
	s.lock.Unlock()
}

// Delete deletes the the value associated with the key from the cache in a
// thread-safe way
func (s *syncCache) Delete(key string) {
	s.lock.Lock()
	delete(s.m, key)
	s.lock.Unlock()
}
