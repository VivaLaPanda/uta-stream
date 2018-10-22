package cache

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
var bufferSize int64 = 2000 //kb

type placeholder struct {
	reader   io.Reader
	ipfsPath string
	done     chan bool
}

func (c *Cache) AddPlaceholder(url string) (newPlaceholder *placeholder, hotWriter io.WriteCloser) {
	newPlaceholder = &placeholder{nil, "", make(chan bool, 1)}
	// If we don't have enough buffer, prepare the placeholder for passing data
	// directly to the mixer
	if len(c.Placeholders) < numBuffered {
		buf := buffer.New(bufferSize * 1024) // 2000 KB In memory Buffer
		newPlaceholder.reader, hotWriter = nio.Pipe(buf)
	}
	c.Placeholders[url] = newPlaceholder

	return newPlaceholder, hotWriter
}

// HardResolve will take the url and check it against the Placeholders
// it ensures that you will always get a reader, blocking if necessary
func (c *Cache) HardResolve(resourceID string) (ipfsPath string, hotReader io.Reader, err error) {
	if len(resourceID) < 6 {
		return "", nil, fmt.Errorf("All resource should be at least 6 char. provided: %s", resourceID)
	}
	if resourceID[:6] == "/ipfs/" {
		r, err := c.FetchIpfs(resourceID)
		return resourceID, r, err
	}

	// If we're resolving something it should no longer be held as a placeholder
	pHolder, exists := c.Placeholders[resourceID]
	if !exists {
		return "", nil, fmt.Errorf("Queue contained a resource that was never fetched (%s). Cannot resolve!\n", resourceID)
	}
	defer delete(c.Placeholders, resourceID)

	// If we don't have a reader and we're being asked to resolve
	// we just have to block until we're done with the DL/Conversion
	if pHolder.reader == nil {
		if pHolder.ipfsPath == "" {
			// Block until the placeholder is done processing
			<-pHolder.done
		}
		r, err := c.FetchIpfs(pHolder.ipfsPath)
		return pHolder.ipfsPath, r, err
	}

	return "", pHolder.reader, nil
}

func (c *Cache) SoftResolve(url string) (ipfsPath string, err error) { // If we're resolving something it should no longer be held as a placeholder
	pHolder, exists := c.Placeholders[url]
	if !exists {
		return "", fmt.Errorf("Queue contained a resource that was never fetched (%s). Cannot resolve!\n", url)
	}

	if pHolder.ipfsPath == "" {
		return "", nil
	}
	defer delete(c.Placeholders, url)

	return pHolder.ipfsPath, nil
}
