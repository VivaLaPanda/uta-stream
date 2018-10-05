package download

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/rylio/ytdl"
)

var knownProviders = [...]string{"youtube.com"}
var tempDLFolder = "TEMP-DL"

// Master download router. Looks at the url and determins which service needs
// to hand the url
func Download(rawurl string, ipfs *shell.Shell) (ipfsPath string, err error) {
	// Ensure the temporary directory for storing downloads exists
	if _, err = os.Stat(tempDLFolder); os.IsNotExist(err) {
		os.Mkdir(tempDLFolder, os.ModePerm)
	}

	// Parse the URL
	urlToDL, err := url.Parse(rawurl)
	if err != nil {
		// TODO: Eventually do a text-search of youtube and just DL top result
		return "", err
	}

	// Route to different handlers based on hostname
	switch urlToDL.Hostname() {
	case "youtube.com":
		return downloadYoutube(*urlToDL, ipfs)
	default:
		// TODO: Eventually do a text-search of youtube and just DL top result
		return "", fmt.Errorf("URL hostname (%v) doesn't match a known provider.\n"+
			"Should be one of: %v\n", urlToDL.Hostname(), knownProviders)
	}
}

func downloadYoutube(urlToDL url.URL, ipfs *shell.Shell) (ipfsPath string, err error) {
	// Get the info for the video
	vidInfo, err := ytdl.GetVideoInfo(urlToDL.EscapedPath())
	if err != nil {
		return "", fmt.Errorf("failed to fetch provided Youtube url. Err: %v", err)
	}

	// Figure out the highest bitrate format
	formats := vidInfo.Formats
	bestFormat := formats.Best(ytdl.FormatAudioBitrateKey)[0] // Format with highest bitrate

	// Download the mp4
	log.Printf("Downloading mp4 from %v\n", urlToDL.EscapedPath())
	fileLocation := filepath.Join(tempDLFolder, vidInfo.Title)
	file, err := os.Create(fileLocation + ".mp4")
	if err != nil {
		return "", fmt.Errorf("failed to create mp4 file. Err: %v", err)
	}
	vidInfo.Download(bestFormat, file)
	if err = file.Close(); err != nil {
		return "", fmt.Errorf("failed to write mp4. Err: %v", err)
	}
	log.Printf("Downloading of %v complete\n", urlToDL.EscapedPath())

	// Split out the audio part
	ffmpeg, err := exec.LookPath("ffmpeg")
	var mp3 string
	if err != nil {
		return "", fmt.Errorf("ffmpeg was not found in PATH. Please install ffmpeg")
	} else {
		log.Printf("Attempting to isolate audio as mp3 from %v\n", fileLocation+".mp4")
		fname := fileLocation
		mp3 = strings.TrimRight(fname, filepath.Ext(fname)) + ".mp3"
		cmd := exec.Command(ffmpeg, "-y", "-loglevel", "quiet", "-i", fname, "-vn", mp3)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			fmt.Println("Failed to extract audio:", err)
		} else {
			fmt.Println()
			fmt.Println("Extracted audio:", mp3)
			err = os.Remove(fileLocation + ".mp4")
			log.Printf("Failed to remove mp4. Err: %v\n", err)
		}
	}

	// Add to IPFS
	mp3File, err := os.Open(mp3)
	defer mp3File.Close()
	if err != nil {
		return "", fmt.Errorf("Failed to open downloaded mp3. Err: %v\n", err)
	}
	return ipfs.Add(mp3File)
}
