// Package cache contains components used to wrap resource.Song download logic so that
// tracks need only be downloaded/converted once.
package cache

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/VivaLaPanda/uta-stream/resource"
	"github.com/VivaLaPanda/uta-stream/resource/download"
	shell "github.com/ipfs/go-ipfs-api"
)

// Cache is a struct which tracks the necessary state to translate
// resourceIDs into resolveable ipfs hashes or readers
type Cache struct {
	songMap         *map[string]*resource.Song
	ipfs            *shell.Shell
	cacheFilename   string
	activeDownloads chan bool
}

// Used to limit how many ongoing downloads we have. useful to make sure
// Youtube doesn't get mad at us
var maxActiveDownloads = 3

// Function which will provide a new cache struct
// An cache must be provided a file that it can read/write it's data to
// so that the cache is preserved between launches. The ipfsurl will determine
// the daemon used to store/fetch ipfs resources. Allows for decoupling the storage
// engine from the cache.
func NewCache(cacheFilename string, ipfsUrl string) *Cache {
	songMap := make(map[string]*resource.Song)
	c := &Cache{
		songMap:         &songMap,
		ipfs:            shell.NewShell(ipfsUrl),
		cacheFilename:   cacheFilename,
		activeDownloads: make(chan bool, maxActiveDownloads),
	}

	// Confirm we can interact with our persitent storage
	_, err := os.Stat(cacheFilename)
	if err == nil {
		err = c.Load(cacheFilename)
	} else if os.IsNotExist(err) {
		log.Printf("cacheFilename %s doesn't exist. Creating new cacheFilename", cacheFilename)
		err = c.Write(cacheFilename)
	}

	if err != nil {
		errString := fmt.Sprintf("Fatal error when interacting with cacheFilename on launch.\nErr: %v\n", err)
		panic(errString)
	}

	return c
}

// Method which will write the cache data to the provided file. Will overwrite
// a file if one already exists at that location.
func (c *Cache) Write(filename string) error {
	cacheFilename, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0660)
	defer cacheFilename.Close()
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(cacheFilename)
	encoder.Encode(c.songMap)

	return nil
}

// Method which will load the provided cache data file. Will overwrite the internal
// state of the object. Should pretty much only be used when the object is created
// but it is left public in case a client needs to load old data or something
func (c *Cache) Load(filename string) error {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0660)
	defer file.Close()
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(c.songMap)

	return nil
}

// UrlCacheLookup will check the cache for the provided url, but on a cache miss
// it will download the resource and add it to the cache, then return the hash
func (c *Cache) Lookup(resourceID string, urgent bool, noDownload bool) (song *resource.Song, err error) {
	song, err = resource.NewSong(resourceID, urgent)
	if err != nil {
		return nil, err
	}

	if !resource.IsIpfs(resourceID) {
		// normalize
		url, err := urlNormalize(resourceID)
		if err != nil {
			return nil, fmt.Errorf("Provided resource doesn't appear to be a link: %v. \nErr: %v", url, err)
		}

		// Check the cache for the provided URL
		cachedSong, exists := (*c.songMap)[url]

		if !exists || cachedSong.IpfsPath() == "" || cachedSong.Title == "" {
			if !noDownload {
				song, err = download.Download(song, c.ipfs)
				if err == nil {
					(*c.songMap)[url] = song
					c.Write(c.cacheFilename)
				} else {
					return nil, err
				}
			}
		} else {
			song = cachedSong
		}
	} else {
		// TODO: This is potentially horribly slow. Find a better way.
		for _, value := range *c.songMap {
			if song.IpfsPath() == value.IpfsPath() {
				song = value
			}
		}
	}

	return song, nil
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
