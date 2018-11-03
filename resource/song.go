package resource

import (
	"fmt"
	"io"
	"net/url"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	"gopkg.in/djherbis/buffer.v1"
	"gopkg.in/djherbis/nio.v2"
)

var bufferSize int64 = 10000 //kb

type Song struct {
	ipfsPath  string
	url       *url.URL
	Title     string
	Duration  time.Duration
	DLResult  chan string
	DLFailure chan error
	reader    io.ReadCloser
	Writer    io.WriteCloser
}

func NewSong(resourceID string, hotwriter bool) (song *Song, err error) {
	song = &Song{
		ipfsPath:  "",
		url:       nil,
		Title:     "",
		Duration:  0,
		DLResult:  make(chan string, 1),
		DLFailure: make(chan error, 1),
	}

	if IsIpfs(resourceID) {
		song.ipfsPath = resourceID
	}

	song.url, err = url.Parse(resourceID)
	if err != nil {
		return nil, fmt.Errorf("ResourceID can't be used to make song. Err: %s", err)
	}

	if hotwriter {
		buf := buffer.New(bufferSize * 1024) // In memory Buffer
		song.reader, song.Writer = nio.Pipe(buf)
	}

	return song, nil
}

func (s *Song) ResourceID() (resourceID string, isCached bool) {
	// If we have the IPFS path fetch it right away
	if s.ipfsPath != "" {
		return s.ipfsPath, true
	}

	// Check to see if a download we were wairing on finished, if so
	// return the IPFS path, otherwise just return the URL
	select {
	case resourceID = <-s.DLResult:
		s.ipfsPath = resourceID
		return resourceID, true
	default:
		return s.url.String(), false
	}
}

func (s *Song) Url() *url.URL {
	return s.url
}

func (s *Song) Resolve(ipfs *shell.Shell) (reader io.ReadCloser, err error) {
	// Check to see if we had a DL we were waiting on, if so store the result
	select {
	case err = <-s.DLFailure:
		return nil, err
	case s.ipfsPath = <-s.DLResult:
	default:
		// default case so we move on if we don't have anything to recieve
	}

	// If we have a reader from the DL, that's the priority, otherwise return the
	// ipfs reader if we can
	if s.reader != nil {
		return s.reader, nil
	} else if s.ipfsPath != "" {
		return ipfs.Cat(s.ipfsPath)
	}

	// We need to play the song but aren't ready, block until the DL is finished
	select {
	case s.ipfsPath = <-s.DLResult:
		return ipfs.Cat(s.ipfsPath)
	case err = <-s.DLFailure:
		return nil, err
	}
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
