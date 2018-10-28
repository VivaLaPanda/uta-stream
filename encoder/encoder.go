// Package encoder provides utilities for moving between raw MP3 data and
// the packet stream expected by most uta-stream components.
package encoder

import (
	"bufio"
	"io"

	"github.com/tcolgate/mp3"
)

// Bigger packet buffer means more resiliance but may cause
// strange behavior when skipping a song. Shouldn't need to be changed often
// so we're not exposing it as an arg.
var packetBufferSize = 16

// EncodeMP3 returns a channel containing the data found at the provided file
// Works off a reader. PacketsPerSecond MUST match that provided to the mixer
// a mismatch will result in either stuttering playback(mixer>encoder) or client
// desync(encoder>mixer). 2 is a resonable default.
func EncodeMP3(mp3Data io.Reader, packetsPerSecond int) (*chan []byte, error) {
	// create the pipe and tee reader
	pipeReader, pipeWriter := io.Pipe()
	bufPipeWriter := bufio.NewWriter(pipeWriter)
	sourceReader := io.TeeReader(mp3Data, bufPipeWriter)
	bitrate := 190000

	// Start estimating the bitrate
	currentEstimate := estimateBitrate(pipeReader)

	tmpSong := make(chan []byte, packetBufferSize)

	go func() {
		// Handling panic on closed send. If the encoder dies, so be it.
		defer func() {
			recover()
		}()

		// Read file for audio stream
		var err error
		var n int
		for err == nil {
			// Here we convert the kbps into bytes/seconds per packet so that the stream
			// rate is correct
			select {
			case bitrate = <-currentEstimate:
			default:
			}
			dataPacket := make([]byte, bitrate/(8*packetsPerSecond))
			n, err = sourceReader.Read(dataPacket)
			dataPacket = dataPacket[:n] // Shrink the packet if necessary
			tmpSong <- dataPacket
		}
		close(tmpSong)
	}()

	return &tmpSong, nil
}

// Returns the average bitrate of the file
func estimateBitrate(reader *io.PipeReader) (currentEstimate chan int) {
	var err error
	var f mp3.Frame
	currentEstimate = make(chan int, 5)

	// Decode over the file, reading the bitrate from the frames
	// until we reach the end. Calculate the average
	go func() {
		decoder := mp3.NewDecoder(reader)
		skipped := 0
		averageBitrate := 0
		sumBitrate := 0
		for count := 1; err == nil; count++ {
			err = decoder.Decode(&f, &skipped)
			sumBitrate = sumBitrate + int(f.Header().BitRate())
			averageBitrate = sumBitrate / count
			select {
			case currentEstimate <- averageBitrate:
			default:
			}
		}
		close(currentEstimate)
		reader.Close()
	}()

	return currentEstimate
}
