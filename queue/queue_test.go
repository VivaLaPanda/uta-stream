package queue

import (
	"os"
	"testing"

	"github.com/VivaLaPanda/uta-stream/queue/auto"
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

func TestPop(t *testing.T) {
	autoqTestfile := "autoqTestPop.test"
	// Make sure the q starts empty
	a := auto.NewAQEngine(autoqTestfile, false, 1)
	q := NewQueue(a, false)
	song, isEmpty := q.Pop()
	if isEmpty == false {
		t.Errorf("Queue didn't start empty. isEmpty was false.\n")
	}

	// Pop then push
	q.AddToQueue("test_1")
	q.AddToQueue("test_2")
	song, isEmpty = q.Pop()
	if isEmpty == true {
		t.Errorf("Queue still reporting empty after enqueue.\n")
	}
	if song != "test_1" {
		t.Errorf("Popped_1 != enqueue_1: %v != %v\n", song, "test_1")
	}
	song, isEmpty = q.Pop()
	if song != "test_2" {
		t.Errorf("Popped_2 != enqueue_2: %v != %v\n", song, "test_2")
	}

	cleanupAutoq(autoqTestfile)
}

func TestPlayNext(t *testing.T) {
	autoqTestfile := "autoqTestPlayNext.test"
	// Make sure the q starts empty
	a := auto.NewAQEngine(autoqTestfile, false, 1)
	q := NewQueue(a, false)
	song, isEmpty := q.Pop()
	if isEmpty == false {
		t.Errorf("Queue didn't start empty. isEmpty was false.\n")
	}

	// Pop then push
	q.PlayNext("test_1")
	q.PlayNext("test_2")
	song, isEmpty = q.Pop()
	if isEmpty == true {
		t.Errorf("Queue still reporting empty after enqueue.\n")
	}
	if song != "test_2" {
		t.Errorf("Popped_1 != pushed_1: %v != %v\n", song, "test_2")
	}
	song, isEmpty = q.Pop()
	if song != "test_1" {
		t.Errorf("Popped_2 != pushed_2: %v != %v\n", song, "test_1")
	}

	cleanupAutoq(autoqTestfile)
}
