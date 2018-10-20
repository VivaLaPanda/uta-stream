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

// Make a new q structure. allowChainbreak will make the autoq more random
func NewQueue(aqEngine *auto.AQEngine, cache *cache.Cache, enableAutoq bool) *Queue {
	return &Queue{
		lock:         &sync.Mutex{},
		autoq:        aqEngine,
		AutoqEnabled: enableAutoq,
		cache:        cache}
}

// Returns the audio resource next in the queue
func (q *Queue) Pop() (ipfsPath string, songReader io.Reader, emptyq bool) {
	// If there is nothing to queue and we have autoq enabled,
	// get from autoq. If autoq gives us an empty string (no audio to play)
	// or autoq is off, return that the queue is empty
	if len(q.fifo) == 0 {
		if q.AutoqEnabled {
			ipfsPath = q.autoq.Vpop()
			if ipfsPath == "" {
				return "", nil, true
			}

			ipfsPath, songReader, err := q.cache.HardResolve(ipfsPath)
			if err != nil {
				log.Printf("Issue when resolving resource from AutoQ. Err: %v\n", err)
				return "", nil, true
			}

			return ipfsPath, songReader, false
		} else {
			return "", nil, true
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
		return "", nil, true
	}

	return ipfsPath, songReader, false
}

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

// Remove all items from the queue. Will not dump the encoder (current and next song)
func (q *Queue) Dump() {
	q.lock.Lock()
	q.fifo = make([]string, 0)
	q.lock.Unlock()
}

func (q *Queue) Length() int {
	return len(q.fifo)
}

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

// Used as a gateway to let the autoq know a song was played. For training the
// qutoqueue
func (q *Queue) NotifyDone(ipfsPath string) {
	q.autoq.NotifyPlayed(ipfsPath)
}
