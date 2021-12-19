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
	Output            chan []byte
	bitrate           int
	currentSongReader io.ReadCloser
	queue             *queue.Queue
	CurrentSongInfo   *resource.Song
	skipped           bool
	learnFrom         bool
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
		Output:            make(chan []byte, 4), // Needs to have space to handle song transition
		bitrate:           bitrate,
		currentSongReader: nil,
		queue:             queue,
		CurrentSongInfo:   &resource.Song{},
		skipped:           false,
		learnFrom:         false,
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
			tempSongData, tempSongReader, queueIsEmpty, fromAuto := mixer.fetchNextSong()
			if tempSongReader == nil {
				log.Printf("Song to be played doesn't have a valid reader: %s", tempSongData.ResourceID())
			}
			if !queueIsEmpty && (tempSongReader != nil) {
				// We are good to play the song
				mixer.learnFrom = !fromAuto
				mixer.currentSongReader = tempSongReader
				mixer.CurrentSongInfo = tempSongData

				// Take the current song and put it into the encoder
				_, err = io.Copy(wavInput, mixer.currentSongReader)

				if err != nil {
					// If we skipped we'll always get an error, so ignore it
					if !mixer.skipped {
						// We can't send data to the encoder for some reason
						// This usually means ffmpeg is struggling. Let's give it a break
						log.Printf("Error copying into mixer output: %v\n", err)
						time.Sleep(10 * time.Second)
						continue
					}
				}

				// Avoid double closes, if we skipped we already closed the reader
				// Seems like there should be a better way...
				if !mixer.skipped {
					// testing without reader close
					mixer.currentSongReader.Close()
				}
				mixer.skipped = false

				// We finished playing the song, record that unless we've decided not to
				if err != nil && mixer.CurrentSongInfo.IpfsPath() != "" {
					mixer.queue.NotifyDone(mixer.CurrentSongInfo.IpfsPath(), mixer.learnFrom)
				}

				// Put a placeholder in the song info in case the next fetch
				// from the ipfs takes a long time
				mixer.CurrentSongInfo = &resource.Song{
					Title:    "Loading Next Song",
					Duration: 0,
				}

				mixer.learnFrom = true
			} else if queueIsEmpty {
				// If the queue is empty wait a bit before trying to fetch another song
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
		if wasPanic := recover(); wasPanic != nil {
			log.Printf("Close on closed channel error. THIS IS BAD IT SHOULDN'T HAPPEN!!\n")
		}
	}()

	m.skipped = true
	m.learnFrom = false
	m.currentSongReader.Close()
}

// Will go to queue and get the next track and associated metadata
func (m *Mixer) fetchNextSong() (
	nextSong *resource.Song,
	mp3Reader io.ReadCloser,
	queueIsEmpty bool,
	fromAuto bool) {

	// Get MP3 reader.
	nextSong, nextSongReader, queueIsEmpty, fromAuto := m.queue.Pop()
	if queueIsEmpty {
		return nil, nil, true, fromAuto
	}
	log.Printf("About to play %s\n", nextSong.ResourceID())

	return nextSong, nextSongReader, false, fromAuto
}

func byteReader(r io.ReadCloser, ch chan []byte, bytesPerSecond int) chan bool {
	if bytesPerSecond <= 0 {
		bytesPerSecond = 2048
	}

	// Bump content rate to account for misc slowdown
	bytesPerSecond += 25

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
