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
		return
	}

	go func() {
		_, err := io.Copy(convInput, inputFile)
		if err != nil {
			t.Errorf("failed to copy audio from input into converter: %v", err)
			return
		}
		convInput.Close()
		inputFile.Close()
	}()
	go func() {
		_, err := io.Copy(outputFile, convOutput)
		if err != nil {
			t.Errorf("failed to copy audio from converter into output: %v", err)
			return
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
	songToTest, _ := resource.NewSong(rawUrl)

	// Commence the download
	song, err := downloadYoutube(songToTest, ipfs)
	if err != nil {
		t.Errorf("TestDownloadYoutube failed due to an error: %v", err)
		return
	}

	_, err = song.Resolve(ipfs)
	if err != nil {
		t.Errorf("TestDownloadYoutube failed due to an error: %v", err)
		return
	}

	expectedTitle := "1080 Hz Sine Wave 30 sec"
	if song.Title != expectedTitle {
		t.Errorf("Song title doesn't equal expected. e: %s, a:%s\n", expectedTitle, song.Title)
		return
	}

}

func TestDownloadMP3(t *testing.T) {
	rawUrl := "https://www.mediacollege.com/audio/tone/files/100Hz_44100Hz_16bit_05sec.mp3"
	ipfsUrl := "localhost:5001"

	// Setup shell and testing url
	ipfs := shell.NewShell(ipfsUrl)
	songToTest, _ := resource.NewSong(rawUrl)

	// Commence the download
	song, err := downloadMp3(songToTest, ipfs)
	if err != nil {
		t.Errorf("TestDownloadMP3 failed due to an error: %v", err)
		return
	}

	song.Resolve(ipfs)
}

func TestFetchIpfs(t *testing.T) {
	ipfsUrl := "localhost:5001"
	ipfsPath := "/ipfs/QmZem7HHzLuhq8Qa4CHD6Q4VUdn9ihP5vaEihhfUhqqyPN"

	// Setup shell and testing url
	ipfs := shell.NewShell(ipfsUrl)

	reader, err := FetchIpfs(ipfsPath, ipfs)
	if reader == nil {
		t.Errorf("Failed to fetch IPFS path. Err: %s", err)
	}
}
