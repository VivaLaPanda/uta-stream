package stream

import (
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
	mediaConsumer := make(chan []byte, 128)
	consumerID := randIDGenerator(32)
	//defer func() { killConsumer <- consumerID }()
	consumerWLock.Lock()
	consumers[consumerID] = mediaConsumer
	consumerWLock.Unlock()

	// If the connection is closed, kill the consumer
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		killConsumer <- consumerID
	}()

	// Recive bytes from the channel and respond with them
	var err error
	for bytesToStream := range mediaConsumer {
		_, err = w.Write(bytesToStream)
		if err != nil {
			errString := fmt.Sprintf("Copying audio data into response failed: %v", err)
			log.Fatalf(errString)
			http.Error(w, errString, 500)
			return
		}
		flusher.Flush() // Trigger "chunked" encoding and send a chunk...
	}
}

func ServeAudioOverHttp(inputAudio <-chan []byte, packetsPerSecond int, port int) {
	/* Net listener */
	n := "tcp"
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	l, err := net.Listen(n, addr)
	if err != nil {
		panic("Failed to start server")
	}

	// Listen for channels that need to be closed
	// Potential race condition if the consumer is deleted after the broadcaster
	// below already enters it in the loop
	// TODO: Fix that race condition
	go func() {
		for idToKill := range killConsumer {
			chanCopy := consumers[idToKill]

			consumerWLock.Lock()
			delete(consumers, idToKill)
			consumerWLock.Unlock()

			close(chanCopy)
		}
	}()

	// Listen to incoming audio bytes and push them out to all consumers
	// If a consumer is blocking, just ignore it and keep going
	go func() {
		for audioBytes := range inputAudio {
			for _, consumer := range consumers {
				select {
				case consumer <- audioBytes:
					// Send was good, do nothing
				default:
					// Send failed, we don't care
					// This indicates an overburdened connection and will cause dropped
					// audio
				}
			}

			time.Sleep(time.Duration(1000/packetsPerSecond) * time.Millisecond)
		}
	}()

	/* HTTP server */
	server := http.Server{
		Handler: http.HandlerFunc(generateNewStream),
	}
	log.Printf("Server is listening at %s", addr)
	if err := server.Serve(l); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %s: %v\n", addr, err)
	}
}
