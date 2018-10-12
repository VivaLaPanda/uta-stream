package main

import (
	"flag"

	"github.com/VivaLaPanda/uta-stream/api"
	"github.com/VivaLaPanda/uta-stream/encoder"
	"github.com/VivaLaPanda/uta-stream/queue"
	"github.com/VivaLaPanda/uta-stream/resource/cache"
	"github.com/VivaLaPanda/uta-stream/stream"
)

// Various runtime flags
var autoqFilename = flag.String("autoqFilename", "autoq.db", "Where to store autoq database")
var cacheFilename = flag.String("cacheFilename", "cache.db", "Where to store cache database")
var enableAutoq = flag.Bool("enableAutoq", true, "Whether to use autoq feature")
var allowChainbreak = flag.Bool("allowChainbreak", true, "Allows more random autoq")
var packetsPerSecond = flag.Int("packetsPerSecond", 2, "Affects stream smoothness/synchro")
var apiPort = flag.Int("apiPort", 8085, "Which port to serve the API on")
var audioPort = flag.Int("audioPort", 9090, "Which port to serve the audio stream on")

func main() {
	flag.Parse()

	q := queue.NewQueue(*autoqFilename, *enableAutoq, *allowChainbreak)
	c := cache.NewCache(*cacheFilename)
	e := encoder.NewEncoder(q, c, *packetsPerSecond)

	go func() {
		stream.ServeAudioOverHttp(e.Output, *packetsPerSecond, *audioPort)
	}()
	api.ServeApi(e, c, q, *apiPort)
}
