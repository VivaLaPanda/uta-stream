// Package download provides functions that given a url to certain sites
// can fetch mp3 from them.
package download

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/VivaLaPanda/uta-stream/resource"
	shell "github.com/ipfs/go-ipfs-api"
	ytdl "github.com/kkdai/youtube/v2"
)

var knownProviders = [...]string{"youtube.com", "youtu.be"}
var tempDLFolder = "TEMP-DL"
var maxYTDownloaders = make(chan int, 3)

// Master download router. Looks at the url and determins which service needs
// to hand the url. hotWriter is used to allow for playing the audio
// without waiting for the DL to finish. If you pass a writer the data will be
// pushed into that reader at the same time it's written to disk. I recommend
// a buffered reader, as I'm using TeeReader which works best with buffers
func Download(song *resource.Song, ipfs *shell.Shell) (*resource.Song, error) {
	// Ensure the temporary directory for storing downloads exists
	if _, err := os.Stat(tempDLFolder); os.IsNotExist(err) {
		os.Mkdir(tempDLFolder, os.ModePerm)
	}

	// Route to different handlers based on hostname
	switch song.URL().Hostname() {
	case "youtu.be":
		return downloadYoutube(song, ipfs)
	default:
		// Get the ext
		ext := path.Ext(song.URL().Path)

		if ext == ".mp3" {
			return downloadMp3(song, ipfs)
		}

		return song, fmt.Errorf("URL hostname (%v) doesn't match a known provider.\n"+
			"Should be one of: %v\n", song.URL().Hostname(), knownProviders)
	}
}

func downloadMp3(song *resource.Song, ipfs *shell.Shell) (*resource.Song, error) {
	// Get the filename from the web
	webPath := song.URL().Path
	filename := path.Base(webPath)

	if song.Title == "" {
		song.Title = filename
	}

	// Prepare the piping
	output, input := io.Pipe()

	// Prepare the mp3 file we'll write to
	fileLocation := filepath.Join(tempDLFolder, filename)
	_ = os.MkdirAll(filepath.Dir(fileLocation), os.ModePerm)

	// if the file already exists it's been queued already and is being downloaded
	// by another thread. Don't error, but don't start the DL, resource resolution
	// will handle the dupes
	_, err := os.Stat(fileLocation)
	var mp3File *os.File
	if os.IsNotExist(err) {
		mp3File, err = os.Create(fileLocation)
		if err != nil {
			return song, fmt.Errorf("failed to create mp3 file. Err: %v", err)
		}
	} else {
		return song, nil
	}

	// Download the mp3 to the buffer
	dlError := make(chan error)
	go func() {
		log.Printf("Starting download of mp3 from %v\n", song.URL().String())
		// Get the data
		resp, err := http.Get(song.URL().String())
		if err != nil {
			dlError <- fmt.Errorf("MP3 DL encountered an error: %v\n", err)
			return
		}
		defer resp.Body.Close()

		io.Copy(input, resp.Body)
		defer input.Close()

		log.Printf("Downloading of %v complete\n", song.URL().String())
		dlError <- nil
	}()

	// Read from converter and write to the file and potentially the provided hotWriter
	go func() {
		var sharedReader io.Reader
		bufStreamData := bufio.NewWriter(song.Writer)
		if song.Writer != nil {
			sharedReader = io.TeeReader(output, bufStreamData)
		} else {
			sharedReader = output
		}

		io.Copy(mp3File, sharedReader)
		output.Close()
		if song.Writer != nil {
			bufStreamData.Flush()
			song.Writer.Close()
		}
		mp3File.Close()
	}()

	// Place into IPFS and resolve the placeholder
	go func() {
		// BLock until DL finishes, nil for success, else will be an error
		err := <-dlError
		if err != nil {
			song.DLFailure <- fmt.Errorf("failed to download %s. Err: %v\n", song.URL().String(), err)
			return
		}

		// Add to ipfs
		ipfsPath, err := addToIpfs(fileLocation, ipfs)
		if err != nil {
			song.DLFailure <- fmt.Errorf("failed to add %s to IPFS. Err: %v\n", song.URL().String(), err)
			return
		}
		song.DLResult <- ipfsPath

		// Remove the mp3 now that we've added
		if err = os.Remove(fileLocation); err != nil {
			log.Printf("Failed to remove mp3 for %s. Err: %v\n", song.URL().String(), err)
		}
	}()

	return song, nil
}

func bestAudio(formats []ytdl.Format) (best *ytdl.Format) {
	best = &ytdl.Format{AudioSampleRate: ""}

	for idx, format := range formats {
		if format.AudioSampleRate > best.AudioSampleRate && format.AudioChannels >= best.AudioChannels {
			// Adding this code to make sure the formats are actually downloadable
			resp, err := http.Get(format.URL)
			if err != nil {
				continue
			}
			if resp.StatusCode == http.StatusOK {
				best = &formats[idx]
			}
		}
	}

	return
}

func downloadYoutube(song *resource.Song, ipfs *shell.Shell) (*resource.Song, error) {
	// Setup Client
	dlClient := ytdl.Client{}

	// Get the info for the video
	vidInfo, err := dlClient.GetVideo(song.URL().String())
	if err != nil {
		return song, fmt.Errorf("failed to fetch provided Youtube url. Err: %v", err)
	}

	// Figure out the highest bitrate format
	bestFormat := bestAudio(vidInfo.Formats)

	// Add metadata to resource.Song
	song.Title = vidInfo.Title
	song.Duration = vidInfo.Duration

	// NOTE: The following gets a little confusing because of the io piping
	// the mp3 is being downloaded into a writer. That writer was provided by
	// the splitAudio function which will convert the audio into an mp3 and expose
	// it via the reader at convOutput

	// Prepare the audio extraction pipeline
	// Prepare the converter
	convInput, convOutput, convProgress, err := splitAudio()
	if err != nil {
		return song, err
	}

	// Prepare the mp3 file we'll write to
	fileLocation := filepath.Join(tempDLFolder, vidInfo.ID+".mp3")
	_ = os.MkdirAll(filepath.Dir(fileLocation), os.ModePerm)

	// if the file already exists it's been queued already and is being downloaded
	// by another thread. Don't error, but don't start the DL, resource resolution
	// will handle the dupes
	_, err = os.Stat(fileLocation)
	var mp3File *os.File
	if os.IsNotExist(err) {
		mp3File, err = os.Create(fileLocation)
		if err != nil {
			return song, fmt.Errorf("failed to create mp3 file. Err: %v", err)
		}
	} else {
		return song, nil
	}

	// Download the mp4 into the converter
	dlError := make(chan error)
	go func() {
		log.Printf("Queuing download of mp4 from %v\n", song.URL().String())
		maxYTDownloaders <- 0
		log.Printf("Starting download of mp4 from %v\n", song.URL().String())
		rawRespStream, err := dlClient.GetStream(vidInfo, bestFormat)
		if err != nil {
			dlError <- fmt.Errorf("ytdl encountered an error: %v\n", err)
			return
		}
		defer rawRespStream.Body.Close()

		io.Copy(convInput, rawRespStream.Body)
		defer convInput.Close()

		log.Printf("Downloading of %v complete\n", song.URL().String())
		<-maxYTDownloaders
		dlError <- nil
	}()

	// Read from converter and write to the file and potentially the provided hotWriter
	go func() {
		log.Printf("Converting %s mp4 to mp3\n", song.URL().String())
		var sharedReader io.Reader
		bufStreamData := bufio.NewWriter(song.Writer)
		if song.Writer != nil {
			sharedReader = io.TeeReader(convOutput, bufStreamData)
		} else {
			sharedReader = convOutput
		}

		io.Copy(mp3File, sharedReader)
		log.Printf("Conversion of %s to mp3 complete\n", song.URL().String())
		convOutput.Close()
		if song.Writer != nil {
			bufStreamData.Flush()
			song.Writer.Close()
		}
		mp3File.Close()
	}()

	// Place into IPFS and resolve the placeholder
	go func() {
		// BLock until DL finishes, nil for success, else will be an error
		err := <-dlError
		if err != nil {
			song.DLFailure <- fmt.Errorf("failed to download %s. Err: %v\n", song.URL().String(), err)
			return
		}

		// Wait for convert to finish
		convProgress.Wait()

		// Add to ipfs
		ipfsPath, err := addToIpfs(fileLocation, ipfs)
		if err != nil {
			song.DLFailure <- fmt.Errorf("failed to add %s to IPFS. Err: %v\n", song.URL().String(), err)
			return
		}
		song.DLResult <- ipfsPath

		// Remove the mp3 now that we've added
		if err = os.Remove(fileLocation); err != nil {
			log.Printf("Failed to remove mp3 for %s. Err: %v\n", song.URL().String(), err)
		}
	}()

	return song, nil
}

// Fetch IPFS will get the provided IPFS resource and return the reader of its
// data
func FetchIpfs(ipfsPath string, ipfs *shell.Shell) (r io.ReadCloser, err error) {
	go func() {
		err := ipfs.Pin(ipfsPath) // Any time we fetch we also pin. This goes away eventually
		if err != nil {
			log.Printf("Failed to pin IPFS path! %v may not play later\n", err)
		}
	}()

	return ipfs.Cat(ipfsPath)
}

// Add the file at the provided location to ipfs and return its IPFS
// path
func addToIpfs(fileLocation string, ipfs *shell.Shell) (ipfsPath string, err error) {
	mp3File, err := os.Open(fileLocation)
	if err != nil {
		return "", fmt.Errorf("Failed to open downloaded mp3. Err: %v\n", err)
	}

	fileInfo, _ := os.Stat(fileLocation)

	if fileInfo.Size() == 0 {
		return "", fmt.Errorf("File was 0 bytes, didn didn't cache.\n")
	}

	ipfsPath, err = ipfs.Add(mp3File)
	if err != nil {
		return "", fmt.Errorf("Failed to add to IPFS. Err: %v\n", err)
	}

	// Formatting as proper ipfs path, not just hash
	ipfsPath = "/ipfs/" + ipfsPath

	return ipfsPath, err
}
