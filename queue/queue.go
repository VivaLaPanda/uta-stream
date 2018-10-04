package queue

import (
	"github.com/VivaLaPanda/uta-stream/queue/auto"
)

type Queue struct {
	fifo         []string
	autoq        *auto.Queue
	AutoqEnabled bool
}

// Make a new q structure
func NewQueue(autoqFilename string, enableAutoq bool) *Queue {
	return &Queue{autoq: auto.NewQueue(autoqFilename), AutoqEnabled: enableAutoq}
}

// Returns the audio resource next in the queue
func (q *Queue) Pop() (filename string, emptyq bool) {
	if len(q.fifo) == 0 {
		if q.AutoqEnabled {
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

func (q *Queue) AddToQueue(filename string) {
	q.fifo = append(q.fifo, filename)
}

func (q *Queue) PlayNext(filename string) {
	q.fifo = append([]string{filename}, q.fifo...)
}

// Used as a gateway to let the autoq know a song was played. For training the
// qutoqueue
func (q *Queue) NotifyDone(filename string) {
	q.autoq.NotifyPlayed(filename)
}
