package requester

import (
	"errors"
	"fmt"
	"time"
)

func (r *Requester) makeRequest(requestType workType, arg1 string, arg2 []byte) (interface{}, error) {
	retryErrs := []string{}

	var cache *syncCache
	switch requestType {
	case workReadFile:
		cache = r.fileCache
	case workReadDir:
		cache = r.dirCache
	case workDecryptName:
		cache = r.decryptCache
	default:
		panic("Invalid request type")
	}
	key := arg1
	if len(arg2) > 0 {
		key = complexKey(arg1, arg2)
	}

	for i := 0; i < cacheErrRetryAttempts; i++ {
		// Return immediately if there is valid data in the cache, otherwise
		// make a new request.
		if cached, found := cache.Get(key); found {
			if cached.data != nil {
				return cached.data, nil
			}
		}
		respChan := make(chan *workTicket)
		r.workQueue <- work{
			requestType: requestType,
			arg1:        arg1,
			arg2:        arg2,
			respChan:    respChan,
		}
		select {
		case ticket, ok := <-respChan:
			cached, found := cache.Get(key)
			if !found {
				// When a ticket is sent back, an entry for the data must exist
				// in cache. If it got here, then for some reason the cache was wiped at
				// some point between receiving a valid ticket and trying to retrieve
				// from it. Therefore, we retry the request.
				retryErrs = append(retryErrs, "cache wiped before ticket could be handled")
				continue
			}
			if cached.data != nil {
				// The request completed before we even started waiting on it.
				return cached.data, nil
			}
			if !ok {
				// Closing the respChan means a ticket won't be sent back to wait on
				// because what we're looking for is already in cache. But if the
				// request got to this point then there was no data in cache.
				// Therefore we should retry the request.
				if cached.err != nil {
					retryErrs = append(retryErrs, "retrying because of cached error")
				} else {
					retryErrs = append(retryErrs, "empty data for path in cache")
				}
				cache.Delete(key)
				continue
			}

			// There are 4 possible cases here:
			// - We got a ticket back for a worker handling this request.
			// - We got a ticket back for a worker handling another request.
			// - We got a ticket corresponding to lockerNum 0 (which is left unused).
			// - We got a ticket back for a worker not handling any request.
			// In the first case, we'll just wait for the request to be done and
			// return the newly written cached data.
			// In the second case, we'll eat the loss from having to wait for a lock
			// for the other request, but when we wake up and retrieve from the
			// cache we should see the entry in the cache. Even if we end up
			// waiting on the request, when the other request finishes we'll wake
			// up as part of the condition variable, see that the ticket is done
			// yet no data is available, and retry.
			// In the third case, we quickly pass the lock since no worker should
			// be holding onto that lock, and we should see the entry in the cache.
			// If there is no data in the cache, since the ticket is never running
			// anything, we can retry.
			// In the last case, if the worker is not handling the request, then
			// we'll instantly acquire the lock. If the worker is not handling the
			// request, then either the requested item is in the cache or it's not
			// found. In either case, we'll skip waiting and just grab the data.
			//
			// For any of these cases, we also need to worry about a case in which the
			// cached data is somehow deleted while Wait() is happening, and also in
			// which it's somehow overwritten by another ticket, thereby invalidating
			// the result when this thread wakes up. To prevent this, we must check
			// that the ticket has the same lockerNum as our cached workload,
			// otherwise we must retry.
			ticket.Lock()
			for {
				cached, found = cache.Get(key)
				dataAvailable := cached.data != nil || cached.err != nil
				wrongLocker := ticket.lockerNum == 0 || cached.lockerNum != ticket.lockerNum

				if !found || dataAvailable || wrongLocker {
					break
				}
				ticket.Wait()
			}

			d, err := cached.data, cached.err
			ticket.Unlock()

			// We should never enter a situation where the cached data isn't found
			// by this point, so we need to retry.
			if !found {
				retryErrs = append(retryErrs, "cache entry deleted while waiting")
				continue
			}
			if d == nil && err == nil {
				retryErrs = append(retryErrs, "data missing by the time request completed")
				continue
			}

			if err != nil {
				return nil, err
			}
			return d, nil
		case <-time.After(20 * time.Second):
			return nil, errors.New("nonresponsive work queue")
		}
	}
	return nil, fmt.Errorf("somehow could not succeed after retries. Retry errors: %s", retryErrs)

}
