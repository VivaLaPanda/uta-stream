package cache

import (
	"io"
	"os"
	"testing"

	"github.com/VivaLaPanda/uta-stream/resource/metadata"
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
	cacheTestfile := "TestWriteCacheFile.test"
	metadataTestfile := "metadataCache.test"
	cleanupCache(cacheTestfile)
	cleanupCache(metadataTestfile)
	meta := metadata.NewCache(metadataTestfile)
	c := NewCache(cacheTestfile, meta, "localhost:5001")
	_, err := os.Stat(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to stat cacheFile after initing cache. Err: %v\n", err)
	}

	// Try writing again
	err = c.Write(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to write after launching. Err: %v\n", err)
	}

	cleanupCache(cacheTestfile)
	cleanupCache(metadataTestfile)
}

func TestLoad(t *testing.T) {
	// Ensure the file isn't already there.
	cacheTestfile := "TestLoadCacheFile.test"
	metadataTestfile := "metadataCache.test"
	cleanupCache(cacheTestfile)
	cleanupCache(metadataTestfile)
	meta := metadata.NewCache(metadataTestfile)
	c := NewCache(cacheTestfile, meta, "localhost:5001")
	_, err := os.Stat(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to stat cacheFile after initing cache. Err: %v\n", err)
	}

	// Now try to load
	err = c.Load(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to load cacheFile. Err: %v\n", err)
	}

	cleanupCache(cacheTestfile)
	cleanupCache(metadataTestfile)
}

func TestFetchUrl(t *testing.T) {
	// Ensure the file isn't already there.
	cacheTestfile := "TestLoadCacheFile.test"
	metadataTestfile := "metadataCache.test"
	cleanupCache(cacheTestfile)
	cleanupCache(metadataTestfile)
	meta := metadata.NewCache(metadataTestfile)
	c := NewCache(cacheTestfile, meta, "localhost:5001")
	_, err := os.Stat(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to stat cacheFile after initing cache. Err: %v\n", err)
	}

	// Get the reader which should contain the song data
	ipfsPath, songReader, err := c.FetchUrl("https://youtu.be/nAwTw1aYy6M")
	if err != nil {
		t.Errorf("Failed to get song from ipfs. Err: %v\n", err)
		return
	}
	expectedPath := "/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf"
	if ipfsPath != expectedPath {
		t.Errorf("IPFS path doesn't match testing default. Expected: %v\nActual: %v\n", expectedPath, ipfsPath)
		return
	}

	// Open file for writing
	songFile, err := os.OpenFile("test_song.mp3", os.O_RDWR|os.O_CREATE, 0660)
	if err != nil {
		t.Errorf("Failed to open song file for writing. Err: %v\n", err)
		return
	}

	// Copy data from reader to writer and then close both
	io.Copy(songFile, songReader)
	songFile.Close()
	songReader.Close()

	// File should be written now. Manual verification is needed to confirm
	// the data is correct
	cleanupCache(cacheTestfile)
	cleanupCache(metadataTestfile)
}
