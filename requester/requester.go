package requester

import (
	"os"
	"sync"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6  os.FileInfo

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Backend

// Backend describes the interface for making a request to some file system
type Backend interface {
	ReadFile(path string) ([]byte, error)
	ReadDir(path string) ([]os.FileInfo, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Decrypter

// Decrypt describes the interface for making a request to decrypt the given
// ciphertext name with the given initialization vector
type Decrypter interface {
	DecryptName(cName string, iv []byte) (string, error)
}

type genCacheEntry struct {
	data      interface{}
	err       error
	lockerNum int
}

type Requester struct {
	numWorkers int

	backend   Backend
	decrypter Decrypter

	fileCache    *syncCache
	dirCache     *syncCache
	decryptCache *syncCache

	workQueue chan work
	lockers   []workTicket
}

type workType byte

const (
	workNone workType = iota
	workReadFile
	workReadDir
	workDecryptName
)

type work struct {
	requestType workType
	arg1        string
	arg2        []byte
	respChan    chan *workTicket
}

type workTicket struct {
	mtx sync.Mutex
	cv  *sync.Cond

	lockerNum int
}

func (wt *workTicket) Lock() {
	wt.mtx.Lock()
}

func (wt *workTicket) Unlock() {
	wt.mtx.Unlock()
}

func (wt *workTicket) Wait() {
	wt.cv.Wait()
}

const cacheErrRetryAttempts = 3
const initialCacheSize = 1000
const workQueueSize = 1000

func New(numWorkers int, backend Backend, decrypter Decrypter) *Requester {
	r := &Requester{
		numWorkers: numWorkers,

		backend:   backend,
		decrypter: decrypter,

		fileCache:    newSyncCache(initialCacheSize),
		dirCache:     newSyncCache(initialCacheSize),
		decryptCache: newSyncCache(initialCacheSize),

		workQueue: make(chan work, workQueueSize),
		lockers:   make([]workTicket, numWorkers+1),
	}
	for i := 0; i < numWorkers+1; i++ {
		l := &r.lockers[i]
		l.cv = sync.NewCond(&l.mtx)
		l.lockerNum = i
	}
	return r
}

func (r *Requester) Start() {
	for i := 1; i <= r.numWorkers; i++ {
		go r.worker(i)
	}
}

func (r *Requester) Stop() {
	close(r.workQueue)
}

func (r *Requester) ClearCache() {
	r.fileCache.ClearCache()
	r.dirCache.ClearCache()
	r.decryptCache.ClearCache()
}

func (r *Requester) performRequestAndCache(
	key string, num int, cache *syncCache, responseCh chan *workTicket,
	request func() (interface{}, error),
) {
	cacheEntry, loaded := cache.GetOrInsert(key, genCacheEntry{lockerNum: num})
	if !loaded {
		wt := &r.lockers[num]
		responseCh <- wt
		wt.Lock()

		data, err := request()
		cache.Set(key, genCacheEntry{data: data, err: err})

		wt.cv.Broadcast()
		wt.Unlock()
	} else {
		if cacheEntry.data != nil || cacheEntry.err != nil {
			// The work has been done, so just use what's in the cache.
			close(responseCh)
		} else {
			// Otherwise, send the current work ticket for the work.
			wt := &r.lockers[cacheEntry.lockerNum]
			responseCh <- wt
		}
	}
}

func (r *Requester) worker(num int) {
	for {
		w, ok := <-r.workQueue
		switch w.requestType {
		case workReadFile:
			r.performRequestAndCache(w.arg1, num, r.fileCache, w.respChan,
				func() (interface{}, error) {
					fileData, err := r.backend.ReadFile(w.arg1)
					if len(fileData) > 0 {
						return fileData, err
					}
					return nil, err
				},
			)
		case workReadDir:
			r.performRequestAndCache(w.arg1, num, r.dirCache, w.respChan,
				func() (interface{}, error) {
					dirData, err := r.backend.ReadDir(w.arg1)
					if len(dirData) > 0 {
						return dirData, err
					}
					return nil, err
				},
			)
		case workDecryptName:
			key := complexKey(w.arg1, w.arg2)
			r.performRequestAndCache(key, num, r.decryptCache, w.respChan,
				func() (interface{}, error) {
					decryptedName, err := r.decrypter.DecryptName(w.arg1, w.arg2)
					if len(decryptedName) > 0 {
						return decryptedName, err
					}
					return nil, err
				},
			)
		case workNone:
		}
		if !ok {
			return
		}
	}
}
