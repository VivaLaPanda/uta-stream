package encoder

import (
	"io"
	"os"

	"github.com/VivaLaPanda/uta-stream/queue"
	"github.com/tcolgate/mp3"
)

type Encoder struct {
	Output      chan []byte
	currentSong *chan []byte
	nextSong    *chan []byte
	queue       *queue.Queue
}

func NewEncoder(queue *queue.Queue) *Encoder {
	currentSong := make(chan []byte, 128)
	nextSong := make(chan []byte, 128)
	encoder := &Encoder{
		Output:      make(chan []byte, 128),
		currentSong: &currentSong,
		nextSong:    &nextSong,
		queue:       queue}

	// Spin up the job to cast from the current song to our output
	// and handle song transitions
	go func() {
		for true {
			var broadcastPacket []byte
			select {
			case broadcastPacket = <-*encoder.currentSong:
				// We can succesfully read from the current song, all is good
				encoder.Output <- broadcastPacket
			case broadcastPacket = <-*encoder.nextSong:
				// We couldn't play from current, assume that the song ended
				encoder.Output <- broadcastPacket
				encoder.currentSong = encoder.nextSong
				encoder.nextSong = fetchNextSong()
			default:
				// There is nothing to play, do nothing
			}
		}
	}()

	return encoder
}

// Will go to queue and get the next track
func fetchNextSong() *chan []byte {
	return nil
}

// Returns the average bitrate of the file
func estimateBitrate(reader io.Reader) (int, error) {
	var err error
	// Read bitrate from frame, make sure to copy our reader so that
	// The actual audio stream starts from the beginning
	// TODO: Spin this off so that the avg bitrate is calculated, not just
	// the starting bitrate
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

// Returns a byte channel that will be filled with packets from the file
// Packets-per-second sacrifices reliability for synchronization
// Higher means more synchornized streams. Minimum should be 1, super large
// values have undefined behaviour
// 2 is a reasonable default
func (e *Encoder) EncodeMP3(filename string, packetsPerSecond int) error {
	reader, err := os.Open(filename)
	if err != nil {
		return err
	}

	// Get bitrate from first frame, and then keep updating as we calculate the avg
	bitrate, err := estimateBitrate(reader)
	if err != nil {
		return err
	}
	reader.Close()

	// Pretty much have ot reopen a new reader to start from the beginning
	// of the file
	reader, err = os.Open(filename)
	if err != nil {
		return err
	}

	// Warning, this will kill anything already in the nextsong spot
	// Meaning, EncodeMP3 should never be called unless nextSong is empty
	tmpSong := make(chan []byte, 128)
	e.nextSong = &tmpSong

	go func() {
		// Read file for audio stream
		for err == nil {
			// Here we convert the kbps into bytes/seconds per packet so that the stream
			// rate is correct
			dataPacket := make([]byte, bitrate/(8*packetsPerSecond))
			_, err = reader.Read(dataPacket)
			*e.nextSong <- dataPacket
		}
	}()

	return nil
}
