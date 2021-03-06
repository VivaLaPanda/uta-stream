// Package stream provides the logic to connect a packet stream to a plurality
// of HTTP clients.
package stream

import (
	"container/list"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"
)

var consumers = make(map[string]chan []byte)
var killConsumer = make(chan string)
var consumerWLock = sync.Mutex{}
var lastChunks = list.New()
var lastChunksLock = sync.RWMutex{}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Seed the random generator
func init() {
	rand.Seed(time.Now().UnixNano())
}

// Create an n character string of random letters
func randIDGenerator(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func generateNewStream(w http.ResponseWriter, req *http.Request) {
	// Setup flusher and headers
	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("expected http.ResponseWriter to be an http.Flusher")
	}
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-cache")

	// Register stream
	mediaConsumer := make(chan []byte, 4)
	consumerID := randIDGenerator(32)

	consumerWLock.Lock()
	consumers[consumerID] = mediaConsumer
	consumerWLock.Unlock()

	// If the connection is closed, kill the consumer
	done := req.Context().Done()
	go func() {
		<-done
		log.Printf("User %s disconnected", req.RemoteAddr)
		killConsumer <- consumerID
	}()

	log.Printf("User %s connected", req.RemoteAddr)

	// Write the last chunk to bootstrap the stream
	lastChunksLock.RLock()
	for bytes := lastChunks.Front(); bytes != nil; bytes = bytes.Next() {
		w.Write(bytes.Value.([]byte))
	}
	lastChunksLock.RUnlock()
	flusher.Flush()

	// Recive bytes from the channel and respond with them
	var err error
	for bytesToStream := range mediaConsumer {
		_, err = w.Write(bytesToStream)
		if err != nil {
			_, err = w.Write(bytesToStream) // Retry once
			if err != nil {
				log.Printf("User %s disconnected", req.RemoteAddr)
				return
			}
		}
		flusher.Flush() // Trigger "chunked" encoding and send a chunk...
	}
}

func ListenerCount() int {
	return len(consumers)
}

func ServeAudioOverHttp(inputAudio <-chan []byte, port int) {
	/* Net listener */
	n := "tcp"
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	l, err := net.Listen(n, addr)
	if err != nil {
		panic("Failed to start audio server")
	}

	// Listen for channels that need to be closed
	// Potential race condition if the consumer is deleted after the broadcaster
	// below already enters it in the loop
	// TODO: Fix that race condition https://github.com/VivaLaPanda/uta-stream/issues/2
	go func() {

	}()

	// Init fifo queue of size 3
	var emptyArr []byte
	for idx := 0; idx < 16; idx++ {
		lastChunks.PushBack(emptyArr)
	}

	// Listen to incoming audio bytes and push them out to all consumers
	// If a consumer is blocking, just ignore it and keep going
	go func() {
		for audioBytes := range inputAudio {
			// Bytes need to be spaced out to keep the client from getting too
			// far ahead
			time.Sleep(500 * time.Millisecond)

			badConsumerCounter := make(map[string]int, len(consumers))

			// If we've been given a kill signal for a consumer handle that now
			select {
			case consumerToKill := <-killConsumer:
				chanCopy := consumers[consumerToKill]

				consumerWLock.Lock()
				delete(consumers, consumerToKill)
				consumerWLock.Unlock()

				close(chanCopy)
			default:
			}

			for id, consumer := range consumers {

				select {
				case consumer <- audioBytes:
					// Send was good, do nothing
				default:
					// Consumers that refuse to consume data will eventually cause a fatal overflow
					// If a consumer repeatedly fails, forcibly disconnect them.
					log.Printf("Overburdened consumer")

					badConsumerCounter[id] += 1
					if badConsumerCounter[id] > 10 {
						delete(badConsumerCounter, id)

						killConsumer <- id
					}
				}
			}

			// Maintain fifo queue
			lastChunksLock.Lock()
			lastChunks.Remove(lastChunks.Front())
			lastChunks.PushBack(audioBytes)
			lastChunksLock.Unlock()
		}
	}()

	/* HTTP server */
	server := http.Server{
		Handler: http.HandlerFunc(generateNewStream),
	}
	log.Printf("Audio server is listening at %s", addr)
	if err := server.Serve(l); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %s: %v\n", addr, err)
	}
}
