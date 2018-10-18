package download

import (
	"bufio"
	"io"
	"net/url"
	"os"
	"testing"

	"github.com/VivaLaPanda/uta-stream/resource/metadata"
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
	expectedIpfsHash := "/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf"

	// Setup shell and testing url
	sh := shell.NewShell(ipfsUrl)
	urlToDL, err := url.Parse(rawUrl)
	if err != nil {
		t.Errorf("TestDownloadYoutube failed to parse the test url: %v", err)
	}

	// Setup file for streamData
	outputFile, _ := os.Create("test_split.mp3.test")
	defer outputFile.Close()
	bWriter := bufio.NewWriter(outputFile)

	metadata := metadata.NewCache("metadata.db.test")

	// Commence the download
	ipfsPath, err := downloadYoutube(*urlToDL, sh, metadata, bWriter)
	if err != nil {
		t.Errorf("TestDownloadYoutube failed due to an error: %v", err)
	}
	if ipfsPath != expectedIpfsHash {
		t.Errorf("TestDownloadYoutube failed. \nExpected hash:%v\nActual hash:%v\n", expectedIpfsHash, ipfsPath)
	}
}

func TestDownload(t *testing.T) {
	rawUrl := "https://www.youtube.com/watch?v=WKzG9R-AxpE&feature=youtu.be"
	ipfsUrl := "localhost:5001"

	// Setup shell and testing url
	sh := shell.NewShell(ipfsUrl)

	metadata := metadata.NewCache("metadata.db.test")

	ipfsPath, err := Download(rawUrl, sh, metadata, nil)
	if err != nil {
		t.Errorf("TestDownloadYoutube failed due to an error: %v", err)
	}
	_ = ipfsPath
}
