package metadata

import (
	"os"
	"strconv"
	"testing"
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
	cacheTestfile := "TestWriteMetadataFile.test"
	cleanupCache(cacheTestfile)
	c := NewCache(cacheTestfile)
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
}

func TestLoad(t *testing.T) {
	// Ensure the file isn't already there.
	cacheTestfile := "TestLoadMetadataFile.test"
	cleanupCache(cacheTestfile)
	c := NewCache(cacheTestfile)
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
}

func TestStore(t *testing.T) {
	// Ensure the file isn't already there.
	cacheTestfile := "TestStore.test"
	cleanupCache(cacheTestfile)
	c := NewCache(cacheTestfile)
	_, err := os.Stat(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to stat cacheFile after initing cache. Err: %v\n", err)
	}

	// Add key/value pair
	err = c.Store("/ipfs/foofoofoofoo", "/ipfs/barbarbarbar")
	if err != nil {
		t.Errorf("Failed to load cacheFile. Err: %v\n", err)
	}

	cleanupCache(cacheTestfile)
}

func TestLookup(t *testing.T) {
	// Ensure the file isn't already there.
	cacheTestfile := "TestLookup.test"
	cleanupCache(cacheTestfile)
	c := NewCache(cacheTestfile)
	_, err := os.Stat(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to stat cacheFile after initing cache. Err: %v\n", err)
	}

	result := c.Lookup("/ipfs/foofoofoofoo")
	if result != "Unknown Track" {
		t.Errorf("Lookup of nonexistent key returned result.\n")
	}

	// Add key/value pair
	err = c.Store("/ipfs/foofoofoofoo", "/ipfs/barbarbarbar")
	if err != nil {
		t.Errorf("Failed to store new k/v pair. Err: %v\n", err)
	}

	result = c.Lookup("/ipfs/foofoofoofoo")
	if result != "/ipfs/barbarbarbar" {
		t.Errorf("Expected != actual. c.Lookup(\"/ipfs/foofoofoofoo\") => %v. Expected %v", result, "/ipfs/barbarbarbar")
	}

	// Now try to load
	err = c.Load(cacheTestfile)
	if err != nil {
		t.Errorf("Failed to load cacheFile. Err: %v\n", err)
	}

	result = c.Lookup("/ipfs/foofoofoofoo")
	if result != "/ipfs/barbarbarbar" {
		t.Errorf("Expected != actual after write/load. c.Lookup(\"/ipfs/foofoofoofoo\") => %v. Expected %v", result, "/ipfs/barbarbarbar")
	}

	cleanupCache(cacheTestfile)
}

func benchmarkWrite(numEntries int, b *testing.B) {
	// Ensure the file isn't already there.
	cacheTestfile := "BenchWrite.test"
	cleanupCache(cacheTestfile)
	c := NewCache(cacheTestfile)
	_, err := os.Stat(cacheTestfile)
	if err != nil {
		b.Errorf("Failed to stat cacheFile after initing cache. Err: %v\n", err)
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for entry := 0; entry < numEntries; entry++ {
			val := strconv.Itoa(entry)
			c.Store(val, val)
		}
	}
}

func BenchmarkWrite1(b *testing.B)    { benchmarkWrite(1, b) }
func BenchmarkWrite10(b *testing.B)   { benchmarkWrite(10, b) }
func BenchmarkWrite100(b *testing.B)  { benchmarkWrite(100, b) }
func BenchmarkWrite500(b *testing.B)  { benchmarkWrite(500, b) }
func BenchmarkWrite5000(b *testing.B) { benchmarkWrite(5000, b) }

func BenchmarkBatchWrite100000(b *testing.B) {
	// Ensure the file isn't already there.
	cacheTestfile := "BenchWrite.test"
	cleanupCache(cacheTestfile)
	c := NewCache(cacheTestfile)
	_, err := os.Stat(cacheTestfile)
	if err != nil {
		b.Errorf("Failed to stat cacheFile after initing cache. Err: %v\n", err)
	}

	for entry := 0; entry < 100000; entry++ {
		val := strconv.Itoa(entry)
		(*c.titleMap)[val] = val
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Store("val", "val")
	}
}

func BenchmarkBatchLoad100000(b *testing.B) {
	// Ensure the file isn't already there.
	cacheTestfile := "BenchWrite.test"
	cleanupCache(cacheTestfile)
	c := NewCache(cacheTestfile)
	_, err := os.Stat(cacheTestfile)
	if err != nil {
		b.Errorf("Failed to stat cacheFile after initing cache. Err: %v\n", err)
	}

	for entry := 0; entry < 100000; entry++ {
		val := strconv.Itoa(entry)
		(*c.titleMap)[val] = val
	}
	c.Store("val", "val")

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// Now try to load
		err = c.Load(cacheTestfile)
		if err != nil {
			b.Errorf("Failed to load cacheFile. Err: %v\n", err)
		}
	}
}
