// This mixer package provides the link between songs in a queue and
// a continuous playble byte stream
// Mixer is in charge of interacting with the queue to use the provided reader
// to construct a stream of audio packets that are sized such to keep the stream
// rate equal to the playback rate. Additionally Mixer is tasked with transitioning
// between songs, and trying to make that as smooth as possible.package mixer
package mixer

import (
	"io"
	"log"
	"time"

	"github.com/VivaLaPanda/uta-stream/mp3"
	"github.com/VivaLaPanda/uta-stream/queue"
	"github.com/VivaLaPanda/uta-stream/resource"
)

// Mixer is a struct which contains the persistent state necessary to talk
// to the queue and to interact with playback as it happens
type Mixer struct {
	Output          chan []byte
	bitrate         int
	currentSongData io.ReadCloser
	queue           *queue.Queue
	CurrentSongInfo *resource.Song
	skipped         bool
	learnFrom       bool
}

// NewMixer will return a mixer struct. Said struct will have the provided queue
// attached for internal use. The Output channel is public, and the only way
// consume the mixer's output. You are also provided the Current song path so you
// can check what is currently playing.
// A reasonable default for packetsPerSecond is 2, but it determines whether
// we send data in larger  or smaller chunks to the clients
// The mixer object will be tied to a goroutine which will populate the output
func NewMixer(queue *queue.Queue, bitrate int) *Mixer {
	mixer := &Mixer{
		Output:          make(chan []byte, 4), // Needs to have space to handle song transition
		bitrate:         bitrate,
		currentSongData: nil,
		queue:           queue,
		CurrentSongInfo: &resource.Song{},
		skipped:         false,
		learnFrom:       false,
	}

	// Prep to encode the mp3
	wavInput, mp3Output, _, err := mp3.WavToMp3(mixer.bitrate)
	if err != nil {
		log.Printf("Failed to prepare mp3 encoder. Err: %v\n", err)
		return nil
	}

	// Take all output from the encoder and put it on the Output channel
	go func() {
		done := byteReader(mp3Output, mixer.Output, 500*(bitrate/8))
		<-done
		log.Panicf("Encoder stopped producing output\n")
	}()

	// Take song data and put that into the encoder
	// also handle song transitions
	go func() {
		for {
			// Get the next song channel and associated metadata
			// Start broadcasting right away and set some flags/state values
			tempSong, tempPath, isEmpty, fromAuto := mixer.fetchNextSong()
			if !isEmpty && (tempSong != nil) {
				mixer.learnFrom = !fromAuto
				mixer.currentSongData = tempSong
				mixer.CurrentSongInfo = tempPath
			} else {
				time.Sleep(2 * time.Second)
			}

			// Check if we even have anything to try and play
			if mixer.currentSongData != nil {
				// Take the current song and put it into the encoder
				_, err = io.Copy(wavInput, mixer.currentSongData)

				if err != nil {
					// If we skipped we'll always get an error, so ignore it
					if !mixer.skipped {
						log.Printf("Error copying into mixer output: %v\n", err)
					}
				}

				// Avoid double closes, if we skipped we already closed the reader
				// Seems like there should be a better way...
				if !mixer.skipped {
					// testing without reader close
					mixer.currentSongData.Close()
				}
				mixer.skipped = false

				// We couldn't play from current, assume that the song ended
				// Also, if we just recieved a skip, then we don't want to use that
				// song to train qutoq
				if mixer.CurrentSongInfo.IpfsPath() != "" {
					mixer.queue.NotifyDone(mixer.CurrentSongInfo.IpfsPath(), mixer.learnFrom)
				}

				// Put a placeholder in the song info in case the next fetch
				// from the ipfs takes a long time
				mixer.CurrentSongInfo = &resource.Song{
					Title:    "Loading Next Song",
					Duration: 0,
				}

				mixer.learnFrom = true
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
		if wasPanic := recover(); wasPanic != nil {
			log.Printf("Close on closed channel error. THIS IS BAD IT SHOULDN'T HAPPEN!!\n")
		}
	}()

	m.skipped = true
	m.learnFrom = false
	m.currentSongData.Close()
}

// Will go to queue and get the next track and associated metadata
func (m *Mixer) fetchNextSong() (
	mp3Reader io.ReadCloser,
	nextSong *resource.Song,
	isEmpty bool,
	fromAuto bool) {

	// Get MP3 reader.
	nextSong, nextSongReader, isEmpty, fromAuto := m.queue.Pop()
	log.Printf("About to play %s\n", nextSong.ResourceID())
	if isEmpty {
		return nil, nextSong, true, fromAuto
	}

	return nextSongReader, nextSong, false, fromAuto
}

func byteReader(r io.ReadCloser, ch chan []byte, bytesPerSecond int) chan bool {
	if bytesPerSecond <= 0 {
		bytesPerSecond = 2048
	}

	// Bump content rate to account for misc slowdown
	bytesPerSecond += 50

	done := make(chan bool)

	go func() {
		var err error
		for err == nil {
			dataPacket := make([]byte, bytesPerSecond)
			for idx, n := 0, 0; idx < bytesPerSecond; idx += n {
				readByte := make([]byte, 1)
				n, err = r.Read(readByte)
				copy(dataPacket[idx:idx+n], readByte)
			}
			ch <- dataPacket
		}

		done <- true
	}()

	return done
}
