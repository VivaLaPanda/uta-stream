package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/VivaLaPanda/uta-stream/mixer"
	"github.com/VivaLaPanda/uta-stream/queue"
	"github.com/VivaLaPanda/uta-stream/resource/cache"
)

type key int

const (
	requestIDKey key = 0
)

var (
	healthy int32
)

func ServeApi(m *mixer.Mixer, c *cache.Cache, q *queue.Queue, port int) {
	logger := log.New(os.Stdout, "http: ", log.LstdFlags)
	logger.Println("Server is starting...")

	// Router setup
	router := http.NewServeMux()
	router.Handle("/", index())
	router.Handle("/enqueue", enqueue(q, c))
	router.Handle("/playnext", playnext(q, c))
	router.Handle("/skip", skip(m))
	router.Handle("/play", play(m))
	router.Handle("/pause", pause(m))

	nextRequestID := func() string {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// Basic server setup
	listenAddr := fmt.Sprintf("127.0.0.1:%d", port)
	server := &http.Server{
		Addr:         listenAddr,
		Handler:      tracing(nextRequestID)(logging(logger)(router)),
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		<-quit
		logger.Println("Server is shutting down...")
		atomic.StoreInt32(&healthy, 0)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()

	logger.Println("Server is ready to handle requests at", listenAddr)
	atomic.StoreInt32(&healthy, 1)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}

	<-done
	logger.Println("Server stopped")
}

func index() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "This is the UtaStream client API. Documentation on routes is at https://github.com/VivaLaPanda/uta-stream")
	})
}

func enqueue(q *queue.Queue, c *cache.Cache) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		resourceToQueue := r.URL.Query().Get("song")
		if resourceToQueue == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "/enqueue expects a song resource identifier in the request.\n"+
				"eg api.example/enqueue?song1=https%3A%2F%2Fyoutu.be%2FnAwTw1aYy6M") // https://youtu.be/nAwTw1aYy6M
		}

		// If we're looking at an ipfs path just leave as is
		// Otherwise go and fetch it
		urgent := q.Length() == 0
		if resourceToQueue[:6] != "/ipfs/" {
			var err error
			resourceToQueue, err = c.UrlCacheLookup(resourceToQueue, urgent)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "enqueue encountered an unexpected error: %v", err)
				return
			}
		}

		if !urgent {
			q.AddToQueue(resourceToQueue)
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "enqueue successfully enqueued audio at: %v", resourceToQueue)
	})
}

// TODO: This should be considered non-functional until we work out
// how to do this properly with the mixer
func playnext(q *queue.Queue, c *cache.Cache) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		resourceToQueue := r.URL.Query().Get("song")
		if resourceToQueue == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "/playnext expects a song resource identifier in the request.\n"+
				"eg api.example/enqueue?song1=https%3A%2F%2Fyoutu.be%2FnAwTw1aYy6M") // https://youtu.be/nAwTw1aYy6M
		}

		// TODO: This assumes we're only dealing with urls, doesn't check ipfs
		ipfsPath, err := c.UrlCacheLookup(resourceToQueue, false)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "playnext encountered an unexpected error: %v", err)
			return
		}
		q.PlayNext(ipfsPath)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "playnext successfully enqueued audio at: %v", ipfsPath)
	})
}

func skip(e *mixer.Mixer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		// Encoder is in charge of skipping, not the queue
		// Kinda weird, but it was the best way to reduce component interdependency
		e.Skip()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "song skipped successfully")
	})
}

func play(e *mixer.Mixer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		e.Play()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "song skipped successfully")
	})
}

func pause(e *mixer.Mixer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		e.Pause()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "song skipped successfully")
	})
}

func logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.Println(requestID, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func tracing(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = nextRequestID()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("X-Request-Id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
