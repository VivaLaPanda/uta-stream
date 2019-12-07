package resource

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	"gopkg.in/djherbis/buffer.v1"
	"gopkg.in/djherbis/nio.v2"
)

var bufferSize int64 = 10000 //kb

type Song struct {
	ipfsPath      string `json:"currentSong"`
	url           *url.URL
	Title         string
	Duration      time.Duration
	DLResult      chan string
	DLFailure     chan error
	reader        io.ReadCloser
	Writer        io.WriteCloser
	resolved      *sync.WaitGroup
	resolutionErr error
}

func NewSong(resourceID string, hotwriter bool) (song *Song, err error) {
	song = &Song{
		ipfsPath:  "",
		url:       nil,
		Title:     "",
		Duration:  0,
		DLResult:  make(chan string, 1),
		DLFailure: make(chan error, 1),
		resolved:  &sync.WaitGroup{},
	}

	if IsIpfs(resourceID) {
		song.ipfsPath = resourceID
	} else {
		song.url, err = url.Parse(resourceID)
		if err != nil {
			return nil, fmt.Errorf("ResourceID can't be used to make song. Err: %s", err)
		}
	}

	if hotwriter {
		buf := buffer.New(bufferSize * 1024) // In memory Buffer
		song.reader, song.Writer = nio.Pipe(buf)
	}

	// Start making sure the song is playable
	song.resolved.Add(1)
	go song.resolver()

	return song, nil
}

func (s *Song) MarshalJSON() ([]byte, error) {
	var rawURL string
	if s.URL() != nil {
		rawURL = s.URL().String()
	} else {
		rawURL = ""
	}

	// Sane defaults
	if rawURL == "" {
		rawURL = "https://ipfs.io" + s.IpfsPath()
	}
	if s.Title == "" {
		s.Title = "Unknown Track"
	}

	return json.Marshal(&struct {
		IpfsPath string        `json:"ipfsPath"`
		URL      string        `json:"url"`
		Title    string        `json:"title"`
		Duration time.Duration `json:"duration"`
	}{
		IpfsPath: s.IpfsPath(),
		URL:      rawURL,
		Title:    s.Title,
		Duration: s.Duration,
	})
}

func (s *Song) UnmarshalJSON(data []byte) error {
	aux := &struct {
		IpfsPath string        `json:"ipfsPath"`
		URL      string        `json:"url"`
		Title    string        `json:"title"`
		Duration time.Duration `json:"duration"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Sane defaults
	if aux.URL == "" {
		aux.URL = "https://ipfs.io" + aux.IpfsPath
	}
	if aux.Title == "" {
		aux.Title = "Unknown Track"
	}

	// Construct the song
	s.ipfsPath = aux.IpfsPath
	s.Title = aux.Title
	s.Duration = aux.Duration
	var err error
	if s.url, err = url.Parse(aux.URL); err != nil {
		s.url = nil
	}
	if s.resolved == nil {
		s.resolved = &sync.WaitGroup{}
	}

	return nil
}

func (s *Song) ResourceID() (resourceID string) {
	// If we have the IPFS path fetch it right away
	if s.ipfsPath != "" {
		return s.ipfsPath
	}

	// Check to see if a download we were wairing on finished, if so
	// return the IPFS path, otherwise just return the URL
	select {
	case resourceID = <-s.DLResult:
		s.ipfsPath = resourceID
		return resourceID
	default:
		return s.url.String()
	}
}

func (s *Song) IpfsPath() string {
	return s.ipfsPath
}

func (s *Song) URL() *url.URL {
	return s.url
}

func (s *Song) resolver() {
	defer s.resolved.Done()

	if s.reader != nil {
		return
	} else if s.ipfsPath != "" {
		return
	} else if s.DLResult == nil {
		s.resolutionErr = fmt.Errorf("song was cached without download hash")
		return
	}

	// Check to see if we had a DL we were waiting on, if so store the result
	select {
	case s.resolutionErr = <-s.DLFailure:
		return
	case s.ipfsPath = <-s.DLResult:
		return
	}
}

// Resolve works sort of like a js Observable, in that n callers will wait
// until the song is resolved, and then all get the same data.
func (s *Song) Resolve(ipfs *shell.Shell) (reader io.ReadCloser, err error) {
	s.resolved.Wait()

	// If we have a reader from the DL, that's the priority, otherwise return the
	// ipfs reader if we can
	if s.resolutionErr != nil {
		return nil, s.resolutionErr
	} else if s.reader != nil {
		return s.reader, nil
	} else if s.ipfsPath != "" {
		reader, err = ipfs.Cat(s.ipfsPath)

		// Sometimes ipfs just stops responding under heavy load
		// Wait 5 sec and retry
		if err != nil {
			time.Sleep(5 * time.Second)
			return ipfs.Cat(s.ipfsPath)
		}
		return reader, err
	}

	return nil, fmt.Errorf("Song in an unknown state: %v", s)
}

func (s *Song) CheckFailure() (err error) {
	select {
	case err = <-s.DLFailure:
		return err
	default:
		return nil
	}
}

func IsIpfs(resourceID string) bool {
	return len(resourceID) >= 6 && resourceID[:6] == "/ipfs/"
}
