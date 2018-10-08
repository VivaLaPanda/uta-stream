# Uta Stream

Simple(ish) webservice in Go for managing an online radio.
Core features:

* Interact via HTTP API or as a Discord bot (maybe Slack, etc eventually)
* Pull from various media sources (ipfs, youtube, soundcloud, etc) (playlist support is a must)
* Do more than just a streaming mp3/m3u, create a Discord bot that plays the stream
* Highly configurable options for automatic/manual queue management
    - Auto mode: Simple markov-based playing of previously queued songs
    - DJ mode: Single user queues tracks (requests?)
    - Community mode: Any user queues tracks
    - Hybrid mode: Auto mode, but users can queue and all manually queued songs will be played before going back to the auto-queue

## Installing
* Install IPFS (https://ipfs.io/) and start daemon
* Install ffmpeg in your PATH
* `go get github.com/VivaLaPanda/uta-stream`
    - During development you may need to `go get` extra stuff because there isn't a `main.go` yet

## Progress:

Done:
* Core streaming library. One mp3 data stream to multiple clients in sync
* MP3 data processing, can read files and get necessary header data
* Queuing system, including auto mode
* Resource cache

In Progress:
* Downloaders
    - youtube
    - soundcloud
    - ipfs
* API

Ready to start:
* DJ Mode
* Permissions (Key-based)

Stretch goals:
* Discord bot stuff
* Currently designed with no IPFS garbage collection, should eventually support that
* Fixing up encoder to work off stream not file
