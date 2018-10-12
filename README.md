# Uta Stream [![](https://godoc.org/github.com/VivaLaPanda/uta-stream?status.svg)](http://godoc.org/github.com/VivaLaPanda/uta-stream)

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

## Contributing
The progress is now in a state where contributions would be welcome. Pull requests
are welcome and I'm excited to work with the community on this project.
The code is scattered with TODO tags, those could use work and I'll
convert them to issues soon.

Contribution guidelines are there, but to be honest
this is a small project, just use common sense.

## Progress:

Done:
* Core streaming library. One mp3 data stream to multiple clients in sync
* MP3 data processing, can read files and get necessary header data
* Queuing system, including auto mode
* Resource cache
* Downloaders
    - youtube
    - ipfs
* API

In Progress:
* Downloaders
    - soundcloud

Ready to start:
* DJ Mode (only one user can queue)
* Permissions (Key-based)
* Fixing up encoder to work off stream not file

Stretch goals:
* Discord bot stuff
* Currently designed with no IPFS garbage collection, should eventually support that
