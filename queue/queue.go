package queue

import (
	"github.com/VivaLaPanda/uta-stream/queue/auto"
)

type Queue struct {
	fifo         []string
	autoq        *auto.Queue
	AutoqEnabled bool
}

// Make a new q structure. allowChainbreak will make the autoq more random
// TODO: Split this and pass in an autoq instead of making one during construction
func NewQueue(autoqFilename string, enableAutoq bool, allowChainbreak bool, prefixLength int) *Queue {
	return &Queue{
		autoq:        auto.NewQueue(autoqFilename, allowChainbreak, prefixLength),
		AutoqEnabled: enableAutoq}
}

// Returns the audio resource next in the queue
func (q *Queue) Pop() (ipfsPath string, emptyq bool) {
	// If there is nothing to queue and we have autoq enabled,
	// get from autoq. If autoq gives us an empty string (no audio to play)
	// or autoq is off, return that the queue is empty
	if len(q.fifo) == 0 {
		if q.AutoqEnabled {
			ipfsPath = q.autoq.Vpop()
			if ipfsPath == "" {
				return "", true
			}

			return q.autoq.Vpop(), false
		} else {
			return "", true
		}
	}

	// Top (just get next element, don't remove it)
	song := q.fifo[0]
	// Discard top element
	q.fifo = q.fifo[1:]

	return song, false
}

// Add the provided song to the queue at the back
func (q *Queue) AddToQueue(ipfsPath string) {
	q.fifo = append(q.fifo, ipfsPath)
}

// Add the provided song to the queue at the front
// TODO: This is sorta broken because of how me mix current/next song
// Will actually add as second-next song. Need some way to go to Encoder
// and requeue next song, dump it from the encoder, and then have the encoder pop the q
func (q *Queue) PlayNext(ipfsPath string) {
	q.fifo = append([]string{ipfsPath}, q.fifo...)
}

// Remove all items from the queue. Will not dump the encoder (current and next song)
func (q *Queue) Dump() {
	q.fifo = make([]string, 0)
}

// Used as a gateway to let the autoq know a song was played. For training the
// qutoqueue
func (q *Queue) NotifyDone(ipfsPath string) {
	q.autoq.NotifyPlayed(ipfsPath)
}
