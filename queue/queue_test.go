package queue

import (
	"os"
	"testing"

	"github.com/VivaLaPanda/uta-stream/queue/auto"
	"github.com/VivaLaPanda/uta-stream/resource"
	"github.com/VivaLaPanda/uta-stream/resource/cache"
)

var (
	cacheFile = "cache.db.test"
	testSongA *resource.Song
	testSongB *resource.Song
)

func init() {
	testSongA, _ = resource.NewSong("/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf", false)
	testSongB, _ = resource.NewSong("/ipfs/QmeFmYKQD6ky5d2uB7qSBBDpo8XtSP3iSfATEpxj6KULSX", false)
}

func cleanup(file string) {
	_, err := os.Stat(file)
	if err == nil {
		err := os.Remove(file)
		if err != nil {
			panic("Test cleanup failed")
		}
	}
}

func TestPop(t *testing.T) {
	autoqTestfile := "autoqTestPop.test"
	ipfsUrl := "localhost:5001"
	c := cache.NewCache(cacheFile, "localhost:5001")
	// Make sure the q starts empty
	a := auto.NewAQEngine(autoqTestfile, c, 0, 1, 0)
	q := NewQueue(a, false, ipfsUrl)
	song, _, isEmpty, _ := q.Pop()
	if isEmpty == false {
		t.Errorf("Queue didn't start empty. isEmpty was false.\n")
	}

	// Pop then push
	q.AddToQueue(testSongA)
	q.AddToQueue(testSongB)
	song, _, isEmpty, _ = q.Pop()
	if isEmpty == true {
		t.Errorf("Queue still reporting empty after enqueue.\n")
		return
	}
	if song.IpfsPath() != testSongA.IpfsPath() {
		t.Errorf("Popped_1 != enqueue_1: %v != %v\n", song, testSongA)
	}
	song, _, isEmpty, _ = q.Pop()
	if song.IpfsPath() != testSongB.IpfsPath() {
		t.Errorf("Popped_2 != enqueue_2: %v != %v\n", song, testSongB)
	}

	cleanup(autoqTestfile)
	cleanup(cacheFile)
}

func TestPlayNext(t *testing.T) {
	autoqTestfile := "autoqTestPlayNext.test"
	// Make sure the q starts empty
	c := cache.NewCache(cacheFile, "localhost:5001")
	a := auto.NewAQEngine(autoqTestfile, c, 0, 1, 0)
	q := NewQueue(a, false, "localhost:5001")
	song, _, isEmpty, _ := q.Pop()
	if isEmpty == false {
		t.Errorf("Queue didn't start empty. isEmpty was false.\n")
	}

	// Pop then push
	q.PlayNext(testSongA)
	q.PlayNext(testSongB)
	song, _, isEmpty, _ = q.Pop()
	if isEmpty == true {
		t.Errorf("Queue still reporting empty after enqueue.\n")
		return
	}
	if song.IpfsPath() != testSongB.IpfsPath() {
		t.Errorf("Popped_1 != pushed_1: %v != %v\n", song, testSongB)
	}
	song, _, isEmpty, _ = q.Pop()
	if song.IpfsPath() != testSongA.IpfsPath() {
		t.Errorf("Popped_2 != pushed_2: %v != %v\n", song, testSongA)
	}

	cleanup(autoqTestfile)
	cleanup(cacheFile)
}

func TestIsEmpty(t *testing.T) {
	autoqTestfile := "autoqTestIsEmpty.test"
	// Make sure the q starts empty
	c := cache.NewCache(cacheFile, "localhost:5001")
	a := auto.NewAQEngine(autoqTestfile, c, 0, 1, 0)
	q := NewQueue(a, false, "localhost:5001")
	if q.IsEmpty() == false {
		t.Errorf("Queue didn't start empty. isEmpty was false.\n")
		return
	}

	q.PlayNext(testSongB)
	if q.IsEmpty() == true {
		t.Errorf("Queue still reporting empty after enqueue.\n")
		return
	}

	cleanup(autoqTestfile)
	cleanup(cacheFile)
}

func TestDump(t *testing.T) {
	autoqTestfile := "autoqTestDump.test"
	// Make sure the q starts empty
	c := cache.NewCache(cacheFile, "localhost:5001")
	a := auto.NewAQEngine(autoqTestfile, c, 0, 1, 0)
	q := NewQueue(a, false, "localhost:5001")

	q.PlayNext(testSongB)
	q.PlayNext(testSongB)
	q.PlayNext(testSongB)
	q.PlayNext(testSongB)

	q.Dump()

	if q.IsEmpty() == false {
		t.Errorf("Queue not reporting empty after dump.\n")
		return
	}

	cleanup(autoqTestfile)
	cleanup(cacheFile)
}

func TestGetQueue(t *testing.T) {
	autoqTestfile := "autoqTestGetQueue.test"
	// Make sure the q starts empty
	c := cache.NewCache(cacheFile, "localhost:5001")
	a := auto.NewAQEngine(autoqTestfile, c, 0, 1, 0)
	q := NewQueue(a, false, "localhost:5001")

	q.PlayNext(testSongA)
	q.PlayNext(testSongA)
	q.PlayNext(testSongA)
	q.PlayNext(testSongA)

	songs := q.GetQueue()

	if songs[0].ResourceID() != "/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf" {
		t.Errorf("GetQueue didn't give us the song we put in. Output: %v\n", songs[0])
		return
	}

	cleanup(autoqTestfile)
	cleanup(cacheFile)
}
