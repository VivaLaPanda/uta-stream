package stream

import (
	"os"
	"testing"

	"github.com/VivaLaPanda/uta-stream/encoder"
)

func TestServeAudioOverHttp(t *testing.T) {
	filename := "test_counting.mp3"
	fileR, err := os.Open(filename)

	// Start pulling data from file into the channel
	bytesToServe, err := encoder.EncodeMP3(fileR, 2)
	if err != nil {
		t.Errorf("Failed to generate data stream: %v", err)
	}

	ServeAudioOverHttp(*bytesToServe, 2, 9091)
}
