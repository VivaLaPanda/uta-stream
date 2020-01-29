// Package queue provides components to manage a queue of songs in a variety of formats
// and provide them as readable resources to consumers.
package queue

import (
	"io"
	"log"
	"sync"
	"time"

	"github.com/VivaLaPanda/uta-stream/queue/auto"
	"github.com/VivaLaPanda/uta-stream/resource"

	shell "github.com/ipfs/go-ipfs-api"
)

type Queue struct {
	fifo         []*resource.Song
	lock         *sync.Mutex
	autoq        *auto.AQEngine
	ipfs         *shell.Shell
	AutoqEnabled bool
}

// NeqQueue will return a queue structure with the provided autoq engine and cache
// attached. enableAutoq will determine whether a Pop will attempt to fetch
// from the autoq.
func NewQueue(aqEngine *auto.AQEngine, enableAutoq bool, ipfsUrl string) *Queue {
	q := &Queue{
		lock:         &sync.Mutex{},
		autoq:        aqEngine,
		AutoqEnabled: enableAutoq,
		ipfs:         shell.NewShell(ipfsUrl),
	}
	q.ipfs.SetTimeout(time.Minute * 15)

	return q
}

// Pop returns the audio resource next in the queue along with state flags.
func (q *Queue) Pop() (song *resource.Song, songReader io.ReadCloser, emptyq bool, fromAuto bool) {
	// If there is nothing to queue and we have autoq enabled,
	// get from autoq. If autoq gives us an empty string (no audio to play)
	// or autoq is off, return that the queue is empty
	fromAuto = false
	if len(q.fifo) == 0 {
		if q.AutoqEnabled {
			fromAuto = true
			song, err := q.autoq.Vpop()
			if err != nil {
				return nil, nil, true, fromAuto
			}

			// TODO: If resource is IPFS but can't be fetched this blocks, effectivelly
			// killing the server. Fix this.
			songReader, err = song.Resolve(q.ipfs)
			if err != nil {
				songData, _ := song.MarshalJSON()
				log.Printf("Issue when resolving song (%s). Err: %v\n", songData, err)
				return nil, nil, true, fromAuto
			}

			return song, songReader, false, fromAuto
		} else {
			return nil, nil, true, fromAuto
		}
	}

	q.lock.Lock()
	// Top (just get next element, don't remove it)
	song = q.fifo[0]
	// Discard top element
	q.fifo = q.fifo[1:]
	q.lock.Unlock()

	// Resolve the resource ID in the queue
	songReader, err := song.Resolve(q.ipfs)
	if err != nil {
		log.Printf("Issue when resolving resource from Queue. Err: %v\n", err)
		return q.Pop()
	}

	return song, songReader, false, fromAuto
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
func (q *Queue) AddToQueue(song *resource.Song) {
	q.lock.Lock()
	defer q.lock.Unlock()
	for _, elem := range q.fifo {
		if elem.URL() == song.URL() {
			log.Printf("Tried to queue a duplicate (%s), rejecting", song.Title)
			return
		}
	}
	q.fifo = append(q.fifo, song)
}

// Add the provided song to the queue at the front
func (q *Queue) PlayNext(song *resource.Song) {
	q.lock.Lock()
	log.Printf("Adding %s(%s) to queue", song.Title, song.URL())
	q.fifo = append([]*resource.Song{song}, q.fifo...)
	q.lock.Unlock()
}

// Remove all items from the queue. Will not dump the encoder (current song)
func (q *Queue) Dump() {
	q.lock.Lock()
	q.fifo = make([]*resource.Song, 0)
	q.lock.Unlock()
}

// Length returns the length of the real queue
func (q *Queue) Length() int {
	return len(q.fifo)
}

// Get queue returns a copy of the real queue, and while it does so
// attempts to resolve any placeholders
func (q *Queue) GetQueue() []*resource.Song {
	// Go through the queue and try to resolve any placeholders
	q.lock.Lock()
	indexesToDelete := make([]int, 0)
	for idx, elem := range q.fifo {
		err := elem.CheckFailure()
		if err != nil {
			log.Printf("Song %s had a download error: %s", elem.URL(), err)
			indexesToDelete = append(indexesToDelete, idx)
		}
	}
	for _, elem := range indexesToDelete {
		q.fifo = remove(q.fifo, elem)
	}
	q.lock.Unlock()

	// Make a copy so whoever is reading this can't write it
	qCopy := make([]*resource.Song, len(q.fifo))
	q.lock.Lock()
	copy(qCopy, q.fifo)
	q.lock.Unlock()

	return qCopy
}

func remove(s []*resource.Song, i int) []*resource.Song {
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
