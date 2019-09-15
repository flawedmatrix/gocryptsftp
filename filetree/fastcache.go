package filetree

import (
	"sync"
)

type DirMapping struct {
	PlaintextPath  string
	CiphertextPath string

	prev *DirMapping
	next *DirMapping
}

type FastCache struct {
	storage map[string]*DirMapping
	lock    sync.Mutex

	cacheSize int
	count     int

	head *DirMapping
	last *DirMapping
}

func NewFastCache(cacheSize int) *FastCache {
	return &FastCache{
		storage:   make(map[string]*DirMapping, cacheSize*2),
		cacheSize: cacheSize,
	}
}

// Find tries to find the given plaintext directory in the cache, returning
// false if it is not found.
// Because FastCache is an LRU cache, successfully resolving the entry will
// make it the most recently used entry in the cache.
func (f *FastCache) Find(pPath string) (DirMapping, bool) {
	f.lock.Lock()
	defer f.lock.Unlock()

	d, found := f.storage[pPath]
	if !found {
		return DirMapping{}, false
	}
	f.refreshNode(d)
	return *d, found
}

// refreshNode makes this node the most recently used by putting it at the
// end of the linked hash map
func (f *FastCache) refreshNode(d *DirMapping) {
	if d.next != nil {
		if d.prev != nil {
			d.prev.next = d.next
		} else {
			f.head = d.next
		}
		d.prev = f.last
		d.next = nil
		f.last.next = d
		f.last = d
	}
}

// Clear removes all entries from the fast cache and the tree cache.
func (f *FastCache) Clear() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.head = nil
	f.last = nil
	f.storage = make(map[string]*DirMapping, f.cacheSize*2)
	f.count = 0
}

// Store stores the given directory mapping in the fast cache.
// The cache behaves like an LRU cache with a capacity of 16 entries. When
// storing an entry would cause the cache to exceed its capacity, it will evict
// the least recently used item in the cache.
//
// This could technically be used for files, but it's probably more efficient
// to use for directories.
func (f *FastCache) Store(pPath, cPath string) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if d, found := f.storage[pPath]; found {
		f.refreshNode(d)
		d.CiphertextPath = cPath
		return
	}

	f.count++
	if f.count > f.cacheSize {
		// Reuse old allocated resource if we're at the cache limit
		oldHeadPath := f.head.PlaintextPath
		d := f.head
		f.refreshNode(d)
		d.PlaintextPath = pPath
		d.CiphertextPath = cPath
		f.count = f.cacheSize
		delete(f.storage, oldHeadPath)
		f.storage[pPath] = d
	} else {
		newNode := &DirMapping{
			CiphertextPath: cPath,
			PlaintextPath:  pPath,
		}

		f.storage[pPath] = newNode

		if f.head == nil {
			f.head = newNode
			f.last = newNode
		} else {
			newNode.prev = f.last
			f.last.next = newNode
			f.last = newNode
		}
	}
}
