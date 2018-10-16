package encoder

import (
	"os"
	"testing"
)

func TestEncodeMP3(t *testing.T) {
	filename := "test_counting.mp3"

	fileR, err := os.Open(filename)
	if err != nil {
		t.Errorf("Failed to open test file: %v", err)
	}

	// Start pulling data from file into the channel
	testChan, err := EncodeMP3(fileR, 2)
	if err != nil {
		t.Errorf("Failed to generate data stream: %v", err)
	}
	for val := range *testChan {
		_ = val
	}
}
