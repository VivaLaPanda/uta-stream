package download

import (
	"io"
	"os"
	"testing"

	"github.com/VivaLaPanda/uta-stream/resource"
	shell "github.com/ipfs/go-ipfs-api"
)

func TestSplitAudio(t *testing.T) {
	inputFile, _ := os.Open("test_vid.mp4")
	outputFile, _ := os.Create("test_audio.mp3.test")
	convInput, convOutput, convProgress, err := splitAudio()
	if err != nil {
		t.Errorf("testsplitaudio failed due to an error: %v", err)
	}

	go func() {
		_, err := io.Copy(convInput, inputFile)
		if err != nil {
			t.Errorf("failed to copy audio from input into converter: %v", err)
		}
		convInput.Close()
		inputFile.Close()
	}()
	go func() {
		_, err := io.Copy(outputFile, convOutput)
		if err != nil {
			t.Errorf("failed to copy audio from converter into output: %v", err)
		}
		convOutput.Close()
		outputFile.Close()
	}()
	convProgress.Wait()
}

func TestDownloadYoutube(t *testing.T) {
	rawUrl := "https://youtu.be/nAwTw1aYy6M"
	ipfsUrl := "localhost:5001"

	// Setup shell and testing url
	ipfs := shell.NewShell(ipfsUrl)
	songToTest, _ := resource.NewSong(rawUrl, false)

	// Commence the download
	song, err := downloadYoutube(songToTest, ipfs)
	if err != nil {
		t.Errorf("TestDownloadYoutube failed due to an error: %v", err)
	}
	expectedTitle := "1080 Hz Sine Wave 30 sec"
	if song.Title != expectedTitle {
		t.Errorf("Song title doesn't equal expected. e: %s, a:%s\n", expectedTitle, song.Title)
	}
	ipfsPath := <-song.DLResult
	expectedIPFS := "/ipfs/QmRRKwCPfmAf8A9crYCisfFuSDbwerthf5NBQ2h334vQsb"
	if ipfsPath != expectedIPFS {
		t.Errorf("IPFS path doesn't equal expected. e: %s, a:%s\n", expectedIPFS, ipfsPath)
	}
}

func TestFetchIpfs(t *testing.T) {
	ipfsUrl := "localhost:5001"
	ipfsPath := "/ipfs/QmRRKwCPfmAf8A9crYCisfFuSDbwerthf5NBQ2h334vQsb"

	// Setup shell and testing url
	ipfs := shell.NewShell(ipfsUrl)

	reader, err := FetchIpfs(ipfsPath, ipfs)
	if reader == nil {
		t.Errorf("Failed to fetch IPFS path. Err: %s", err)
	}
}
