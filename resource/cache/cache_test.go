package cache

import (
	"os"
	"testing"

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
	// Ensure the file isn't already there.
	cacheTestfile := "cache.db.test"
	cleanupCache(cacheTestfile)
	c := NewCache(cacheTestfile, "localhost:5001")
	_, err := os.Stat(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to stat cacheFile after initing cache. Err: %v\n", err)
	}

	// Now try to load
	err = c.Load(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to load cacheFile. Err: %v\n", err)
	}
}

func TestLookup(t *testing.T) {
	testUrl := "https://youtu.be/nAwTw1aYy6M"
	testIpfsPath := "/ipfs/QmRRKwCPfmAf8A9crYCisfFuSDbwerthf5NBQ2h334vQsb"
	ipfsUrl := "localhost:5001"
	cacheTestfile := "cache.db.test"
	cleanupCache(cacheTestfile)
	c := NewCache(cacheTestfile, "localhost:5001")
	ipfs := shell.NewShell(ipfsUrl)

	// Lookup the url, the result shouldn't be able to find the IPFS url right away
	song, _ := c.Lookup(testUrl, false, false)
	if resourceID, isCached := song.ResourceID(); resourceID != testUrl || isCached != false {
		t.Errorf("cache lookup resulted in incorrect resourceID")
	}

	// Block until we're done with the DL
	_, _ = song.Resolve(ipfs)
	song, _ = c.Lookup(testUrl, false, false)
	if resourceID, isCached := song.ResourceID(); resourceID != testIpfsPath || isCached != true {
		t.Errorf("cache didn't find url even after it should have stored")
	}

	song, _ = c.Lookup(testIpfsPath, false, false)
	if resourceID, isCached := song.ResourceID(); resourceID != testIpfsPath || isCached != true {
		t.Errorf("cache lookup resulted in incorrect resourceID")
	}
}
