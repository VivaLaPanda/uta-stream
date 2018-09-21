package stream

import (
	"testing"
)

func TestServeAudioOverHttp(t *testing.T) {
	filename := "test_counting.mp3"

	// Start pulling data from file into the channel
	bytesToServe, err := Mp3ToPacketStream(filename, 2)
	if err != nil {
		t.Errorf("Failed to generate data stream: %v", err)
	}

	ServeAudioOverHttp(bytesToServe, 2)
}
