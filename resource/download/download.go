package download

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/VivaLaPanda/uta-stream/resource/metadata"
	shell "github.com/ipfs/go-ipfs-api"
	"github.com/rylio/ytdl"
)

var knownProviders = [...]string{"youtube.com", "youtu.be"}
var tempDLFolder = "TEMP-DL"

// Master download router. Looks at the url and determins which service needs
// to hand the url. streamData is used to allow for playing the audio
// without waiting for the DL to finish. If you pass a writer the data will be
// pushed into that reader at the same time it's written to disk. I recommend
// a buffered reader, as I'm using TeeReader which works best with buffers
func Download(rawurl string, ipfs *shell.Shell, metadata *metadata.Cache, streamData io.WriteCloser) (ipfsPath string, err error) {
	// Ensure the temporary directory for storing downloads exists
	if _, err = os.Stat(tempDLFolder); os.IsNotExist(err) {
		os.Mkdir(tempDLFolder, os.ModePerm)
	}

	// Parse the URL
	urlToDL, err := url.Parse(rawurl)
	if err != nil {
		// TODO: Eventually do a text-search of youtube and just DL top result
		// https://github.com/VivaLaPanda/uta-stream/issues/1
		return "", err
	}

	// Route to different handlers based on hostname
	switch urlToDL.Hostname() {
	case "youtube.com", "youtu.be", "www.youtube.com":
		return downloadYoutube(*urlToDL, ipfs, metadata, streamData)
	default:
		return "", fmt.Errorf("URL hostname (%v) doesn't match a known provider.\n"+
			"Should be one of: %v\n", urlToDL.Hostname(), knownProviders)
	}
}

func downloadYoutube(urlToDL url.URL, ipfs *shell.Shell, metadata *metadata.Cache, streamData io.WriteCloser) (ipfsPath string, err error) {
	// Get the info for the video
	var vidInfo *ytdl.VideoInfo
	switch urlToDL.Hostname() {
	case "youtube.com", "www.youtube.com":
		vidInfo, err = ytdl.GetVideoInfoFromURL(&urlToDL)
	case "youtu.be":
		vidInfo, err = ytdl.GetVideoInfoFromShortURL(&urlToDL)
	default:
		// This should never run
		panic(fmt.Sprintf("Youtube download recieved impossible URL hostname: %v", urlToDL.Hostname()))
	}
	if err != nil {
		return "", fmt.Errorf("failed to fetch provided Youtube url. Err: %v", err)
	}

	// Figure out the highest bitrate format
	formats := vidInfo.Formats
	bestFormat := formats.Best(ytdl.FormatAudioBitrateKey)[0] // Format with highest bitrate

	// NOTE: The following gets a little confusing because of the io piping
	// the mp3 is being downloaded into a writer. That writer was provided by
	// the splitAudio function which will convert the audio into an mp3 and expose
	// it via the reader at convOutput

	// Prepare the audio extraction pipeline
	convInput, convOutput, convProgress, err := splitAudio()
	if err != nil {
		return "", err
	}

	// Download the mp4
	var dlDone sync.WaitGroup
	dlDone.Add(1)
	go func() {
		log.Printf("Downloading mp4 from %v\n", urlToDL.EscapedPath())
		err = vidInfo.Download(bestFormat, convInput)
		if err != nil {
			log.Printf("ytdl encountered an error: %v\n", err)
		}
		log.Printf("Downloading of %v complete\n", urlToDL.EscapedPath())
		convInput.Close()
		dlDone.Done()
	}()

	fileTitle := vidInfo.Title

	// Write to file and potentially the provided streamData
	fileLocation := filepath.Join(tempDLFolder, vidInfo.ID+".mp3")
	_ = os.MkdirAll(filepath.Dir(fileLocation), os.ModePerm)
	mp3File, err := os.Create(fileLocation)
	if err != nil {
		return "", fmt.Errorf("failed to create mp3 file. Err: %v", err)
	}
	go func() {
		log.Printf("Converting mp4 to mp3\n")
		var sharedReader io.Reader
		bufStreamData := bufio.NewWriter(streamData)
		if streamData != nil {
			sharedReader = io.TeeReader(convOutput, bufStreamData)
		} else {
			sharedReader = convOutput
		}

		io.Copy(mp3File, sharedReader)
		log.Printf("Conversion to mp3 complete\n")
		convOutput.Close()
		if streamData != nil {
			bufStreamData.Flush()
			streamData.Close()
		}
		mp3File.Close()
	}()

	// Wait until everything is done
	dlDone.Wait()
	convProgress.Wait()

	// Add to ipfs
	ipfsPath, err = addToIpfs(fileLocation, ipfs)
	if err != nil {
		return "", err
	}

	metadata.Store(ipfsPath, fileTitle)

	// Remove the mp3 now that we've added
	if err = os.Remove(fileLocation); err != nil {
		log.Printf("Failed to remove mp3. Err: %v\n", err)
	}

	return ipfsPath, nil
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
