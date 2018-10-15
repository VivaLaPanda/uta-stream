package mixer

import (
	"log"
	"sync"
	"time"

	"github.com/VivaLaPanda/uta-stream/encoder"
	"github.com/VivaLaPanda/uta-stream/queue"
	"github.com/VivaLaPanda/uta-stream/resource/cache"
)

// Mixer is in charge of interacting with the queue and resource cache
// to go from a queue of song hashes to a stream of audio data. It has
// logic to ensure minimal delay between songs by processing current/next
// in parallel. Mixer can be considered the key component that ties together
// all the rest
type Mixer struct {
	Output           chan []byte
	packetsPerSecond int
	currentSong      *chan []byte
	nextSong         *chan []byte
	queue            *queue.Queue
	cache            *cache.Cache
	currentSongPath  string
	nextSongPath     string
	playLock         *sync.Mutex
}

// Bigger packet buffer means more resiliance but may cause
// strange behavior when skipping a song.
var packetBufferSize = 32

// Packets-per-second sacrifices reliability for synchronization
// Higher means more synchornized streams. Minimum should be 1, super large
// values have undefined behaviour
// 2 is a reasonable default
func NewMixer(queue *queue.Queue, cache *cache.Cache, packetsPerSecond int) *Mixer {
	currentSong := make(chan []byte, packetBufferSize)
	nextSong := make(chan []byte, packetBufferSize)
	mixer := &Mixer{
		Output:           make(chan []byte, packetBufferSize),
		packetsPerSecond: packetsPerSecond,
		currentSong:      &currentSong,
		nextSong:         &nextSong,
		queue:            queue,
		cache:            cache,
		currentSongPath:  "",
		nextSongPath:     "",
		playLock:         &sync.Mutex{}}

	// Spin up the job to cast from the current song to our output
	// and handle song transitions
	go func() {
		for true {
			var broadcastPacket []byte
			select {
			case broadcastPacket = <-*mixer.currentSong:
				// We can succesfully read from the current song, all is good
			case broadcastPacket = <-*mixer.nextSong:
				// We couldn't play from current, assume that the song ended
				mixer.currentSong = mixer.nextSong
				mixer.currentSongPath = mixer.nextSongPath
				temp, isEmpty := mixer.fetchNextSong()
				if !isEmpty {
					mixer.nextSong = temp
				}
			default:
				// Both current and next are empty,
				temp, isEmpty := mixer.fetchNextSong()
				if !isEmpty {
					mixer.nextSong = temp
				}
				time.Sleep(10 * time.Second)
			}

			// This lock is used to remotely pause here if necessary.
			// If the lock is unlocked, all that will happen is the program moving on,
			// otherwise we will wait until the lock is released elsewhere
			mixer.playLock.Lock()
			mixer.playLock.Unlock()
			mixer.Output <- broadcastPacket
		}
	}()

	return mixer
}

// Will swap the next song in place of the cuurent one.
func (m *Mixer) Skip() {
	m.currentSong = m.nextSong
	m.currentSongPath = m.nextSongPath
	m.nextSong, _ = m.fetchNextSong()
}

// Will toggle playing by allowing writes to output
func (m *Mixer) Play() {
	m.playLock.Unlock()
}

// Will toggle playing by preventing writes to output
// TODO: FiX THIS. BORKED AS HELL
// If people keep calling pause then it will keep spawning deadlocked routines
// until someone hits play, at which point all extra paused routines will die
// Need someway to check mutex or some different pause approach entirely
func (m *Mixer) Pause() {
	go func() {
		m.playLock.Lock()
	}()
}

// Will go to queue and get the next track
func (m *Mixer) fetchNextSong() (nextSongChan *chan []byte, isEmpty bool) {
	nextSongPath, isEmpty := m.queue.Pop()
	if isEmpty {
		return nil, true
	}

	nextSongReader, err := m.cache.FetchIpfs(nextSongPath)
	if err != nil {
		log.Printf("Failed to fetch song (%v). Err: %v\n", nextSongPath, err)
		return nil, true
	}

	nextSongChan, err = encoder.EncodeMP3(nextSongReader, m.packetsPerSecond)
	if err != nil {
		log.Printf("Failed to encode song (%v). Err: %v\n", nextSongPath, err)
		return nil, true
	}
	m.nextSongPath = nextSongPath

	return nextSongChan, false
}
