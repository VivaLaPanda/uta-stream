package cache

import (
	"os"
	"testing"

	"github.com/VivaLaPanda/uta-stream/resource"
	shell "github.com/ipfs/go-ipfs-api"
)

func cleanupCache(cacheTestfile string) {
	_, err := os.Stat(cacheTestfile)
	if err == nil {
		err := os.Remove(cacheTestfile)
		if err != nil {
			panic("Test cleanup failed")
		}
	}
}

func TestWrite(t *testing.T) {
	// Ensure the file isn't already there.
	cacheTestfile := "cache.db.test"
	cleanupCache(cacheTestfile)
	c := NewCache(cacheTestfile, "localhost:5001")
	_, err := os.Stat(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to stat cacheFile after initing cache. Err: %v\n", err)
	}

	// Try writing again
	err = c.Write(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to write after launching. Err: %v\n", err)
	}
}

func TestLoad(t *testing.T) {
	testIpfsPath := "/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf"
	// Ensure the file isn't already there.
	cacheTestfile := "cache.db.test"
	cleanupCache(cacheTestfile)
	c := NewCache(cacheTestfile, "localhost:5001")
	_, err := os.Stat(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to stat cacheFile after initing cache. Err: %v\n", err)
		return
	}

	// Now try to load
	err = c.Load(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to load cacheFile. Err: %v\n", err)
		return
	}

	testSong, _ := resource.NewSong(testIpfsPath, false)
	(*c.songMap)["foo"] = testSong

	// Try write and loading
	c.Write(cacheTestfile)
	c.Load(cacheTestfile)

	storedSong, exists := (*c.songMap)["foo"]
	if !exists {
		t.Errorf("Value we stored didn't load properly\n")
		return
	}
	if storedSong.IpfsPath() != testIpfsPath {
		t.Errorf("Value changed after store and load. e: %v, a: %v\n", testIpfsPath, storedSong.IpfsPath())
	}
}

func TestUrlNormalize(t *testing.T) {
	testTable := []struct {
		url      string
		expected string
	}{
		{"https://youtu.be/nAwTw1aYy6M", "https://youtu.be/nAwTw1aYy6M"},
		{"https://www.youtube.com/watch?v=JLpJPzKy6fY&feature=youtu.be", "https://youtu.be/JLpJPzKy6fY"},
		{"http://youtube.com/watch?v=JLpJPzKy6fY", "https://youtu.be/JLpJPzKy6fY"},
	}

	for _, test := range testTable {
		actual, err := urlNormalize(test.url)
		if err != nil || actual != test.expected {
			t.Errorf("Url normalization didn't produce expected result: E: %s, A: %s\n", test.expected, actual)
		}
	}
}

func TestLookup(t *testing.T) {
	testUrl := "https://youtu.be/nAwTw1aYy6M"
	testIpfsPath := "/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf"
	ipfsUrl := "localhost:5001"
	cacheTestfile := "cache.db.test"
	cleanupCache(cacheTestfile)
	c := NewCache(cacheTestfile, "localhost:5001")
	ipfs := shell.NewShell(ipfsUrl)

	// Lookup the url, the result shouldn't be able to find the IPFS url right away
	song, _ := c.Lookup(testUrl, false, false)
	if resourceID := song.ResourceID(); resourceID != testUrl {
		t.Errorf("cache lookup resulted in incorrect resourceID")
	}

	// Block until we're done with the DL
	_, _ = song.Resolve(ipfs)
	song, _ = c.Lookup(testUrl, false, false)
	if resourceID := song.ResourceID(); resourceID != testIpfsPath {
		t.Errorf("cache didn't find url even after it should have stored")
	}

	song, _ = c.Lookup(testIpfsPath, false, false)
	if resourceID := song.ResourceID(); resourceID != testIpfsPath {
		t.Errorf("cache lookup resulted in incorrect resourceID")
	}

	c.Write(cacheTestfile)
}
