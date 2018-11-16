package main

import (
	"flag"

	"github.com/VivaLaPanda/uta-stream/api"
	"github.com/VivaLaPanda/uta-stream/mixer"
	"github.com/VivaLaPanda/uta-stream/queue"
	"github.com/VivaLaPanda/uta-stream/queue/auto"
	"github.com/VivaLaPanda/uta-stream/resource/cache"
	"github.com/VivaLaPanda/uta-stream/stream"
)

// Various runtime flags
var autoqFilename = flag.String("autoqFilename", "autoq.db", "Where to store autoq database")
var cacheFilename = flag.String("cacheFilename", "cache.db", "Where to store cache database")
var authCfgFilename = flag.String("authCfgFilename", "", "Where to find auth config json")
var ipfsUrl = flag.String("ipfsUrl", "localhost:5001", "The url of the local IPFS instance")
var enableAutoq = flag.Bool("enableAutoq", true, "Whether to use autoq feature")
var chainbreakProb = flag.Float64("chainbreakProb", .05, "Allows more random autoq")
var bitrate = flag.Int("bitrate", 128, "Affects stream smoothness/synchro")
var autoQPrefixLen = flag.Int("autoQPrefixLen", 1, "Smaller = more random") // Large values will be random if the history is short
var apiPort = flag.Int("apiPort", 8085, "Which port to serve the API on")
var audioPort = flag.Int("audioPort", 9090, "Which port to serve the audio stream on")

func main() {
	flag.Parse()

	c := cache.NewCache(*cacheFilename, *ipfsUrl)
	a := auto.NewAQEngine(*autoqFilename, c, *chainbreakProb, *autoQPrefixLen)
	q := queue.NewQueue(a, *enableAutoq, *ipfsUrl)
	e := mixer.NewMixer(q, *bitrate)

	go func() {
		stream.ServeAudioOverHttp(e.Output, *audioPort)
	}()
	api.ServeApi(e, c, q, *apiPort, *authCfgFilename)
}
