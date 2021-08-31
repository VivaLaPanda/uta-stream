// Package autoq provides a set of components to suggest songs to play based on a
// fairly simple markov chain trained on the provided play history.
package auto

import (
	"encoding/gob"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/VivaLaPanda/uta-stream/resource"
	"github.com/VivaLaPanda/uta-stream/resource/cache"
)

type AQEngine struct {
	markovChain *chain
	playedSongs chan string
	cache       *cache.Cache

	recent       []string
	recentLength int
	shuffle      bool
}

// How many minutes to wait between saves of the autoq state
var autosaveTimer time.Duration = 5

// Function which will provide a new autoq struct
// An autoq must be provided a file that it can read/write it's data to
// so that the chain is preserved between launches. Chainbreak prob will determine
// how often to give a random suggestion instead of the *real* one. Prefix length
// determines how far back the autoq's "memory" goes back. Longer = more predictable
func NewAQEngine(qfile string, cache *cache.Cache, chainbreakProb float64, prefixLength int, recentLength int) *AQEngine {
	q := &AQEngine{
		markovChain:  newChain(prefixLength, chainbreakProb),
		playedSongs:  make(chan string),
		cache:        cache,
		recent:       make([]string, recentLength),
		recentLength: recentLength,
		shuffle:      false,
	}

	// Confirm we can interact with our persitent storage
	_, err := os.Stat(qfile)
	if err == nil {
		err = q.Load(qfile)
	} else if os.IsNotExist(err) {
		log.Printf("qfile %s doesn't exist. Creating new qfile", qfile)
		err = q.Write(qfile)
	}

	if err != nil {
		errString := fmt.Sprintf("Fatal error when interacting with qfile on launch.\nErr: %v\n", err)
		panic(errString)
	}

	// Write the chain to disk occasionally to preserve it between runs
	go func() {
		for {
			time.Sleep(autosaveTimer * time.Minute)
			err := q.Write(qfile)
			if err != nil {
				log.Printf("WARNING! Failed to write qfile. Data will not persist until"+
					"this is fixed. \n Err: %v\n", err)
			}
		}
	}()

	return q
}

// Method which will write the autoq data to the provided file. Will overwrite
// a file if one already exists at that location.
func (q *AQEngine) Write(filename string) error {
	qfile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0660)
	defer qfile.Close()
	if err != nil {
		return err
	}
	encoder := gob.NewEncoder(qfile)
	q.markovChain.chainLock.RLock()
	encoder.Encode(q.markovChain.chainData)
	q.markovChain.chainLock.RUnlock()

	return nil
}

// Method which will load the provided autoq data file. Will overwrite the internal
// state of the object. Should pretty much only be used when the object is created
// but it is left public in case a client needs to load old data or something
func (q *AQEngine) Load(filename string) error {
	file, err := os.Open(filename)
	defer file.Close()
	if err == nil {
		decoder := gob.NewDecoder(file)
		q.markovChain.chainLock.Lock()
		err = decoder.Decode(q.markovChain.chainData)
		q.markovChain.chainLock.Unlock()
	}
	if err != nil {
		return err
	}

	return nil
}

// Vpop simply returns the next song according to the Markov chain
func (q *AQEngine) Vpop() (*resource.Song, error) {
	return q.cache.Lookup(q.generateFresh(), false)
}

// The interface for external callers to add to the markov chain
// In our case we use it to notify the chain that a song was played in full
// learn from allows you to advance the chain without adding data if false
func (q *AQEngine) NotifyPlayed(resourceID string, learnFrom bool) {
	// if shuffle flag is set, ignore the song being given and just
	// pick a random one
	if q.shuffle {
		resourceID = q.markovChain.getRandom()
		q.shuffle = false
	}

	// Put it in the recent so we don't play it again too soon
	q.pushRecent(resourceID)

	q.markovChain.chainLock.Lock()
	defer q.markovChain.chainLock.Unlock()

	key := q.markovChain.prefix.String()
	// Don't put more than one of the same song in the predict list
	if learnFrom {
		duplicate := false
		for _, value := range (*q.markovChain.chainData)[key] {
			if value == resourceID {
				duplicate = true
			}
		}

		if !duplicate {
			// Make sure we aren't creating a loop
			if key != resourceID {
				log.Printf("Adding new song %s to autoqueuer\n", resourceID)
				(*q.markovChain.chainData)[key] = append((*q.markovChain.chainData)[key], resourceID)
			}
		}
	}
	q.markovChain.prefix.shift(resourceID)
}

func (q *AQEngine) Shuffle() {
	q.shuffle = true
}

func (q *AQEngine) generateFresh() (song string) {
	// Add defer to store whatever ends up getting returned
	count := 0
	for song = q.markovChain.generate(); !q.isFresh(song); song = q.markovChain.generate() {
		if count > 5 {
			// We can't seem to get a fresh song, just ask for a random one
			log.Printf("Couldn't get a fresh song, shuffling...\n")
			return q.markovChain.getRandom()
		}
		count++
	}
	return song
}

// Return what was passed in for chaining
func (q *AQEngine) pushRecent(s string) string {
	q.recent = append(q.recent, s)
	q.recent = q.recent[1:]

	return s
}

func (q *AQEngine) isFresh(s string) bool {
	for _, elem := range q.recent {
		if s == elem {
			return false
		}
	}

	return true
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
	chainData      *map[string][]string
	prefix         prefix
	chainLock      *sync.RWMutex
	prefixLen      int
	chainbreakProb float64
}

// newChain returns a new chain with prefixes of prefixLen songs
func newChain(prefixLen int, chainbreakProb float64) *chain {
	chainData := make(map[string][]string)
	return &chain{&chainData, make(prefix, prefixLen), &sync.RWMutex{}, prefixLen, chainbreakProb}
}

// Returns the next song to play
func (c *chain) generate() (song string) {
	// Choices represents songs it might be good to play next
	c.chainLock.RLock()
	choices := (*c.chainData)[c.prefix.String()]
	c.chainLock.RUnlock()

	// If there are no known songs, just pick something at random
	if len(choices) == 0 {
		return c.getRandom()
	}

	// Some chance of picking a random song based on chainbreakProb
	if c.chainbreakProb != 0 && len(choices) < 4 {
		randInt := int(1 / c.chainbreakProb)
		if rand.Intn(randInt) == 1 {
			return c.getRandom()
		}
	}

	// Randomly select one of the choices
	song = choices[rand.Intn(len(choices))]

	// Handle nullsong
	if song == "" {
		return c.getRandom()
	}

	return song
}

func (c *chain) getRandom() (randChoice string) {
	// Randchoice provides a song randomly from the chain, without regard to the last
	// song
	c.chainLock.RLock()
	defer c.chainLock.RUnlock()
	if len(*c.chainData) == 0 {
		return ""
	}
	idxToTarget := rand.Intn(len(*c.chainData))
	idx := 0
	for _, v := range *c.chainData {
		if idx == idxToTarget {
			randChoice = v[rand.Intn(len(v))]
			if randChoice != c.prefix[len(c.prefix)-1] {
				break
			}
			idxToTarget += 1
		}
		idx += 1
	}

	return
}
