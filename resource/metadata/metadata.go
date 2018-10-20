package metadata

import (
	"encoding/gob"
	"fmt"
	"os"
	"sync"
)

type Cache struct {
	titleMap      *map[string]string
	titleMapLock  *sync.RWMutex
	cacheFilename string
}

// Function which will provide a new cache struct
// An cache must be provided a file that it can read/write it's data to
// so that the cache is preserved between launches
func NewCache(cacheFilename string) *Cache {
	titleMap := make(map[string]string)
	c := &Cache{&titleMap, &sync.RWMutex{}, cacheFilename}

	// Confirm we can interact with our persitent storage
	_, err := os.Stat(cacheFilename)
	if err == nil {
		err = c.Load(cacheFilename)
	} else if os.IsNotExist(err) {
		//log.Printf("Metadata cache %s doesn't exist. Creating new cache", cacheFilename)
		err = c.Write(cacheFilename)
	}

	if err != nil {
		errString := fmt.Sprintf("Fatal error when interacting with metadata cache on launch.\nErr: %v\n", err)
		panic(errString)
	}

	return c
}

// Method which will write the cache data to the provided file. Will overwrite
// a file if one already exists at that location.
func (c *Cache) Write(filename string) error {
	cacheFilename, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0660)
	defer cacheFilename.Close()
	if err != nil {
		return err
	}
	encoder := gob.NewEncoder(cacheFilename)
	c.titleMapLock.RLock()
	encoder.Encode(c.titleMap)
	c.titleMapLock.RUnlock()

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
		c.titleMapLock.Lock()
		err = decoder.Decode(c.titleMap)
		c.titleMapLock.Unlock()
	}
	if err != nil {
		return err
	}

	return nil
}

func (c *Cache) Lookup(resourceID string) (title string) {
	if len(resourceID) < 6 {
		return "Unknown Track"
	}
	if resourceID[:6] != "/ipfs/" {
		return resourceID
	}

	// Check the cache for the provided URL
	c.titleMapLock.RLock()
	title, exists := (*c.titleMap)[resourceID]
	c.titleMapLock.RUnlock()

	if !exists {
		title = "Unknown Track"
	}

	return title
}

func (c *Cache) Store(ipfsPath string, title string) (err error) {
	// Create the URL => ipfs mapping in the cache
	c.titleMapLock.Lock()
	(*c.titleMap)[ipfsPath] = title
	c.titleMapLock.Unlock()

	// Try to write the cache to disk, maximum 3 failures
	err = fmt.Errorf("temp") // make sure err isn't nil
	for count := 0; err != nil && count < 3; count += 1 {
		err = c.Write(c.cacheFilename)
	}

	return err
}
