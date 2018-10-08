package stream

import (
	"testing"

	"github.com/VivaLaPanda/uta-stream/encoder"
)

func TestServeAudioOverHttp(t *testing.T) {
	filename := "test_counting.mp3"

	// Start pulling data from file into the channel
	bytesToServe, err := encoder.EncodeMP3(filename, 2)
	if err != nil {
		t.Errorf("Failed to generate data stream: %v", err)
	}

	ServeAudioOverHttp(*bytesToServe, 2, 9091)
}
