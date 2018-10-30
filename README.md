# UtaStream [![](https://godoc.org/github.com/VivaLaPanda/uta-stream?status.svg)](http://godoc.org/github.com/VivaLaPanda/uta-stream) [![Build Status](https://travis-ci.org/VivaLaPanda/uta-stream.svg?branch=master)](https://travis-ci.org/VivaLaPanda/uta-stream) [![codecov](https://codecov.io/gh/VivaLaPanda/uta-stream/branch/master/graph/badge.svg)](https://codecov.io/gh/VivaLaPanda/uta-stream)

Simple(ish) webservice in Go for managing an online radio.
Core features:

* Interact via HTTP API or as a Discord bot (maybe Slack, etc eventually)
* Pull from various media sources (ipfs, youtube, soundcloud, etc) (playlist support is a must)
* Do more than just a streaming mp3/m3u, create a Discord bot that plays the stream
* Highly configurable options for automatic/manual queue management
    - DJ mode: Single user queues tracks (requests?)
    - Community mode: Any user queues tracks
    - Hybrid mode: Community mode but imple markov-based playing of previously queued songs if the queue is empty

## Installing
* Install IPFS (https://ipfs.io/) and start daemon
* Install ffmpeg in your PATH
* `go get github.com/VivaLaPanda/uta-stream`

## Contributing
The progress is now in a state where contributions would be welcome. Pull requests
are welcome and I'm excited to work with the community on this project.

Contribution guidelines are there, but to be honest
this is a small project, just use common sense.

## Progress:

Implemented:
* Core streaming library. One mp3 data stream to multiple clients in sync
* MP3 data processing, can read files and get necessary header data
* Queuing system, including auto mode
* Resource cache
* Downloaders
    - youtube
    - ipfsxx
* API
* Permissions (Key-based)
* Song info cache (ipfsHash -> Title/etc.)

In Progress:
* Downloaders
    - soundcloud
* Optional tiny frontend for interacting with API (https://github.com/VivaLaPanda/uta-steam-frontend)
* Playlist support

Ready to start:
* DJ Mode (only one user can queue)
* Share download/info caches (support network urls instead of local FS, or both at once and a syncing mechanism)
    - Possibly means moving caches into SQL

Stretch goals:
* Discord bot stuff
* Currently designed with no IPFS garbage collection, should eventually support that
