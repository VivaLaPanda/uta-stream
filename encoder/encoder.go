package encoder

import (
	"bytes"
	"io"
	"log"
	"sync"
	"time"

	"github.com/VivaLaPanda/uta-stream/queue"
	"github.com/VivaLaPanda/uta-stream/resource/cache"
	"github.com/tcolgate/mp3"
)

type Encoder struct {
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
func NewEncoder(queue *queue.Queue, cache *cache.Cache, packetsPerSecond int) *Encoder {
	currentSong := make(chan []byte, packetBufferSize)
	nextSong := make(chan []byte, packetBufferSize)
	encoder := &Encoder{
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
			case broadcastPacket = <-*encoder.currentSong:
				// We can succesfully read from the current song, all is good
			case broadcastPacket = <-*encoder.nextSong:
				// We couldn't play from current, assume that the song ended
				encoder.currentSong = encoder.nextSong
				encoder.currentSongPath = encoder.nextSongPath
				temp, isEmpty := encoder.fetchNextSong()
				if !isEmpty {
					encoder.nextSong = temp
				}
			default:
				// Both current and next are empty,
				temp, isEmpty := encoder.fetchNextSong()
				if !isEmpty {
					encoder.nextSong = temp
				}
				time.Sleep(10 * time.Second)
			}

			// This lock is used to remotely pause here if necessary.
			// If the lock is unlocked, all that will happen is the program moving on,
			// otherwise we will wait until the lock is released elsewhere
			encoder.playLock.Lock()
			encoder.playLock.Unlock()
			encoder.Output <- broadcastPacket
		}
	}()

	return encoder
}

// EncodeMP3 returns a channel containing the data found at the provided file
// Works off a reader. WARNING: Requires 2x the reader in memory until it's
// done. Eventually this will be fixed by handling bitrate estimation and
// streaming in parallel
func EncodeMP3(mp3Data io.Reader, packetsPerSecond int) (*chan []byte, error) {
	// create the pipe and tee reader
	var bufferReader bytes.Buffer
	sourceReader := io.TeeReader(mp3Data, &bufferReader)

	// Get bitrate from first frame, and then keep updating as we calculate the avg
	bitrate, err := estimateBitrate(sourceReader)
	if err != nil {
		return nil, err
	}

	tmpSong := make(chan []byte, packetBufferSize)

	go func() {
		// Read file for audio stream
		for err == nil {
			// Here we convert the kbps into bytes/seconds per packet so that the stream
			// rate is correct
			dataPacket := make([]byte, bitrate/(8*packetsPerSecond))
			_, err = bufferReader.Read(dataPacket)
			tmpSong <- dataPacket
		}
	}()

	return &tmpSong, nil
}

// Will swap the next song in place of the cuurent one.
func (e *Encoder) Skip() {
	e.currentSong = e.nextSong
	e.currentSongPath = e.nextSongPath
	e.nextSong, _ = e.fetchNextSong()
}

// Will toggle playing by allowing writes to output
func (e *Encoder) Play() {
	e.playLock.Unlock()
}

// Will toggle playing by preventing writes to output
// TODO: FiX THIS. BORKED AS HELL
// If people keep calling pause then it will keep spawning deadlocked routines
// until someone hits play, at which point all extra paused routines will die
// Need someway to check mutex or some different pause approach entirely
func (e *Encoder) Pause() {
	go func() {
		e.playLock.Lock()
	}()
}

// Will go to queue and get the next track
func (e *Encoder) fetchNextSong() (nextSongChan *chan []byte, isEmpty bool) {
	nextSongPath, isEmpty := e.queue.Pop()
	if isEmpty {
		return nil, true
	}

	nextSongReader, err := e.cache.FetchIpfs(nextSongPath)
	if err != nil {
		log.Printf("Failed to fetch song (%v). Err: %v\n", nextSongPath, err)
		return nil, true
	}

	nextSongChan, err = EncodeMP3(nextSongReader, e.packetsPerSecond)
	if err != nil {
		log.Printf("Failed to encode song (%v). Err: %v\n", nextSongPath, err)
		return nil, true
	}
	e.nextSongPath = nextSongPath

	return nextSongChan, false
}

// Returns the average bitrate of the file
func estimateBitrate(reader io.Reader) (int, error) {
	var err error
	var f mp3.Frame

	// Decode over the file, reading the bitrate from the frames
	// until we reach the end. Calculate the average
	decoder := mp3.NewDecoder(reader)
	skipped := 0
	averageBitrate := 0
	sumBitrate := 0
	for count := 1; err == nil; count++ {
		err = decoder.Decode(&f, &skipped)
		sumBitrate = sumBitrate + int(f.Header().BitRate())
		averageBitrate = sumBitrate / count
	}

	return averageBitrate, nil
}
