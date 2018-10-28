// Package queue provides components to manage a queue of songs in a variety of formats
// and provide them as readable resources to consumers.
package queue

import (
	"io"
	"log"
	"sync"

	"github.com/VivaLaPanda/uta-stream/queue/auto"
	"github.com/VivaLaPanda/uta-stream/resource/cache"
)

type Queue struct {
	fifo         []string
	lock         *sync.Mutex
	autoq        *auto.AQEngine
	cache        *cache.Cache
	AutoqEnabled bool
}

// NeqQueue will return a queue structure with the provided autoq engine and cache
// attached. enableAutoq will determine whether a Pop will attempt to fetch
// from the autoq.
func NewQueue(aqEngine *auto.AQEngine, cache *cache.Cache, enableAutoq bool) *Queue {
	return &Queue{
		lock:         &sync.Mutex{},
		autoq:        aqEngine,
		AutoqEnabled: enableAutoq,
		cache:        cache}
}

// Pop returns the audio resource next in the queue along with state flags.
func (q *Queue) Pop() (ipfsPath string, songReader io.Reader, emptyq bool, fromAuto bool) {
	// If there is nothing to queue and we have autoq enabled,
	// get from autoq. If autoq gives us an empty string (no audio to play)
	// or autoq is off, return that the queue is empty
	fromAuto = false
	if len(q.fifo) == 0 {
		if q.AutoqEnabled {
			fromAuto = true
			ipfsPath = q.autoq.Vpop()
			if ipfsPath == "" {
				return "", nil, true, fromAuto
			}

			// TODO: If resource is IPFS but can't be fetched this blocks, effectivelly
			// killing the server. Fix this.
			ipfsPath, songReader, err := q.cache.HardResolve(ipfsPath)
			if err != nil {
				log.Printf("Issue when resolving resource from AutoQ. Err: %v\n", err)
				return "", nil, true, fromAuto
			}

			return ipfsPath, songReader, false, fromAuto
		} else {
			return "", nil, true, fromAuto
		}
	}

	q.lock.Lock()
	// Top (just get next element, don't remove it)
	resourceID := q.fifo[0]
	// Discard top element
	q.fifo = q.fifo[1:]
	q.lock.Unlock()

	// Resolve the resource ID in the queue
	ipfsPath, songReader, err := q.cache.HardResolve(resourceID)
	if err != nil {
		log.Printf("Issue when resolving resource from Queue. Err: %v\n", err)
		return "", nil, true, fromAuto
	}

	return ipfsPath, songReader, false, fromAuto
}

// IsEmpty returns a boolean indicating whether the queue should be considered empty
// given the state of the real queue and autoq
func (q *Queue) IsEmpty() bool {
	if len(q.fifo) == 0 {
		if !q.AutoqEnabled {
			return true
		}
	}

	return false
}

// Add the provided song to the queue at the back
func (q *Queue) AddToQueue(ipfsPath string) {
	q.lock.Lock()
	q.fifo = append(q.fifo, ipfsPath)
	q.lock.Unlock()
}

// Add the provided song to the queue at the front
func (q *Queue) PlayNext(ipfsPath string) {
	q.lock.Lock()
	q.fifo = append([]string{ipfsPath}, q.fifo...)
	q.lock.Unlock()
}

// Remove all items from the queue. Will not dump the encoder (current song)
func (q *Queue) Dump() {
	q.lock.Lock()
	q.fifo = make([]string, 0)
	q.lock.Unlock()
}

// Length returns the length of the real queue
func (q *Queue) Length() int {
	return len(q.fifo)
}

// Get queue returns a copy of the real queue, and while it does so
// attempts to resolve any placeholders
func (q *Queue) GetQueue() []string {
	// Go through the queue and try to resolve any placeholders
	q.lock.Lock()
	indexesToDelete := make([]int, 0)
	for idx, elem := range q.fifo {
		ipfsPath, err := q.cache.SoftResolve(elem)
		if err != nil {
			log.Printf("unresolvable resource in queue, removing now.")
			indexesToDelete = append(indexesToDelete, idx)
		} else {
			if ipfsPath != "" {
				q.fifo[idx] = ipfsPath
			}
		}
	}
	for _, elem := range indexesToDelete {
		q.fifo = remove(q.fifo, elem)
	}
	q.lock.Unlock()

	// Make a copy so whoever is reading this can't write it
	qCopy := make([]string, len(q.fifo))
	q.lock.Lock()
	copy(qCopy, q.fifo)
	q.lock.Unlock()

	return qCopy
}

func remove(s []string, i int) []string {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

// Used as a gateway to let the autoq know a song was played.
// learnFrom being false indicates you want to let the autoq know you finished
// the last song (so it doesn't suggest it again), but you *don't* want to train it
// off what you just finished
func (q *Queue) NotifyDone(ipfsPath string, learnFrom bool) {
	q.autoq.NotifyPlayed(ipfsPath, learnFrom)
}
