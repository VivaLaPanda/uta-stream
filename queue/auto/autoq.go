package auto

import (
	"encoding/gob"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

type Queue struct {
	markovChain *Chain
	playedSongs chan string
}

func NewQueue() *Queue {
	q := &Queue{NewChain(1), make(chan string)}
	q.markovChain.StartBuildListener(q.playedSongs)

	// Write the chain to disk occasionally to preserve it between runs
	go func() {
		// Testing we can write the q
		qfile, err := os.Open("autoq.db")
		if err != nil {
			panic("Failed to open the queue")
		}
		qfile.Close()

		for {
			time.Sleep(10 * time.Minute)
			qfile, err := os.Open("autoq.db")
			if err != nil {
				panic("Failed to open the queue")
			}
			encoder := gob.NewEncoder(qfile)
			encoder.Encode(q.markovChain.chain)
		}
	}()

	return q
}

func (q *Queue) Vpop() string {
	return q.markovChain.Generate()
}

func (q *Queue) NotifyPlayed(filename string) {
	q.playedSongs <- filename
}

// Prefix is a Markov chain prefix of one or more song.
type Prefix []string

// String returns the Prefix as a string (for use as a map key).
func (p Prefix) String() string {
	return strings.Join(p, " ")
}

// Shift removes the first song from the Prefix and appends the given song.
func (p Prefix) Shift(word string) {
	copy(p, p[1:])
	p[len(p)-1] = word
}

// Chain contains a map ("chain") of prefixes to a list of suffixes.
// A prefix is a string of prefixLen songs joined with spaces.
// A suffix is a single song. A prefix can have multiple suffixes.
type Chain struct {
	chain      map[string][]string
	chainWLock *sync.Mutex
	prefixLen  int
}

// NewChain returns a new Chain with prefixes of prefixLen songs
func NewChain(prefixLen int) *Chain {
	return &Chain{make(map[string][]string), &sync.Mutex{}, prefixLen}
}

// Build reads song uris from the provided channel
// parses it into prefixes and suffixes that are stored in Chain.
func (c *Chain) StartBuildListener(input chan string) {
	go func() {
		p := make(Prefix, c.prefixLen)
		for s := range input {
			key := p.String()
			c.chainWLock.Lock()
			c.chain[key] = append(c.chain[key], s)
			c.chainWLock.Unlock()
			p.Shift(s)
		}
	}()
}

// Returns the next song to play
func (c *Chain) Generate() string {
	p := make(Prefix, c.prefixLen)

	// Choices represents songs it might be good to play next
	choices := c.chain[p.String()]

	// If there are no known songs, just pick something at random
	if len(choices) == 0 {
		for _, v := range c.chain {
			return v[0]
		}
	}

	// Randomly select one of the choices
	song := choices[rand.Intn(len(choices))]
	p.Shift(song)

	return song
}
