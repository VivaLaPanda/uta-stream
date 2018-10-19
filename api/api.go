package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VivaLaPanda/uta-stream/mixer"
	"github.com/VivaLaPanda/uta-stream/queue"
	"github.com/VivaLaPanda/uta-stream/resource/cache"
	"github.com/VivaLaPanda/uta-stream/resource/metadata"
	"github.com/gorilla/mux"
)

type QFunc func(ipfsPath string)

type key int

const (
	requestIDKey key = 0
)

var (
	healthy int32
)

func ServeApi(m *mixer.Mixer, c *cache.Cache, q *queue.Queue, info *metadata.Cache, port int, authCfgFilename string) {
	logger := log.New(os.Stdout, "http: ", log.LstdFlags)
	logger.Println("Server is starting...")

	basePath := "/api"

	amw, err := NewAuthMiddleware(authCfgFilename, basePath)
	if err != nil {
		logger.Fatalf("Couldn't find/parse provided auth config file. Err: %v\n", err)
	}

	// Router setup
	baseRouter := mux.NewRouter()
	router := baseRouter.PathPrefix(basePath).Subrouter()
	router.Use(amw.Middleware)
	router.Handle("/", index()).
		Methods("GET")
	router.Handle("/enqueue", queuer(q, c, info, q.AddToQueue)).
		Methods("POST")
	router.Handle("/playnext", queuer(q, c, info, q.PlayNext)).
		Methods("POST")
	router.Handle("/skip", skip(m)).
		Methods("POST")
	router.Handle("/play", play(m)).
		Methods("PUT")
	router.Handle("/pause", pause(m)).
		Methods("PUT")
	router.Handle("/playing", playing(m, q, info)).
		Methods("GET")
	router.NotFoundHandler = http.HandlerFunc(notFound)

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

	logger.Println("Server is ready to handle requests at", listenAddr, "/api")
	atomic.StoreInt32(&healthy, 1)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}

	<-done
	logger.Println("Server stopped")
}

func notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintln(w, "Endpoint not found. Doublecheck your query or take a look at the"+
		"docs: https://github.com/VivaLaPanda/uta-stream")
}

func index() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "This is the UtaStream client API. Documentation on routes is at https://github.com/VivaLaPanda/uta-stream")
	})
}

func queuer(q *queue.Queue, c *cache.Cache, info *metadata.Cache, qFunc QFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		resourceToQueue := r.URL.Query().Get("song")
		if resourceToQueue == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "/enqueue and /playnext expect a song resource identifier in the request.\n"+
				"eg api.example/enqueue?song=https://youtu.be/N8nGig78lNs") // https://youtu.be/nAwTw1aYy6M
			return
		}
		if len(resourceID) < 6 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "url should be at least 6 characters.\n")
			return
		}

		// If we're looking at an ipfs path just leave as is
		// Otherwise go and fetch it
		if resourceToQueue[:6] != "/ipfs/" {
			resourceToQueue, err = c.UrlCacheLookup(resourceToQueue)
			if err != nil {
				log.Printf("Failed to enqueue song, err: %v", err)
				return
			}
			qFunc(resourceToQueue)
		}

		title := info.Lookup(resourceToQueue)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "successfully added \"%v\" to queue", title)
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

func playing(m *mixer.Mixer, q *queue.Queue, info *metadata.Cache) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		// Note that these string cats are less expensive than they look
		// Go's compiler optimzes them pretty well
		// source: https://syslog.ravelin.com/bytes-buffer-i-thought-you-were-my-friend-4148fd001229

		// Format current
		currentString := "Now Playing: " + info.Lookup(m.CurrentSongPath) + ""

		// Format playing
		queued := q.GetQueue()
		for idx, song := range queued {
			queued[idx] = fmt.Sprintf("%d: %s", idx+1, info.Lookup(song))
		}
		queuedString := strings.Join(queued, "\n")

		output := currentString + "\n\nQueued:\n" + queuedString

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, output)
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
