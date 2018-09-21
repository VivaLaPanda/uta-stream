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
