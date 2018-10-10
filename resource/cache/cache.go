package cache

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
)

type Cache struct {
	urlMap *map[string]string
	ipfs   *shell.Shell
}

// How many minutes to wait between saves of the cache state
// TODO: make ipfs url a parameter
var autosaveTimer time.Duration = 10
var ipfsUrl = "localhost:5001"

// Function which will provide a new cache struct
// An cache must be provided a file that it can read/write it's data to
// so that the cache is preserved between launches
func NewCache(cacheFile string) *Cache {
	urlMap := make(map[string]string)
	c := &Cache{&urlMap, shell.NewShell(ipfsUrl)}

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
	encoder.Encode(c.urlMap)

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
		err = decoder.Decode(c.urlMap)
	}
	if err != nil {
		return err
	}

	return nil
}

func (c *Cache) UrlCacheLookup(url string) (ipfsPath string) {
	ipfsPath, exists := (*c.urlMap)[url]
	if !exists {
		// TODO: Go to downloaders and get the resource, add to ipfs, get the hash
		ipfsPath = "/ipfs/QmeX7q8umBijLRQJT28XteuBTEtxUYZgSruZF3H3N5EPv7" // test file until this is implemented
		(*c.urlMap)[url] = ipfsPath
	}

	return ipfsPath
}

func (c *Cache) FetchUrl(url string) (ipfsPath string, r io.ReadCloser, err error) {
	ipfsPath = c.UrlCacheLookup(url)
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
