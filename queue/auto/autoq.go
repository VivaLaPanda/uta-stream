package auto

import (
	"encoding/gob"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

type Queue struct {
	markovChain *chain
	playedSongs chan string
}

// How many minutes to wait between saves of the autoq state
var autosaveTimer time.Duration = 10

// Function which will provide a new autoq struct
// An autoq must be provided a file that it can read/write it's data to
// so that the chain is preserved between launches
func NewQueue(qfile string) *Queue {
	q := &Queue{newChain(1), make(chan string)}

	// startBuildListener will watch a channel for new songs and add their data into
	// the chain
	q.markovChain.startBuildListener(q.playedSongs)

	// Confirm we can interact with our persitent storage
	_, err := os.Stat(qfile)
	if err == nil {
		q.Load(qfile)
	} else if os.IsNotExist(err) {
		log.Printf("qfile %s doesn't exist. Creating new qfile", qfile)
		q.Write(qfile)
	} else {
		log.Printf("Failed to stat qfile %s, error: %v", qfile, err)
	}

	// Write the chain to disk occasionally to preserve it between runs
	go func() {
		for {
			q.Write(qfile)
			time.Sleep(autosaveTimer * time.Minute)
		}
	}()

	return q
}

// Method which will write the autoq data to the provided file. Will overwrite
// a file if one already exists at that location.
func (q *Queue) Write(filename string) {
	qfile, err := os.Open(filename)
	if err != nil {
		panic("Failed to open the queue")
	}
	encoder := gob.NewEncoder(qfile)
	encoder.Encode(q.markovChain.chainData)
}

// Method which will load the provided autoq data file. Will overwrite the internal
// state of the object. Should pretty much only be used when the object is created
// but it is left public in case a client needs to load old data or something
func (q *Queue) Load(filename string) {
	file, err := os.Open(filename)
	defer file.Close()
	if err == nil {
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(q)
	}
	if err != nil {
		panic("Fatal error while loading queue")
	}
}

// Vpop simply returns the next song according to the Markov chain
func (q *Queue) Vpop() string {
	return q.markovChain.generate()
}

// The interface for external callers to add to the markov chain
// In our case we use it to notify the chain that a song was played in full
func (q *Queue) NotifyPlayed(filename string) {
	q.playedSongs <- filename
}

// prefix is a Markov chain prefix of one or more song.
type prefix []string

// String returns the prefix as a string (for use as a map key).
func (p prefix) String() string {
	return strings.Join(p, " ")
}

// shift removes the first song from the prefix and appends the given song.
func (p prefix) shift(word string) {
	copy(p, p[1:])
	p[len(p)-1] = word
}

// chain contains a map ("chain") of prefixes to a list of suffixes.
// A prefix is a string of prefixLen songs joined with spaces.
// A suffix is a single song. A prefix can have multiple suffixes.
type chain struct {
	chainData  map[string][]string
	chainWLock *sync.Mutex
	prefixLen  int
}

// newChain returns a new chain with prefixes of prefixLen songs
func newChain(prefixLen int) *chain {
	return &chain{make(map[string][]string), &sync.Mutex{}, prefixLen}
}

// Build reads song uris from the provided channel
// parses it into prefixes and suffixes that are stored in chain.
func (c *chain) startBuildListener(input chan string) {
	go func() {
		p := make(prefix, c.prefixLen)
		for s := range input {
			key := p.String()
			c.chainWLock.Lock()
			c.chainData[key] = append(c.chainData[key], s)
			c.chainWLock.Unlock()
			p.shift(s)
		}
	}()
}

// Returns the next song to play
func (c *chain) generate() string {
	p := make(prefix, c.prefixLen)

	// Choices represents songs it might be good to play next
	choices := c.chainData[p.String()]

	// If there are no known songs, just pick something at random
	if len(choices) == 0 {
		for _, v := range c.chainData {
			return v[0]
		}
	}

	// Randomly select one of the choices
	song := choices[rand.Intn(len(choices))]
	p.shift(song)

	return song
}
