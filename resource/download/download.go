package download

import (
	"fmt"
	"net/url"
	"os"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/rylio/ytdl"
)

var knownProviders = [...]string{"youtube.com"}
var tempDLFolder = "TEMP-DL"

// Master download router. Looks at the url and determins which service needs
// to hand the url
func Download(rawurl string, ipfs *shell.Shell) (ipfsPath string, err error) {
	// Ensure the temporary directory for storing downloads exists
	if _, err := os.Stat(tempDLFolder); os.IsNotExist(err) {
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
		return downloadYoutube(urlToDL, ipfs)
	default:
		// TODO: Eventually do a text-search of youtube and just DL top result
		return "", fmt.Errorf("URL hostname (%v) doesn't match a known provider.\n"+
			"Should be one of: %v\n", urlToDL.Hostname(), knownProviders)
	}
}

func downloadYoutube(urlToDL url.URL, ipfs *shell.Shell) (ipfsPath string, err error) {
	vidInfo, err := ytdl.GetVideoInfo(urlToDL.EscapedPath())

	file, _ = os.Create(vid.Title + ".mp4")
	defer file.Close()
	vid.Download(file)

	return "", nil
}
