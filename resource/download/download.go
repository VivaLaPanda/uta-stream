// Package download provides functions that given a url to certain sites
// can fetch mp3 from them.
package download

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/VivaLaPanda/uta-stream/resource"
	shell "github.com/ipfs/go-ipfs-api"
)

var knownProviders = [...]string{"youtube.com", "youtu.be"}
var youtubeHosts = map[string]bool{
	"youtu.be":          true,
	"youtube.com":       true,
	"www.youtube.com":   true,
	"m.youtube.com":     true,
	"music.youtube.com": true,
}
var tempDLFolder = "TEMP-DL"
var maxYTDownloaders = make(chan int, 3)

// cookiesFile is resolved relative to the process working directory
// (the systemd unit sets WorkingDirectory to the uta-stream dir).
var cookiesFile = "cookies.txt"

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
	if youtubeHosts[song.URL().Hostname()] {
		return downloadYoutube(song, ipfs)
	}

	// Get the ext
	ext := path.Ext(song.URL().Path)
	if ext == ".mp3" || ext == ".flac" {
		return downloadMp3(song, ipfs)
	}

	return song, fmt.Errorf("URL hostname (%v) doesn't match a known provider. "+
		"Should be one of: %v", song.URL().Hostname(), knownProviders)
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
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
	fileLocation := filepath.Join(tempDLFolder, filename, randSeq(4))
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
			dlError <- fmt.Errorf("MP3 DL encountered an error: %v", err)
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
		io.Copy(mp3File, output)
		output.Close()
		if song.Writer != nil {
			song.Writer.Close()
		}
		mp3File.Close()
	}()

	// Place into IPFS and resolve the placeholder
	go func() {
		// BLock until DL finishes, nil for success, else will be an error
		err := <-dlError
		if err != nil {
			song.DLFailure <- fmt.Errorf("failed to download %s. Err: %v", song.URL().String(), err)
			return
		}

		// Add to ipfs
		ipfsPath, err := addToIpfs(fileLocation, ipfs)
		if err != nil {
			song.DLFailure <- fmt.Errorf("failed to add %s to IPFS. Err: %v", song.URL().String(), err)
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

// downloadYoutube fetches audio from YouTube using yt-dlp. yt-dlp handles the
// bot-check (via the cookies file), the n-challenge (via a JS runtime), and the
// PoToken (via the bgutil provider), then extracts the audio to mp3. We add the
// resulting file to IPFS and resolve the song via its DLResult channel.
//
// Requires yt-dlp (and ffmpeg for the audio extraction) to be in PATH.
func downloadYoutube(song *resource.Song, ipfs *shell.Shell) (*resource.Song, error) {
	ytDlp, err := exec.LookPath("yt-dlp")
	if err != nil {
		return song, fmt.Errorf("yt-dlp was not found in PATH. Please install yt-dlp")
	}

	rawURL := song.URL().String()

	// Fetch metadata up front so the queue has a title/duration immediately, and
	// so auth/bot-check failures surface synchronously to the caller (the enqueue
	// request) rather than disappearing into a background goroutine.
	metaOut, err := exec.Command(ytDlp,
		"--no-playlist", "--cookies", cookiesFile, "--skip-download",
		"--print", "%(title)s", "--print", "%(duration)s", rawURL).Output()
	if err != nil {
		return song, fmt.Errorf("failed to fetch provided Youtube url. Err: %v", err)
	}
	metaLines := strings.Split(strings.TrimSpace(string(metaOut)), "\n")
	if len(metaLines) >= 1 {
		song.Title = strings.TrimSpace(metaLines[0])
	}
	if len(metaLines) >= 2 {
		if secs, perr := strconv.ParseFloat(strings.TrimSpace(metaLines[1]), 64); perr == nil {
			song.Duration = time.Duration(secs) * time.Second
		}
	}

	// yt-dlp extracts to <fileBase>.mp3 (it downloads bestaudio then converts).
	fileBase := filepath.Join(tempDLFolder, randSeq(12))
	fileLocation := fileBase + ".mp3"

	go func() {
		// Bound concurrent YouTube downloads
		maxYTDownloaders <- 0
		defer func() { <-maxYTDownloaders }()

		log.Printf("Starting yt-dlp download of %v\n", rawURL)
		out, err := exec.Command(ytDlp,
			"--no-playlist", "--cookies", cookiesFile,
			"-f", "bestaudio", "-x", "--audio-format", "mp3",
			"-o", fileBase+".%(ext)s", rawURL).CombinedOutput()
		if err != nil {
			song.DLFailure <- fmt.Errorf("yt-dlp failed to download %s. Err: %v. Output: %s",
				rawURL, err, string(out))
			return
		}
		log.Printf("Downloading of %v complete\n", rawURL)

		// Add to ipfs
		ipfsPath, err := addToIpfs(fileLocation, ipfs)
		if err != nil {
			song.DLFailure <- fmt.Errorf("failed to add %s to IPFS. Err: %v", rawURL, err)
			return
		}
		song.DLResult <- ipfsPath

		// Remove the mp3 now that we've added
		if err = os.Remove(fileLocation); err != nil {
			log.Printf("Failed to remove mp3 for %s. Err: %v\n", rawURL, err)
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
		return "", fmt.Errorf("failed to open downloaded mp3. Err: %v", err)
	}

	fileInfo, _ := os.Stat(fileLocation)

	if fileInfo.Size() == 0 {
		return "", fmt.Errorf("file was 0 bytes, didn't cache")
	}

	ipfsPath, err = ipfs.Add(mp3File)
	if err != nil {
		return "", fmt.Errorf("failed to add to IPFS. Err: %v", err)
	}

	// Formatting as proper ipfs path, not just hash
	ipfsPath = "/ipfs/" + ipfsPath

	return ipfsPath, err
}
