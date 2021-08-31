package mp3

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
)

// splitAudio will provide a reader and a writer that are connected, like an io pipe
// however, mp4 data passed into the writer will be returned as mp3 data from the reader
// The done waitgroup will be marked as done when the ffmpeg process is done running
// TODO: improve error handling. Any errors *during* ffmpeg run just vanish
// Requires ffmpeg to be in PATH
func WavToMp3(bitrate int) (input io.WriteCloser, output io.ReadCloser, done *sync.WaitGroup, err error) {
	// Ensure we have ffmpeg
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("ffmpeg was not found in PATH. Please install ffmpeg")
	}

	bitrateString := fmt.Sprintf("%dk", bitrate)

	subProcess := exec.Command(ffmpeg, "-y", "-loglevel", "panic", "-i", "pipe:0", "-filter:a", "loudnorm=I=-16", "-b:a", bitrateString, "-f", "mp3", "pipe:1")
	input, err = subProcess.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to pipe input into audio converter, err: %v", err)
	}
	output, err = subProcess.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to pipe output from audio converter, err: %v", err)
	}
	subProcess.Stderr = os.Stderr

	done = &sync.WaitGroup{}
	done.Add(1)
	if err = subProcess.Start(); err != nil { //Use start, not run
		return nil, nil, nil, fmt.Errorf("failed to start conversion, err: %v", err)
	}

	go func() {
		err := subProcess.Wait()
		if err != nil {
			log.Printf("ffmpeg encountered an error while encoding: %v\n", err)
		}
		done.Done()
	}()

	return input, output, done, nil
}
