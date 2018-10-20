package cache

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/VivaLaPanda/uta-stream/resource/download"
	"github.com/VivaLaPanda/uta-stream/resource/metadata"
	shell "github.com/ipfs/go-ipfs-api"
)

type Cache struct {
	urlMap        *map[string]string
	urlMapLock    *sync.RWMutex
	ipfs          *shell.Shell
	cacheFilename string
	metadata      *metadata.Cache
	Placeholders  map[string]placeholder
}

type placeholder struct {
	reader   io.Reader
	ipfsPath string
	done     chan bool
}

// How many minutes to wait between saves of the cache state
// This can be long because normal changes to the cache *should* save as well
// Autosave just helps in case of write failures
var autosaveTimer time.Duration = 30

// This number determines how many buffered readers to keep for unresolved
// songs. The higher the number the less chance we are forced to block
// when we want to play something, but higher numbers also increase memory usage
var numBuffered = 2

// Function which will provide a new cache struct
// An cache must be provided a file that it can read/write it's data to
// so that the cache is preserved between launches
func NewCache(cacheFile string, metadata *metadata.Cache, ipfsUrl string) *Cache {
	urlMap := make(map[string]string)
	placeholders := make(map[string]placeholder)
	c := &Cache{&urlMap,
		&sync.RWMutex{},
		shell.NewShell(ipfsUrl),
		cacheFile,
		metadata,
		placeholders}

	// Confirm we can interact with our persitent storage
	_, err := os.Stat(cacheFile)
	if err == nil {
		err = c.Load(cacheFile)
	} else if os.IsNotExist(err) {
		log.Printf("cacheFile %s doesn't exist. Creating new cacheFile", cacheFile)
		err = c.Write(cacheFile)
	}

	if err != nil {
		errString := fmt.Sprintf("Fatal error when interacting with cacheFile on launch.\nErr: %v\n", err)
		panic(errString)
	}

	// Write the cache to disk occasionally to preserve it between runs
	go func() {
		for {
			time.Sleep(autosaveTimer * time.Minute)
			err := c.Write(cacheFile)
			if err != nil {
				log.Printf("WARNING! Failed to write cacheFile. Data will not persist until"+
					"this is fixed. \n Err: %v\n", err)
			}
		}
	}()

	return c
}

// Method which will write the cache data to the provided file. Will overwrite
// a file if one already exists at that location.
func (c *Cache) Write(filename string) error {
	cacheFile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0660)
	defer cacheFile.Close()
	if err != nil {
		return err
	}
	encoder := gob.NewEncoder(cacheFile)
	c.urlMapLock.RLock()
	encoder.Encode(c.urlMap)
	c.urlMapLock.RUnlock()

	return nil
}

// Method which will load the provided cache data file. Will overwrite the internal
// state of the object. Should pretty much only be used when the object is created
// but it is left public in case a client needs to load old data or something
func (c *Cache) Load(filename string) error {
	file, err := os.Open(filename)
	defer file.Close()
	if err == nil {
		decoder := gob.NewDecoder(file)
		c.urlMapLock.Lock()
		err = decoder.Decode(c.urlMap)
		c.urlMapLock.Unlock()
	}
	if err != nil {
		return err
	}

	return nil
}

func (c *Cache) QuickLookup(url string) (ipfsPath string, exists bool) {
	// normalize
	url, err := urlNormalize(url)
	if err != nil {
		return "", false
	}
	// Check the cache for the provided URL
	c.urlMapLock.RLock()
	ipfsPath, exists = (*c.urlMap)[url]
	c.urlMapLock.RUnlock()

	return ipfsPath, exists
}

func (c *Cache) UrlCacheLookup(url string) (resourceID string, err error) {
	// normalize
	url, err = urlNormalize(url)
	if err != nil {
		return "", fmt.Errorf("Provided resource doesn't appear to be a link: %v. \nErr: %v", url, err)
	}

	// Check the cache for the provided URL
	c.urlMapLock.RLock()
	resourceID, exists := (*c.urlMap)[url]
	c.urlMapLock.RUnlock()

	if !exists {
		// If it doesn't exist make a placeholder
		// If we only have two or less placeholders prepare to expose the
		// download/convert data early
		newPlaceholder, hotWriter := c.AddPlaceholder(url)

		// Downloading could take a bit, do that on a new routine so we can return
		go func(url string, pHolder placeholder) {
			// Go fetch the provided URL
			ipfsPath, err := download.Download(url, c.ipfs, c.metadata, hotWriter)
			if err != nil {
				log.Printf("failed to DL requested resource: %v\nerr:%v", url, err)
			}

			// Create the URL => ipfs mapping in the cache
			c.urlMapLock.Lock()
			(*c.urlMap)[url] = ipfsPath
			c.urlMapLock.Unlock()
			c.Write(c.cacheFilename)

			// Deal with placeholder
			pHolder.ipfsPath = ipfsPath
			pHolder.done <- true
		}(url, newPlaceholder)
	}

	return resourceID, nil
}

func (c *Cache) FetchUrl(url string) (ipfsPath string, r io.ReadCloser, err error) {
	ipfsPath, err = c.UrlCacheLookup(url)
	if err != nil {
		return ipfsPath, nil, err
	}
	r, err = c.FetchIpfs(ipfsPath)
	return ipfsPath, r, err
}

func (c *Cache) FetchIpfs(ipfsPath string) (r io.ReadCloser, err error) {
	err = c.ipfs.Pin(ipfsPath) // Any time we fetch we also pin. This goes away eventually
	if err != nil {
		return nil, err
	}

	return c.ipfs.Cat(ipfsPath)
}

func (c *Cache) AddPlaceholder(url string) (newPlaceholder placeholder, hotWriter *bufio.Writer) {
	newPlaceholder = placeholder{nil, "", make(chan bool)}
	if len(c.Placeholders) < numBuffered+1 {
		pReader, pWriter := io.Pipe()
		hotWriter = bufio.NewWriter(pWriter)
		newPlaceholder.reader = pReader
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

// Try and normalize URLs to reduce duplication in resource cache
func urlNormalize(rawUrl string) (normalizedUrl string, err error) {
	parsedUrl, err := url.Parse(rawUrl)
	if err != nil {
		return "", err
	}

	// Handle Youtube URLs
	if parsedUrl.Hostname() == "youtube.com" || parsedUrl.Hostname() == "www.youtube.com" {
		vidID := parsedUrl.Query().Get("v")
		normalizedUrl = fmt.Sprintf("https://youtu.be/%s", vidID)
	}

	return parsedUrl.String(), nil
}
