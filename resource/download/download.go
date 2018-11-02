// Package download provides functions that given a url to certain sites
// can fetch mp3 from them.
package download

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/VivaLaPanda/uta-stream/resource/placeholders"
	shell "github.com/ipfs/go-ipfs-api"
	"github.com/rylio/ytdl"
)

var knownProviders = [...]string{"youtube.com", "youtu.be"}
var tempDLFolder = "TEMP-DL"

// Master download router. Looks at the url and determins which service needs
// to hand the url. hotWriter is used to allow for playing the audio
// without waiting for the DL to finish. If you pass a writer the data will be
// pushed into that reader at the same time it's written to disk. I recommend
// a buffered reader, as I'm using TeeReader which works best with buffers
func Download(request Song, pHolders *placeholders.List, ipfs *shell.Shell) (result Song, err error) {
	// Ensure the temporary directory for storing downloads exists
	if _, err = os.Stat(tempDLFolder); os.IsNotExist(err) {
		os.Mkdir(tempDLFolder, os.ModePerm)
	}

	// Route to different handlers based on hostname
	switch request.url.Hostname() {
	case "youtu.be":
		return downloadYoutube(request, pHolders, ipfs)
	default:
		return "", fmt.Errorf("URL hostname (%v) doesn't match a known provider.\n"+
			"Should be one of: %v\n", request.url.Hostname(), knownProviders)
	}
}

func downloadYoutube(request Song, pHolders *placeholders.List, ipfs *shell.Shell) (result Song, err error) {
	// Get the info for the video
	var vidInfo *ytdl.VideoInfo
	vidInfo, err = ytdl.GetVideoInfoFromShortURL(Song.url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch provided Youtube url. Err: %v", err)
	}

	// Figure out the highest bitrate format
	formats := vidInfo.Formats
	bestFormat := formats.Best(ytdl.FormatAudioBitrateKey)[0] // Format with highest bitrate

	// Add metadata to Song
	request.Title = vidInfo.Title
	request.Duration = vidInfo.Duration

	// NOTE: The following gets a little confusing because of the io piping
	// the mp3 is being downloaded into a writer. That writer was provided by
	// the splitAudio function which will convert the audio into an mp3 and expose
	// it via the reader at convOutput

	// Prepare the audio extraction pipeline
	// Prepare the converter
	convInput, convOutput, convProgress, err := splitAudio()
	if err != nil {
		return nil, err
	}

	// Prepare the mp3 file we'll write to
	fileLocation := filepath.Join(tempDLFolder, vidInfo.ID+".mp3")
	_ = os.MkdirAll(filepath.Dir(fileLocation), os.ModePerm)
	mp3File, err := os.Create(fileLocation)
	if err != nil {
		return nil, fmt.Errorf("failed to create mp3 file. Err: %v", err)
	}

	// Create placeholder
	newPlaceholder, hotWriter := pHolders.AddPlaceholder(request.url.String())

	// Download the mp4 into the converter
	dlError := make(chan error)
	go func() {
		log.Printf("Downloading mp4 from %v\n", request.url.String())
		err = vidInfo.Download(bestFormat, convInput)
		defer convInput.Close()
		if err != nil {
			dlError <- fmt.Errorf("ytdl encountered an error: %v\n", err)
			return
		}
		log.Printf("Downloading of %v complete\n", request.url.String())
		dlError <- nil
	}()

	// Read from converter and write to the file and potentially the provided hotWriter
	go func() {
		log.Printf("Converting %s mp4 to mp3\n", request.url.String())
		var sharedReader io.Reader
		if hotWriter != nil {
			bufStreamData := bufio.NewWriter(hotWriter)
			sharedReader = io.TeeReader(convOutput, bufStreamData)
		} else {
			sharedReader = convOutput
		}

		io.Copy(mp3File, sharedReader)
		log.Printf("Conversion of %s to mp3 complete\n", request.url.String())
		convOutput.Close()
		if hotWriter != nil {
			bufStreamData.Flush()
			hotWriter.Close()
		}
		mp3File.Close()
	}()

	// Place into IPFS and resolve the placeholder
	go func() {
		// Wait until everything is done
		convProgress.Wait()
		err := <-dlError

		if err {
			pHolders.RemovePlaceholder(request.url.String())
			return "", fmt.Errorf("failed to download %s. Err: %v", request.url.String(), err)
		}

		// Add to ipfs
		ipfsPath, err = addToIpfs(fileLocation, ipfs)
		if err != nil {
			pHolders.RemovePlaceholder(request.url.String())
			return "", err
		}

		// Put result into the placeholder
		newPlaceholder.ipfsPath <- ipfsPath

		// Remove the mp3 now that we've added
		if err = os.Remove(fileLocation); err != nil {
			log.Printf("Failed to remove mp3 for %s. Err: %v\n", request.url.String(), err)
		}
	}()

	return request, nil
}

func IsIpfs(resourceID string) bool {
	if len(resourceID) >= 6 && resourceID[:6] == "/ipfs/" {
		return true
	}
}

// Fetch IPFS will get the provided IPFS resource and return the reader of its
// data
func FetchIpfs(ipfsPath string, ipsf *shell.Shell) (r io.ReadCloser, err error) {
	go func() {
		err := c.ipfs.Pin(ipfsPath) // Any time we fetch we also pin. This goes away eventually
		if err != nil {
			log.Printf("Failed to pin IPFS path! %v may not play later\n", err)
		}
	}()

	return c.ipfs.Cat(ipfsPath)
}

// Add the file at the provided location to ipfs and return its IPFS
// path
func addToIpfs(fileLocation string, ipfs *shell.Shell) (ipfsPath string, err error) {
	mp3File, err := os.Open(fileLocation)
	if err != nil {
		return "", fmt.Errorf("Failed to open downloaded mp3. Err: %v\n", err)
	}
	ipfsPath, err = ipfs.Add(mp3File)
	if err != nil {
		return "", fmt.Errorf("Failed to add to IPFS. Err: %v\n", err)
	}
	if err = mp3File.Close(); err != nil {
		return "", fmt.Errorf("failed to close mp3 after ipfs write. Err: %v", err)
	}

	// Formatting as proper ipfs path, not just hash
	ipfsPath = "/ipfs/" + ipfsPath

	return ipfsPath, err
}
