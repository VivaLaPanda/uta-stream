// This mixer package provides the link between songs in a queue and
// a continuous playble byte stream
// Mixer is in charge of interacting with the queue to use the provided reader
// to construct a stream of audio packets that are sized such to keep the stream
// rate equal to the playback rate. Additionally Mixer is tasked with transitioning
// between songs, and trying to make that as smooth as possible.package mixer
package mixer

import (
	"log"
	"sync"
	"time"

	"github.com/VivaLaPanda/uta-stream/encoder"
	"github.com/VivaLaPanda/uta-stream/queue"
)

// Mixer is a struct which contains the persistent state necessary to talk
// to the queue and to interact with playback as it happens
type Mixer struct {
	Output           chan []byte
	packetsPerSecond int
	currentSong      *chan []byte
	queue            *queue.Queue
	CurrentSongPath  string
	playLock         *sync.Mutex
	learnFrom        bool
}

// NewMixer will return a mixer struct. Said struct will have the provided queue
// attached for internal use. The Output channel is public, and the only way
// consume the mixer's output. You are also provided the Current song path so you
// can check what is currently playing.
// A reasonable default for packetsPerSecond is 2, but it determines whether
// we send data in larger  or smaller chunks to the clients
// The mixer object will be tied to a goroutine which will populate the output
func NewMixer(queue *queue.Queue, packetsPerSecond int) *Mixer {
	currentSong := make(chan []byte, 1)
	mixer := &Mixer{
		Output:           make(chan []byte, 8), // Needs to have space to handle song transition
		packetsPerSecond: packetsPerSecond,
		currentSong:      &currentSong,
		queue:            queue,
		CurrentSongPath:  "",
		playLock:         &sync.Mutex{},
		learnFrom:        false}
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
			// Send an empty byte so the consumer can determine the song ended
			mixer.Output <- make([]byte, 0)

			// We couldn't play from current, assume that the song ended
			// Also, if we just recieved a skip, then we don't want to use that
			// song to train qutoq
			if mixer.CurrentSongPath != "" {
				mixer.queue.NotifyDone(mixer.CurrentSongPath, mixer.learnFrom)
				mixer.learnFrom = true
			}

			// Get the next song channel and associated metadata
			// Start broadcasting right away and set some flags/state values
			tempSong, tempPath, isEmpty, fromAuto := mixer.fetchNextSong()
			if !isEmpty && (tempSong != nil) {
				mixer.learnFrom = !fromAuto
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

// Skip will force the current song to end, thus triggering an attempt to
// fetch the next song. Will not result in the autoq being trained
func (m *Mixer) Skip() {
	// We *could* get a close on closed channel error, which we want to ignore.
	defer func() {
		recover()
	}()

	m.learnFrom = false
	close(*m.currentSong)
}

// Will allow playing by allowing writes to output
func (m *Mixer) Play() {
	m.playLock.Unlock()
}

// Will stop playing by preventing writes to output
// TODO: FiX THIS. BORKED AS HELL https://github.com/VivaLaPanda/uta-stream/issues/3
// If people keep calling pause then it will keep spawning deadlocked routines
// until someone hits play, at which point all extra paused routines will die
// Need someway to check mutex or some different pause approach entirely
func (m *Mixer) Pause() {
	go func() {
		m.playLock.Lock()
	}()
}

// Will go to queue and get the next track and associated metadata
func (m *Mixer) fetchNextSong() (nextSongChan *chan []byte, nextSongPath string, isEmpty bool, fromAuto bool) {
	nextSongPath, nextSongReader, isEmpty, fromAuto := m.queue.Pop()
	var err error
	if isEmpty {
		return nil, "", true, fromAuto
	}

	// Start encoding for broadcast
	nextSongChan, err = encoder.EncodeMP3(nextSongReader, m.packetsPerSecond)
	if err != nil {
		log.Printf("Failed to encode song (%v). Err: %v\n", nextSongPath, err)
		return nil, "", true, fromAuto
	}

	return nextSongChan, nextSongPath, false, fromAuto
}
