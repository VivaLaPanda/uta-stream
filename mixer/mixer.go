package mixer

import (
	"io"
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
	queue            *queue.Queue
	cache            *cache.Cache
	CurrentSongPath  string
	playLock         *sync.Mutex
	skipped          bool
}

// Bigger packet buffer means more resiliance but may cause
// strange behavior when skipping a song. In my experience a small value is best
var packetBufferSize = 8

// Packets-per-second sacrifices reliability for synchronization
// Higher means more synchornized streams. Minimum should be 1, super large
// values have undefined behaviour
// 2 is a reasonable default
func NewMixer(queue *queue.Queue, cache *cache.Cache, packetsPerSecond int) *Mixer {
	currentSong := make(chan []byte, packetBufferSize)
	mixer := &Mixer{
		Output:           make(chan []byte, packetBufferSize),
		packetsPerSecond: packetsPerSecond,
		currentSong:      &currentSong,
		queue:            queue,
		cache:            cache,
		CurrentSongPath:  "",
		playLock:         &sync.Mutex{},
		skipped:          false}
	close(currentSong)
	// Spin up the job to cast from the current song to our output
	// and handle song transitions
	go func() {
		for true {
			for broadcastPacket := range *mixer.currentSong {
				// We can succesfully read from the current song, all is good

				// This lock is used to remotely pause here if necessary.
				// If the lock is unlocked, all that will happen is the program moving on,
				// otherwise we will wait until the lock is released elsewhere
				mixer.playLock.Lock()
				mixer.playLock.Unlock()
				mixer.Output <- broadcastPacket
			}

			// We couldn't play from current, assume that the song ended
			// Also, if we just recieved a skip, then we don't want to use that
			// song to train qutoq
			if mixer.CurrentSongPath != "" && !mixer.skipped {
				// If we were just playing something unknown, the autoq don't care
				// TODO: This should detect skips and not notify if the song was skipped
				mixer.queue.NotifyDone(mixer.CurrentSongPath)
			}
			if mixer.skipped {
				mixer.skipped = false
			}

			tempSong, tempPath, isEmpty := mixer.fetchNextSong()
			if !isEmpty && (tempSong != nil) {
				mixer.currentSong = tempSong
				mixer.CurrentSongPath = tempPath
				broadcastPacket := <-*mixer.currentSong
				mixer.Output <- broadcastPacket
			} else {
				time.Sleep(2 * time.Second)
			}
		}
	}()

	return mixer
}

// Will swap the next song in place of the current one.
func (m *Mixer) Skip() {
	m.skipped = true
	close(*m.currentSong)
}

// Will toggle playing by allowing writes to output
func (m *Mixer) Play() {
	m.playLock.Unlock()
}

// Will toggle playing by preventing writes to output
// TODO: FiX THIS. BORKED AS HELL https://github.com/VivaLaPanda/uta-stream/issues/3
// If people keep calling pause then it will keep spawning deadlocked routines
// until someone hits play, at which point all extra paused routines will die
// Need someway to check mutex or some different pause approach entirely
func (m *Mixer) Pause() {
	go func() {
		m.playLock.Lock()
	}()
}

// Will go to queue and get the next track
func (m *Mixer) fetchNextSong() (nextSongChan *chan []byte, nextSongPath string, isEmpty bool) {
	nextSongPath, isEmpty = m.queue.Pop()
	var nextSongReader io.Reader
	var err error
	if isEmpty {
		if m.cache.Hotstream != nil {
			// The queue is empty but we have a hotstream, which means something
			// is being converted urgently for us. Just start playing, ipfs/songdata
			// will show up as unknown
			nextSongReader = m.cache.Hotstream
		} else {
			// Empty and now hotstream, there really is nothing for us to do
			return nil, "", true
		}
	} else {
		// The queue isn't empty so we'll go get the provided song
		nextSongReader, err = m.cache.FetchIpfs(nextSongPath)
		if err != nil {
			log.Printf("Failed to fetch song (%v). Err: %v\n", nextSongPath, err)
			return nil, "", true
		}
	}

	// Start encoding for broadcast
	nextSongChan, err = encoder.EncodeMP3(nextSongReader, m.packetsPerSecond)
	if err != nil {
		log.Printf("Failed to encode song (%v). Err: %v\n", nextSongPath, err)
		return nil, "", true
	}

	return nextSongChan, nextSongPath, false
}
