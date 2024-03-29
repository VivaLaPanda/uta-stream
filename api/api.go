// Package api provides the exposed HTTP interface to modify the state of the
// UtaStream server.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/VivaLaPanda/uta-stream/mixer"
	"github.com/VivaLaPanda/uta-stream/queue"
	"github.com/VivaLaPanda/uta-stream/resource"
	"github.com/VivaLaPanda/uta-stream/resource/cache"
	"github.com/gorilla/mux"
)

// QFunc describes a function that takes a resource ID and attempts to add it to
// the queue in some way
type QFunc func(song *resource.Song)

type key int

const (
	requestIDKey key = 0
)

var (
	healthy int32
)

// ServeAPI is a function that will expose the interface through which one
// modifies the state of the server. Several components are passed in and then
// requests to the API translate into operations against those components
// This function call will block the caller until the server is killed
func ServeApi(m *mixer.Mixer, c *cache.Cache, q *queue.Queue, listenerCount func() int, port int, authCfgFilename string) {
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
	router.Use(headerMiddleware)
	router.Handle("/", index()).
		Methods("GET")
	router.Handle("/auth", authTest(amw)).
		Methods("GET")
	router.Handle("/enqueue", queuer(q, c, q.AddToQueue)).
		Methods("POST")
	router.Handle("/playnext", queuer(q, c, q.PlayNext)).
		Methods("POST")
	router.Handle("/skip", skip(m)).
		Methods("POST")
	router.Handle("/shuffle", shuffle(m, q)).
		Methods("POST")
	router.Handle("/playing", playing(m, q, listenerCount)).
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

// notFound is the function in charge of responding to 404s
func notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintln(w, "{\"error\":\"Endpoint not found. Doublecheck your query or take a look at the"+
		"docs: https://github.com/VivaLaPanda/uta-stream\"}")
}

// index is a utility function to provide guidance if you hit the root
// TODO: eventually this should list all routes
func index() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "{\"message\":\"This is the UtaStream client API."+
			"Documentation on routes is at https://github.com/VivaLaPanda/uta-stream\"}")
	})
}

// authCanary is used to validate whether you have access to a particular route
func authTest(amw *authMiddleware) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		route := "/api" + r.URL.Query().Get("route")

		if amw.ValidateToken(token, route) {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
	})
}

// queuer is a function which will handle requests to add a song unto the queue
// in some way (front of queue, back of queue, etc). Queues may result in immediate
// queueing of cached resource, or of a placeholder to be swapped once we are done with the DL
func queuer(q *queue.Queue, c *cache.Cache, qFunc QFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceToQueue := r.URL.Query().Get("song")
		if resourceToQueue == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "{\"error\":\"/enqueue and /playnext expect a song resource identifier in the request.\n"+
				"eg api.example/enqueue?song=https://youtu.be/N8nGig78lNs\"}") // https://youtu.be/nAwTw1aYy6M
			return
		}
		if len(resourceToQueue) < 6 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "{\"error\":\"url should be at least 6 characters.\"}")
			return
		}

		// If we're looking at an ipfs path just leave as is
		// Otherwise go and fetch it
		songToQueue, err := c.Lookup(resourceToQueue)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "{\"error\":\"failed to enqueue url.\"}")
			log.Printf("Failed to enqueue song, err: %v", err)
			return
		}

		if r.URL.Query().Get("title") != "" {
			songToQueue.Title = r.URL.Query().Get("title")
		}

		qFunc(songToQueue)

		w.WriteHeader(http.StatusOK)
		jsonData, _ := songToQueue.MarshalJSON()
		fmt.Fprintf(w, `{"message": "successfully added",
			               "track":%s}`, jsonData)
	})
}

// skip will skip the currently playing song. Expect some delay
func skip(e *mixer.Mixer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Encoder is in charge of skipping, not the queue
		// Kinda weird, but it was the best way to reduce component interdependency
		e.Skip()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "{\"message\":\"song skipped successfully\"}")
	})
}

// skip will skip the currently playing song. Expect some delay
func shuffle(e *mixer.Mixer, q *queue.Queue) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Go tell the queue to force a chainbreak/randomize
		q.Shuffle()
		// Encoder is in charge of skipping, not the queue
		// Kinda weird, but it was the best way to reduce component interdependency
		e.Skip()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "{\"message\":\"autoq shuffled successfully\"}")
	})
}

func playing(m *mixer.Mixer, q *queue.Queue, listenerCount func() int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Note that these string cats are less expensive than they look
		// Go's compiler optimzes them pretty well
		// source: https://syslog.ravelin.com/bytes-buffer-i-thought-you-were-my-friend-4148fd001229

		// Format playing
		queued := q.GetQueue()

		respStruct := struct {
			CurrentSong   *resource.Song   `json:"currentSong"`
			Upcoming      []*resource.Song `json:"upcoming"`
			Dj            string           `json:"dj"`
			ListenerCount int              `json:"listenerCount"`
		}{
			m.CurrentSongInfo,
			queued,
			"",
			listenerCount(),
		}

		respString, err := json.Marshal(respStruct)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "{\"error\":\"Failed to format response: %v\"}", err)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, string(respString))
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

				if r.URL.Path != "/api/playing" && r.URL.Path != "/api/auth" {
					logger.Println(requestID, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
				}
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

func headerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add headers to all responses
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		next.ServeHTTP(w, r)
	})
}
