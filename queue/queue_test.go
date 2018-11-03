package queue

import (
	"os"
	"testing"

	"github.com/VivaLaPanda/uta-stream/queue/auto"
	"github.com/VivaLaPanda/uta-stream/resource/cache"
)

func cleanup(file string) {
	_, err := os.Stat(file)
	if err == nil {
		err := os.Remove(file)
		if err != nil {
			panic("Test cleanup failed")
		}
	}
}

var (
	metadataCacheFile = "metadata.db.test"
	cacheFile         = "cache.db.test"
)

func TestPop(t *testing.T) {
	autoqTestfile := "autoqTestPop.test"
	ipfsUrl := "localhost:5001"
	// Make sure the q starts empty
	a := auto.NewAQEngine(autoqTestfile, 0, 1)
	q := NewQueue(a, c, false, ipfsUrl)
	song, _, isEmpty, _ := q.Pop()
	if isEmpty == false {
		t.Errorf("Queue didn't start empty. isEmpty was false.\n")
	}

	// Pop then push
	q.AddToQueue("/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf")
	q.AddToQueue("/ipfs/QmeFmYKQD6ky5d2uB7qSBBDpo8XtSP3iSfATEpxj6KULSX")
	song, _, isEmpty, _ = q.Pop()
	if isEmpty == true {
		t.Errorf("Queue still reporting empty after enqueue.\n")
	}
	if song != "/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf" {
		t.Errorf("Popped_1 != enqueue_1: %v != %v\n", song, "/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf")
	}
	song, _, isEmpty, _ = q.Pop()
	if song != "/ipfs/QmeFmYKQD6ky5d2uB7qSBBDpo8XtSP3iSfATEpxj6KULSX" {
		t.Errorf("Popped_2 != enqueue_2: %v != %v\n", song, "/ipfs/QmeFmYKQD6ky5d2uB7qSBBDpo8XtSP3iSfATEpxj6KULSX")
	}

	cleanup(autoqTestfile)
	cleanup(metadataCacheFile)
	cleanup(cacheFile)
}

func TestPlayNext(t *testing.T) {
	autoqTestfile := "autoqTestPlayNext.test"
	// Make sure the q starts empty
	a := auto.NewAQEngine(autoqTestfile, 0, 1)
	info := metadata.NewCache(metadataCacheFile)
	c := cache.NewCache(cacheFile, info, "localhost:5001")
	q := NewQueue(a, c, false)
	song, _, isEmpty, _ := q.Pop()
	if isEmpty == false {
		t.Errorf("Queue didn't start empty. isEmpty was false.\n")
	}

	// Pop then push
	q.PlayNext("/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf")
	q.PlayNext("/ipfs/QmeFmYKQD6ky5d2uB7qSBBDpo8XtSP3iSfATEpxj6KULSX")
	song, _, isEmpty, _ = q.Pop()
	if isEmpty == true {
		t.Errorf("Queue still reporting empty after enqueue.\n")
	}
	if song != "/ipfs/QmeFmYKQD6ky5d2uB7qSBBDpo8XtSP3iSfATEpxj6KULSX" {
		t.Errorf("Popped_1 != pushed_1: %v != %v\n", song, "/ipfs/QmeFmYKQD6ky5d2uB7qSBBDpo8XtSP3iSfATEpxj6KULSX")
	}
	song, _, isEmpty, _ = q.Pop()
	if song != "/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf" {
		t.Errorf("Popped_2 != pushed_2: %v != %v\n", song, "/ipfs/QmQmjmsqhvTNsvZGrwBMhGEX5THCoWs2GWjszJ48tnr3Uf")
	}

	cleanup(autoqTestfile)
	cleanup(metadataCacheFile)
	cleanup(cacheFile)
}
