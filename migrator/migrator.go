package migrator

import (
	"encoding/gob"
	"encoding/json"
	"os"
	"time"
)

func Migrate(cacheFilename string, metadataFilename string) {
	cacheFile, _ := os.Open(cacheFilename)
	defer cacheFile.Close()
	metaFile, _ := os.Open(metadataFilename)
	defer metaFile.Close()
	outputFile, _ := os.OpenFile("migrated.cache.db", os.O_RDWR|os.O_CREATE, 0660)
	defer outputFile.Close()

	urlMap := make(map[string]string)

	decoder := gob.NewDecoder(cacheFile)
	_ = decoder.Decode(&urlMap)

	metaMap := make(map[string]string)

	decoder = gob.NewDecoder(metaFile)
	_ = decoder.Decode(&metaMap)

	type songStruct struct {
		IpfsPath string         `json:"ipfsPath"`
		Url      string         `json:"url"`
		Title    string         `json:"title"`
		Duration *time.Duration `json:"duration"`
	}

	endMap := make(map[string]songStruct)

	for key, value := range urlMap {
		song := songStruct{
			IpfsPath: value,
			Url:      key,
			Title:    metaMap[value],
			Duration: nil,
		}

		endMap[key] = song
	}

	encoder := json.NewEncoder(outputFile)
	encoder.Encode(endMap)

	return
}
