package resource

import (
	"testing"

	shell "github.com/ipfs/go-ipfs-api"
)

func TestNewSong(t *testing.T) {
	rawUrl := "https://youtu.be/nAwTw1aYy6M"
	testSongA, _ := NewSong(rawUrl)

	if testSongA.Writer != nil {
		t.Errorf("testSongA has a writer and shouldn't")
	}
}

func TestJson(t *testing.T) {
	rawUrl := "https://youtu.be/nAwTw1aYy6M"
	testSongA, _ := NewSong(rawUrl)

	json, err := testSongA.MarshalJSON()
	if err != nil {
		t.Errorf("failed to marshal JSON. Err: %s", err)
	}
	testSongB := &Song{}
	if err = testSongB.UnmarshalJSON(json); err != nil {
		t.Errorf("failed to unmarshal JSON. Err: %s", err)
	}

	if testSongA.URL().String() != testSongB.URL().String() {
		t.Errorf("url changed after JSON marshal and unmarshal.\n")
	}
}

func TestResourceID(t *testing.T) {
	rawUrl := "https://youtu.be/nAwTw1aYy6M"
	song, _ := NewSong(rawUrl)
	resourceID := song.ResourceID()
	isCached := IsIpfs(resourceID)
	if resourceID != rawUrl {
		t.Errorf("Expected resourceID didn't match actua. E:%s, A:%s", rawUrl, resourceID)
	}
	if isCached != false {
		t.Errorf("Song shouldn't report as cached")
	}
	if song.ipfsPath != "" {
		t.Errorf("ipfsPath should be empty, isn't")
	}

	expectedIpfs := "/ipfs/QmRRKwCPfmAf8A9crYCisfFuSDbwerthf5NBQ2h334vQsb"
	song.DLResult <- expectedIpfs
	resourceID = song.ResourceID()
	isCached = IsIpfs(resourceID)
	if resourceID != expectedIpfs {
		t.Errorf("Expected resourceID didn't match actua. E:%s, A:%s", expectedIpfs, resourceID)
	}
	if isCached != true {
		t.Errorf("Song should report as cached")
	}
	if song.ipfsPath != expectedIpfs {
		t.Errorf("ipfsPath shouldn't be empty, is")
	}
}

func TestResolve(t *testing.T) {
	rawUrl := "https://youtu.be/nAwTw1aYy6M"
	ipfsUrl := "localhost:5001"

	// Setup shell and testing url
	sh := shell.NewShell(ipfsUrl)
	song, _ := NewSong(rawUrl)

	expectedIpfs := "/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf"
	song.DLResult <- expectedIpfs
	reader, err := song.Resolve(sh)
	if reader == nil {
		t.Errorf("Resolve failed to produce a reader. Err: %s", err)
	}
}
