package download

import (
	"net/url"
	"testing"

	shell "github.com/ipfs/go-ipfs-api"
)

func TestSplitAudio(t *testing.T) {
	_, err := splitAudio("test_vid", false)
	if err != nil {
		t.Errorf("testsplitaudio failed due to an error: %v", err)
	}
}

func TestDownloadYoutube(t *testing.T) {
	rawUrl := "https://youtu.be/nAwTw1aYy6M"
	ipfsUrl := "localhost:5001"
	expectedIpfsHash := "Qmcyp23gdiP6oGCp9jJqydkYboCQoCFj5yuiM3nnqzDbqn"

	// Setup shell and testing url
	sh := shell.NewShell(ipfsUrl)
	urlToDL, err := url.Parse(rawUrl)
	if err != nil {
		t.Errorf("TestDownloadYoutube failed to parse the test url: %v", err)
	}

	ipfsPath, err := downloadYoutube(*urlToDL, sh, false)
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

	ipfsPath, err := Download(rawUrl, sh, false)
	if err != nil {
		t.Errorf("TestDownloadYoutube failed due to an error: %v", err)
	}
	_ = ipfsPath
}
