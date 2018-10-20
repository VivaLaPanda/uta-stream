package main

import (
	"flag"

	"github.com/VivaLaPanda/uta-stream/api"
	"github.com/VivaLaPanda/uta-stream/mixer"
	"github.com/VivaLaPanda/uta-stream/queue"
	"github.com/VivaLaPanda/uta-stream/queue/auto"
	"github.com/VivaLaPanda/uta-stream/resource/cache"
	"github.com/VivaLaPanda/uta-stream/resource/metadata"
	"github.com/VivaLaPanda/uta-stream/stream"
)

// Various runtime flags
var autoqFilename = flag.String("autoqFilename", "autoq.db", "Where to store autoq database")
var metadataFilename = flag.String("metadataFilename", "metadata.db", "Where to store metadata database")
var cacheFilename = flag.String("cacheFilename", "cache.db", "Where to store cache database")
var authCfgFilename = flag.String("authCfgFilename", "", "Where to find auth config json")
var ipfsUrl = flag.String("ipfsUrl", "localhost:5001", "The url of the local IPFS instance")
var enableAutoq = flag.Bool("enableAutoq", true, "Whether to use autoq feature")
var chainbreakProb = flag.Float64("chainbreakProb", .05, "Allows more random autoq")
var packetsPerSecond = flag.Int("packetsPerSecond", 2, "Affects stream smoothness/synchro")
var autoQPrefixLen = flag.Int("autoQPrefixLen", 1, "Smaller = more random") // Large values will be random if the history is short
var apiPort = flag.Int("apiPort", 8085, "Which port to serve the API on")
var audioPort = flag.Int("audioPort", 9090, "Which port to serve the audio stream on")

func main() {
	flag.Parse()

	info := metadata.NewCache(*metadataFilename)
	a := auto.NewAQEngine(*autoqFilename, *chainbreakProb, *autoQPrefixLen)
	c := cache.NewCache(*cacheFilename, info, *ipfsUrl)
	q := queue.NewQueue(a, c, *enableAutoq)
	e := mixer.NewMixer(q, c, *packetsPerSecond)

	go func() {
		stream.ServeAudioOverHttp(e.Output, *packetsPerSecond, *audioPort)
	}()
	api.ServeApi(e, c, q, info, *apiPort, *authCfgFilename)
}
