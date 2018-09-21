package stream

import (
	"io"
	"os"

	"github.com/tcolgate/mp3"
)

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
func Mp3ToPacketStream(filename string, packetsPerSecond int) (chan []byte, error) {
	bytesToServe := make(chan []byte, 128)
	reader, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	// Get bitrate from first frame, and then keep updating as we calculate the avg
	bitrate, err := estimateBitrate(reader)
	if err != nil {
		return nil, err
	}
	reader.Close()

	// Pretty much have ot reopen a new reader to start from the beginning
	// of the file
	reader, err = os.Open(filename)
	if err != nil {
		return nil, err
	}

	go func() {
		// Read file for audio stream
		for err == nil {
			// Here we convert the kbps into bytes/seconds per packet so that the stream
			// rate is correct
			dataPacket := make([]byte, bitrate/(8*packetsPerSecond))
			_, err = reader.Read(dataPacket)
			bytesToServe <- dataPacket
		}
	}()

	return bytesToServe, nil
}
