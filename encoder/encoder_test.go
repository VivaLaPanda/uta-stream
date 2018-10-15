package encoder

import (
	"os"
	"testing"
)

func TestEncodeMP3(t *testing.T) {
	filename := "test_counting.mp3"

	fileR, err := os.Open(filename)
	defer fileR.Close()

	// Start pulling data from file into the channel
	_, err = EncodeMP3(fileR, 2)
	if err != nil {
		t.Errorf("Failed to generate data stream: %v", err)
	}
}
