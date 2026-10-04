package resource

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestResourceIDOnEmptySongs(t *testing.T) {
	var missing *Song
	if id := missing.ResourceID(); id != "" {
		t.Errorf("nil song ResourceID = %q, want empty", id)
	}
	if id := (&Song{Title: "Loading Next Song"}).ResourceID(); id != "" {
		t.Errorf("song with no path or url ResourceID = %q, want empty", id)
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

func TestResolveMissingSongFailsFast(t *testing.T) {
	// A fake IPFS API that only answers offline requests, the way Kubo does for a
	// block it doesn't have
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v0/cat" || r.URL.Query().Get("offline") != "true" {
			t.Errorf("unexpected request %s (want an offline cat)", r.URL)
			select {} // a non-offline cat hangs, as a network search would
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"Message":"block was not found locally (offline)","Code":0,"Type":"error"}`))
	}))
	defer api.Close()

	song, _ := NewSong("/ipfs/QmMissing")
	start := time.Now()
	reader, err := song.Resolve(shell.NewShell(strings.TrimPrefix(api.URL, "http://")))
	if err == nil || reader != nil {
		t.Fatalf("Resolve of a missing song = (%v, %v), want an error", reader, err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Resolve took %v; a missing song should fail at once, without the 5s retry", elapsed)
	}
}
