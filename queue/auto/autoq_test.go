package auto

import (
	"os"
	"testing"
	"time"

	"github.com/VivaLaPanda/uta-stream/resource/cache"
)

func cleanupAutoq(autoqTestfile string) {
	_, err := os.Stat(autoqTestfile)
	if err == nil {
		err := os.Remove(autoqTestfile)
		if err != nil {
			panic("Test cleanup failed")
		}
	}
}

func TestWrite(t *testing.T) {
	// Ensure the file isn't already there.
	autoqTestfile := "autoq.db.test"
	cacheFile := "cache.db.test"
	c := cache.NewCache(cacheFile, "localhost:5001")
	q := NewAQEngine(autoqTestfile, c, 0, 1, 0)
	_, err := os.Stat(autoqTestfile)
	if err != nil {
		t.Errorf("Failed to stat qfile after initing autoq. Err: %v\n", err)
	}

	// Try writing again
	err = q.Write(autoqTestfile)
	if err != nil {
		t.Errorf("Failed to write after launching. Err: %v\n", err)
	}

	cleanupAutoq(autoqTestfile)
	cleanupAutoq(cacheFile)
}

func TestLoad(t *testing.T) {
	// Ensure the file isn't already there.
	autoqTestfile := "TestLoadQfile.test"
	cacheFile := "cache.db.test"
	c := cache.NewCache(cacheFile, "localhost:5001")
	q := NewAQEngine(autoqTestfile, c, 0, 1, 0)
	_, err := os.Stat(autoqTestfile)
	if err != nil {
		t.Errorf("Failed to stat qfile after initing autoq. Err: %v\n", err)
	}

	// Now try to load
	err = q.Load(autoqTestfile)
	if err != nil {
		t.Errorf("Failed to load qfile. Err: %v\n", err)
	}

	cleanupAutoq(autoqTestfile)
	cleanupAutoq(cacheFile)
}

func TestNotifyPlayed(t *testing.T) {
	// Bare bones notifyplayed test
	// Ensure the file isn't already there.
	autoqTestfile := "TestLoadQfile.test"
	cacheFile := "cache.db.test"
	c := cache.NewCache(cacheFile, "localhost:5001")
	q := NewAQEngine(autoqTestfile, c, 0, 1, 0)
	_, err := os.Stat(autoqTestfile)
	if err != nil {
		t.Errorf("Failed to stat qfile after initing autoq. Err: %v\n", err)
	}

	q.NotifyPlayed("test", true)

	// If we didn't panic than this test is a pass
	cleanupAutoq(autoqTestfile)
	cleanupAutoq(cacheFile)
}

func TestVpop(t *testing.T) {
	// Simple test of vpop
	// Ensure the file isn't already there.
	autoqTestfile := "TestLoadQfile.test"
	cacheFile := "cache.db.test"
	c := cache.NewCache(cacheFile, "localhost:5001")
	q := NewAQEngine(autoqTestfile, c, 1, 1, 0)
	_, err := os.Stat(autoqTestfile)
	if err != nil {
		t.Errorf("Failed to stat qfile after initing autoq. Err: %v\n", err)
	}

	// Create a chain which is a cycle between test_a and test_b states
	q.NotifyPlayed("test_a", true)
	q.NotifyPlayed("test_b", true)
	q.NotifyPlayed("test_a", true)
	q.NotifyPlayed("test_b", true)
	time.Sleep(1) // Necessary because NotifyPlayed is async

	// Since the last song was a b, the next should be an a
	song, _ := q.Vpop()
	songPath := song.ResourceID()
	if songPath != "test_a" {
		t.Errorf("Autoq produced unexpected song (expected != actual). %v != %v", "test_a", song)
	}

	// Play an a, now the next should be a b
	q.NotifyPlayed("test_a", true)
	time.Sleep(1)
	song, _ = q.Vpop()

	songPath = song.ResourceID()
	if songPath != "test_b" {
		t.Errorf("Autoq produced unexpected song (expected != actual). %v != %v", "test_b", song)
	}

	cleanupAutoq(autoqTestfile)
	cleanupAutoq(cacheFile)
}

func TestMigrate(t *testing.T) {
	// Ensure the file isn't already there.
	autoqTestfile := "autoq.db"
	cacheFile := "cache.db.test"
	c := cache.NewCache(cacheFile, "localhost:5001")
	q := NewAQEngine(autoqTestfile, c, 0, 1, 0)
	_, err := os.Stat(autoqTestfile)
	if err != nil {
		t.Errorf("Failed to stat qfile after initing autoq. Err: %v\n", err)
	}

	// Now try to load
	err = q.Load(autoqTestfile)
	if err != nil {
		t.Errorf("Failed to load qfile. Err: %v\n", err)
	}

	for k, v := range *q.markovChain.chainData {
		(*q.markovChain.chainData)[k] = cleanAutoq(v, k)
		if len((*q.markovChain.chainData)[k]) > 3 {
			(*q.markovChain.chainData)[k] = (*q.markovChain.chainData)[k][:3]
		}
	}

	q.Write(autoqTestfile)
}

func cleanAutoq(elements []string, key string) []string {
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	result := []string{}

	for v := range elements {
		if encountered[elements[v]] == true || key == elements[v] {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[elements[v]] = true
			// Append to result slice.
			result = append(result, elements[v])
		}
	}
	// Return the new slice.
	return result
}
