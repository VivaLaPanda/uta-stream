package placeholders

import (
	"fmt"
	"io"

	"gopkg.in/djherbis/buffer.v1"
	"gopkg.in/djherbis/nio.v2"
)

// This number determines how many buffered readers to keep for unresolved
// songs. The higher the number the less chance we are forced to block
// when we want to play something, but higher numbers also increase memory usage
var numBuffered = 1
var bufferSize int64 = 10000 //kb

type List struct {
	Placeholders    map[string]*placeholder
	activeDownloads chan bool
}

type placeholder struct {
	reader   io.Reader
	ipfsPath chan string
}

func (l *List) AddPlaceholder(url string) (newPlaceholder *placeholder, hotWriter io.WriteCloser) {
	newPlaceholder = &placeholder{nil, make(chan string, 1)}
	// If we don't have enough buffer, prepare the placeholder for passing data
	// directly to the mixer
	if len(l.Placeholders) < numBuffered {
		buf := buffer.New(bufferSize * 1024) // 2000 KB In memory Buffer
		newPlaceholder.reader, hotWriter = nio.Pipe(buf)
	}
	l.Placeholders[url] = newPlaceholder

	return newPlaceholder, hotWriter
}

func (l *List) RemovePlaceholder(url string) {
	delete(l.Placeholders, url)
}

// HardResolve will take the url and check it against the Placeholders
// it ensures that you will always get a reader, blocking if necessary
func (l *List) HardResolve(resourceID string) (ipfsPath string, hotReader io.Reader, err error) {
	if isIpfs(resourceID) {
		return resourceID, nil, nil
	}

	// If we're resolving something it should no longer be held as a placeholder
	pHolder, exists := l.Placeholders[resourceID]
	if !exists {
		return "", nil, fmt.Errorf("Queue contained a resource that was never fetched (%s). Cannot resolve!\n", resourceID)
	}
	defer delete(l.Placeholders, resourceID)

	// If we don't have a reader and we're being asked to resolve
	// we just have to block until we're done with the DL/Conversion
	if pHolder.reader == nil {
		// Block until the placeholder is done processing
		return <-pHolder.ipfsPath, nil, nil
	}

	select {
	case ipfsPath = <-pHolder.ipfsPath:
	default:
		ipfsPath = ""
	}

	// We have a Reader, just go off that
	return ipfsPath, pHolder.reader, nil
}

func (l *List) SoftResolve(resourceID string) (ipfsPath string, err error) {
	if isIpfs(resourceID) {
		return resourceID, nil
	}
	pHolder, exists := l.Placeholders[resourceID]
	if !exists {
		return "", fmt.Errorf("Queue contained a resource that was never fetched (%s). Cannot resolve!\n", resourceID)
	}

	select {
	case ipfsPath = <-pHolder.ipfsPath:
		// If we're resolving something it should no longer be held as a placeholder
		defer delete(l.Placeholders, resourceID)
	default:
		ipfsPath = ""
	}

	return ipfsPath, nil
}

func isIpfs(resourceID string) bool {
	return len(resourceID) >= 6 && resourceID[:6] == "/ipfs/"
}
