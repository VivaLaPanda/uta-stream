package cache

import (
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
	Hotstream     io.Reader
	metadata      *metadata.Cache
}

// How many minutes to wait between saves of the cache state
// This can be long because normal changes to the cache *should* save as well
// Autosave just helps in case of write failures
var autosaveTimer time.Duration = 30

// Function which will provide a new cache struct
// An cache must be provided a file that it can read/write it's data to
// so that the cache is preserved between launches
func NewCache(cacheFile string, metadata *metadata.Cache, ipfsUrl string) *Cache {
	urlMap := make(map[string]string)
	c := &Cache{&urlMap,
		&sync.RWMutex{},
		shell.NewShell(ipfsUrl),
		cacheFile,
		nil,
		metadata}

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
	url, err = urlNormalize(url)
	if err != nil {
		return "", fmt.Errorf("Provided resource doesn't appear to be a link: %v. \nErr: %v", url, err)
	}
	// Check the cache for the provided URL
	c.urlMapLock.RLock()
	ipfsPath, exists = (*c.urlMap)[url]
	c.urlMapLock.RUnlock()

	return ipfsPath, exists
}

func (c *Cache) UrlCacheLookup(url string, urgent bool) (ipfsPath string, err error) {
	// normalize
	url, err = urlNormalize(url)
	if err != nil {
		return "", fmt.Errorf("Provided resource doesn't appear to be a link: %v. \nErr: %v", url, err)
	}

	// Check the cache for the provided URL
	c.urlMapLock.RLock()
	ipfsPath, exists := (*c.urlMap)[url]
	c.urlMapLock.RUnlock()

	if !exists {
		// We've been told the request is urgent, expose the audio data
		// as we work so consumers can get it before we are finished
		var hotstreamWriter io.Writer
		if urgent {
			c.Hotstream, hotstreamWriter = io.Pipe()
		} else {
			c.Hotstream = nil
		}

		// Go fetch the provided URL
		ipfsPath, err = download.Download(url, c.ipfs, c.metadata, hotstreamWriter)
		if err != nil {
			return "", fmt.Errorf(("failed to DL requested resource: %v\nerr:%v"), url, err)
		}

		// Create the URL => ipfs mapping in the cache
		c.urlMapLock.Lock()
		(*c.urlMap)[url] = ipfsPath
		c.urlMapLock.Unlock()
		c.Write(c.cacheFilename)
	}

	return ipfsPath, nil
}

func (c *Cache) FetchUrl(url string) (ipfsPath string, r io.ReadCloser, err error) {
	ipfsPath, err = c.UrlCacheLookup(url, false)
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
