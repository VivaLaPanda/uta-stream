package mixer

import (
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VivaLaPanda/uta-stream/resource"
)

type popResult struct {
	song     *resource.Song
	reader   io.ReadCloser
	empty    bool
	fromAuto bool
}

// scriptedQueue returns the given Pop results in order
func scriptedQueue(t *testing.T, results ...popResult) func() (*resource.Song, io.ReadCloser, bool, bool) {
	return func() (*resource.Song, io.ReadCloser, bool, bool) {
		if len(results) == 0 {
			t.Fatal("nextPlayable popped past the end of the script")
		}
		r := results[0]
		results = results[1:]
		return r.song, r.reader, r.empty, r.fromAuto
	}
}

func TestNextPlayableSkipsUnresolvableSongs(t *testing.T) {
	broken, _ := resource.NewSong("/ipfs/QmBroken")
	good, _ := resource.NewSong("/ipfs/QmGood")
	reader := io.NopCloser(strings.NewReader("audio"))

	var slept []time.Duration
	song, gotReader, fromAuto := nextPlayable(scriptedQueue(t,
		popResult{empty: true},                  // nothing queued yet
		popResult{song: broken, fromAuto: true}, // autoq song that failed to resolve
		popResult{song: nil},                    // no song and no reader: must not panic
		popResult{song: good, reader: reader, fromAuto: true},
	), func(d time.Duration) { slept = append(slept, d) })

	if song != good || gotReader != reader || !fromAuto {
		t.Fatalf("got song %v, reader %v, fromAuto %v; want the good song", song, gotReader, fromAuto)
	}
	want := []time.Duration{2 * time.Second, 1 * time.Second, 2 * time.Second}
	if !reflect.DeepEqual(slept, want) {
		t.Errorf("slept %v, want %v", slept, want)
	}
}

func TestFailureBackoff(t *testing.T) {
	cases := map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second,
		5: 16 * time.Second, 6: 30 * time.Second, 50: 30 * time.Second}
	for failures, want := range cases {
		if got := failureBackoff(failures); got != want {
			t.Errorf("failureBackoff(%d) = %v, want %v", failures, got, want)
		}
	}
}
