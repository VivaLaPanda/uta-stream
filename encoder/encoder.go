package encoder

import (
	"bytes"
	"io"

	"github.com/tcolgate/mp3"
)

// Bigger packet buffer means more resiliance but may cause
// strange behavior when skipping a song. Shouldn't need to be changed often
// so we're not exposing it as an arg.
var packetBufferSize = 32

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
