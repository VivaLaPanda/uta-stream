package mp3

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestWavToMp3(t *testing.T) {
	inputFile, _ := os.Open("test_clip.wav")
	defer inputFile.Close()
	outputFile, _ := os.Create("test_clip.mp3")
	defer outputFile.Close()
	convInput, convOutput, convProgress, err := WavToMp3(128)
	if err != nil {
		t.Errorf("TestWavToMp3 failed due to an error: %v", err)
	}

	go func() {
		_, err := io.Copy(convInput, inputFile)
		if err != nil {
			t.Errorf("failed to copy audio from input into converter: %v", err)
		}
		convInput.Close()
	}()
	go func() {
		_, err := io.Copy(outputFile, convOutput)
		if err != nil {
			t.Errorf("failed to copy audio from converter into output: %v", err)
		}
	}()
	convProgress.Wait()

	expectedSample := make([]byte, 1000)
	actualSample := make([]byte, 1000)

	inputFile.Read(expectedSample)
	outputFile.Read(actualSample)

	if !bytes.Equal(actualSample, expectedSample) {
		t.Errorf("Conversion output didn't match precomputed file")
	}
}
