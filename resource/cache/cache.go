// Package cache contains components used to wrap Song download logic so that
// tracks need only be downloaded/converted once.
package cache

import (
	"encoding/gob"
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"

	"github.com/VivaLaPanda/uta-stream/resource/download"
	"github.com/VivaLaPanda/uta-stream/resource/metadata"
	shell "github.com/ipfs/go-ipfs-api"
)

// Cache is a struct which tracks the necessary state to translate
// resourceIDs into resolveable ipfs hashes or readers
type Cache struct {
	songMap       *map[string]Song
	ipfs          *shell.Shell
	cacheFilename string
}

type Song struct {
	ipfsPath string
	url      url.URL
	Title    string
	Duration string
}

func (s Song) ResourceID() (resourceID string, isCached bool) {
	if ipfsPath != "" {
		return ipfsPath, true
	} else if url != "" {
		return url, false
	} else {
		log.Panicf("Song has no associated resource ID. Song: %v\n", resourceID)
	}
}

// Used to limit how many ongoing downloads we have. useful to make sure
// Youtube doesn't get mad at us
var maxActiveDownloads = 3

// Function which will provide a new cache struct
// An cache must be provided a file that it can read/write it's data to
// so that the cache is preserved between launches. The ipfsurl will determine
// the daemon used to store/fetch ipfs resources. Allows for decoupling the storage
// engine from the cache.
func NewCache(cacheFile string, metadata *metadata.Cache, ipfsUrl string) *Cache {
	songMap := make(map[string]string)
	placeholders := make(map[string]*placeholder)
	c := &Cache{&songMap,
		&sync.RWMutex{},
		shell.NewShell(ipfsUrl),
		cacheFile,
		metadata,
		placeholders,
		make(chan bool, maxActiveDownloads)}

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

	return c
}

// Method which will write the cache data to the provided file. Will overwrite
// a file if one already exists at that location.
func (c *Cache) Write(filename string) error {
	cacheFile, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0660)
	defer cacheFile.Close()
	if err != nil {
		return err
	}
	encoder := gob.NewEncoder(cacheFile)
	encoder.Encode(c.songMap)

	return nil
}

// Method which will load the provided cache data file. Will overwrite the internal
// state of the object. Should pretty much only be used when the object is created
// but it is left public in case a client needs to load old data or something
func (c *Cache) Load(filename string) error {
	file, err := os.OpenFile(filename, os.O_RDONLY)
	defer file.Close()
	if err == nil {
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(c.songMap)
	}
	if err != nil {
		return err
	}

	return nil
}

// UrlCacheLookup will check the cache for the provided url, but on a cache miss
// it will download the resource and add it to the cache, then return the hash
func (c *Cache) Lookup(resourceID string) (result Song, err error) {
	if !isIpfs(resourceID) {
		// normalize
		url, err := urlNormalize(resourceID)
		if err != nil {
			return nil, fmt.Errorf("Provided resource doesn't appear to be a link: %v. \nErr: %v", url, err)
		}

		// Check the cache for the provided URL
		result, exists := (*c.songMap)[url]

		if !exists {

		}
	}

	return resourceID, nil
}

func (c *Cache) download(url string) (result Song) {
	// If it doesn't exist make a placeholder
	// If we only have two or less placeholders prepare to expose the
	// download/convert data early
	result = Song{
		ipfsPath: "",
		url:      url,
		Title:    "",
		Duration: "",
	}

	// Downloading could take a bit, do that on a new routine so we can return
	go func(url string, pHolder *placeholder, activeDownloads chan bool) {
		// Make sure we aren't past the download limit
		activeDownloads <- true
		// Go fetch the provided URL
		ipfsPath, err := download.Download(url, c.ipfs, c.metadata, hotWriter)
		if err != nil {
			log.Printf("failed to DL requested resource: %v\nerr:%v", url, err)
			delete(c.Placeholders, url)
			return
		}
		// Register that we're done with the DL
		<-activeDownloads

		// Create the URL => ipfs mapping in the cache
		c.urlMapLock.Lock()
		(*c.songMap)[url] = ipfsPath
		c.urlMapLock.Unlock()
		c.Write(c.cacheFilename)

		// Deal with placeholder
		pHolder.ipfsPath = ipfsPath
		pHolder.done <- true
	}(url, newPlaceholder, c.activeDownloads)

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
	} else { // Youtube url is short
		values := parsedUrl.Query()
		values.Del("list")
		parsedUrl.RawQuery = values.Encode()
	}

	return parsedUrl.String(), nil
}
